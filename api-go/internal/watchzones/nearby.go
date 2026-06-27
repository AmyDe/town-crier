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
	"github.com/AmyDe/town-crier/api-go/internal/notificationstate"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// quotaExceededMessage is the error text for a watch-zone quota breach (403).
// The iOS client (tc-gpjk) treats any 403 on create as a quota breach and routes
// to the paywall, so this prose body produces the same Upgrade-Required UX as a
// structured one.
const quotaExceededMessage = "Watch zone quota exceeded. Upgrade your subscription for more zones."

// errProfileCASNotWired signals a wiring bug: the create path requires the
// CAS-backed profile store and refuses to run an unprotected quota check
// without it. Unreachable in production, where NearbyRoutes is always wired
// WithProfileCAS.
var errProfileCASNotWired = errors.New("profile CAS store not wired")

// profileReader loads the caller's profile so the create handler can read the
// subscription tier for the watch-zone quota check.
type profileReader interface {
	Get(ctx context.Context, userID string) (*profiles.UserProfile, error)
}

// authorityResolver reverse-geocodes coordinates to a PlanIt authority id when
// the create request omits one. *geocoding.Client satisfies it.
type authorityResolver interface {
	ResolveAuthority(ctx context.Context, latitude, longitude float64) (int, error)
}

// appFinder runs the spatial lookup that backs both the create response's nearby
// applications and the per-zone applications list. It is authority-agnostic and
// cross-partition, so a border-spanning zone surfaces neighbour-authority apps
// (tc-zldl). The fetch is bounded: it returns at most `limit` rows for the given
// cursor plus an opaque continuation token for the next page (tc-fm8f).
//
// FindNearbyPage is the legacy default-distance path (byte-identical param-less
// contract). FindInZonePage adds the server-side ?sort= surface (epic #682 slices
// 1-3: distance/newest/oldest/status/recent-activity) with a sort-aware keyset
// cursor; userID scopes the per-user notification join the recent-activity sort
// needs. *applications.PostgresStore satisfies both.
type appFinder interface {
	FindNearbyPage(ctx context.Context, latitude, longitude, radiusMetres float64, limit int, cursor string) ([]applications.PlanningApplication, string, error)
	FindInZonePage(ctx context.Context, q applications.InZoneQuery) ([]applications.PlanningApplication, string, error)
}

// watermarkReader reads the caller's notification read-watermark. A nil return
// is the first-touch signal (no watermark yet). *notificationstate.CosmosStore
// satisfies it.
type watermarkReader interface {
	Get(ctx context.Context, userID string) (*notificationstate.State, error)
}

// unreadReader batches the per-application latest-unread lookup.
// *notifications.CosmosStore satisfies it.
type unreadReader interface {
	GetLatestUnreadByApplications(ctx context.Context, userID string, applicationUIDs []string, lastReadAt time.Time) (map[string]notifications.LatestUnread, error)
}

