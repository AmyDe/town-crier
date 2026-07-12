package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/designations"
	"github.com/AmyDe/town-crier/api-go/internal/geocoding"
	"github.com/AmyDe/town-crier/api-go/internal/offercodes"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/savedapplications"
	"github.com/AmyDe/town-crier/api-go/internal/subscriptions"
)

// denyAllValidator is the validator the API runs with when Auth0 config is
// absent: every token is rejected, so authenticated routes return the
// fallback-deny 401 — exactly what the contract tests assert.
type denyAllValidator struct{}

func (denyAllValidator) ValidateToken(context.Context, string) (auth.Claims, error) {
	return auth.Claims{}, context.Canceled // any non-nil error denies
}

// staticValidator accepts every token as the given claims, letting wiring tests
// exercise the authenticated pipeline without JWKS.
type staticValidator struct{ claims auth.Claims }

func (v staticValidator) ValidateToken(context.Context, string) (auth.Claims, error) {
	return v.claims, nil
}

// ── hand-written store fakes ──────────────────────────────────────────────────
// The wiring tests drive the real router, handlers and post-auth middlewares
// against these in-memory consumer-side fakes (no datastore dependency).

// fakeProfileStore is a minimal profiles.Store with a Save/Get round-trip so the
// authenticated-pipeline test can create then read a profile through the chain.
type fakeProfileStore struct {
	mu   sync.Mutex
	byID map[string]*profiles.UserProfile
}

func newFakeProfileStore() *fakeProfileStore {
	return &fakeProfileStore{byID: map[string]*profiles.UserProfile{}}
}

func (f *fakeProfileStore) Get(_ context.Context, userID string) (*profiles.UserProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.byID[userID]
	if !ok {
		return nil, profiles.ErrNotFound
	}
	return p, nil
}

func (f *fakeProfileStore) Save(_ context.Context, p *profiles.UserProfile) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[p.UserID] = p
	return nil
}

func (f *fakeProfileStore) Delete(_ context.Context, userID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[userID]; !ok {
		return profiles.ErrNotFound
	}
	delete(f.byID, userID)
	return nil
}

func (f *fakeProfileStore) GetWithETag(_ context.Context, userID string) (*profiles.UserProfile, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.byID[userID]
	if !ok {
		return nil, "", profiles.ErrNotFound
	}
	return p, "etag-" + userID, nil
}

func (f *fakeProfileStore) UpdateZoneCountWithCAS(_ context.Context, userID string, p *profiles.UserProfile, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[userID] = p
	return nil
}

// fakeWatchZoneStore (a full watchzones.Store double) is defined in
// export_adapters_test.go and reused here as the empty-path watch-zone store.

// fakeAppStore is an applications.Store returning the empty path for every read.
type fakeAppStore struct{}

func (fakeAppStore) Upsert(context.Context, applications.PlanningApplication) error { return nil }
func (fakeAppStore) GetByAuthorityAndName(context.Context, string, string) (applications.PlanningApplication, bool, error) {
	return applications.PlanningApplication{}, false, nil
}
func (fakeAppStore) GetByUID(context.Context, string, string) (applications.PlanningApplication, bool, error) {
	return applications.PlanningApplication{}, false, nil
}
func (fakeAppStore) RecentByAuthority(context.Context, string, int) ([]applications.PlanningApplication, error) {
	return nil, nil
}
func (fakeAppStore) BreakdownByAuthority(context.Context, string) ([]applications.StateCount, error) {
	return nil, nil
}
func (fakeAppStore) FindNearbyPage(context.Context, float64, float64, float64, int, string) ([]applications.PlanningApplication, string, error) {
	return nil, "", nil
}
func (fakeAppStore) RecentNearPoint(context.Context, float64, float64, float64, int) ([]applications.PlanningApplication, error) {
	return nil, nil
}
func (fakeAppStore) FindInZonePage(context.Context, applications.InZoneQuery) ([]applications.PlanningApplication, string, error) {
	return nil, "", nil
}
func (fakeAppStore) FindClustersInZone(context.Context, applications.ClusterQuery) ([]applications.Cluster, error) {
	return nil, nil
}
func (fakeAppStore) RecentNearestTown(context.Context, string, float64, float64, float64, []applications.TownCentroid, int) ([]applications.PlanningApplication, error) {
	return nil, nil
}
func (fakeAppStore) BreakdownNearby(context.Context, string, float64, float64, float64) ([]applications.StateCount, error) {
	return nil, nil
}
func (fakeAppStore) Search(context.Context, string, string, int) ([]applications.PlanningApplication, bool, error) {
	return nil, false, nil
}

// foundAppStore is a fakeAppStore that returns one populated application for the
// Croydon area id (301) — the real id, so authorities.NewLookup().SlugForAreaID(301)
// round-trips to "croydon". Every other read stays the empty (not-found) path. It
// gives the anonymous share-page and by-slug routes a real record to render and
// serialise in the end-to-end wiring test.
type foundAppStore struct {
	fakeAppStore
	app applications.PlanningApplication
}

func (f foundAppStore) GetByAuthorityAndName(_ context.Context, authorityCode, _ string) (applications.PlanningApplication, bool, error) {
	if authorityCode == "301" {
		return f.app, true, nil
	}
	return applications.PlanningApplication{}, false, nil
}

