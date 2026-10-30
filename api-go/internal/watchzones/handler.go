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

const invalidPayloadMessage = "Invalid watch zone payload."

// Custom-shape boundary error codes carry a stable machine-readable code
// separate from the message, so a client can route "wrong tier", "bad shape"
// and "too big" to the right UX (paywall, validation error, resize prompt).
const (
	// boundaryRequiresPaidTierCode/-Message: 403, a Free-tier caller supplied a
	// non-null boundary on create, or on a PATCH that sets one.
	boundaryRequiresPaidTierCode    = "boundary_requires_paid_tier"
	boundaryRequiresPaidTierMessage = "Custom-shape watch zones require a paid subscription."
	// boundaryInvalidCode/-Message: 400, the supplied boundary failed
	// NewBoundary's shape validation or was not well-formed GeoJSON.
	boundaryInvalidCode    = "boundary_invalid"
	boundaryInvalidMessage = "The supplied boundary is not a valid shape."
	// boundaryTooLargeCode/-Message: 400, the boundary's enclosing radius
	// exceeds either the hard server ceiling (maxRadiusMetres, nearby.go) or
	// the caller's tier cap (profiles.SubscriptionTier.MaxZoneRadiusMetres).
	boundaryTooLargeCode    = "boundary_too_large"
	boundaryTooLargeMessage = "The boundary is too large for your subscription tier."
	// zoneNameTakenCode/-Message: 409, Store.Save returned ErrDuplicateName:
	// the caller already owns a watch zone with this name.
	zoneNameTakenCode    = "zone_name_taken"
	zoneNameTakenMessage = "You already have a watch zone with this name."
)

// Pre-canned watch-zone filter error codes, shared by the create path
// (nearby.go) and the patch path.
const (
	// filterRequiresProTierCode/-Message: 403, a Free- or Personal-tier caller
	// supplied a non-empty filterKey on create, or on a PATCH that sets one.
	// Pro-only -- deliberately narrower than AllowsCustomBoundary's IsPaid()
	// gate (see profiles.SubscriptionTier.AllowsWatchZoneFilter).
	filterRequiresProTierCode    = "filter_requires_pro_tier"
	filterRequiresProTierMessage = "Pre-canned watch-zone filters require a Pro subscription."
	// filterKeyInvalidCode/-Message: 400, the supplied filterKey is not a
	// member of the catalog (see IsValidFilterKey / ErrUnknownFilterKey).
	filterKeyInvalidCode    = "filter_key_invalid"
	filterKeyInvalidMessage = "Unrecognised filter."
)

// errProfileReaderNotWired signals a wiring bug: the boundary and filter tier
// gates refuse to run without a profile reader.
var errProfileReaderNotWired = errors.New("profile reader not wired for watch-zone tier gate")

// isBoundaryValidationError reports whether err is one of NewBoundary's
// shape-validation sentinels, which WithUpdates propagates unwrapped; these map
// to a 400 boundary_invalid rather than the generic 500 a merge failure gets.
func isBoundaryValidationError(err error) bool {
	return errors.Is(err, ErrBoundaryVertexCount) ||
		errors.Is(err, ErrBoundaryDuplicateVertex) ||
		errors.Is(err, ErrBoundarySelfIntersecting) ||
		errors.Is(err, ErrBoundaryOutOfBounds)
}

// zoneStore is the consumer-side store the handlers use.
type zoneStore interface {
	GetByUserID(ctx context.Context, userID string) ([]WatchZone, error)
	Get(ctx context.Context, userID, zoneID string) (WatchZone, error)
	Save(ctx context.Context, z WatchZone) error
	Delete(ctx context.Context, userID, zoneID string) error
}

// profileCAS is the consumer-side interface for atomically updating the
// watch-zone quota counter on the user's profile.
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

// Option configures the watch-zone routes.
type Option func(*handler)

// WithMetricsRecorder wires the metrics recorder the handlers record the
// watch-zone lifecycle counters on.
func WithMetricsRecorder(rec MetricsRecorder) Option {
	return func(h *handler) { h.metrics = rec }
}

// WithProfileCAS wires the CAS-capable profile store used by the create path
// for atomic quota enforcement and the delete path for quota decrement.
func WithProfileCAS(cas profileCAS) Option {
	return func(h *handler) { h.profileCAS = cas }
}

