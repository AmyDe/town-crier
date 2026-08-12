package watchzones

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"sort"
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

// Custom-shape boundary error codes and messages (tc-6he3x.4). Unlike
// invalidPayloadMessage above (whose text IS the apiErrorResponse "error"
// field, an established watchzones convention this bead does not touch),
// these carry a stable machine-readable code separately from the human
// message -- the same (code, message) shape internal/offercodes and
// internal/subscriptions already use -- because a client (iOS/web) needs to
// tell "wrong tier" apart from "bad shape" apart from "too big" to route to
// the right UX (paywall vs. a validation error vs. a resize prompt).
const (
	// boundaryRequiresPaidTierCode/-Message: 403, a Free-tier caller supplied a
	// non-null boundary on create, or on a PATCH that sets one.
	boundaryRequiresPaidTierCode    = "boundary_requires_paid_tier"
	boundaryRequiresPaidTierMessage = "Custom-shape watch zones require a paid subscription."
	// boundaryInvalidCode/-Message: 400, the supplied boundary failed the
	// domain's own shape validation (NewBoundary's sentinel errors in zone.go
	// -- vertex count, duplicate vertex, self-intersection, out of UK bounds)
	// or was not well-formed GeoJSON.
	boundaryInvalidCode    = "boundary_invalid"
	boundaryInvalidMessage = "The supplied boundary is not a valid shape."
	// boundaryTooLargeCode/-Message: 400, the boundary's enclosing radius
	// exceeds either the hard server ceiling (maxRadiusMetres, nearby.go) or
	// the caller's tier cap (profiles.SubscriptionTier.MaxZoneRadiusMetres).
	boundaryTooLargeCode    = "boundary_too_large"
	boundaryTooLargeMessage = "The boundary is too large for your subscription tier."
)

// errProfileReaderNotWired signals a wiring bug: setting a watch-zone boundary
// requires the profile reader to check the caller's tier entitlement, and the
// handler refuses to run an unverified tier check without it. Unreachable in
// production, where Routes/NearbyRoutes are always wired WithProfileReader
// whenever a profile store is configured (cmd/api/wiring.go).
var errProfileReaderNotWired = errors.New("profile reader not wired for boundary tier gate")

// isBoundaryValidationError reports whether err is one of the shape-validation
// sentinels NewBoundary returns (see zone.go), which WithUpdates propagates
// unwrapped from a PATCH that sets an invalid boundary. The handler maps these
// to a 400 boundary_invalid rather than the generic 500 a merge failure (e.g.
// a blank name) gets -- reusing the domain's own validation rather than
// re-implementing geometry checks at the HTTP layer.
func isBoundaryValidationError(err error) bool {
	return errors.Is(err, ErrBoundaryVertexCount) ||
		errors.Is(err, ErrBoundaryDuplicateVertex) ||
		errors.Is(err, ErrBoundarySelfIntersecting) ||
		errors.Is(err, ErrBoundaryOutOfBounds)
}

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

// WithProfileReader wires the profile reader the list/patch handlers use to
// compute each zone's derived "paused" field (GH#889): a zone whose
// oldest-first rank among the caller's own zones — by (CreatedAt, ID) —
// exceeds their current effective-tier watch-zone limit. Mirrors nearby.go's
// create-path profile read. Not wiring this option leaves h.profiles nil, so
// every zone reports unpaused (fail open, never block a read on a missing
// dependency).
func WithProfileReader(pr profileReader) Option {
	return func(h *handler) { h.profiles = pr }
}

// handler serves the /v1/me/watch-zones surface. The auth middleware guarantees
// a subject in context before these handlers run. The list/update/delete methods
// use only store + logger; the create and applications methods (nearby.go) use
// the remaining dependencies, which Routes leaves nil and NearbyRoutes populates.
type handler struct {
	store      zoneStore
	profiles   profileReader
	profileCAS profileCAS
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
	h := &handler{store: store, logger: logger, now: time.Now}
	for _, opt := range opts {
		opt(h)
	}
	mux.HandleFunc("GET /v1/me/watch-zones", h.list)
	mux.HandleFunc("PATCH /v1/me/watch-zones/{zoneId}", h.patch)
	mux.HandleFunc("DELETE /v1/me/watch-zones/{zoneId}", h.delete)
}