// Search unconditionally returns the fixture application, ignoring the query/
// authority/limit args — this fake exists only to give the anonymous search
// route's end-to-end wiring test a real record to render and serialise.
func (f foundAppStore) Search(context.Context, string, string, int) ([]applications.PlanningApplication, bool, error) {
	return []applications.PlanningApplication{f.app}, false, nil
}

// FindNearbyPage unconditionally returns the fixture application, ignoring the
// lat/lng/radius/limit/cursor args — this fake exists only to give the
// anonymous near-point route's end-to-end wiring test (GH#868 Phase 2) a real
// record to render and serialise.
func (f foundAppStore) FindNearbyPage(context.Context, float64, float64, float64, int, string) ([]applications.PlanningApplication, string, error) {
	return []applications.PlanningApplication{f.app}, "", nil
}

// FindClustersInZone unconditionally returns one single-member cluster carrying
// the fixture application's identity, ignoring the query args — this fake
// exists only to give the anonymous clusters route's end-to-end wiring test
// (GH#924 Phase 1) a real record to render, serialise, and slug-enrich.
func (f foundAppStore) FindClustersInZone(context.Context, applications.ClusterQuery) ([]applications.Cluster, error) {
	return []applications.Cluster{
		{
			Latitude:     51.5,
			Longitude:    -0.1,
			Count:        1,
			StatusCounts: map[string]int{"Permitted": 1},
			Member:       &applications.PlanningApplicationID{Authority: "301", Name: f.app.Name},
		},
	}, nil
}

// fakeSavedStore is a savedapplications.Store returning the empty path.
type fakeSavedStore struct{}

func (fakeSavedStore) Save(context.Context, savedapplications.SavedApplication) error { return nil }
func (fakeSavedStore) Exists(context.Context, string, string) (bool, error)           { return false, nil }
func (fakeSavedStore) Delete(context.Context, string, string) error                   { return nil }
func (fakeSavedStore) GetByUserID(context.Context, string) ([]savedapplications.SavedApplication, error) {
	return nil, nil
}
func (fakeSavedStore) UserIDsForApplication(context.Context, string, int) ([]string, error) {
	return nil, nil
}
func (fakeSavedStore) DeleteAllByUserID(context.Context, string) error { return nil }
func (fakeSavedStore) CountsByUsers(context.Context, []string) (map[string]int, error) {
	return map[string]int{}, nil
}
func (fakeSavedStore) Count(context.Context) (int, error) { return 0, nil }

// fakeAdminStore is a profiles.AdminProfileStore returning the empty path.
type fakeAdminStore struct{}

func (fakeAdminStore) GetByEmail(context.Context, string) (*profiles.UserProfile, error) {
	return nil, profiles.ErrNotFound
}
func (fakeAdminStore) GetByOriginalTransactionID(context.Context, string) (*profiles.UserProfile, error) {
	return nil, profiles.ErrNotFound
}
func (fakeAdminStore) ByDigestDay(context.Context, time.Weekday) ([]*profiles.UserProfile, error) {
	return nil, nil
}
func (fakeAdminStore) Dormant(context.Context, time.Time) ([]*profiles.UserProfile, error) {
	return nil, nil
}
func (fakeAdminStore) LapsedPaid(context.Context, time.Time) ([]*profiles.UserProfile, error) {
	return nil, nil
}
func (fakeAdminStore) Save(context.Context, *profiles.UserProfile) error { return nil }
func (fakeAdminStore) List(context.Context, string, int, string) (profiles.Page, error) {
	return profiles.Page{}, nil
}
func (fakeAdminStore) PaidCandidates(context.Context) ([]*profiles.UserProfile, error) {
	return nil, nil
}
func (fakeAdminStore) UserStats(context.Context, time.Time) (profiles.UserStats, error) {
	return profiles.UserStats{}, nil
}

// fakeSubNotifStore is a subscriptions.Store; nothing is ever processed.
type fakeSubNotifStore struct{}

func (fakeSubNotifStore) IsProcessed(context.Context, string) (bool, error) { return false, nil }
func (fakeSubNotifStore) MarkProcessed(context.Context, string) error       { return nil }

// fakeOfferStore is an offercodes.Store returning the empty path.
type fakeOfferStore struct{}

func (fakeOfferStore) Get(context.Context, string) (offercodes.OfferCode, error) {
	return offercodes.OfferCode{}, offercodes.ErrNotFound
}
func (fakeOfferStore) Save(context.Context, offercodes.OfferCode) error { return nil }
func (fakeOfferStore) RedeemWithCAS(context.Context, string, string, time.Time) error {
	return offercodes.ErrNotFound
}
func (fakeOfferStore) RedeemedByUserID(context.Context, string) ([]offercodes.RedeemedOfferCode, error) {
	return nil, nil
}
func (fakeOfferStore) RedeemedByUsers(context.Context, []string) (map[string][]offercodes.RedeemedOfferCode, error) {
	return map[string][]offercodes.RedeemedOfferCode{}, nil
}
func (fakeOfferStore) AnonymiseRedemptionsByUserID(context.Context, string) error { return nil }
func (fakeOfferStore) List(context.Context, *string, int) ([]offercodes.ListedOfferCode, error) {
	return nil, nil
}

