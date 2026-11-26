package watchzones

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/httputil"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// The iOS client routes any 403 on create to the paywall, so a prose body is
// enough.
const quotaExceededMessage = "Watch zone quota exceeded. Upgrade your subscription for more zones."

// errProfileCASNotWired signals a wiring bug: create refuses to run an
// unprotected quota check without the CAS-backed profile store.
var errProfileCASNotWired = errors.New("profile CAS store not wired")

// profileReader loads the caller's profile so the create handler can read the
// subscription tier for the watch-zone quota check.
type profileReader interface {
	Get(ctx context.Context, userID string) (*profiles.UserProfile, error)
}

// appFinder runs the spatial lookups behind the per-zone applications list and
// the map clusters. Lookups are authority-agnostic,
// so a border-spanning zone surfaces neighbour-authority apps. Circle zones use
// FindNearbyPage, FindInZonePage and FindClustersInZone (ST_DWithin); custom-shape
// zones use the *Boundary* counterparts (ST_Covers against the polygon).
type appFinder interface {
	FindNearbyPage(ctx context.Context, latitude, longitude, radiusMetres float64, limit int, cursor string) ([]applications.PlanningApplication, string, error)
	FindInZonePage(ctx context.Context, q applications.InZoneQuery) ([]applications.PlanningApplication, string, error)
	FindClustersInZone(ctx context.Context, q applications.ClusterQuery) ([]applications.Cluster, error)
	FindInBoundaryPage(ctx context.Context, latitude, longitude float64, boundary []applications.Coordinate, limit int, cursor string) ([]applications.PlanningApplication, string, error)
	FindClustersInBoundary(ctx context.Context, q applications.BoundaryClusterQuery) ([]applications.Cluster, error)
	FindInBoundaryZonePage(ctx context.Context, q applications.InBoundaryQuery) ([]applications.PlanningApplication, string, error)
}

// unreadReader batches the per-application latest-unread lookup (read_at IS NULL,
// ADR 0035). *notifications.PostgresStore satisfies it.
type unreadReader interface {
	GetLatestUnreadByApplications(ctx context.Context, userID string, applicationUIDs []string) (map[string]notifications.LatestUnread, error)
}

// NearbyRoutes registers POST /v1/me/watch-zones (create), GET
// /v1/me/watch-zones/{zoneId}/applications and GET
// /v1/me/watch-zones/{zoneId}/applications/clusters. newID mints the zone id (a
// GUID in production); now stamps the zone's creation time.
func NearbyRoutes(
	mux *http.ServeMux,
	store zoneStore,
	profileReader profileReader,
	apps appFinder,
	unread unreadReader,
	newID func() string,
	now func() time.Time,
	logger *slog.Logger,
	opts ...Option,
) {
	h := &handler{
		store:    store,
		profiles: profileReader,
		apps:     apps,
		unread:   unread,
		newID:    newID,
		now:      now,
		logger:   logger,
	}
	for _, opt := range opts {
		opt(h)
	}
	mux.HandleFunc("POST /v1/me/watch-zones", h.create)
	mux.HandleFunc("GET /v1/me/watch-zones/{zoneId}/applications", h.applications)
	mux.HandleFunc("GET /v1/me/watch-zones/{zoneId}/applications/clusters", h.clusters)
}

// createRequest is the POST body. The optional flags default to true.
// Boundary is an optional GeoJSON polygon: when present,
// Latitude/Longitude/RadiusMetres are ignored and derived server-side from
// the boundary's centroid and enclosing radius instead (see create and
// (createRequest).valid).
type createRequest struct {
	Name                string           `json:"name"`
	Latitude            float64          `json:"latitude"`
	Longitude           float64          `json:"longitude"`
	RadiusMetres        float64          `json:"radiusMetres"`
	PushEnabled         *bool            `json:"pushEnabled"`
	EmailInstantEnabled *bool            `json:"emailInstantEnabled"`
	Boundary            *boundaryGeoJSON `json:"boundary"`
	// FilterKey: nil and "" both mean no filter (unlike PATCH, create has no
	// existing value to leave alone). Non-empty requires Pro and a catalog key.
	FilterKey *string `json:"filterKey"`
}

