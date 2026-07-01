package watchzones

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/httputil"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// maxBodyBytes caps the request body the PATCH handler reads.
const maxBodyBytes = 1 << 20

// invalidPayloadMessage is the error text for a watch-zone validation failure
// (Create/Update endpoints).
const invalidPayloadMessage = "Invalid watch zone payload."

// zoneStore is the consumer-side store the handlers use. *CosmosStore satisfies
// it; tests substitute a hand-written fake.
type zoneStore interface {
	GetByUserID(ctx context.Context, userID string) ([]WatchZone, error)
	Get(ctx context.Context, userID, zoneID string) (WatchZone, error)
	Save(ctx context.Context, z WatchZone) error
	Delete(ctx context.Context, userID, zoneID string) error
}

// profileCAS is the consumer-side interface for atomically updating the
// watch-zone quota counter on the user's profile document. It is satisfied by
// *profiles.CosmosStore when wired with a CAS-capable container. A nil value
// disables atomic quota enforcement (development / legacy path).
type profileCAS interface {
	// GetWithETag reads the profile and its current etag for a CAS operation.
	// Returns (nil, "", nil) when the profile is absent.
	GetWithETag(ctx context.Context, userID string) (*profiles.UserProfile, string, error)
	// UpdateZoneCountWithCAS replaces the profile document only if the stored
	// etag still matches. Returns platform.ErrCASPreconditionFailed on a 412
	// (concurrent writer won — the caller should re-read and retry).
	UpdateZoneCountWithCAS(ctx context.Context, userID string, p *profiles.UserProfile, etag string) error
}

// MetricsRecorder is the consumer-side slice of the metrics registry the
// watch-zone handlers record the lifecycle counters on. *metrics.Registry
// satisfies it. It is exported because Routes / NearbyRoutes accept it as an
// Option from cmd/api's wiring. A nil recorder no-ops every counter.
type MetricsRecorder interface {
	WatchZoneCreated(ctx context.Context)
	WatchZoneUpdated(ctx context.Context)
	WatchZoneDeleted(ctx context.Context)
}

// Option configures the watch-zone routes. WithMetricsRecorder and
// WithProfileCAS are the options today; variadic so existing call sites and
// tests compile unchanged.
type Option func(*handler)

// WithMetricsRecorder wires the metrics recorder the handlers record the
// watch-zone lifecycle counters on.
func WithMetricsRecorder(rec MetricsRecorder) Option {
	return func(h *handler) { h.metrics = rec }
}

// WithProfileCAS wires the CAS-capable profile store used by the create path
// for atomic quota enforcement and the delete path for quota decrement. When
// absent the create path falls back to the non-atomic count-then-save (legacy).
func WithProfileCAS(cas profileCAS) Option {
	return func(h *handler) { h.profileCAS = cas }
}

// handler serves the /v1/me/watch-zones surface. The auth middleware guarantees
// a subject in context before these handlers run. The list/update/delete methods
// use only store + logger; the create and applications methods (nearby.go) use
// the remaining dependencies, which Routes leaves nil and NearbyRoutes populates.
type handler struct {
	store      zoneStore
	profiles   profileReader
	profileCAS profileCAS
	resolver   authorityResolver
	apps       appFinder
	unread     unreadReader
	newID      func() string
	now        func() time.Time
	logger     *slog.Logger
	metrics    MetricsRecorder
}

// Routes registers the watch-zone list/update/delete endpoints on mux. POST
// create and GET /{zoneId}/applications are registered separately by NearbyRoutes
// (they need the profile, application, geocode, notification-state and
// notification stores).
func Routes(mux *http.ServeMux, store zoneStore, logger *slog.Logger, opts ...Option) {
	h := &handler{store: store, logger: logger}
	for _, opt := range opts {
		opt(h)
	}
	mux.HandleFunc("GET /v1/me/watch-zones", h.list)
	mux.HandleFunc("PATCH /v1/me/watch-zones/{zoneId}", h.patch)
	mux.HandleFunc("DELETE /v1/me/watch-zones/{zoneId}", h.delete)
}