// testGeocodeClient and testDesignationClient point at an unroutable address; the
// deny-all wiring tests never reach the handlers, so the upstream is never
// called.
func testGeocodeClient(t *testing.T) *geocoding.Client {
	t.Helper()
	c, err := geocoding.NewClient("http://127.0.0.1:0", http.DefaultClient)
	if err != nil {
		t.Fatalf("geocoding.NewClient: %v", err)
	}
	return c
}

func testDesignationClient(t *testing.T) *designations.Client {
	t.Helper()
	c, err := designations.NewClient("http://127.0.0.1:0", http.DefaultClient)
	if err != nil {
		t.Fatalf("designations.NewClient: %v", err)
	}
	return c
}

// testGeocodeClientWith and testDesignationClientWith build clients pointing at
// a specific upstream (used in integration-style wiring tests).
func testGeocodeClientWith(t *testing.T, baseURL string, httpClient *http.Client) *geocoding.Client {
	t.Helper()
	c, err := geocoding.NewClient(baseURL, httpClient)
	if err != nil {
		t.Fatalf("geocoding.NewClient(%q): %v", baseURL, err)
	}
	return c
}

func testDesignationClientWith(t *testing.T, baseURL string, httpClient *http.Client) *designations.Client {
	t.Helper()
	c, err := designations.NewClient(baseURL, httpClient)
	if err != nil {
		t.Fatalf("designations.NewClient(%q): %v", baseURL, err)
	}
	return c
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	return newRouter(denyAllValidator{}, []string{"https://towncrierapp.uk"}, nil, profiles.NoOpAuth0Client{}, profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, nil, nil, nil, testGeocodeClient(t), testDesignationClient(t), nil, nil, "", "", nil, nil, "", nil, nil, nil, 60, 60, slog.New(slog.DiscardHandler))
}

// TestRouter_AnonymousRoutesServedWithoutToken confirms the iteration-0/1
// anonymous endpoints still serve once the auth fallback owns the chain.
func TestRouter_AnonymousRoutesServedWithoutToken(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t)
	for _, tc := range []struct {
		path       string
		wantStatus int
	}{
		{"/health", http.StatusOK},
		{"/v1/health", http.StatusOK},
		{"/v1/version-config", http.StatusOK},
		{"/v1/legal/privacy", http.StatusOK},
		{"/v1/legal/unknown", http.StatusNotFound}, // anonymous route, bodyless 404 backfilled
	} {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			rec := serveReq(t, h, http.MethodGet, tc.path, "", "")
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

// TestRouter_SharePageAndBySlugAnonymous_ByIdStaysAuthed is the end-to-end wiring
// guard for the share surface (#738 Slice 1). It boots the FULL router with a
// store that returns a real Croydon application, then proves:
//
//   - GET /a/{slug}/{ref}                       -> 200 text/html, no token
//   - GET /v1/applications/by-slug/{slug}/{ref} -> 200 application/json, no token
//   - GET /v1/applications/{authorityCode}/{ref} -> 401, no token (stays authed)
//
// The two new routes serve anonymously only if their anonymousPatterns strings
// match the registered mux patterns byte-for-byte; a drift in either pattern string
// would silently 401 the public page. The by-id contrast (denyAllValidator, no
// token) proves the by-id read did NOT accidentally become anonymous.
func TestRouter_SharePageAndBySlugAnonymous_ByIdStaysAuthed(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	app := applications.PlanningApplication{
		Name:     "23/03456/FUL",
		UID:      "croydon-23-03456-FUL",
		AreaName: "Croydon",
		AreaID:   301, // the real Croydon id, so SlugForAreaID(301) == "croydon"
		Address:  "10 Downing Street, London",
	}
	appStore := foundAppStore{app: app}
	// denyAllValidator rejects every token, so the by-id route can only pass if it
	// were (wrongly) anonymous.
	h := newRouter(denyAllValidator{}, []string{"https://towncrierapp.uk"}, nil, profiles.NoOpAuth0Client{}, profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, nil, appStore, nil, testGeocodeClient(t), testDesignationClient(t), nil, nil, "", "", nil, nil, "", nil, nil, nil, 60, 60, logger)

	// (1) Public HTML share page: anonymous, 200 text/html.
	page := serveReq(t, h, http.MethodGet, "/a/croydon/23/03456/FUL", "", "")
	if page.Code != http.StatusOK {
		t.Fatalf("GET /a/croydon/... status = %d, want 200 (anonymous); body = %s", page.Code, page.Body.String())
	}
	if ct := page.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("share page Content-Type = %q, want text/html", ct)
	}
	if !strings.Contains(page.Body.String(), "10 Downing Street, London") {
		t.Errorf("share page did not render the application address; body = %s", page.Body.String())
	}

	// (2) By-slug JSON read: anonymous, 200 application/json, carries authoritySlug.
	bySlug := serveReq(t, h, http.MethodGet, "/v1/applications/by-slug/croydon/23/03456/FUL", "", "")
	if bySlug.Code != http.StatusOK {
		t.Fatalf("GET /v1/applications/by-slug/... status = %d, want 200 (anonymous); body = %s", bySlug.Code, bySlug.Body.String())
	}
	if ct := bySlug.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("by-slug Content-Type = %q, want application/json", ct)
	}
	if !strings.Contains(bySlug.Body.String(), `"authoritySlug":"croydon"`) {
		t.Errorf("by-slug body missing authoritySlug croydon; body = %s", bySlug.Body.String())
	}

	// (3) By-id read stays authed: same application, NO token -> 401 + Bearer.
	byID := serveReq(t, h, http.MethodGet, "/v1/applications/301/23/03456/FUL", "", "")
	if byID.Code != http.StatusUnauthorized {
		t.Fatalf("GET /v1/applications/301/... status = %d, want 401 (authed, no token); body = %s", byID.Code, byID.Body.String())
	}
	if got := byID.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Errorf("by-id WWW-Authenticate = %q, want Bearer", got)
	}

	// (4) Public og:image card: anonymous, 200 image/png. This application carries
	// no coordinates, so the card takes the branded fallback path and fetches no
	// OSM tiles — the full router wires the real tile client, so a coordinate-less
	// fixture keeps this test off the network while still proving the route is
	// wired and anonymous.
	ogCard := serveReq(t, h, http.MethodGet, "/og/croydon/23/03456/FUL.png", "", "")
	if ogCard.Code != http.StatusOK {
		t.Fatalf("GET /og/croydon/... status = %d, want 200 (anonymous); body len = %d", ogCard.Code, ogCard.Body.Len())
	}
	if ct := ogCard.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("og:image Content-Type = %q, want image/png", ct)
	}
}