// maxRadiusMetres is the server-side ceiling for a watch-zone radius. It matches
// the top-tier iOS UI ceiling (10 km). If a larger radius tier is ever offered,
// bump this value server-side first before shipping the iOS change.
const maxRadiusMetres = 10_000

// Bounded so a dense urban zone cannot blow the server write timeout. The
// param-less path keeps the 500 default as a backward-compat contract.
const (
	defaultNearbyLimit = 500
	defaultSortedLimit = 150
	maxNearbyLimit     = 500
)

const invalidCursorMessage = "Invalid cursor."

const invalidSortMessage = "Invalid sort."

const invalidStatusMessage = "Invalid status filter."

const invalidBBoxMessage = "Invalid bbox."

const invalidZoomMessage = "Invalid zoom."

const invalidUnreadMessage = "Invalid unread filter."

// "All" is not a real status filter, so it does not conflict with ?unread=true.
const filterConflictMessage = "status and unread filters are mutually exclusive."

// A boundary-carrying request skips the circle checks: create overwrites its
// Latitude/Longitude/RadiusMetres from the boundary and validates the shape there.
func (req createRequest) valid() bool {
	if strings.TrimSpace(req.Name) == "" {
		return false
	}
	if req.Boundary != nil {
		return true
	}
	if math.IsNaN(req.Latitude) || math.IsInf(req.Latitude, 0) {
		return false
	}
	if math.IsNaN(req.Longitude) || math.IsInf(req.Longitude, 0) {
		return false
	}
	if math.IsNaN(req.RadiusMetres) || math.IsInf(req.RadiusMetres, 0) {
		return false
	}
	if req.RadiusMetres <= 0 || req.RadiusMetres > maxRadiusMetres {
		return false
	}
	if req.Latitude < -90 || req.Latitude > 90 {
		return false
	}
	if req.Longitude < -180 || req.Longitude > 180 {
		return false
	}
	return true
}

// createResult is the POST /v1/me/watch-zones response. Latitude, Longitude,
// RadiusMetres and Boundary reflect the persisted zone, so a boundary-carrying
// request gets back the server-derived centroid and enclosing radius.
type createResult struct {
	Latitude     float64          `json:"latitude"`
	Longitude    float64          `json:"longitude"`
	RadiusMetres float64          `json:"radiusMetres"`
	Boundary     *boundaryGeoJSON `json:"boundary"`
	// NearbyApplications is always an empty array, kept so the wire shape is stable.
	NearbyApplications []applications.NearbyResult `json:"nearbyApplications"`
	// FilterKey is nil for an unfiltered zone, the catalog key otherwise.
	FilterKey *string `json:"filterKey"`
}