// watchZoneSummary is the per-zone wire shape returned by list and update.
// createdAt is deliberately absent from the summary.
type watchZoneSummary struct {
	ID                  string  `json:"id"`
	Name                string  `json:"name"`
	Latitude            float64 `json:"latitude"`
	Longitude           float64 `json:"longitude"`
	RadiusMetres        float64 `json:"radiusMetres"`
	AuthorityID         int     `json:"authorityId"`
	PushEnabled         bool    `json:"pushEnabled"`
	EmailInstantEnabled bool    `json:"emailInstantEnabled"`
}

func summaryOf(z WatchZone) watchZoneSummary {
	return watchZoneSummary{
		ID:                  z.ID,
		Name:                z.Name,
		Latitude:            z.Latitude,
		Longitude:           z.Longitude,
		RadiusMetres:        z.RadiusMetres,
		AuthorityID:         z.AuthorityID,
		PushEnabled:         z.PushEnabled,
		EmailInstantEnabled: z.EmailInstantEnabled,
	}
}

// listResult is the GET /v1/me/watch-zones response: { zones: [...] }.
type listResult struct {
	Zones []watchZoneSummary `json:"zones"`
}

// updateResult is the PATCH response: { zone: {...} }.
type updateResult struct {
	Zone watchZoneSummary `json:"zone"`
}

// list implements GET /v1/me/watch-zones, returning the user's zones as a
// (possibly empty) array. Always a 200.
func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())
	zones, err := h.store.GetByUserID(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "list watch zones", err)
		return
	}
	summaries := make([]watchZoneSummary, 0, len(zones))
	for _, z := range zones {
		summaries = append(summaries, summaryOf(z))
	}
	h.writeJSON(w, r, http.StatusOK, listResult{Zones: summaries})
}

// patchRequest is the PATCH body: every field optional (nil = unchanged).
type patchRequest struct {
	Name                *string  `json:"name"`
	Latitude            *float64 `json:"latitude"`
	Longitude           *float64 `json:"longitude"`
	RadiusMetres        *float64 `json:"radiusMetres"`
	AuthorityID         *int     `json:"authorityId"`
	PushEnabled         *bool    `json:"pushEnabled"`
	EmailInstantEnabled *bool    `json:"emailInstantEnabled"`
}

// rangeValid reports whether the present coordinate/radius/authority fields are
// in range. A nil field is not checked. Name is deliberately not checked here —
// that is enforced by the domain merge (and a blank name there is a 500, not a 400).
func (req patchRequest) rangeValid() bool {
	if req.Latitude != nil && (math.IsNaN(*req.Latitude) || math.IsInf(*req.Latitude, 0) ||
		*req.Latitude < -90 || *req.Latitude > 90) {
		return false
	}
	if req.Longitude != nil && (math.IsNaN(*req.Longitude) || math.IsInf(*req.Longitude, 0) ||
		*req.Longitude < -180 || *req.Longitude > 180) {
		return false
	}
	if req.RadiusMetres != nil && (math.IsNaN(*req.RadiusMetres) || math.IsInf(*req.RadiusMetres, 0) ||
		*req.RadiusMetres <= 0 || *req.RadiusMetres > maxRadiusMetres) {
		return false
	}
	if req.AuthorityID != nil && *req.AuthorityID <= 0 {
		return false
	}
	return true
}

func (req patchRequest) toUpdate() ZoneUpdate {
	// patchRequest and ZoneUpdate are field-identical; the request type exists
	// only to carry the JSON tags, so a direct conversion is exact.
	return ZoneUpdate(req)
}

// patch implements PATCH /v1/me/watch-zones/{zoneId}: range-validate the body
// (400), load the zone (404 if absent), apply the merge, persist, and return the
// updated summary. A merge that violates a domain invariant (e.g. a blank name)
// is a 500.
func (h *handler) patch(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())
	zoneID := r.PathValue("zoneId")

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req patchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !req.rangeValid() {
		h.writeError(w, r, http.StatusBadRequest, invalidPayloadMessage)
		return
	}

	zone, err := h.store.Get(r.Context(), userID, zoneID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h.serverError(w, r, "load watch zone", err)
		return
	}

	updated, err := zone.WithUpdates(req.toUpdate())
	if err != nil {
		h.serverError(w, r, "apply watch-zone update", err)
		return
	}
	if err := h.store.Save(r.Context(), updated); err != nil {
		h.serverError(w, r, "save watch zone", err)
		return
	}
	if h.metrics != nil {
		h.metrics.WatchZoneUpdated(r.Context())
	}
	h.writeJSON(w, r, http.StatusOK, updateResult{Zone: summaryOf(updated)})
}