// NearbyRoutes registers POST /v1/me/watch-zones (create, returning nearby
// applications) and GET /v1/me/watch-zones/{zoneId}/applications. newID mints the
// zone id (a GUID in production); now stamps the zone's creation time.
func NearbyRoutes(
	mux *http.ServeMux,
	store zoneStore,
	profileReader profileReader,
	resolver authorityResolver,
	apps appFinder,
	state watermarkReader,
	unread unreadReader,
	newID func() string,
	now func() time.Time,
	logger *slog.Logger,
	opts ...Option,
) {
	h := &handler{
		store:    store,
		profiles: profileReader,
		resolver: resolver,
		apps:     apps,
		state:    state,
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
}

// createRequest is the POST body. The optional flags default to true
// and authorityId defaults to nil (resolve from coordinates).
type createRequest struct {
	Name                string  `json:"name"`
	Latitude            float64 `json:"latitude"`
	Longitude           float64 `json:"longitude"`
	RadiusMetres        float64 `json:"radiusMetres"`
	AuthorityID         *int    `json:"authorityId"`
	PushEnabled         *bool   `json:"pushEnabled"`
	EmailInstantEnabled *bool   `json:"emailInstantEnabled"`
}

// maxRadiusMetres is the server-side ceiling for a watch-zone radius. It matches
// the top-tier iOS UI ceiling (10 km). If a larger radius tier is ever offered,
// bump this value server-side first before shipping the iOS change.
const maxRadiusMetres = 10_000

// defaultNearbyLimit, defaultSortedLimit and maxNearbyLimit bound the per-request
// page of nearby applications. The browse path fetches a SINGLE bounded page so a
// dense urban zone can no longer drain tens of thousands of documents and blow the
// server write timeout (tc-fm8f).
//
// The legacy param-less path keeps the 500 default (byte-identical backward-compat
// contract, #541). The sort-aware path (?sort=) uses a smaller 150 default for a
// snappier first paint and infinite-scroll increment (epic #682). maxNearbyLimit
// is the shared clamp ceiling for both. The create + demo paths fetch page one only.
const (
	defaultNearbyLimit = 500
	defaultSortedLimit = 150
	maxNearbyLimit     = 500
)

// invalidCursorMessage is the 400 body when ?cursor= is not a valid continuation
// token: a malformed base64url wrapper, a token that is not a keyset cursor, or a
// cursor minted under a different ?sort= than the request carries.
const invalidCursorMessage = "Invalid cursor."

// invalidSortMessage is the 400 body when ?sort= is outside the supported set
// ({distance, newest, oldest, status, recent-activity}).
const invalidSortMessage = "Invalid sort."

// invalidStatusMessage is the 400 body when ?status= is outside the app_state
// filter vocabulary (and not "All"/absent, which mean no filter).
const invalidStatusMessage = "Invalid status filter."

// valid reports whether the create request passes the pre-handler guard:
// non-blank name, positive radius within the server ceiling, in-range
// coordinates, and a positive authority id when one is supplied.
func (req createRequest) valid() bool {
	if strings.TrimSpace(req.Name) == "" {
		return false
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
	if req.AuthorityID != nil && *req.AuthorityID <= 0 {
		return false
	}
	return true
}

// createResult is the POST /v1/me/watch-zones response: { nearbyApplications: [...] }.
// The applications are the raw-domain wire shape (no latestUnreadEvent).
type createResult struct {
	NearbyApplications []applications.NearbyResult `json:"nearbyApplications"`
}

// create implements POST /v1/me/watch-zones: validate (400), enforce the tier's
// watch-zone quota (403), resolve the authority from coordinates when absent,
// persist the zone, and return 201 Created with the applications already nearby.
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

	// Atomic quota gate: the CAS-backed profile counter is the ONLY create path,
	// so there is no non-atomic footgun. A nil profileCAS at request time is a
	// wiring bug (never reachable in production, where NearbyRoutes is always
	// wired WithProfileCAS); fail closed with a 500 rather than running an
	// unprotected count-then-save.
	if h.profileCAS == nil {
		h.serverError(w, r, "quota gate", errProfileCASNotWired)
		return
	}
	// Quota is keyed on the effective tier: a lapsed paid subscription
	// (EffectiveTier) falls back to the Free limit.
	ok, casErr := h.atomicQuotaIncrement(r.Context(), userID, profile.EffectiveTier(h.now()).WatchZoneLimit())
	if casErr != nil {
		h.serverError(w, r, "atomic quota check", casErr)
		return
	}
	if !ok {
		h.writeError(w, r, http.StatusForbidden, quotaExceededMessage)
		return
	}

	authorityID := 0
	if req.AuthorityID != nil {
		authorityID = *req.AuthorityID
	} else {
		authorityID, err = h.resolver.ResolveAuthority(r.Context(), req.Latitude, req.Longitude)
		if err != nil {
			h.serverError(w, r, "resolve authority from coordinates", err)
			return
		}
	}

	zone, err := NewWatchZone(
		h.newID(), userID, req.Name,
		req.Latitude, req.Longitude, req.RadiusMetres,
		authorityID, h.now(),
		boolOrTrue(req.PushEnabled), boolOrTrue(req.EmailInstantEnabled))
	if err != nil {
		h.serverError(w, r, "build watch zone", err)
		return
	}
	if err := h.store.Save(r.Context(), zone); err != nil {
		h.serverError(w, r, "save watch zone", err)
		return
	}
	if h.metrics != nil {
		h.metrics.WatchZoneCreated(r.Context())
	}

	// The create response carries page one of the bounded fetch; the continuation
	// token is irrelevant here (no "load more" on create), so it is discarded.
	nearby, _, err := h.apps.FindNearbyPage(
		r.Context(), req.Latitude, req.Longitude, req.RadiusMetres, maxNearbyLimit, "")
	if err != nil {
		h.serverError(w, r, "find nearby applications", err)
		return
	}

	results := make([]applications.NearbyResult, 0, len(nearby))
	for _, a := range nearby {
		results = append(results, applications.NearbyResultOf(a))
	}
	h.writeCreated(w, r, "/v1/me/watch-zones/"+zone.ID, createResult{NearbyApplications: results})
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
// its latest unread notification. When the caller has no read-watermark yet
// (first touch) the unread lookup is skipped and every row's latestUnreadEvent
// is null.
//
// Routing: a param-less call (no ?sort=) keeps the legacy nearest-first path
// (FindNearbyPage, default 500) byte-identical for the in-review iOS build (#541).
// A ?sort= call opts into the server-side sort surface (FindInZonePage, default
// 150) with a sort-aware keyset cursor (epic #682 slices 1-3; recent-activity
// joins the caller's own notifications via the threaded userID).
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

	// ?status= filters on the raw app_state; "All"/absent means no filter, any
	// other non-vocabulary value is a clean 400.
	status, statusOK := parseStatus(r.URL.Query().Get("status"))
	if !statusOK {
		h.writeError(w, r, http.StatusBadRequest, invalidStatusMessage)
		return
	}

	// decodeCursor strips the transport-layer base64 wrapping; a malformed wrapper
	// is a clean 400. The unwrapped token is the store's opaque keyset cursor.
	cursor, ok := decodeCursor(r.URL.Query().Get("cursor"))
	if !ok {
		h.writeError(w, r, http.StatusBadRequest, invalidCursorMessage)
		return
	}

	apps, nextCursor, err := h.findZonePage(r.Context(), userID, zone, sort, sortPresent, status, false, r.URL.Query().Get("limit"), cursor)
	if err != nil {
		// A stale or sort-mismatched cursor is a client error (400), not a 500: the
		// keyset cursor is only valid for the sort it was minted under.
		if errors.Is(err, applications.ErrCursorInvalid) || errors.Is(err, applications.ErrCursorSortMismatch) {
			h.writeError(w, r, http.StatusBadRequest, invalidCursorMessage)
			return
		}
		h.serverError(w, r, "find applications in zone", err)
		return
	}

	state, err := h.state.Get(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "load notification state", err)
		return
	}

	// Only classify rows as unread when the user has a watermark; first-touch
	// users get null latestUnreadEvent everywhere (the dedicated notification-state
	// seeder owns watermark creation).
	var unread map[string]notifications.LatestUnread
	if state != nil {
		uids := make([]string, 0, len(apps))
		for _, a := range apps {
			uids = append(uids, a.UID)
		}
		unread, err = h.unread.GetLatestUnreadByApplications(r.Context(), userID, uids, state.LastReadAt)
		if err != nil {
			h.serverError(w, r, "load latest unread", err)
			return
		}
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
func (h *handler) findZonePage(ctx context.Context, userID string, zone WatchZone, sort applications.Sort, sortPresent bool, status string, unread bool, rawLimit, cursor string) ([]applications.PlanningApplication, string, error) {
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

		// Lazy-init: legacy profile has no counter yet. Initialise from the
		// live zone count (once). Subsequent requests will trust the counter.
		currentCount := 0
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

		// Claim the slot.
		newCount := currentCount + 1
		updated := *profile
		updated.WatchZoneCount = &newCount

		err = h.profileCAS.UpdateZoneCountWithCAS(ctx, userID, &updated, etag)
		if err == nil {
			return true, nil // slot claimed
		}
		if errors.Is(err, platform.ErrCASPreconditionFailed) {
			// Lost the race — re-read and retry.
			continue
		}
		return false, fmt.Errorf("quota CAS replace: %w", err)
	}
	// Exhausted retries: conservative 403.
	return false, nil
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