// create implements POST /v1/me/watch-zones: validate (400), gate a
// boundary-carrying request on the caller's tier (403
// boundary_requires_paid_tier) and shape/radius (400 boundary_invalid /
// boundary_too_large), enforce the tier's watch-zone quota (403), persist the
// zone, and return 201 Created. nearbyApplications is always empty: no client
// reads it, and the spatial query it needed could fail after the zone was saved.
func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !req.valid() {
		h.writeError(w, r, http.StatusBadRequest, invalidPayloadMessage)
		return
	}

	profile, err := h.profiles.Get(r.Context(), userID)
	if err != nil || profile == nil {
		// A missing profile is a 500 — the iOS app always registers on first launch.
		h.serverError(w, r, "load profile for quota check", err)
		return
	}
	tier := profile.EffectiveTier(h.now())

	// Overwriting Latitude/Longitude/RadiusMetres with the boundary's derived
	// values lets the circle-shaped NewWatchZone step below run unchanged.
	var boundary Boundary
	if req.Boundary != nil {
		if !tier.AllowsCustomBoundary() {
			h.writeErrorCode(w, r, http.StatusForbidden, boundaryRequiresPaidTierCode, boundaryRequiresPaidTierMessage)
			return
		}
		b, berr := NewBoundary(req.Boundary.vertices())
		if berr != nil {
			h.writeErrorCode(w, r, http.StatusBadRequest, boundaryInvalidCode, boundaryInvalidMessage)
			return
		}
		radius := b.EnclosingRadiusMetres()
		if radius > maxRadiusMetres || radius > tier.MaxZoneRadiusMetres() {
			h.writeErrorCode(w, r, http.StatusBadRequest, boundaryTooLargeCode, boundaryTooLargeMessage)
			return
		}
		boundary = b
		req.Latitude, req.Longitude = b.Centroid()
		req.RadiusMetres = radius
	}

	// A non-empty filterKey is Pro-only, checked before the quota claim so a
	// disallowed or unrecognised filter never consumes a quota slot.
	var filterKey FilterKey
	if req.FilterKey != nil && *req.FilterKey != "" {
		if !tier.AllowsWatchZoneFilter() {
			h.writeErrorCode(w, r, http.StatusForbidden, filterRequiresProTierCode, filterRequiresProTierMessage)
			return
		}
		if !IsValidFilterKey(*req.FilterKey) {
			h.writeErrorCode(w, r, http.StatusBadRequest, filterKeyInvalidCode, filterKeyInvalidMessage)
			return
		}
		filterKey = FilterKey(*req.FilterKey)
	}

	// A nil profileCAS is a wiring bug; fail closed rather than run an
	// unprotected count-then-save.
	if h.profileCAS == nil {
		h.serverError(w, r, "quota gate", errProfileCASNotWired)
		return
	}
	ok, casErr := h.atomicQuotaIncrement(r.Context(), userID, tier.WatchZoneLimit())
	if casErr != nil {
		h.serverError(w, r, "atomic quota check", casErr)
		return
	}
	if !ok {
		h.writeError(w, r, http.StatusForbidden, quotaExceededMessage)
		return
	}

	zone, err := NewWatchZone(
		h.newID(), userID, req.Name,
		req.Latitude, req.Longitude, req.RadiusMetres,
		h.now(),
		boolOrTrue(req.PushEnabled), boolOrTrue(req.EmailInstantEnabled))
	if err != nil {
		h.serverError(w, r, "build watch zone", err)
		return
	}
	zone.Boundary = boundary
	zone.FilterKey = filterKey
	if err := h.store.Save(r.Context(), zone); err != nil {
		if errors.Is(err, ErrDuplicateName) {
			h.writeErrorCode(w, r, http.StatusConflict, zoneNameTakenCode, zoneNameTakenMessage)
			return
		}
		h.serverError(w, r, "save watch zone", err)
		return
	}
	if h.metrics != nil {
		h.metrics.WatchZoneCreated(r.Context())
	}

	h.writeCreated(w, r, "/v1/me/watch-zones/"+zone.ID, createResult{
		Latitude:           zone.Latitude,
		Longitude:          zone.Longitude,
		RadiusMetres:       zone.RadiusMetres,
		Boundary:           boundaryToGeoJSON(zone.Boundary),
		NearbyApplications: []applications.NearbyResult{},
		FilterKey:          encodeFilterKey(zone.FilterKey),
	})
}

// latestUnreadEventWire is the per-row unread descriptor on the applications
// list: { type, decision, createdAt }. type is the NotificationEventType name.
type latestUnreadEventWire struct {
	Type      string              `json:"type"`
	Decision  *string             `json:"decision"`
	CreatedAt platform.DotNetTime `json:"createdAt"`
}