// watchZoneSummary is the per-zone wire shape returned by list and update.
// createdAt is deliberately absent from the summary. Paused is additive
// (GH#889): a derived, never-stored flag — see pausedIDs.
type watchZoneSummary struct {
	ID                  string  `json:"id"`
	Name                string  `json:"name"`
	Latitude            float64 `json:"latitude"`
	Longitude           float64 `json:"longitude"`
	RadiusMetres        float64 `json:"radiusMetres"`
	PushEnabled         bool    `json:"pushEnabled"`
	EmailInstantEnabled bool    `json:"emailInstantEnabled"`
	Paused              bool    `json:"paused"`
	// Boundary is null for a circle zone and a GeoJSON Polygon for a
	// custom-shape one (tc-6he3x.4); Latitude/Longitude/RadiusMetres remain
	// the polygon's derived centroid/enclosing radius either way, so every
	// pre-existing circle-shaped consumer keeps working unchanged.
	Boundary *boundaryGeoJSON `json:"boundary"`
}

func summaryOf(z WatchZone, paused bool) watchZoneSummary {
	return watchZoneSummary{
		ID:                  z.ID,
		Name:                z.Name,
		Latitude:            z.Latitude,
		Longitude:           z.Longitude,
		RadiusMetres:        z.RadiusMetres,
		PushEnabled:         z.PushEnabled,
		EmailInstantEnabled: z.EmailInstantEnabled,
		Paused:              paused,
		Boundary:            boundaryToGeoJSON(z.Boundary),
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
	paused, err := h.pausedZoneIDs(r.Context(), userID, zones)
	if err != nil {
		h.serverError(w, r, "compute paused zones", err)
		return
	}
	summaries := make([]watchZoneSummary, 0, len(zones))
	for _, z := range zones {
		summaries = append(summaries, summaryOf(z, paused[z.ID]))
	}
	h.writeJSON(w, r, http.StatusOK, listResult{Zones: summaries})
}

// pausedZoneIDs resolves which of zones (the caller's full zone set) are
// paused (GH#889): ranked oldest-first by (CreatedAt, ID), a zone is paused
// once its rank exceeds the caller's current effective-tier watch-zone limit.
// Returns a nil map — every zone unpaused — when no profile reader is wired,
// the profile is missing, or the tier is unlimited (Pro): the same "fail
// open" shape nearby.go's create-path quota check uses for a missing profile.
func (h *handler) pausedZoneIDs(ctx context.Context, userID string, zones []WatchZone) (map[string]bool, error) {
	if h.profiles == nil {
		return nil, nil
	}
	profile, err := h.profiles.Get(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load profile for pause check: %w", err)
	}
	if profile == nil {
		return nil, nil
	}
	limit := profile.EffectiveTier(h.now()).WatchZoneLimit()
	if limit >= math.MaxInt32 {
		return nil, nil
	}
	return pausedIDs(zones, limit), nil
}

// pausedIDs partitions zones into the paused set (GH#889): ranked
// oldest-first by (CreatedAt, ID), a zone whose rank exceeds limit is paused.
// Purely derived — no schema change, no stored flag — so a re-upgrade or a
// delete of an older zone revives the next zone in rank with zero extra work.
// Never mutates zones.
func pausedIDs(zones []WatchZone, limit int) map[string]bool {
	if len(zones) <= limit {
		return nil
	}
	sorted := make([]WatchZone, len(zones))
	copy(sorted, zones)
	sort.Slice(sorted, func(i, j int) bool {
		if !sorted[i].CreatedAt.Equal(sorted[j].CreatedAt) {
			return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
		}
		return sorted[i].ID < sorted[j].ID
	})
	paused := make(map[string]bool, len(sorted)-limit)
	for i, z := range sorted {
		if i >= limit {
			paused[z.ID] = true
		}
	}
	return paused
}

// patchRequest is the PATCH body: every field optional (nil = unchanged).
// Boundary is tri-state (absent/null/value, see decodeBoundaryUpdate and
// ZoneUpdate.Boundary) so it cannot be a plain pointer-to-value like the rest
// of the fields; json.RawMessage captures the field's exact presence and raw
// bytes, which a *boundaryGeoJSON cannot: that would collapse absent and
// explicit null to the same nil value.
type patchRequest struct {
	Name                *string         `json:"name"`
	Latitude            *float64        `json:"latitude"`
	Longitude           *float64        `json:"longitude"`
	RadiusMetres        *float64        `json:"radiusMetres"`
	PushEnabled         *bool           `json:"pushEnabled"`
	EmailInstantEnabled *bool           `json:"emailInstantEnabled"`
	Boundary            json.RawMessage `json:"boundary"`
}

// rangeValid reports whether the present coordinate/radius fields are in
// range. A nil field is not checked. Name is deliberately not checked here —
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
	return true
}