// WithProfileReader wires the profile reader the list/patch handlers use to
// compute each zone's derived "paused" field: a zone whose
// oldest-first rank among the caller's own zones — by (CreatedAt, ID) —
// exceeds their current effective-tier watch-zone limit. Not wiring this
// option leaves every zone unpaused.
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
// createdAt is deliberately absent. Paused is derived, never stored (see
// pausedIDs). AuthorityID is always 0 and stays only for older clients'
// strict decoding.
type watchZoneSummary struct {
	ID                  string  `json:"id"`
	Name                string  `json:"name"`
	Latitude            float64 `json:"latitude"`
	Longitude           float64 `json:"longitude"`
	RadiusMetres        float64 `json:"radiusMetres"`
	AuthorityID         int     `json:"authorityId"`
	PushEnabled         bool    `json:"pushEnabled"`
	EmailInstantEnabled bool    `json:"emailInstantEnabled"`
	Paused              bool    `json:"paused"`
	// Boundary is null for a circle zone and a GeoJSON Polygon for a
	// custom-shape one, whose Latitude/Longitude/RadiusMetres hold the
	// derived centroid and enclosing radius.
	Boundary *boundaryGeoJSON `json:"boundary"`
	// FilterKey is null for an unfiltered zone, else the catalog key.
	FilterKey *string `json:"filterKey"`
}