// maxCASRetries is the maximum number of etag-conditional replace attempts for
// quota CAS loops. After this many ErrCASPreconditionFailed responses the
// create path returns 403 (quota-exceeded is the safe failure mode) and the
// delete path gives up silently (the counter will self-correct on next create).
const maxCASRetries = 3

// delete implements DELETE /v1/me/watch-zones/{zoneId}: 204 on success, 404 when
// the zone does not exist. When the CAS profile store is wired, it also
// decrements the quota counter atomically so a slot is freed immediately.
func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())
	zoneID := r.PathValue("zoneId")

	if err := h.store.Delete(r.Context(), userID, zoneID); err != nil {
		if errors.Is(err, ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h.serverError(w, r, "delete watch zone", err)
		return
	}

	if h.profileCAS != nil {
		h.decrementZoneCount(r.Context(), userID)
	}

	if h.metrics != nil {
		h.metrics.WatchZoneDeleted(r.Context())
	}
	w.WriteHeader(http.StatusNoContent)
}

// decrementZoneCount reduces the CAS quota counter by 1 (floor 0) after a
// successful zone delete. It uses a bounded CAS retry loop so concurrent
// deletes do not race. On persistent conflict it logs and continues — the
// counter will converge on the next create's lazy-init read.
func (h *handler) decrementZoneCount(ctx context.Context, userID string) {
	for range maxCASRetries {
		profile, etag, err := h.profileCAS.GetWithETag(ctx, userID)
		if err != nil || profile == nil {
			h.logger.WarnContext(ctx, "decrement zone count: could not load profile", "user", userID, "error", err)
			return
		}
		updated := *profile
		newCount := 0
		if updated.WatchZoneCount != nil && *updated.WatchZoneCount > 0 {
			newCount = *updated.WatchZoneCount - 1
		}
		updated.WatchZoneCount = &newCount

		err = h.profileCAS.UpdateZoneCountWithCAS(ctx, userID, &updated, etag)
		if err == nil {
			return // success
		}
		if errors.Is(err, platform.ErrCASPreconditionFailed) {
			// Concurrent writer — retry
			continue
		}
		h.logger.WarnContext(ctx, "decrement zone count: CAS replace failed", "user", userID, "error", err)
		return
	}
	h.logger.WarnContext(ctx, "decrement zone count: exhausted retries", "user", userID)
}

// apiErrorResponse is the error envelope: { error, message } with message
// serialised as an explicit null when unset.
type apiErrorResponse struct {
	Error   string  `json:"error"`
	Message *string `json:"message"`
}

// writeJSON encodes v as an application/json; charset=utf-8 response with HTML
// escaping off and no trailing newline (the same approach the profiles handler
// uses). status is kept explicit at every call site (mirroring writeCreated's 201
// and writeError) so the success code is visible where the response is written,
// even though every JSON success body is a 200 today.
//
//nolint:unparam // status is intentionally explicit at call sites; 200-only today.
func (h *handler) writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	body, err := httputil.EncodeJSON(v)
	if err != nil {
		h.serverError(w, r, "encode response", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// G705 (gosec taint analysis) flags this since the nearby browse handler reads
	// ?limit=/?cursor= query params, but reflected XSS is not applicable: writeJSON
	// always emits application/json and body is server-encoded JSON of structured
	// data — the query params never reach the body (the cursor returns via a
	// base64url-encoded response header, not here).
	if _, err := w.Write(body); err != nil { //nolint:gosec // G705 false positive: JSON-only response, query params never reach the body
		h.logger.ErrorContext(r.Context(), "write response", "error", err)
	}
}

// writeError emits the error envelope at the given status with
// application/json; charset=utf-8 content type.
func (h *handler) writeError(w http.ResponseWriter, r *http.Request, status int, message string) {
	body, err := httputil.EncodeJSON(apiErrorResponse{Error: message})
	if err != nil {
		h.serverError(w, r, "encode error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write error body", "error", err)
	}
}

// serverError logs and emits a bodyless 500; the error envelope (with Detail) is
// backfilled by middleware.ErrorBody, mirroring the rest of the API.
func (h *handler) serverError(w http.ResponseWriter, r *http.Request, op string, err error) {
	h.logger.ErrorContext(r.Context(), "watch-zone request failed", "op", op, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
}
