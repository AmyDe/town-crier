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
	MarkApplicationsRead(ctx context.Context, userID string, refs []string, authorityIDs []int, now time.Time) (int64, error)
	// MarkReadUpTo backs the TEMPORARY advance compat shim (tc-ekii); see the
	// advance handler. REMOVE per tc-v5w8 once the new iOS build is live.
	MarkReadUpTo(ctx context.Context, userID string, asOf, now time.Time) (int64, error)
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

// applicationRef identifies one application to mark read: a PlanIt case reference
// plus its authority id. Both are needed because the case reference is unique only
// within a council; authorityId disambiguates across councils.
type applicationRef struct {
	// ApplicationUID keeps the JSON key `applicationUid` for wire compatibility
	// with the already-merged iOS client — but the value it carries is the PlanIt
	// case REFERENCE (= a.Name, e.g. "24/0001"), NOT the application_uid column
	// (= a.UID, e.g. "24/0001/FUL"). The store matches it against application_name,
	// never application_uid (#733). This is required because everything that would
	// call mark-read carries a.Name: the push payload sets applicationRef =
	// n.ApplicationName, iOS sends id.name, and web sends summary.name. a.Name is
	// unique within a council, so authorityId disambiguates cross-council. Do not
	// rename this JSON key without shipping a coordinated client change.
	ApplicationUID string `json:"applicationUid"`
	AuthorityID    int    `json:"authorityId"`
}

// markReadRequest is the POST /v1/me/applications/mark-read body: an array of
// (applicationUid, authorityId) pairs. Empty marks nothing (a 204 no-op, never
// "all" — mark-all-read is a separate endpoint).
type markReadRequest struct {
	Applications []applicationRef `json:"applications"`
}

// advanceRequest is the POST /v1/me/notification-state/advance request body.
//
// TEMPORARY BACKWARD-COMPAT SHIM (tc-ekii). This is the exact byte-identical shape
// the old (pre-ADR-0035) server parsed, so fire-and-forget requests from App Store
// iOS builds that predate the per-application read-state change still bind. New
// iOS/web clients do NOT send this — they call POST /v1/me/applications/mark-read.
// REMOVE per bead tc-v5w8 once the new iOS build is live.
type advanceRequest struct {
	AsOf platform.DotNetTime `json:"asOf"`
}

// Routes registers the notification read-state endpoints on mux. All are
// authenticated: the auth middleware guarantees a subject in context.
func Routes(mux *http.ServeMux, store stateStore, now func() time.Time, logger *slog.Logger) {
	h := handler{store: store, now: now, logger: logger}
	mux.HandleFunc("GET /v1/me/notification-state", h.get)
	mux.HandleFunc("POST /v1/me/notification-state/mark-all-read", h.markAllRead)
	mux.HandleFunc("POST /v1/me/applications/mark-read", h.markRead)
	// TEMPORARY BACKWARD-COMPAT SHIM (tc-ekii). ADR 0035 (#733) removed advance in
	// favour of per-application read_at, so new iOS/web clients do NOT call it (they
	// use POST /v1/me/applications/mark-read above). This route is re-added only so
	// the App Store iOS builds that predate the change (live + one in Apple review)
	// keep clearing their push badge on tap during the review window — against the new
	// server those old fire-and-forget calls would 404 and silently stop clearing the
	// badge. It translates the retired watermark advance to a read_at mark (see the
	// advance handler). REMOVE per bead tc-v5w8 once the new iOS build is live; advance
	// 404-ing again is the #733/ADR-0035 end-state.
	mux.HandleFunc("POST /v1/me/notification-state/advance", h.advance)
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
// applications (scoped by the composite case reference + authorityId; the store
// matches the reference against application_name, not application_uid — see
// applicationRef). It is idempotent — 204 even when zero rows changed — and an
// empty array marks nothing. A malformed body or an over-cap array returns a
// bodyless 400.
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

	// refs carry PlanIt case references (a.Name), matched against application_name
	// by the store — the applicationUid JSON key is a wire-compat misnomer (#733).
	refs := make([]string, len(req.Applications))
	authorityIDs := make([]int, len(req.Applications))
	for i, a := range req.Applications {
		refs[i] = a.ApplicationUID
		authorityIDs[i] = a.AuthorityID
	}

	if _, err := h.store.MarkApplicationsRead(r.Context(), userID, refs, authorityIDs, h.now()); err != nil {
		h.logger.ErrorContext(r.Context(), "mark applications read", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// advance is a TEMPORARY backward-compat shim (tc-ekii) for the retired
// scroll-to-clear watermark endpoint. ADR 0035 (#733) removed advance in favour of
// per-application read_at, so new iOS/web clients do NOT call it (they use
// POST /v1/me/applications/mark-read). This handler is re-added only so the App Store
// iOS builds that predate the change (live + one in Apple review) keep clearing their
// push badge on tap during the review window — those old builds fire advance on
// push-tap and a 404 would silently stop the badge clearing.
//
// It parses the old {asOf} body byte-identically (advanceRequest) and translates the
// watermark advance-to-asOf into a read_at mark: every unread notification created at
// or before asOf is marked read (MarkReadUpTo). It is idempotent (a repeat is a 204
// that clears nothing) and returns a bodyless 400 on a malformed body. The retired
// watermark / State.AdvanceTo / first-touch seeding logic is deliberately NOT restored.
//
// REMOVE per bead tc-v5w8 once the new iOS build is live; advance 404-ing again is the
// #733/ADR-0035 end-state.
func (h handler) advance(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req advanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	asOf := time.Time(req.AsOf)
	if _, err := h.store.MarkReadUpTo(r.Context(), userID, asOf, h.now()); err != nil {
		h.logger.ErrorContext(r.Context(), "advance (mark read up to)", "error", err)
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
