package demoaccount

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// The fixed demo identity and zone geometry. The reviewer account is keyed on a
// synthetic Auth0 subject; the zone centres on Westminster with a 2 km radius.
const (
	demoUserID       = "demo|apple-reviewer"
	demoZoneID       = "demo-zone"
	demoZoneName     = "Westminster Demo Zone"
	demoLatitude     = 51.4975
	demoLongitude    = -0.1357
	demoRadiusMetres = 2000
	// demoSubscriptionYears is how far ahead the demo Pro subscription is set to
	// expire (10 years from the seed call time).
	demoSubscriptionYears = 10
)

// profileStore is the consumer-side slice of the profile store the demo handler
// needs: load the demo profile and persist a freshly seeded one.
type profileStore interface {
	Get(ctx context.Context, userID string) (*profiles.UserProfile, error)
	Save(ctx context.Context, p *profiles.UserProfile) error
}

// zoneStore persists the seeded demo watch zone.
type zoneStore interface {
	Save(ctx context.Context, z watchzones.WatchZone) error
}

// appStore seeds the demo applications and runs the spatial lookup that backs
// the response.
type appStore interface {
	Upsert(ctx context.Context, a applications.PlanningApplication) error
	FindNearby(ctx context.Context, authorityCode string, latitude, longitude, radiusMetres float64) ([]applications.PlanningApplication, error)
}

// handler serves GET /v1/demo-account.
type handler struct {
	profiles profileStore
	zones    zoneStore
	apps     appStore
	now      func() time.Time
	logger   *slog.Logger
}

// Routes registers the anonymous demo-account endpoint. The route is added to
// the wiring's anonymousPatterns so it bypasses the Auth0 bearer requirement.
func Routes(mux *http.ServeMux, profiles profileStore, zones zoneStore, apps appStore, now func() time.Time, logger *slog.Logger) {
	h := &handler{profiles: profiles, zones: zones, apps: apps, now: now, logger: logger}
	mux.HandleFunc("GET /v1/demo-account", h.getDemoAccount)
}

// getDemoAccount returns the demo account, seeding Cosmos on first call. The
// seed is gated on the profile's absence, so repeated calls are idempotent —
// they skip straight to the spatial lookup.
func (h *handler) getDemoAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	profile, err := h.profiles.Get(ctx, demoUserID)
	switch {
	case errors.Is(err, profiles.ErrNotFound):
		profile, err = h.seed(ctx)
		if err != nil {
			serverError(w, r, h.logger, "seed demo account", err)
			return
		}
	case err != nil:
		serverError(w, r, h.logger, "load demo profile", err)
		return
	}

	authorityCode := strconv.Itoa(seedAuthorityID)
	apps, err := h.apps.FindNearby(ctx, authorityCode, demoLatitude, demoLongitude, demoRadiusMetres)
	if err != nil {
		serverError(w, r, h.logger, "find nearby demo applications", err)
		return
	}

	appResults := make([]demoApplicationResult, 0, len(apps))
	for _, a := range apps {
		appResults = append(appResults, applicationResultOf(a))
	}

	writeJSON(w, r, h.logger, demoAccountResult{
		UserID: demoUserID,
		Tier:   profile.Tier.String(),
		WatchZone: demoWatchZoneResult{
			ZoneID:        demoZoneID,
			AuthorityName: seedAuthorityName,
			Latitude:      demoLatitude,
			Longitude:     demoLongitude,
			RadiusMetres:  demoRadiusMetres,
		},
		Applications: appResults,
	})
}

// seed provisions the demo profile (Pro, 10-year expiry), the Westminster watch
// zone, and the five fixed applications, returning the saved profile. It runs
// only when the profile is absent; CreatedAt is set to Go's zero time.
func (h *handler) seed(ctx context.Context) (*profiles.UserProfile, error) {
	now := h.now()

	profile, err := profiles.NewProfile(demoUserID, "", now)
	if err != nil {
		return nil, fmt.Errorf("register demo profile: %w", err)
	}
	profile.ActivateSubscription(profiles.TierPro, now.AddDate(demoSubscriptionYears, 0, 0))
	if err := h.profiles.Save(ctx, profile); err != nil {
		return nil, fmt.Errorf("save demo profile: %w", err)
	}

	zone, err := watchzones.NewWatchZone(
		demoZoneID, demoUserID, demoZoneName,
		demoLatitude, demoLongitude, demoRadiusMetres,
		seedAuthorityID, time.Time{}, true, true)
	if err != nil {
		return nil, fmt.Errorf("build demo watch zone: %w", err)
	}
	if err := h.zones.Save(ctx, zone); err != nil {
		return nil, fmt.Errorf("save demo watch zone: %w", err)
	}

	for _, a := range seedApplications(now) {
		if err := h.apps.Upsert(ctx, a); err != nil {
			return nil, fmt.Errorf("seed demo application %q: %w", a.Name, err)
		}
	}

	return profile, nil
}