func summaryOf(z WatchZone, paused bool) watchZoneSummary {
	return watchZoneSummary{
		ID:           z.ID,
		Name:         z.Name,
		Latitude:     z.Latitude,
		Longitude:    z.Longitude,
		RadiusMetres: z.RadiusMetres,
		// Hardcoded 0 for older iOS clients' strict decode; no client reads it.
		AuthorityID:         0,
		PushEnabled:         z.PushEnabled,
		EmailInstantEnabled: z.EmailInstantEnabled,
		Paused:              paused,
		Boundary:            boundaryToGeoJSON(z.Boundary),
		FilterKey:           encodeFilterKey(z.FilterKey),
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

// pausedZoneIDs resolves which of zones (the caller's full zone set) exceed the
// caller's effective-tier limit. It fails open with a nil map (nothing paused)
// when no profile reader is wired, the profile is missing, or the tier is unlimited.
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

// pausedIDs ranks zones oldest-first by (CreatedAt, ID) and marks those beyond
// limit as paused. It is derived, never stored, so an upgrade or a delete of an
// older zone revives the next zone with no extra work.
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
// Boundary and FilterKey are tri-state (absent/null/value), so they are
// json.RawMessage: a pointer would collapse absent and explicit null.
type patchRequest struct {
	Name                *string         `json:"name"`
	Latitude            *float64        `json:"latitude"`
	Longitude           *float64        `json:"longitude"`
	RadiusMetres        *float64        `json:"radiusMetres"`
	PushEnabled         *bool           `json:"pushEnabled"`
	EmailInstantEnabled *bool           `json:"emailInstantEnabled"`
	Boundary            json.RawMessage `json:"boundary"`
	FilterKey           json.RawMessage `json:"filterKey"`
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

func (req patchRequest) toUpdate(boundary *Boundary, filterKey *string) ZoneUpdate {
	return ZoneUpdate{
		Name:                req.Name,
		Latitude:            req.Latitude,
		Longitude:           req.Longitude,
		RadiusMetres:        req.RadiusMetres,
		PushEnabled:         req.PushEnabled,
		EmailInstantEnabled: req.EmailInstantEnabled,
		Boundary:            boundary,
		FilterKey:           filterKey,
	}
}

// decodeBoundaryUpdate maps the tri-state PATCH "boundary" field: absent gives
// (nil, nil), null gives a pointer to an empty Boundary (revert to a circle), and
// a polygon gives its unvalidated vertices, which WithUpdates validates. Valid
// JSON that is not a GeoJSON-shaped object returns an error.
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

// decodeFilterKeyUpdate mirrors decodeBoundaryUpdate: absent gives (nil, nil),
// null gives a pointer to "" (clear the filter), and a string gives its value,
// which WithUpdates validates against the catalog. Non-string JSON returns an error.
func decodeFilterKeyUpdate(raw json.RawMessage) (*string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if bytes.Equal(raw, []byte("null")) {
		empty := ""
		return &empty, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("decode filter key update: %w", err)
	}
	return &s, nil
}

// patch implements PATCH /v1/me/watch-zones/{zoneId}. A missing zone 404s
// before any tier gate, and resending the stored boundary or filterKey unchanged
// is never gated (403 boundary_requires_paid_tier / filter_requires_pro_tier).
// Invalid shapes, keys and oversized boundaries are 400s; any other domain
// invariant violation (e.g. a blank name) is a 500.
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

	filterKeyUpdate, err := decodeFilterKeyUpdate(req.FilterKey)
	if err != nil {
		h.writeErrorCode(w, r, http.StatusBadRequest, filterKeyInvalidCode, filterKeyInvalidMessage)
		return
	}

	// Load the zone before the tier gates: a missing zone must 404 before any
	// gate, and the gates need the stored FilterKey/Boundary to tell a new value
	// from a client resending full state unchanged.
	zone, err := h.store.Get(r.Context(), userID, zoneID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h.serverError(w, r, "load watch zone", err)
		return
	}

	// Only a changed, non-empty value is gated, so a downgraded caller can PATCH
	// unrelated fields or clients can resend full state unchanged.
	settingFilter := filterKeyUpdate != nil && *filterKeyUpdate != "" && *filterKeyUpdate != string(zone.FilterKey)

	settingBoundary := boundaryUpdate != nil && len(*boundaryUpdate) > 0 && !zone.Boundary.Equal(*boundaryUpdate)

	var tier profiles.SubscriptionTier
	if settingBoundary || settingFilter {
		if h.profiles == nil {
			h.serverError(w, r, "watch-zone tier gate", errProfileReaderNotWired)
			return
		}
		profile, perr := h.profiles.Get(r.Context(), userID)
		if perr != nil || profile == nil {
			// A missing profile for an authenticated caller is a server-side
			// inconsistency (the iOS app always registers on first launch) --
			// mirroring nearby.go's create-path quota check.
			h.serverError(w, r, "load profile for watch-zone tier gate", perr)
			return
		}
		tier = profile.EffectiveTier(h.now())
		if settingBoundary && !tier.AllowsCustomBoundary() {
			h.writeErrorCode(w, r, http.StatusForbidden, boundaryRequiresPaidTierCode, boundaryRequiresPaidTierMessage)
			return
		}
		if settingFilter && !tier.AllowsWatchZoneFilter() {
			h.writeErrorCode(w, r, http.StatusForbidden, filterRequiresProTierCode, filterRequiresProTierMessage)
			return
		}
	}

	updated, err := zone.WithUpdates(req.toUpdate(boundaryUpdate, filterKeyUpdate))
	if err != nil {
		if isBoundaryValidationError(err) {
			h.writeErrorCode(w, r, http.StatusBadRequest, boundaryInvalidCode, boundaryInvalidMessage)
			return
		}
		if errors.Is(err, ErrUnknownFilterKey) {
			h.writeErrorCode(w, r, http.StatusBadRequest, filterKeyInvalidCode, filterKeyInvalidMessage)
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
		if errors.Is(err, ErrDuplicateName) {
			h.writeErrorCode(w, r, http.StatusConflict, zoneNameTakenCode, zoneNameTakenMessage)
			return
		}
		h.serverError(w, r, "save watch zone", err)
		return
	}
	if h.metrics != nil {
		h.metrics.WatchZoneUpdated(r.Context())
	}

	// The paused computation needs the caller's full zone set to rank
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

// writeError emits the error envelope with prose in "error" and a null "message".
func (h *handler) writeError(w http.ResponseWriter, r *http.Request, status int, message string) {
	h.writeEnvelope(w, r, status, apiErrorResponse{Error: message})
}

// writeErrorCode emits the error envelope with a stable machine-readable code in
// "error" and a human message in "message", for clients that branch on the code.
func (h *handler) writeErrorCode(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	h.writeEnvelope(w, r, status, apiErrorResponse{Error: code, Message: &message})
}

func (h *handler) writeEnvelope(w http.ResponseWriter, r *http.Request, status int, env apiErrorResponse) {
	body, err := httputil.EncodeJSON(env)
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

// serverError answers a bodyless error status chosen by platform.ClassifyRequestError
// (499 unlogged for a cancelled request, 503 for a timeout, otherwise 500).
// middleware.ErrorBody backfills the envelope.
func (h *handler) serverError(w http.ResponseWriter, r *http.Request, op string, err error) {
	platform.RespondRequestError(w, r, h.logger, "watch-zone request failed", err, "op", op)
}