// applications implements GET /v1/me/watch-zones/{zoneId}/applications: load the
// zone (404 if absent), find the applications in it, and augment each row with
// its latest unread notification.
//
// Routing: a param-less call (no ?sort=) keeps the nearest-first path
// (FindNearbyPage, default 500) for older clients. A ?sort= call uses the
// server-side sort surface (FindInZonePage, default 150) with a sort-aware
// keyset cursor; recent-activity joins the caller's own notifications.
func (h *handler) applications(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())
	zoneID := r.PathValue("zoneId")

	zone, err := h.store.Get(r.Context(), userID, zoneID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h.serverError(w, r, "load watch zone", err)
		return
	}

	sort, sortPresent, sortOK := parseSort(r.URL.Query().Get("sort"))
	if !sortOK {
		h.writeError(w, r, http.StatusBadRequest, invalidSortMessage)
		return
	}

	status, statusOK := parseStatus(r.URL.Query().Get("status"))
	if !statusOK {
		h.writeError(w, r, http.StatusBadRequest, invalidStatusMessage)
		return
	}

	unreadFilter, unreadOK := parseUnread(r.URL.Query().Get("unread"))
	if !unreadOK {
		h.writeError(w, r, http.StatusBadRequest, invalidUnreadMessage)
		return
	}

	// Status and unread are mutually exclusive; reject both together before any
	// query (a status="" means no real filter, so it never conflicts).
	if status != "" && unreadFilter {
		h.writeError(w, r, http.StatusBadRequest, filterConflictMessage)
		return
	}

	cursor, ok := decodeCursor(r.URL.Query().Get("cursor"))
	if !ok {
		h.writeError(w, r, http.StatusBadRequest, invalidCursorMessage)
		return
	}

	apps, nextCursor, err := h.findZonePage(r.Context(), userID, zone, sort, sortPresent, status, unreadFilter, r.URL.Query().Get("limit"), cursor)
	if err != nil {
		// A stale, sort-mismatched or filter-mismatched cursor is a client error
		// (400), not a 500: the keyset cursor is only valid for the sort AND filter
		// it was minted under.
		if errors.Is(err, applications.ErrCursorInvalid) ||
			errors.Is(err, applications.ErrCursorSortMismatch) ||
			errors.Is(err, applications.ErrCursorFilterMismatch) {
			h.writeError(w, r, http.StatusBadRequest, invalidCursorMessage)
			return
		}
		h.serverError(w, r, "find applications in zone", err)
		return
	}

	// Classify each row's latest unread notification. Unread is purely
	// read_at IS NULL (ADR 0035), independent of any notification_state row, so
	// this runs unconditionally.
	uids := make([]string, 0, len(apps))
	for _, a := range apps {
		uids = append(uids, a.UID)
	}
	unread, err := h.unread.GetLatestUnreadByApplications(r.Context(), userID, uids)
	if err != nil {
		h.serverError(w, r, "load latest unread", err)
		return
	}

	results := make([]applications.Result, 0, len(apps))
	for _, a := range apps {
		row := applications.ResultOf(a)
		if lu, ok := unread[a.UID]; ok {
			raw, err := json.Marshal(latestUnreadEventWire{
				Type:      string(lu.EventType),
				Decision:  lu.Decision,
				CreatedAt: platform.DotNetTime(lu.CreatedAt),
			})
			if err != nil {
				h.serverError(w, r, "encode latest unread", err)
				return
			}
			row.LatestUnreadEvent = raw
		}
		results = append(results, row)
	}
	// Hand the opaque continuation token back via the response header (not the
	// body, which stays a bare []Result so existing clients are untouched). Set it
	// before writeJSON, which calls WriteHeader. Omitted when the page is the last.
	if nextCursor != "" {
		w.Header().Set("X-Next-Cursor", encodeCursor(nextCursor))
	}
	h.writeJSON(w, r, http.StatusOK, results)
}