// toUpdate builds the domain ZoneUpdate, threading the already-decoded
// tri-state boundary (see decodeBoundaryUpdate) into ZoneUpdate.Boundary.
func (req patchRequest) toUpdate(boundary *Boundary) ZoneUpdate {
	return ZoneUpdate{
		Name:                req.Name,
		Latitude:            req.Latitude,
		Longitude:           req.Longitude,
		RadiusMetres:        req.RadiusMetres,
		PushEnabled:         req.PushEnabled,
		EmailInstantEnabled: req.EmailInstantEnabled,
		Boundary:            boundary,
	}
}

// decodeBoundaryUpdate resolves the PATCH body's tri-state "boundary" field
// (raw, exactly as captured by patchRequest.Boundary's json.RawMessage) into
// ZoneUpdate.Boundary's pointer-to-slice shape (see that field's doc comment):
//
//   - raw is nil (the JSON key was absent): (nil, nil) -- leave the zone's
//     shape alone.
//   - raw is the 4-byte JSON literal null (the key was present, explicitly
//     null): a pointer to an empty Boundary -- revert to a circle.
//   - raw is a GeoJSON Polygon value: a pointer to its raw, UNVALIDATED
//     vertices. WithUpdates validates them (via NewBoundary) when it applies
//     the merge; this function does not re-implement that check.
//
// A value that is valid JSON but not a GeoJSON-shaped object (e.g. a bare
// string or number) returns an error; the caller maps it to the
// boundary_invalid response, matching the malformed-shape case WithUpdates
// itself would otherwise report.
func decodeBoundaryUpdate(raw json.RawMessage) (*Boundary, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if bytes.Equal(raw, []byte("null")) {
		empty := Boundary{}
		return &empty, nil
	}
	var g boundaryGeoJSON
	if err := json.Unmarshal(raw, &g); err != nil {
		return nil, fmt.Errorf("decode boundary update: %w", err)
	}
	b := Boundary(g.vertices())
	return &b, nil
}

