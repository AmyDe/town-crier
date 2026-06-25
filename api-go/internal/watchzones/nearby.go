package watchzones

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
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
// (tc-zldl). *applications.CosmosStore satisfies it.
type appFinder interface {
	FindNearby(ctx context.Context, latitude, longitude, radiusMetres float64) ([]applications.PlanningApplication, error)
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

	nearby, err := h.apps.FindNearby(
		r.Context(), req.Latitude, req.Longitude, req.RadiusMetres)
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

	apps, err := h.apps.FindNearby(
		r.Context(), zone.Latitude, zone.Longitude, zone.RadiusMetres)
	if err != nil {
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
	h.writeJSON(w, r, http.StatusOK, results)
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