// findZonePage runs the bounded spatial page for the zone. A truly param-less
// request (no ?sort=, no status filter, no unread filter) keeps the legacy
// nearest-first finder at the 500 default (byte-identical param-less contract).
// As soon as a sort OR a filter is requested it uses the sort-aware finder at the
// 150 default and a sort-and-filter-aware keyset cursor. userID scopes the
// per-user notification data the recent-activity sort joins and the unread filter
// restricts on. rawLimit is the unparsed ?limit= value; cursor is the
// transport-unwrapped continuation token.
//
// Custom-shape zones branch the same way using the boundary-scoped finders
// (FindInBoundaryPage, FindInBoundaryZonePage).
func (h *handler) findZonePage(ctx context.Context, userID string, zone WatchZone, sort applications.Sort, sortPresent bool, status string, unread bool, rawLimit, cursor string) ([]applications.PlanningApplication, string, error) {
	if zone.IsCustomShape() {
		if !sortPresent && status == "" && !unread {
			limit := parseLimit(rawLimit, defaultNearbyLimit)
			return h.apps.FindInBoundaryPage(ctx, zone.Latitude, zone.Longitude, boundaryCoordinates(zone.Boundary), limit, cursor)
		}
		limit := parseLimit(rawLimit, defaultSortedLimit)
		return h.apps.FindInBoundaryZonePage(ctx, applications.InBoundaryQuery{
			UserID:    userID,
			Latitude:  zone.Latitude,
			Longitude: zone.Longitude,
			Boundary:  boundaryCoordinates(zone.Boundary),
			Sort:      sort,
			Status:    status,
			Unread:    unread,
			Limit:     limit,
			Cursor:    cursor,
		})
	}
	if !sortPresent && status == "" && !unread {
		limit := parseLimit(rawLimit, defaultNearbyLimit)
		return h.apps.FindNearbyPage(ctx, zone.Latitude, zone.Longitude, zone.RadiusMetres, limit, cursor)
	}
	limit := parseLimit(rawLimit, defaultSortedLimit)
	return h.apps.FindInZonePage(ctx, applications.InZoneQuery{
		UserID:       userID,
		Latitude:     zone.Latitude,
		Longitude:    zone.Longitude,
		RadiusMetres: zone.RadiusMetres,
		Sort:         sort,
		Status:       status,
		Unread:       unread,
		Limit:        limit,
		Cursor:       cursor,
	})
}

// clusters implements GET /v1/me/watch-zones/{zoneId}/applications/clusters:
// load the zone (404 if not owned, like the sibling applications
// route), parse the viewport (?bbox=) and zoom (?zoom=) and optional ?status=
// filter (each malformed value is a clean 400 before any query), translate the
// zoom into a PostGIS grid cell size, and return the grid-aggregated clusters for
// the visible rect as a JSON array. The store stays a pure spatial primitive: the
// zoom -> grid-size policy lives here so density can be tuned without a store change.
func (h *handler) clusters(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())
	zoneID := r.PathValue("zoneId")

	zone, err := h.store.Get(r.Context(), userID, zoneID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h.serverError(w, r, "load watch zone", err)
		return
	}

	box, ok := applications.ParseBBox(r.URL.Query().Get("bbox"))
	if !ok {
		h.writeError(w, r, http.StatusBadRequest, invalidBBoxMessage)
		return
	}

	gridSize, ok := parseZoom(r.URL.Query().Get("zoom"))
	if !ok {
		h.writeError(w, r, http.StatusBadRequest, invalidZoomMessage)
		return
	}

	// ?status= filters on the raw app_state; "All"/absent means no filter, any
	// other non-vocabulary value is a clean 400 (same vocabulary as the list).
	status, ok := parseStatus(r.URL.Query().Get("status"))
	if !ok {
		h.writeError(w, r, http.StatusBadRequest, invalidStatusMessage)
		return
	}

	// Custom-shape zones branch to the boundary-scoped
	// cluster query: ST_Covers against the polygon rather than ST_DWithin
	// against the circle. Every other param (viewport, status, coalesce
	// threshold) carries the same meaning as the circle path unchanged.
	var clusters []applications.Cluster
	if zone.IsCustomShape() {
		clusters, err = h.apps.FindClustersInBoundary(r.Context(), applications.BoundaryClusterQuery{
			Boundary:        boundaryCoordinates(zone.Boundary),
			West:            box.West,
			South:           box.South,
			East:            box.East,
			North:           box.North,
			GridSizeDegrees: gridSize,
			Status:          status,
			// See the circle path below for the coalesce-threshold rationale --
			// identical here.
			CoalesceThresholdDegrees: applications.FinestGridDegrees(),
		})
	} else {
		clusters, err = h.apps.FindClustersInZone(r.Context(), applications.ClusterQuery{
			Latitude:        zone.Latitude,
			Longitude:       zone.Longitude,
			RadiusMetres:    zone.RadiusMetres,
			West:            box.West,
			South:           box.South,
			East:            box.East,
			North:           box.North,
			GridSizeDegrees: gridSize,
			Status:          status,
			// The coalesce threshold is the finest (zoom-20) grid cell size, not the
			// request's own grid: a multi-member cell whose member points already span
			// less than this can never be split by zooming, so the store attaches an
			// applicationIds member list. applications.FinestGridDegrees() keeps it
			// tracking the shared zoom -> grid policy with no separate constant.
			CoalesceThresholdDegrees: applications.FinestGridDegrees(),
		})
	}
	if err != nil {
		h.serverError(w, r, "find clusters in zone", err)
		return
	}
	// Encode a bare JSON array, never null, for an empty viewport.
	if clusters == nil {
		clusters = []applications.Cluster{}
	}
	h.writeJSON(w, r, http.StatusOK, clusters)
}