// TestRouter_ApplicationSearchAnonymous is the end-to-end wiring guard for the
// anonymous application search endpoint (#821 Phase 3, tc-geq7h.3): it boots the
// FULL router with a store that returns a real Croydon application, then proves
// GET /v1/applications/search serves anonymously (no token, 200 application/json)
// and its response carries the fields a client needs to build a share URL
// (authoritySlug + reference), byte-identical to what the by-slug read exposes.
// The pattern string match is a drift guard: a rename here without updating
// anonymousPatterns would silently 401 the route.
func TestRouter_ApplicationSearchAnonymous(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	app := applications.PlanningApplication{
		Name:     "23/03456/FUL",
		UID:      "croydon-23-03456-FUL",
		AreaName: "Croydon",
		AreaID:   301, // the real Croydon id, so SlugForAreaID(301) == "croydon"
		Address:  "10 Downing Street, London",
	}
	appStore := foundAppStore{app: app}
	h := newRouter(denyAllValidator{}, []string{"https://towncrierapp.uk"}, nil, profiles.NoOpAuth0Client{}, profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, nil, appStore, nil, testGeocodeClient(t), testDesignationClient(t), nil, nil, "", "", nil, nil, "", nil, nil, nil, 60, 60, logger)

	rec := serveReq(t, h, http.MethodGet, "/v1/applications/search?q=downing", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/applications/search status = %d, want 200 (anonymous); body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if !strings.Contains(rec.Body.String(), `"authoritySlug":"croydon"`) {
		t.Errorf("body missing authoritySlug croydon; body = %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"reference":"23/03456/FUL"`) {
		t.Errorf("body missing reference 23/03456/FUL (planit_name, not uid); body = %s", rec.Body.String())
	}
}

// TestRouter_NearPointAnonymous is the end-to-end wiring guard for the public
// applications-near-a-point endpoint (GH#868 Phase 2): it boots the FULL
// router with a store that returns a real application, then proves
// GET /v1/applications/near-point serves anonymously (no token, 200
// application/json) with a bare-array body — the same raw-domain wire shape
// (NearbyResult) the authed nearby page emits, plus a resolved authoritySlug
// (GH#879 Phase 1) so an anonymously-loaded application can build a share URL
// or a by-slug detail fetch. The pattern string match is a drift guard: a
// rename here without updating anonymousPatterns would silently 401 the
// route.
func TestRouter_NearPointAnonymous(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	app := applications.PlanningApplication{
		Name:     "23/03456/FUL",
		UID:      "croydon-23-03456-FUL",
		AreaName: "Croydon",
		AreaID:   301, // the real Croydon id, so SlugForAreaID(301) == "croydon"
		Address:  "10 Downing Street, London",
	}
	appStore := foundAppStore{app: app}
	h := newRouter(denyAllValidator{}, []string{"https://towncrierapp.uk"}, nil, profiles.NoOpAuth0Client{}, profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, nil, appStore, nil, testGeocodeClient(t), testDesignationClient(t), nil, nil, "", "", nil, nil, "", nil, nil, nil, 60, 60, logger)

	rec := serveReq(t, h, http.MethodGet, "/v1/applications/near-point?lat=51.5&lng=-0.1", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/applications/near-point status = %d, want 200 (anonymous); body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if !strings.Contains(rec.Body.String(), `"name":"23/03456/FUL"`) {
		t.Errorf("body missing planit_name 23/03456/FUL; body = %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"authoritySlug":"croydon"`) {
		t.Errorf("body missing authoritySlug croydon; body = %s", rec.Body.String())
	}
	if !strings.HasPrefix(strings.TrimSpace(rec.Body.String()), "[") {
		t.Errorf("body must be a bare JSON array, got: %s", rec.Body.String())
	}

	badReq := serveReq(t, h, http.MethodGet, "/v1/applications/near-point", "", "")
	if badReq.Code != http.StatusBadRequest {
		t.Fatalf("GET /v1/applications/near-point (no lat/lng) status = %d, want 400", badReq.Code)
	}
}

// GET /v1/applications/clusters serves anonymously (no token, 200
// application/json) with a bare-array body of PostGIS grid-aggregated clusters
// (GH#924 Phase 1), each member identity carrying a resolved authoritySlug so
// the anonymous map can point-read a tapped pin by slug, mirroring
// TestRouter_NearPointAnonymous. The pattern string match is a drift guard: a
// rename here without updating anonymousPatterns would silently 401 the route.
func TestRouter_ClustersAnonymous(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	app := applications.PlanningApplication{
		Name:     "23/03456/FUL",
		UID:      "croydon-23-03456-FUL",
		AreaName: "Croydon",
		AreaID:   301, // the real Croydon id, so SlugForAreaID(301) == "croydon"
		Address:  "10 Downing Street, London",
	}
	appStore := foundAppStore{app: app}
	h := newRouter(denyAllValidator{}, []string{"https://towncrierapp.uk"}, nil, profiles.NoOpAuth0Client{}, profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, nil, appStore, nil, testGeocodeClient(t), testDesignationClient(t), nil, nil, "", "", nil, nil, "", nil, nil, nil, 60, 60, logger)

	url := "/v1/applications/clusters?lat=51.5&lng=-0.1&bbox=-0.2,51.4,-0.05,51.6&zoom=14"
	rec := serveReq(t, h, http.MethodGet, url, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/applications/clusters status = %d, want 200 (anonymous); body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if !strings.Contains(rec.Body.String(), `"name":"23/03456/FUL"`) {
		t.Errorf("body missing planit_name 23/03456/FUL; body = %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"authoritySlug":"croydon"`) {
		t.Errorf("body missing authoritySlug croydon; body = %s", rec.Body.String())
	}
	if !strings.HasPrefix(strings.TrimSpace(rec.Body.String()), "[") {
		t.Errorf("body must be a bare JSON array, got: %s", rec.Body.String())
	}

	badReq := serveReq(t, h, http.MethodGet, "/v1/applications/clusters", "", "")
	if badReq.Code != http.StatusBadRequest {
		t.Fatalf("GET /v1/applications/clusters (no params) status = %d, want 400", badReq.Code)
	}
}

// TestRouter_FallbackDeny pins the 401 surface: protected routes, unmatched
// paths, the root, and non-int authority ids all return 401 with
// WWW-Authenticate: Bearer and the PascalCase envelope.
func TestRouter_FallbackDeny(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t)
	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{"api me without token", http.MethodGet, "/api/me"},
		{"me without token", http.MethodGet, "/v1/me"},
		{"root", http.MethodGet, "/"},
		{"unknown path", http.MethodGet, "/v1/nope"},
		// Authorities routes are authed (GH#418): no token -> 401.
		{"authorities list without token", http.MethodGet, "/v1/authorities"},
		{"authority by id without token", http.MethodGet, "/v1/authorities/384"},
		{"non-int authority id", http.MethodGet, "/v1/authorities/abc"},
		{"wrong method on me", http.MethodPut, "/api/me"},
		// Geocode and designations are authed, not anonymous: no token -> 401.
		{"geocode without token", http.MethodGet, "/v1/geocode/SW1A1AA"},
		{"designations without token", http.MethodGet, "/v1/designations?latitude=55&longitude=2"},
		// Offer-code redeem is authed: no token -> 401.
		{"redeem without token", http.MethodPost, "/v1/offer-codes/redeem"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := serveReq(t, h, tc.method, tc.path, "", "")
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Errorf("WWW-Authenticate = %q, want Bearer", got)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			if want := `{"Status":401,"Title":"Unauthorized","Detail":null}`; rec.Body.String() != want {
				t.Errorf("body = %s, want %s", rec.Body.String(), want)
			}
		})
	}
}

// TestRouter_CorsLayeredOnAllResponses confirms CORS is the outermost layer:
// the matched-origin header appears on a 401 just as on a 200; CORS runs
// before all other middleware.
func TestRouter_CorsLayeredOnAllResponses(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t)
	for _, path := range []string{"/v1/health", "/api/me"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			rec := serveReq(t, h, http.MethodGet, path, "https://towncrierapp.uk", "")
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://towncrierapp.uk" {
				t.Errorf("Access-Control-Allow-Origin = %q, want echoed origin", got)
			}
		})
	}
}

