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

// maxMarkReadApplications caps the number of applications a single mark-read
// request may clear, bounding the UPDATE. Clients send exactly one today; the
// array is forward-compat for a future "mark several read" without a contract
// change. Over the cap is a 400.
const maxMarkReadApplications = 500

// stateStore is the consumer-side slice of the store the handlers use.
// *PostgresStore satisfies it; tests substitute a hand-written fake.
type stateStore interface {
	Get(ctx context.Context, userID string) (*State, error)
	UnreadCount(ctx context.Context, userID string) (int, error)
	MarkAllRead(ctx context.Context, userID string, now time.Time) (int64, error)
	MarkApplicationsRead(ctx context.Context, userID string, uids []string, authorityIDs []int, now time.Time) (int64, error)
}

// stateResult is the GET /v1/me/notification-state response body:
// camelCase keys, lastReadAt in DateTimeOffset wire format. The shape is frozen
// for client compatibility (ADR 0035): lastReadAt is vestigial, version is the
// change token, totalUnreadCount is the read_at IS NULL tally.
type stateResult struct {
	LastReadAt       platform.DotNetTime `json:"lastReadAt"`
	Version          int                 `json:"version"`
	TotalUnreadCount int                 `json:"totalUnreadCount"`
}

// applicationRef identifies one application to mark read: the bare per-council
// PlanIt ref plus its authority id. Both are needed because applicationUid is
// not unique across authorities (the client exposes uid + areaId on every row).
type applicationRef struct {
	ApplicationUID string `json:"applicationUid"`
	AuthorityID    int    `json:"authorityId"`
}

// markReadRequest is the POST /v1/me/applications/mark-read body: an array of
// (applicationUid, authorityId) pairs. Empty marks nothing (a 204 no-op, never
// "all" — mark-all-read is a separate endpoint).
type markReadRequest struct {
	Applications []applicationRef `json:"applications"`
}

// Routes registers the notification read-state endpoints on mux. All are
// authenticated: the auth middleware guarantees a subject in context.
func Routes(mux *http.ServeMux, store stateStore, now func() time.Time, logger *slog.Logger) {
	h := handler{store: store, now: now, logger: logger}
	mux.HandleFunc("GET /v1/me/notification-state", h.get)
	mux.HandleFunc("POST /v1/me/notification-state/mark-all-read", h.markAllRead)
	mux.HandleFunc("POST /v1/me/applications/mark-read", h.markRead)
}

type handler struct {
	store  stateStore
	now    func() time.Time
	logger *slog.Logger
}

// get returns the user's read state. It never writes: for a user with a state
// row it returns that row's last_read_at and version; for a user with none it
// returns version 0 and lastReadAt computed at now (not persisted — this avoids
// an epoch-zero surprise in the payload while keeping GET side-effect-free, ADR
// 0035). totalUnreadCount is always the live read_at IS NULL tally.
func (h handler) get(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())

	st, err := h.store.Get(r.Context(), userID)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "read notification state", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	unread, err := h.store.UnreadCount(r.Context(), userID)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "count unread notifications", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	lastReadAt := h.now()
	version := 0
	if st != nil {
		lastReadAt = st.LastReadAt
		version = st.Version
	}

	h.writeJSON(w, r, stateResult{
		LastReadAt:       platform.DotNetTime(lastReadAt),
		Version:          version,
		TotalUnreadCount: unread,
	})
}

// markAllRead clears every unread notification for the user and bumps the
// version change token (the store upserts the state row when absent). 204 on
// success.
func (h handler) markAllRead(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())

	if _, err := h.store.MarkAllRead(r.Context(), userID, h.now()); err != nil {
		h.logger.ErrorContext(r.Context(), "mark all notifications read", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// markRead clears the caller's unread notifications for the requested
// applications (scoped by the composite application_uid + authorityId). It is
// idempotent — 204 even when zero rows changed — and an empty array marks
// nothing. A malformed body or an over-cap array returns a bodyless 400.
func (h handler) markRead(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req markReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if len(req.Applications) > maxMarkReadApplications {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	uids := make([]string, len(req.Applications))
	authorityIDs := make([]int, len(req.Applications))
	for i, a := range req.Applications {
		uids[i] = a.ApplicationUID
		authorityIDs[i] = a.AuthorityID
	}

	if _, err := h.store.MarkApplicationsRead(r.Context(), userID, uids, authorityIDs, h.now()); err != nil {
		h.logger.ErrorContext(r.Context(), "mark applications read", "error", err)
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