// parseZoom resolves ?zoom= to a grid cell size via
// applications.GridDegreesForZoom. An absent or non-integer value is rejected
// locally; an out-of-range one is rejected by GridDegreesForZoom — either way a
// garbage zoom is a clean 400, never a silent default.
func parseZoom(raw string) (float64, bool) {
	if raw == "" {
		return 0, false
	}
	z, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return applications.GridDegreesForZoom(z)
}

// parseLimit resolves ?limit= to a bounded page size: absent, non-numeric, or
// non-positive falls back to def; anything above the ceiling clamps to
// maxNearbyLimit. It never rejects — an existing client that omits or fat-fingers
// the parameter still gets a sane bounded page. def lets the legacy path keep its
// 500 default while the sort-aware path defaults to 150.
func parseLimit(raw string, def int) int {
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	if n > maxNearbyLimit {
		return maxNearbyLimit
	}
	return n
}

// parseSort resolves ?sort= to a supported Sort. An absent value defaults to
// SortDistance (the legacy nearest-first behaviour). The boolean reports whether
// the value is supported ({distance, newest, oldest, status, recent-activity});
// an unsupported value (garbage) is a clean 400. The caller treats an absent value
// (ok with SortDistance and present=false) as the legacy param-less path.
func parseSort(raw string) (sort applications.Sort, present, ok bool) {
	if raw == "" {
		return applications.SortDistance, false, true
	}
	s := applications.Sort(raw)
	return s, true, s.Supported()
}

// parseStatus resolves ?status= to an app_state filter value. An absent value or
// the sentinel "All" both mean "no status filter" (returns "", ok). A recognised
// app_state value passes through. Any other value is an unsupported filter (ok ==
// false) so the handler returns a clean 400 rather than silently ignoring it.
func parseStatus(raw string) (status string, ok bool) {
	if raw == "" || raw == "All" {
		return "", true
	}
	if applications.StatusSupported(raw) {
		return raw, true
	}
	return "", false
}

// parseUnread resolves ?unread= to the unread-only filter flag. "true" turns it
// on; "false" or absent leave it off. Any other value is rejected (ok == false)
// so a typo'd flag is a clean 400 rather than being silently treated as off.
func parseUnread(raw string) (unread, ok bool) {
	switch raw {
	case "":
		return false, true
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		return false, false
	}
}

