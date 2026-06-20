package notificationstate

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// stateStore is the consumer-side slice of the store the handlers use.
// *CosmosStore satisfies it; tests substitute a hand-written fake.
type stateStore interface {
	Get(ctx context.Context, userID string) (*State, error)
	Save(ctx context.Context, st State) error
	UnreadCount(ctx context.Context, userID string, lastReadAt time.Time) (int, error)
}

// stateResult is the GET /v1/me/notification-state response body:
// camelCase keys, lastReadAt in DateTimeOffset wire format.
type stateResult struct {
	LastReadAt       platform.DotNetTime `json:"lastReadAt"`
	Version          int                 `json:"version"`
	TotalUnreadCount int                 `json:"totalUnreadCount"`
}

// advanceRequest is the POST /v1/me/notification-state/advance request body.
type advanceRequest struct {
	AsOf platform.DotNetTime `json:"asOf"`
}

// Routes registers the notification-state endpoints on mux. All are
// authenticated: the auth middleware guarantees a subject in context.
func Routes(mux *http.ServeMux, store stateStore, now func() time.Time, logger *slog.Logger) {
	h := handler{store: store, now: now, logger: logger}
	mux.HandleFunc("GET /v1/me/notification-state", h.get)
	mux.HandleFunc("POST /v1/me/notification-state/mark-all-read", h.markAllRead)
	mux.HandleFunc("POST /v1/me/notification-state/advance", h.advance)
}

type handler struct {
	store  stateStore
	now    func() time.Time
	logger *slog.Logger
}

// get loads the watermark — seeding and persisting a first-touch one at now,
// so the unread count is zero by definition — then returns it with the unread
// tally, mirroring GetNotificationStateQueryHandler.
func (h handler) get(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())

	st, err := h.store.Get(r.Context(), userID)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "read notification state", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if st == nil {
		seeded, err := NewState(userID, h.now())
		if err != nil {
			h.logger.ErrorContext(r.Context(), "seed notification state", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := h.store.Save(r.Context(), seeded); err != nil {
			h.logger.ErrorContext(r.Context(), "persist seeded notification state", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		st = &seeded
	}

	unread, err := h.store.UnreadCount(r.Context(), userID, st.LastReadAt)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "count unread notifications", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, r, stateResult{
		LastReadAt:       platform.DotNetTime(st.LastReadAt),
		Version:          st.Version,
		TotalUnreadCount: unread,
	})
}

// markAllRead moves the watermark to now. First-touch users get a fresh seed
// (same end-state as create-then-mark, without the redundant version bump),
// mirroring MarkAllNotificationsReadCommandHandler. 204 on success.
func (h handler) markAllRead(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())
	now := h.now()

	st, err := h.store.Get(r.Context(), userID)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "read notification state", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if st == nil {
		seeded, err := NewState(userID, now)
		if err != nil {
			h.logger.ErrorContext(r.Context(), "seed notification state", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		st = &seeded
	} else {
		st.MarkAllReadAt(now)
	}
	if err := h.store.Save(r.Context(), *st); err != nil {
		h.logger.ErrorContext(r.Context(), "save notification state", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// advance moves the watermark forward to the request's asOf. First-touch users
// are seeded at now and the advance applied on top; for existing state a stale
// asOf is a no-op without a write. 204 on success; a malformed body returns a
// bodyless 400.
func (h handler) advance(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req advanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	asOf := time.Time(req.AsOf)

	st, err := h.store.Get(r.Context(), userID)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "read notification state", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if st == nil {
		// First-touch: seed at now, attempt the advance, persist regardless so a
		// subsequent GET sees the seed even when asOf was stale against it.
		seeded, err := NewState(userID, h.now())
		if err != nil {
			h.logger.ErrorContext(r.Context(), "seed notification state", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		seeded.AdvanceTo(asOf)
		if err := h.store.Save(r.Context(), seeded); err != nil {
			h.logger.ErrorContext(r.Context(), "save notification state", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if !st.AdvanceTo(asOf) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := h.store.Save(r.Context(), *st); err != nil {
		h.logger.ErrorContext(r.Context(), "save notification state", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeJSON encodes compactly with HTML escaping off and the trailing newline
// trimmed (same idiom as the legal and profiles handlers).
func (h handler) writeJSON(w http.ResponseWriter, r *http.Request, v any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		h.logger.ErrorContext(r.Context(), "encode notification state", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(bytes.TrimRight(buf.Bytes(), "\n")); err != nil {
		h.logger.ErrorContext(r.Context(), "write notification state", "error", err)
	}
}