// patch implements PATCH /v1/me/watch-zones/{zoneId}: range-validate the body
// (400), decode the tri-state boundary field (400 boundary_invalid on
// malformed GeoJSON), gate a boundary-setting update on the caller's tier
// (403 boundary_requires_paid_tier), load the zone (404 if absent), apply the
// merge (400 boundary_invalid on an invalid shape, 500 on any other domain
// invariant violation e.g. a blank name), gate the resulting radius (400
// boundary_too_large), persist, and return the updated summary.
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

	boundaryUpdate, err := decodeBoundaryUpdate(req.Boundary)
	if err != nil {
		h.writeErrorCode(w, r, http.StatusBadRequest, boundaryInvalidCode, boundaryInvalidMessage)
		return
	}
	// "Setting" a boundary (as opposed to leaving it alone, or explicitly
	// nulling it back to a circle) is the only case the paid-tier gate and the
	// radius ceiling apply to (tc-6he3x.4 acceptance criteria 4 and 6).
	settingBoundary := boundaryUpdate != nil && len(*boundaryUpdate) > 0

	var tier profiles.SubscriptionTier
	if settingBoundary {
		if h.profiles == nil {
			h.serverError(w, r, "boundary tier gate", errProfileReaderNotWired)
			return
		}
		profile, perr := h.profiles.Get(r.Context(), userID)
		if perr != nil || profile == nil {
			// A missing profile for an authenticated caller is a server-side
			// inconsistency (the iOS app always registers on first launch) --
			// mirroring nearby.go's create-path quota check.
			h.serverError(w, r, "load profile for boundary tier gate", perr)
			return
		}
		tier = profile.EffectiveTier(h.now())
		if !tier.AllowsCustomBoundary() {
			h.writeErrorCode(w, r, http.StatusForbidden, boundaryRequiresPaidTierCode, boundaryRequiresPaidTierMessage)
			return
		}
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

	updated, err := zone.WithUpdates(req.toUpdate(boundaryUpdate))
	if err != nil {
		if isBoundaryValidationError(err) {
			h.writeErrorCode(w, r, http.StatusBadRequest, boundaryInvalidCode, boundaryInvalidMessage)
			return
		}
		h.serverError(w, r, "apply watch-zone update", err)
		return
	}

	// Gate A (hard server ceiling) and Gate B (per-tier cap) both apply only
	// when this update actually set a new shape -- reverting to a circle or
	// leaving the shape alone never re-checks a radius that was already
	// accepted (or is unrelated to a boundary) on a prior write.
	if settingBoundary && (updated.RadiusMetres > maxRadiusMetres || updated.RadiusMetres > tier.MaxZoneRadiusMetres()) {
		h.writeErrorCode(w, r, http.StatusBadRequest, boundaryTooLargeCode, boundaryTooLargeMessage)
		return
	}

	if err := h.store.Save(r.Context(), updated); err != nil {
		h.serverError(w, r, "save watch zone", err)
		return
	}
	if h.metrics != nil {
		h.metrics.WatchZoneUpdated(r.Context())
	}

	// The paused computation (GH#889) needs the caller's full zone set to rank
	// the edited zone, unlike list (which already has it); skip the extra
	// store round trip entirely when no profile reader is wired.
	var paused bool
	if h.profiles != nil {
		allZones, zerr := h.store.GetByUserID(r.Context(), userID)
		if zerr != nil {
			h.serverError(w, r, "list watch zones for pause check", zerr)
			return
		}
		pausedSet, perr := h.pausedZoneIDs(r.Context(), userID, allZones)
		if perr != nil {
			h.serverError(w, r, "compute paused zones", perr)
			return
		}
		paused = pausedSet[updated.ID]
	}
	h.writeJSON(w, r, http.StatusOK, updateResult{Zone: summaryOf(updated, paused)})
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

// writeErrorCode emits the error envelope with a stable, machine-readable code
// in the "error" field and a human message in "message" -- the same shape
// internal/offercodes and internal/subscriptions use. writeError (above) stays
// exactly as-is for its pre-existing call sites, whose "error" field carries
// prose text with "message" always null; this is for new call sites (the
// boundary_* errors, tc-6he3x.4) that need a stable discriminator a client can
// branch on.
func (h *handler) writeErrorCode(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	msg := message
	body, err := httputil.EncodeJSON(apiErrorResponse{Error: code, Message: &msg})
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
// backfilled by middleware.ErrorBody, mirroring the rest of the API. A
// context.Canceled error means the caller disconnected before the response was
// ready (e.g. a rapid map pan cancelling an in-flight fetch) rather than a
// genuine server fault — that case is deliberately not logged at error level
// and not given a 500: the caller is already gone, so nothing meaningfully
// needs to be written back (tc-ftccw).
func (h *handler) serverError(w http.ResponseWriter, r *http.Request, op string, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	h.logger.ErrorContext(r.Context(), "watch-zone request failed", "op", op, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
}