// decodeCursor base64url-decodes an opaque ?cursor= value into the store's raw
// keyset continuation token (backend-agnostic; sort-aware for the ?sort= path).
// An empty value means the first page (ok). A malformed value is rejected
// (ok == false) so a garbage cursor is a clean 400, not a silent reset to the
// first page.
func decodeCursor(raw string) (string, bool) {
	if raw == "" {
		return "", true
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", false
	}
	return string(b), true
}

// encodeCursor base64url-encodes the store's raw keyset continuation token for
// the X-Next-Cursor response header — header- and URL-safe, unpadded.
func encodeCursor(token string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(token))
}

// atomicQuotaIncrement tries to atomically claim one slot in the user's
// watch-zone quota using a bounded CAS retry loop on the profile document.
// Returns (true, nil) when the slot was claimed, (false, nil) when the quota
// is already exhausted, or (false, err) on a persistent store failure.
//
// The algorithm:
//  1. Read the profile with its current etag.
//  2. If WatchZoneCount is nil (legacy profile), lazy-init from the live
//     zone count — do NOT run a separate migration.
//  3. Check count < limit; if at/over, return (false, nil).
//  4. Increment the counter and ReplaceWithETag. On 412 (CAS conflict),
//     re-read and retry. After maxCASRetries failures return (false, nil)
//     — quota-exceeded is the safe failure mode.
func (h *handler) atomicQuotaIncrement(ctx context.Context, userID string, limit int) (bool, error) {
	for range maxCASRetries {
		profile, etag, err := h.profileCAS.GetWithETag(ctx, userID)
		if err != nil {
			return false, fmt.Errorf("read profile for quota CAS: %w", err)
		}
		if profile == nil {
			return false, errors.New("profile not found for quota check")
		}

		// Unlimited tier: no slot to claim.
		if limit >= math.MaxInt32 {
			return true, nil
		}

		// A legacy profile has no counter yet, so seed it from the live zone count.
		var currentCount int
		if profile.WatchZoneCount == nil {
			existing, lerr := h.store.GetByUserID(ctx, userID)
			if lerr != nil {
				return false, fmt.Errorf("lazy-init zone count: %w", lerr)
			}
			currentCount = len(existing)
		} else {
			currentCount = *profile.WatchZoneCount
		}

		if currentCount >= limit {
			return false, nil
		}

		newCount := currentCount + 1
		updated := *profile
		updated.WatchZoneCount = &newCount

		err = h.profileCAS.UpdateZoneCountWithCAS(ctx, userID, &updated, etag)
		if err == nil {
			return true, nil
		}
		if errors.Is(err, platform.ErrCASPreconditionFailed) {
			continue
		}
		return false, fmt.Errorf("quota CAS replace: %w", err)
	}
	// Exhausted retries: conservative 403.
	return false, nil
}

// boundaryCoordinates converts a watch zone's Boundary ring into the
// applications package's own Coordinate slice. applications cannot import
// watchzones (which already imports applications), so each polygon-scoped
// store call re-encodes the ring into that package's boundary parameter type.
func boundaryCoordinates(b Boundary) []applications.Coordinate {
	out := make([]applications.Coordinate, len(b))
	for i, v := range b {
		out[i] = applications.Coordinate{Longitude: v.Longitude, Latitude: v.Latitude}
	}
	return out
}

// boolOrTrue resolves an optional bool flag, defaulting an absent value to true.
func boolOrTrue(p *bool) bool {
	if p == nil {
		return true
	}
	return *p
}

// writeCreated emits a 201 Created with a Location header and the JSON body.
func (h *handler) writeCreated(w http.ResponseWriter, r *http.Request, location string, v any) {
	body, err := httputil.EncodeJSON(v)
	if err != nil {
		h.serverError(w, r, "encode response", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Location", location)
	w.WriteHeader(http.StatusCreated)
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write response", "error", err)
	}
}