// TestRouter_AuthenticatedPipeline runs the full wired chain with a store: a
// valid token reaches the /v1/me handlers through rate limiting (headers set,
// rate-limit-before-handler order) and activity recording.
func TestRouter_AuthenticatedPipeline(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	store := newFakeProfileStore()
	watchZoneStore := &fakeWatchZoneStore{}
	appStore := fakeAppStore{}
	savedStore := fakeSavedStore{}
	validator := staticValidator{claims: auth.Claims{Subject: "auth0|wiretest", Email: "wire@example.com", EmailVerified: true}}
	h := newRouter(validator, []string{"https://towncrierapp.uk"}, store, profiles.NoOpAuth0Client{}, profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, watchZoneStore, appStore, savedStore, testGeocodeClient(t), testDesignationClient(t), nil, nil, "", "", nil, nil, "", nil, nil, nil, 60, 60, logger)

	// Create the profile, then read it back through the same chain.
	rec := serveReq(t, h, http.MethodPost, "/v1/me", "", "Bearer tok")
	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/me status = %d body = %s", rec.Code, rec.Body.String())
	}

	rec = serveReq(t, h, http.MethodGet, "/v1/me", "", "Bearer tok")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/me status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"userId":"auth0|wiretest"`) {
		t.Errorf("GET /v1/me body missing userId: %s", rec.Body.String())
	}
	if got := rec.Header().Get("X-RateLimit-Limit"); got != "60" {
		t.Errorf("X-RateLimit-Limit = %q, want 60 (free tier)", got)
	}
	if rec.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("X-RateLimit-Remaining missing — rate limiter not in the dispatch path")
	}

	// Watch-zone routes are wired behind the same auth + dispatch chain: a
	// valid token reaches the list handler and gets the empty-array body.
	rec = serveReq(t, h, http.MethodGet, "/v1/me/watch-zones", "", "Bearer tok")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/me/watch-zones status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"zones":[]}` {
		t.Errorf("GET /v1/me/watch-zones body = %s, want {\"zones\":[]}", got)
	}

	// Saved-application routes are wired behind the same chain (empty list array).
	rec = serveReq(t, h, http.MethodGet, "/v1/me/saved-applications", "", "Bearer tok")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/me/saved-applications status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `[]` {
		t.Errorf("GET /v1/me/saved-applications body = %s, want []", got)
	}

	// application-authorities is wired off the watch-zone store (empty set).
	rec = serveReq(t, h, http.MethodGet, "/v1/me/application-authorities", "", "Bearer tok")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/me/application-authorities status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"authorities":[],"count":0}` {
		t.Errorf("GET /v1/me/application-authorities body = %s", got)
	}

	// Authorities routes are authed (GH#418): a valid token reaches the handlers,
	// which are backed by embedded reference data (independent of the stores).
	rec = serveReq(t, h, http.MethodGet, "/v1/authorities", "", "Bearer tok")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/authorities status = %d body = %s", rec.Code, rec.Body.String())
	}
	rec = serveReq(t, h, http.MethodGet, "/v1/authorities/384", "", "Bearer tok")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/authorities/384 status = %d body = %s", rec.Code, rec.Body.String())
	}
	rec = serveReq(t, h, http.MethodGet, "/v1/authorities/99999999", "", "Bearer tok")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /v1/authorities/99999999 status = %d body = %s", rec.Code, rec.Body.String())
	}

	// Anonymous routes stay unmetered even on the store-wired router.
	rec = serveReq(t, h, http.MethodGet, "/v1/health", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/health status = %d", rec.Code)
	}
	if got := rec.Header().Get("X-RateLimit-Limit"); got != "" {
		t.Errorf("anonymous route got rate-limit header %q", got)
	}
}

// TestRouter_GeocodeAndDesignationsDispatch proves an authenticated request
// reaches the geocode and designation handlers (not just the auth fallback). A
// single stub upstream backs both outbound clients, routing by path.
func TestRouter_GeocodeAndDesignationsDispatch(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/postcodes/"):
			_, _ = w.Write([]byte(`{"status":200,"result":{"latitude":51.5,"longitude":-0.14}}`))
		case r.URL.Path == "/api/v1/entity.json":
			// No intersecting entity -> the none context.
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(upstream.Close)

	logger := slog.New(slog.DiscardHandler)
	validator := staticValidator{claims: auth.Claims{Subject: "auth0|wiretest", Email: "wire@example.com", EmailVerified: true}}
	geocodeClient := testGeocodeClientWith(t, upstream.URL, upstream.Client())
	designationClient := testDesignationClientWith(t, upstream.URL, upstream.Client())
	h := newRouter(validator, []string{"https://towncrierapp.uk"}, nil, profiles.NoOpAuth0Client{}, profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, nil, nil, nil, geocodeClient, designationClient, nil, nil, "", "", nil, nil, "", nil, nil, nil, 60, 60, logger)

	rec := serveReq(t, h, http.MethodGet, "/v1/geocode/SW1A%201AA", "", "Bearer tok")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/geocode status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != `{"coordinates":{"latitude":51.5,"longitude":-0.14}}` {
		t.Errorf("geocode body = %s", got)
	}

	rec = serveReq(t, h, http.MethodGet, "/v1/designations?latitude=55&longitude=2", "", "Bearer tok")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/designations status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != `{"isWithinConservationArea":false,"conservationAreaName":null,"isWithinListedBuildingCurtilage":false,"listedBuildingGrade":null,"isWithinArticle4Area":false}` {
		t.Errorf("designations body = %s", got)
	}
}

// TestRouter_SubscriptionsWired confirms the verify endpoint is authed and the
// App Store webhook is anonymous to Auth0 (the signed JWS is its auth). A
// deny-all validator is used: the webhook still reaches its handler (a malformed
// body returns 400 malformed_request), while verify falls to the 401 fallback.
func TestRouter_SubscriptionsWired(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	store := newFakeProfileStore()
	adminStore := fakeAdminStore{}
	notifStore := fakeSubNotifStore{}
	roots, err := subscriptions.LoadAppleRootCertificates()
	if err != nil {
		t.Fatalf("LoadAppleRootCertificates: %v", err)
	}
	verifier, err := subscriptions.NewJWSVerifier(roots, time.Now)
	if err != nil {
		t.Fatalf("NewJWSVerifier: %v", err)
	}

	h := newRouter(denyAllValidator{}, []string{"https://towncrierapp.uk"}, store, profiles.NoOpAuth0Client{}, profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, nil, nil, nil, testGeocodeClient(t), testDesignationClient(t), nil, adminStore, "", "", verifier, notifStore, "uk.towncrierapp.mobile", []string{"Production"}, nil, nil, 60, 60, logger)

	// Webhook is anonymous: a malformed body reaches the handler -> 400 with the
	// malformed_request envelope, not the WWW-Authenticate 401 fallback.
	rec := serveReq(t, h, http.MethodPost, "/v1/webhooks/appstore", "{not json", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("webhook status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "malformed_request") {
		t.Errorf("webhook body = %s, want malformed_request", rec.Body.String())
	}

	// Verify is authed: with no token the deny-all fallback returns 401.
	rec = serveReq(t, h, http.MethodPost, "/v1/subscriptions/verify", `{"signedTransaction":"x"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("verify status = %d, want 401", rec.Code)
	}
}

// TestRouter_AdminGate confirms the admin routes are wired, anonymous to Auth0,
// and gated solely by the X-Admin-Key. A deny-all validator is used: if the
// routes were behind the Auth0 fallback they would 401 with WWW-Authenticate:
// Bearer; the admin gate's 401 carries no such header, and a correct key reaches
// the handler.
func TestRouter_AdminGate(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	offerStore := fakeOfferStore{}
	adminStore := fakeAdminStore{}
	h := newRouter(denyAllValidator{}, []string{"https://towncrierapp.uk"}, nil, profiles.NoOpAuth0Client{}, profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, nil, nil, nil, testGeocodeClient(t), testDesignationClient(t), offerStore, adminStore, "s3cret", "", nil, nil, "", nil, nil, nil, 60, 60, logger)

	// No key: the admin gate rejects with a bodyless 401 and NO WWW-Authenticate
	// (distinguishing it from the Auth0 fallback-deny).
	noKey := adminRequest(t, h, "")
	if noKey.Code != http.StatusUnauthorized {
		t.Fatalf("no key: status = %d, want 401", noKey.Code)
	}
	if got := noKey.Header().Get("WWW-Authenticate"); got != "" {
		t.Errorf("no key: WWW-Authenticate = %q, want empty (admin gate, not Auth0)", got)
	}

	// Correct key: the request reaches the list handler and returns the empty page.
	withKey := adminRequest(t, h, "s3cret")
	if withKey.Code != http.StatusOK {
		t.Fatalf("with key: status = %d body = %s", withKey.Code, withKey.Body.String())
	}
	if got := withKey.Body.String(); got != `{"items":[],"continuationToken":null}` {
		t.Errorf("with key: body = %s", got)
	}
}

// TestRouter_AdminOfferCodesListGate confirms GET /v1/admin/offer-codes is
// wired into the admin router and gated solely by the X-Admin-Key, the same
// as every other admin route.
func TestRouter_AdminOfferCodesListGate(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	offerStore := fakeOfferStore{}
	adminStore := fakeAdminStore{}
	h := newRouter(denyAllValidator{}, []string{"https://towncrierapp.uk"}, nil, profiles.NoOpAuth0Client{}, profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, nil, nil, nil, testGeocodeClient(t), testDesignationClient(t), offerStore, adminStore, "s3cret", "", nil, nil, "", nil, nil, nil, 60, 60, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/admin/offer-codes", nil)
	req.Header.Set("X-Admin-Key", "s3cret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "[]" {
		t.Errorf("body = %s, want empty list", got)
	}
}

// TestRouter_AdminStatsGate confirms GET /v1/admin/stats is wired into the admin
// router, gated by the same X-Admin-Key, and serves the pinned contract even
// when the saved/device/notif reach stores are unwired (nil) — the reach block
// falls back to zeros rather than 500ing.
func TestRouter_AdminStatsGate(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	offerStore := fakeOfferStore{}
	adminStore := fakeAdminStore{}
	// notif (pos 9), saved (pos 12), device (pos 7) all nil: the reach stores are
	// unwired, exercising the nil-safe reach path through the full router chain.
	h := newRouter(denyAllValidator{}, []string{"https://towncrierapp.uk"}, nil, profiles.NoOpAuth0Client{}, profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, nil, nil, nil, testGeocodeClient(t), testDesignationClient(t), offerStore, adminStore, "s3cret", "", nil, nil, "", nil, nil, nil, 60, 60, logger)

	statsReq := func(key string) *httptest.ResponseRecorder {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/admin/stats", nil)
		if key != "" {
			req.Header.Set("X-Admin-Key", key)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	// No key: bodyless 401 with no WWW-Authenticate (admin gate, not Auth0).
	noKey := statsReq("")
	if noKey.Code != http.StatusUnauthorized {
		t.Fatalf("no key: status = %d, want 401", noKey.Code)
	}
	if got := noKey.Header().Get("WWW-Authenticate"); got != "" {
		t.Errorf("no key: WWW-Authenticate = %q, want empty (admin gate)", got)
	}

	// Correct key: reaches the stats handler; the empty fakeAdminStore yields the
	// all-zero contract with a null mostRecent.
	withKey := statsReq("s3cret")
	if withKey.Code != http.StatusOK {
		t.Fatalf("with key: status = %d body = %s", withKey.Code, withKey.Body.String())
	}
	want := `{` +
		`"users":{"total":0,"byTier":{"Free":0,"Personal":0,"Pro":0}},` +
		`"paying":{"effectivePaid":0,"appStore":0,"comped":0,"lapsed":0,"inGrace":0,"appStoreByTier":{"Personal":0,"Pro":0}},` +
		`"signups":{"last24h":0,"last7d":0,"last30d":0,"mostRecent":null},` +
		`"activity":{"active24h":0,"active7d":0,"zeroWatchZones":0,"noEmail":0},` +
		`"reach":{"watchZones":0,"savedApplications":0,"deviceRegistrations":0,"notificationsSent":0,"notificationsUnread":0}` +
		`}`
	if got := withKey.Body.String(); got != want {
		t.Errorf("with key: body =\n  %s\nwant\n  %s", got, want)
	}
}

// TestRouter_RecentApplicationsBuildKeyGate confirms the build-time SEO endpoint
// is wired, anonymous to Auth0, and gated solely by the X-Build-Key. A deny-all
// validator is used: if the route were behind the Auth0 fallback it would 401
// with WWW-Authenticate: Bearer; the build gate's 401 carries no such header, and
// a correct key reaches the handler.
func TestRouter_RecentApplicationsBuildKeyGate(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	appStore := fakeAppStore{}
	h := newRouter(denyAllValidator{}, []string{"https://towncrierapp.uk"}, nil, profiles.NoOpAuth0Client{}, profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, nil, appStore, nil, testGeocodeClient(t), testDesignationClient(t), nil, nil, "", "buildsecret", nil, nil, "", nil, nil, nil, 60, 60, logger)

	// No key: the build gate rejects with a bodyless 401 and NO WWW-Authenticate
	// (distinguishing it from the Auth0 fallback-deny).
	noKey := recentRequest(t, h, "")
	if noKey.Code != http.StatusUnauthorized {
		t.Fatalf("no key: status = %d, want 401", noKey.Code)
	}
	if got := noKey.Header().Get("WWW-Authenticate"); got != "" {
		t.Errorf("no key: WWW-Authenticate = %q, want empty (build gate, not Auth0)", got)
	}

	// Correct key: the request reaches the handler and returns a non-null
	// applications array (the fake store yields no documents).
	withKey := recentRequest(t, h, "buildsecret")
	if withKey.Code != http.StatusOK {
		t.Fatalf("with key: status = %d body = %s", withKey.Code, withKey.Body.String())
	}
	if got := withKey.Body.String(); !strings.Contains(got, `"applications":[]`) {
		t.Errorf("with key: body = %s, want a non-null applications array", got)
	}
}

func recentRequest(t *testing.T, h http.Handler, key string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/authorities/471/applications", nil)
	if key != "" {
		req.Header.Set("X-Build-Key", key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestRouter_NearApplicationsBuildKeyGate confirms the build-time town-level SEO
// endpoint (GET /v1/applications/near) is wired, anonymous to Auth0, and gated
// solely by the X-Build-Key. The deny-all validator distinguishes the gates: the
// Auth0 fallback would 401 with WWW-Authenticate: Bearer, whereas the build gate's
// 401 carries no such header, and a correct key reaches the handler.
func TestRouter_NearApplicationsBuildKeyGate(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	appStore := fakeAppStore{}
	h := newRouter(denyAllValidator{}, []string{"https://towncrierapp.uk"}, nil, profiles.NoOpAuth0Client{}, profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, nil, appStore, nil, testGeocodeClient(t), testDesignationClient(t), nil, nil, "", "buildsecret", nil, nil, "", nil, nil, nil, 60, 60, logger)

	noKey := nearRequest(t, h, "")
	if noKey.Code != http.StatusUnauthorized {
		t.Fatalf("no key: status = %d, want 401", noKey.Code)
	}
	if got := noKey.Header().Get("WWW-Authenticate"); got != "" {
		t.Errorf("no key: WWW-Authenticate = %q, want empty (build gate, not Auth0)", got)
	}

	withKey := nearRequest(t, h, "buildsecret")
	if withKey.Code != http.StatusOK {
		t.Fatalf("with key: status = %d body = %s", withKey.Code, withKey.Body.String())
	}
	if got := withKey.Body.String(); !strings.Contains(got, `"applications":[]`) {
		t.Errorf("with key: body = %s, want a non-null applications array", got)
	}
}

func nearRequest(t *testing.T, h http.Handler, key string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1", nil)
	if key != "" {
		req.Header.Set("X-Build-Key", key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func adminRequest(t *testing.T, h http.Handler, key string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/admin/users", nil)
	if key != "" {
		req.Header.Set("X-Admin-Key", key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func serveReq(t *testing.T, h http.Handler, method, path, origin, authz string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req := httptest.NewRequestWithContext(ctx, method, path, nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
