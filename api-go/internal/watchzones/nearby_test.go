package watchzones

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/notificationstate"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

var nearbyNow = time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC)

type fakeProfileReader struct {
	profile *profiles.UserProfile
	err     error
}

func (f *fakeProfileReader) Get(_ context.Context, _ string) (*profiles.UserProfile, error) {
	return f.profile, f.err
}

type fakeResolver struct {
	id     int
	err    error
	called bool
}

func (f *fakeResolver) ResolveAuthority(_ context.Context, _, _ float64) (int, error) {
	f.called = true
	return f.id, f.err
}

type fakeAppFinder struct {
	apps         []applications.PlanningApplication
	next         string
	err          error
	called       bool
	inZoneCalled bool
	inZoneErr    error
	lastSort     applications.Sort
	lastLimit    int
	lastCursor   string
	lastUserID   string
	lastStatus   string
	lastUnread   bool

	clusters         []applications.Cluster
	clusterErr       error
	clustersCalled   bool
	lastClusterQuery applications.ClusterQuery
}

func (f *fakeAppFinder) FindNearbyPage(_ context.Context, _, _, _ float64, limit int, cursor string) ([]applications.PlanningApplication, string, error) {
	f.called = true
	f.lastLimit = limit
	f.lastCursor = cursor
	if f.err != nil {
		return nil, "", f.err
	}
	// Mirror the bounded store contract: never hand back more than `limit` rows.
	// The production store caps at the query layer (the page-size hint); the fake
	// caps here so handler tests can prove the downstream unread lookup receives a
	// bounded UID set (tc-fm8f).
	apps := f.apps
	if limit > 0 && len(apps) > limit {
		apps = apps[:limit]
	}
	return apps, f.next, nil
}

func (f *fakeAppFinder) FindInZonePage(_ context.Context, q applications.InZoneQuery) ([]applications.PlanningApplication, string, error) {
	f.inZoneCalled = true
	f.lastUserID = q.UserID
	f.lastSort = q.Sort
	f.lastLimit = q.Limit
	f.lastCursor = q.Cursor
	f.lastStatus = q.Status
	f.lastUnread = q.Unread
	if f.inZoneErr != nil {
		return nil, "", f.inZoneErr
	}
	apps := f.apps
	if q.Limit > 0 && len(apps) > q.Limit {
		apps = apps[:q.Limit]
	}
	return apps, f.next, nil
}

func (f *fakeAppFinder) FindClustersInZone(_ context.Context, q applications.ClusterQuery) ([]applications.Cluster, error) {
	f.clustersCalled = true
	f.lastClusterQuery = q
	if f.clusterErr != nil {
		return nil, f.clusterErr
	}
	return f.clusters, nil
}

type fakeWatermark struct {
	state *notificationstate.State
	err   error
}

func (f *fakeWatermark) Get(_ context.Context, _ string) (*notificationstate.State, error) {
	return f.state, f.err
}

type fakeUnread struct {
	result   map[string]notifications.LatestUnread
	err      error
	called   bool
	lastUIDs []string
}

func (f *fakeUnread) GetLatestUnreadByApplications(_ context.Context, _ string, uids []string) (map[string]notifications.LatestUnread, error) {
	f.called = true
	f.lastUIDs = uids
	return f.result, f.err
}

type nearbyDeps struct {
	store    *fakeZoneStore
	profiles *fakeProfileReader
	resolver *fakeResolver
	apps     *fakeAppFinder
	state    *fakeWatermark
	unread   *fakeUnread
}

func newNearbyMux(t *testing.T, d nearbyDeps) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	// The CAS gate is the only create path, so every create test wires a CAS
	// fake seeded from the same profile the reader returns. With a legacy (nil
	// counter) profile the gate lazy-inits from the live fakeZoneStore count, so
	// the existing quota assertions (Free at limit -> 403, Pro -> unlimited)
	// continue to hold.
	cas := newFakeProfileCAS(d.profiles.profile)
	NearbyRoutes(mux, d.store, d.profiles, d.resolver, d.apps, d.state, d.unread,
		func() string { return "zone-123" }, func() time.Time { return nearbyNow },
		slog.New(slog.DiscardHandler), WithProfileCAS(cas))
	return mux
}

func proProfile(t *testing.T) *profiles.UserProfile {
	t.Helper()
	p, err := profiles.NewProfile(testUser, "", nearbyNow)
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	p.Tier = profiles.TierPro
	return p
}

func freeProfile(t *testing.T) *profiles.UserProfile {
	t.Helper()
	p, err := profiles.NewProfile(testUser, "", nearbyNow)
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	return p
}

func testApp(uid, name string) applications.PlanningApplication {
	appType := "Full"
	appState := "Permitted"
	return applications.PlanningApplication{
		Name:          name,
		UID:           uid,
		AreaName:      "City of London",
		AreaID:        471,
		Address:       "1 Test St",
		Description:   "An extension",
		AppType:       &appType,
		AppState:      &appState,
		LastDifferent: nearbyNow,
	}
}

// testAppInAuthority builds a nearby application tagged to a specific authority,
// for asserting the browse path surfaces neighbour-authority apps across a
// border (tc-zldl).
func testAppInAuthority(uid, name string, areaID int) applications.PlanningApplication {
	a := testApp(uid, name)
	a.AreaID = areaID
	a.AreaName = fmt.Sprintf("Authority %d", areaID)
	return a
}

func TestCreate_PersistsZoneAndReturnsNearbyApplications(t *testing.T) {
	t.Parallel()
	// A border-spanning zone (pinned to authority 471) whose circle also covers a
	// neighbour authority (246). The browse path is now authority-agnostic, so the
	// create response must surface the neighbour's app too (tc-zldl).
	d := nearbyDeps{
		store:    &fakeZoneStore{},
		profiles: &fakeProfileReader{profile: proProfile(t)},
		resolver: &fakeResolver{},
		apps: &fakeAppFinder{apps: []applications.PlanningApplication{
			testAppInAuthority("uid-1", "24/001", 471),
			testAppInAuthority("uid-2", "24/002", 246),
		}},
		state:  &fakeWatermark{},
		unread: &fakeUnread{},
	}
	mux := newNearbyMux(t, d)

	body := `{"name":"My Zone","latitude":51.5,"longitude":-0.12,"radiusMetres":1000,"authorityId":471}`
	rec := doReq(t, mux, http.MethodPost, "/v1/me/watch-zones", body)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/v1/me/watch-zones/zone-123" {
		t.Errorf("Location: got %q", loc)
	}
	if d.store.saved == nil || d.store.saved.AuthorityID != 471 || d.store.saved.ID != "zone-123" {
		t.Errorf("saved zone: got %+v", d.store.saved)
	}
	if !d.store.saved.PushEnabled || !d.store.saved.EmailInstantEnabled {
		t.Errorf("flags should default true: got push=%v email=%v", d.store.saved.PushEnabled, d.store.saved.EmailInstantEnabled)
	}
	if d.resolver.called {
		t.Error("resolver must not run when authorityId is supplied")
	}
	if !d.apps.called {
		t.Error("FindNearby must run to populate the create response")
	}
	var got struct {
		NearbyApplications []struct {
			UID               string          `json:"uid"`
			LatestUnreadEvent json.RawMessage `json:"latestUnreadEvent"`
		} `json:"nearbyApplications"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v; raw=%s", err, rec.Body.String())
	}
	gotUIDs := map[string]bool{}
	for _, a := range got.NearbyApplications {
		gotUIDs[a.UID] = true
	}
	if len(got.NearbyApplications) != 2 || !gotUIDs["uid-1"] || !gotUIDs["uid-2"] {
		t.Fatalf("nearbyApplications must surface both home and neighbour authority apps: got %+v", got.NearbyApplications)
	}
	// The create response carries the raw-domain shape — no latestUnreadEvent key.
	if got.NearbyApplications[0].LatestUnreadEvent != nil {
		t.Errorf("create response must not include latestUnreadEvent")
	}
}

func TestCreate_ResolvesAuthorityWhenAbsent(t *testing.T) {
	t.Parallel()
	d := nearbyDeps{
		store:    &fakeZoneStore{},
		profiles: &fakeProfileReader{profile: proProfile(t)},
		resolver: &fakeResolver{id: 326},
		apps:     &fakeAppFinder{},
		state:    &fakeWatermark{},
		unread:   &fakeUnread{},
	}
	mux := newNearbyMux(t, d)

	body := `{"name":"My Zone","latitude":51.5,"longitude":-0.12,"radiusMetres":1000}`
	rec := doReq(t, mux, http.MethodPost, "/v1/me/watch-zones", body)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if !d.resolver.called {
		t.Error("resolver must run when authorityId is absent")
	}
	if d.store.saved == nil || d.store.saved.AuthorityID != 326 {
		t.Errorf("saved zone authority: got %+v", d.store.saved)
	}
}

func TestCreate_QuotaExceededIs403(t *testing.T) {
	t.Parallel()
	// Free tier limit is 1; one existing zone means the next create is forbidden.
	d := nearbyDeps{
		store:    &fakeZoneStore{zones: []WatchZone{authorityZone(t, 471)}},
		profiles: &fakeProfileReader{profile: freeProfile(t)},
		resolver: &fakeResolver{},
		apps:     &fakeAppFinder{},
		state:    &fakeWatermark{},
		unread:   &fakeUnread{},
	}
	mux := newNearbyMux(t, d)

	body := `{"name":"My Zone","latitude":51.5,"longitude":-0.12,"radiusMetres":1000,"authorityId":471}`
	rec := doReq(t, mux, http.MethodPost, "/v1/me/watch-zones", body)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want 403", rec.Code)
	}
	if d.store.saved != nil {
		t.Error("must not save a zone when quota is exceeded")
	}
	var env apiErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if env.Error != quotaExceededMessage {
		t.Errorf("error message: got %q", env.Error)
	}
}

func TestCreate_ProTierBypassesQuota(t *testing.T) {
	t.Parallel()
	manyZones := make([]WatchZone, 10)
	for i := range manyZones {
		manyZones[i] = authorityZone(t, 471)
	}
	d := nearbyDeps{
		store:    &fakeZoneStore{zones: manyZones},
		profiles: &fakeProfileReader{profile: proProfile(t)},
		resolver: &fakeResolver{},
		apps:     &fakeAppFinder{},
		state:    &fakeWatermark{},
		unread:   &fakeUnread{},
	}
	mux := newNearbyMux(t, d)

	body := `{"name":"My Zone","latitude":51.5,"longitude":-0.12,"radiusMetres":1000,"authorityId":471}`
	rec := doReq(t, mux, http.MethodPost, "/v1/me/watch-zones", body)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Pro tier should bypass quota: got %d", rec.Code)
	}
}

func TestCreate_ExpiredProTierQuotaIs403(t *testing.T) {
	t.Parallel()
	// A Pro tier whose subscription has lapsed (past expiry, no grace) reads as
	// Free via EffectiveTier, so the Free limit of 1 applies — a user already over
	// that limit is forbidden a new zone.
	lapsed := proProfile(t)
	past := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) // before nearbyNow (2026-06-14)
	lapsed.SubscriptionExpiry = &past

	manyZones := make([]WatchZone, 10)
	for i := range manyZones {
		manyZones[i] = authorityZone(t, 471)
	}
	d := nearbyDeps{
		store:    &fakeZoneStore{zones: manyZones},
		profiles: &fakeProfileReader{profile: lapsed},
		resolver: &fakeResolver{},
		apps:     &fakeAppFinder{},
		state:    &fakeWatermark{},
		unread:   &fakeUnread{},
	}
	mux := newNearbyMux(t, d)

	body := `{"name":"My Zone","latitude":51.5,"longitude":-0.12,"radiusMetres":1000,"authorityId":471}`
	rec := doReq(t, mux, http.MethodPost, "/v1/me/watch-zones", body)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("lapsed Pro tier should fall back to the Free quota: got %d, want 403", rec.Code)
	}
	if d.store.saved != nil {
		t.Error("must not save a zone when the effective quota is exceeded")
	}
}

func TestCreate_InvalidPayloadIs400(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"blank name":       `{"name":"  ","latitude":51.5,"longitude":-0.12,"radiusMetres":1000,"authorityId":471}`,
		"zero radius":      `{"name":"Z","latitude":51.5,"longitude":-0.12,"radiusMetres":0,"authorityId":471}`,
		"lat out of range": `{"name":"Z","latitude":91,"longitude":-0.12,"radiusMetres":1000,"authorityId":471}`,
		"lng out of range": `{"name":"Z","latitude":51.5,"longitude":-181,"radiusMetres":1000,"authorityId":471}`,
		"authority <= 0":   `{"name":"Z","latitude":51.5,"longitude":-0.12,"radiusMetres":1000,"authorityId":0}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d := nearbyDeps{
				store:    &fakeZoneStore{},
				profiles: &fakeProfileReader{profile: proProfile(t)},
				resolver: &fakeResolver{}, apps: &fakeAppFinder{}, state: &fakeWatermark{}, unread: &fakeUnread{},
			}
			mux := newNearbyMux(t, d)
			rec := doReq(t, mux, http.MethodPost, "/v1/me/watch-zones", body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d, want 400", rec.Code)
			}
			if d.store.saved != nil {
				t.Error("must not save on invalid payload")
			}
		})
	}
}

func TestCreate_MissingProfileIs500(t *testing.T) {
	t.Parallel()
	d := nearbyDeps{
		store:    &fakeZoneStore{},
		profiles: &fakeProfileReader{profile: nil},
		resolver: &fakeResolver{}, apps: &fakeAppFinder{}, state: &fakeWatermark{}, unread: &fakeUnread{},
	}
	mux := newNearbyMux(t, d)
	body := `{"name":"Z","latitude":51.5,"longitude":-0.12,"radiusMetres":1000,"authorityId":471}`
	rec := doReq(t, mux, http.MethodPost, "/v1/me/watch-zones", body)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
}

func TestCreate_ResolverErrorIs500(t *testing.T) {
	t.Parallel()
	d := nearbyDeps{
		store:    &fakeZoneStore{},
		profiles: &fakeProfileReader{profile: proProfile(t)},
		resolver: &fakeResolver{err: errors.New("upstream down")},
		apps:     &fakeAppFinder{}, state: &fakeWatermark{}, unread: &fakeUnread{},
	}
	mux := newNearbyMux(t, d)
	body := `{"name":"Z","latitude":51.5,"longitude":-0.12,"radiusMetres":1000}`
	rec := doReq(t, mux, http.MethodPost, "/v1/me/watch-zones", body)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
}

func TestApplications_AugmentsUnreadAndNullsTheRest(t *testing.T) {
	t.Parallel()
	decision := "Permitted"
	unreadAt := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	d := nearbyDeps{
		store: &fakeZoneStore{zones: []WatchZone{mustZone(t, "zone-1", 471)}},
		apps: &fakeAppFinder{apps: []applications.PlanningApplication{
			testApp("uid-1", "24/001"), testApp("uid-2", "24/002"),
		}},
		profiles: &fakeProfileReader{},
		resolver: &fakeResolver{},
		state:    &fakeWatermark{state: &notificationstate.State{UserID: testUser, LastReadAt: time.Unix(0, 0), Version: 1}},
		unread: &fakeUnread{result: map[string]notifications.LatestUnread{
			"uid-1": {ApplicationUID: "uid-1", EventType: notifications.EventDecisionUpdate, Decision: &decision, CreatedAt: unreadAt},
		}},
	}
	mux := newNearbyMux(t, d)

	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var rows []struct {
		UID               string `json:"uid"`
		LatestUnreadEvent *struct {
			Type      string  `json:"type"`
			Decision  *string `json:"decision"`
			CreatedAt string  `json:"createdAt"`
		} `json:"latestUnreadEvent"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode rows: %v; raw=%s", err, rec.Body.String())
	}
	if len(rows) != 2 {
		t.Fatalf("rows: got %d, want 2", len(rows))
	}
	byUID := map[string]int{}
	for i, r := range rows {
		byUID[r.UID] = i
	}
	r1 := rows[byUID["uid-1"]]
	if r1.LatestUnreadEvent == nil || r1.LatestUnreadEvent.Type != "DecisionUpdate" ||
		r1.LatestUnreadEvent.Decision == nil || *r1.LatestUnreadEvent.Decision != "Permitted" {
		t.Errorf("uid-1 latestUnreadEvent: got %+v", r1.LatestUnreadEvent)
	}
	if rows[byUID["uid-2"]].LatestUnreadEvent != nil {
		t.Errorf("uid-2 should have null latestUnreadEvent")
	}
}

func TestApplications_DefaultsLimitTo500(t *testing.T) {
	t.Parallel()
	d := nearbyDeps{
		store:    &fakeZoneStore{zones: []WatchZone{mustZone(t, "zone-1", 471)}},
		apps:     &fakeAppFinder{},
		profiles: &fakeProfileReader{},
		resolver: &fakeResolver{},
		state:    &fakeWatermark{},
		unread:   &fakeUnread{},
	}
	mux := newNearbyMux(t, d)

	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if d.apps.lastLimit != 500 {
		t.Errorf("default limit: got %d, want 500", d.apps.lastLimit)
	}
}

func TestApplications_LimitParsing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		query     string
		wantLimit int
	}{
		{"valid below max", "?limit=50", 50},
		{"exactly max", "?limit=500", 500},
		{"above max clamps down", "?limit=10000", 500},
		{"zero falls back to default", "?limit=0", 500},
		{"negative falls back to default", "?limit=-5", 500},
		{"non-numeric falls back to default", "?limit=abc", 500},
		{"absent falls back to default", "", 500},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := nearbyDeps{
				store:    &fakeZoneStore{zones: []WatchZone{mustZone(t, "zone-1", 471)}},
				apps:     &fakeAppFinder{},
				profiles: &fakeProfileReader{},
				resolver: &fakeResolver{},
				state:    &fakeWatermark{},
				unread:   &fakeUnread{},
			}
			mux := newNearbyMux(t, d)
			rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications"+tc.query, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status: got %d, want 200", rec.Code)
			}
			if d.apps.lastLimit != tc.wantLimit {
				t.Errorf("limit: got %d, want %d", d.apps.lastLimit, tc.wantLimit)
			}
		})
	}
}

func TestApplications_SetsNextCursorHeaderWhenMorePagesExist(t *testing.T) {
	t.Parallel()
	d := nearbyDeps{
		store:    &fakeZoneStore{zones: []WatchZone{mustZone(t, "zone-1", 471)}},
		apps:     &fakeAppFinder{apps: []applications.PlanningApplication{testApp("uid-1", "24/001")}, next: "raw-token-123"},
		profiles: &fakeProfileReader{},
		resolver: &fakeResolver{},
		state:    &fakeWatermark{},
		unread:   &fakeUnread{},
	}
	mux := newNearbyMux(t, d)

	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	got := rec.Header().Get("X-Next-Cursor")
	want := base64.RawURLEncoding.EncodeToString([]byte("raw-token-123"))
	if got != want {
		t.Errorf("X-Next-Cursor: got %q, want %q (base64url of the raw token)", got, want)
	}
}

func TestApplications_OmitsNextCursorHeaderWhenExhausted(t *testing.T) {
	t.Parallel()
	d := nearbyDeps{
		store:    &fakeZoneStore{zones: []WatchZone{mustZone(t, "zone-1", 471)}},
		apps:     &fakeAppFinder{apps: []applications.PlanningApplication{testApp("uid-1", "24/001")}}, // next == ""
		profiles: &fakeProfileReader{},
		resolver: &fakeResolver{},
		state:    &fakeWatermark{},
		unread:   &fakeUnread{},
	}
	mux := newNearbyMux(t, d)

	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("X-Next-Cursor"); got != "" {
		t.Errorf("X-Next-Cursor must be absent when the query is exhausted; got %q", got)
	}
}

func TestApplications_ResumesFromCursorParam(t *testing.T) {
	t.Parallel()
	d := nearbyDeps{
		store:    &fakeZoneStore{zones: []WatchZone{mustZone(t, "zone-1", 471)}},
		apps:     &fakeAppFinder{},
		profiles: &fakeProfileReader{},
		resolver: &fakeResolver{},
		state:    &fakeWatermark{},
		unread:   &fakeUnread{},
	}
	mux := newNearbyMux(t, d)

	cursor := base64.RawURLEncoding.EncodeToString([]byte("resume-token-9"))
	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications?cursor="+cursor, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	// The handler must base64url-decode the cursor before handing it to the store.
	if d.apps.lastCursor != "resume-token-9" {
		t.Errorf("decoded cursor: got %q, want %q", d.apps.lastCursor, "resume-token-9")
	}
}

func TestApplications_RejectsUndecodableCursorWith400(t *testing.T) {
	t.Parallel()
	d := nearbyDeps{
		store:    &fakeZoneStore{zones: []WatchZone{mustZone(t, "zone-1", 471)}},
		apps:     &fakeAppFinder{},
		profiles: &fakeProfileReader{},
		resolver: &fakeResolver{},
		state:    &fakeWatermark{},
		unread:   &fakeUnread{},
	}
	mux := newNearbyMux(t, d)

	// "!!!" is not valid base64url — a garbage cursor must be a clean 400, not a
	// silent reset to the first page.
	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications?cursor=%21%21%21", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
	if d.apps.called {
		t.Error("must not run the spatial query when the cursor is malformed")
	}
}

func TestApplications_BoundsUnreadLookupToReturnedPage(t *testing.T) {
	t.Parallel()
	// The finder has more than a page worth of apps available; the bounded fetch
	// returns only `limit` (default 500), so the unread lookup must receive a
	// bounded UID set — never every app in a dense zone (tc-fm8f).
	d := nearbyDeps{
		store:    &fakeZoneStore{zones: []WatchZone{mustZone(t, "zone-1", 471)}},
		apps:     &fakeAppFinder{apps: manyApps(600)},
		profiles: &fakeProfileReader{},
		resolver: &fakeResolver{},
		state:    &fakeWatermark{state: &notificationstate.State{UserID: testUser, LastReadAt: time.Unix(0, 0), Version: 1}},
		unread:   &fakeUnread{},
	}
	mux := newNearbyMux(t, d)

	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !d.unread.called {
		t.Fatal("unread lookup must run when the user has a watermark")
	}
	if got := len(d.unread.lastUIDs); got != 500 {
		t.Errorf("unread lookup UID set: got %d, want 500 (bounded to the returned page)", got)
	}
}

func TestApplications_SurfacesNeighbourAuthorityApps(t *testing.T) {
	t.Parallel()
	// A zone pinned to authority 471 whose circle crosses into authority 246. The
	// applications list is now authority-agnostic, so both sides must appear
	// (tc-zldl / tc-w11n).
	d := nearbyDeps{
		store: &fakeZoneStore{zones: []WatchZone{mustZone(t, "zone-1", 471)}},
		apps: &fakeAppFinder{apps: []applications.PlanningApplication{
			testAppInAuthority("uid-1", "24/001", 471),
			testAppInAuthority("uid-2", "24/002", 246),
		}},
		profiles: &fakeProfileReader{},
		resolver: &fakeResolver{},
		state:    &fakeWatermark{},
		unread:   &fakeUnread{},
	}
	mux := newNearbyMux(t, d)

	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var rows []struct {
		UID string `json:"uid"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode rows: %v; raw=%s", err, rec.Body.String())
	}
	gotUIDs := map[string]bool{}
	for _, r := range rows {
		gotUIDs[r.UID] = true
	}
	if len(rows) != 2 || !gotUIDs["uid-1"] || !gotUIDs["uid-2"] {
		t.Fatalf("applications list must surface both home and neighbour authority apps: got %+v", rows)
	}
}

func TestApplications_NoWatermarkSkipsUnreadLookup(t *testing.T) {
	t.Parallel()
	d := nearbyDeps{
		store:    &fakeZoneStore{zones: []WatchZone{mustZone(t, "zone-1", 471)}},
		apps:     &fakeAppFinder{apps: []applications.PlanningApplication{testApp("uid-1", "24/001")}},
		profiles: &fakeProfileReader{},
		resolver: &fakeResolver{},
		state:    &fakeWatermark{state: nil}, // first touch: no watermark
		unread:   &fakeUnread{},
	}
	mux := newNearbyMux(t, d)

	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if d.unread.called {
		t.Error("unread lookup must be skipped when the user has no watermark")
	}
}

func TestApplications_ZoneNotFoundIs404(t *testing.T) {
	t.Parallel()
	d := nearbyDeps{
		store:    &fakeZoneStore{},
		apps:     &fakeAppFinder{},
		profiles: &fakeProfileReader{},
		resolver: &fakeResolver{},
		state:    &fakeWatermark{},
		unread:   &fakeUnread{},
	}
	mux := newNearbyMux(t, d)

	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/missing/applications", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rec.Code)
	}
}

func TestApplications_EmptyZoneReturnsEmptyArray(t *testing.T) {
	t.Parallel()
	d := nearbyDeps{
		store:    &fakeZoneStore{zones: []WatchZone{mustZone(t, "zone-1", 471)}},
		apps:     &fakeAppFinder{},
		profiles: &fakeProfileReader{},
		resolver: &fakeResolver{},
		state:    &fakeWatermark{},
		unread:   &fakeUnread{},
	}
	mux := newNearbyMux(t, d)

	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "[]" {
		t.Errorf("empty applications: got %s, want []", got)
	}
}

func TestCreate_RejectsOversizedRadius(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		radiusMetres float64
		wantStatus   int
	}{
		{"exactly at limit is valid", 10000, http.StatusCreated},
		{"just above limit is 400", 10001, http.StatusBadRequest},
		{"far above limit is 400", 1e308, http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := nearbyDeps{
				store:    &fakeZoneStore{},
				profiles: &fakeProfileReader{profile: proProfile(t)},
				resolver: &fakeResolver{},
				apps:     &fakeAppFinder{},
				state:    &fakeWatermark{},
				unread:   &fakeUnread{},
			}
			mux := newNearbyMux(t, d)
			body := fmt.Sprintf(`{"name":"Z","latitude":51.5,"longitude":-0.12,"radiusMetres":%v,"authorityId":471}`, tc.radiusMetres)
			rec := doReq(t, mux, http.MethodPost, "/v1/me/watch-zones", body)
			if rec.Code != tc.wantStatus {
				t.Fatalf("radiusMetres=%v: got status %d, want %d", tc.radiusMetres, rec.Code, tc.wantStatus)
			}
		})
	}
}

func TestValid_RejectsNonFiniteCoordinates(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		req  createRequest
	}{
		{"NaN latitude", createRequest{Name: "Z", Latitude: math.NaN(), Longitude: 0, RadiusMetres: 1000}},
		{"Inf latitude", createRequest{Name: "Z", Latitude: math.Inf(1), Longitude: 0, RadiusMetres: 1000}},
		{"NaN longitude", createRequest{Name: "Z", Latitude: 51.5, Longitude: math.NaN(), RadiusMetres: 1000}},
		{"Inf longitude", createRequest{Name: "Z", Latitude: 51.5, Longitude: math.Inf(-1), RadiusMetres: 1000}},
		{"NaN radius", createRequest{Name: "Z", Latitude: 51.5, Longitude: -0.12, RadiusMetres: math.NaN()}},
		{"Inf radius", createRequest{Name: "Z", Latitude: 51.5, Longitude: -0.12, RadiusMetres: math.Inf(1)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.req.valid() {
				t.Errorf("%s: valid() returned true, want false", tc.name)
			}
		})
	}
}

// sortDeps builds a standard dependency set for the applications-list sort tests:
// one seeded zone, a configurable finder, no watermark.
func sortDeps(t *testing.T, apps *fakeAppFinder) nearbyDeps {
	t.Helper()
	return nearbyDeps{
		store:    &fakeZoneStore{zones: []WatchZone{mustZone(t, "zone-1", 471)}},
		apps:     apps,
		profiles: &fakeProfileReader{},
		resolver: &fakeResolver{},
		state:    &fakeWatermark{},
		unread:   &fakeUnread{},
	}
}

// TestApplications_ParamlessUsesLegacyDistancePath proves the byte-identical
// contract's routing: with no params the handler keeps using the legacy distance
// finder (FindNearbyPage) at the default 500, never the sort-aware path.
func TestApplications_ParamlessUsesLegacyDistancePath(t *testing.T) {
	t.Parallel()
	apps := &fakeAppFinder{}
	mux := newNearbyMux(t, sortDeps(t, apps))

	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if !apps.called {
		t.Error("param-less request must use the legacy FindNearbyPage path")
	}
	if apps.inZoneCalled {
		t.Error("param-less request must NOT use the sort-aware path")
	}
	if apps.lastLimit != 500 {
		t.Errorf("legacy default limit: got %d, want 500", apps.lastLimit)
	}
}

// TestApplications_SortRoutesToSortAwarePath proves ?sort= routes to
// FindInZonePage with the parsed sort and the new default page size of 150.
func TestApplications_SortRoutesToSortAwarePath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		query    string
		wantSort applications.Sort
	}{
		{"?sort=distance", applications.SortDistance},
		{"?sort=newest", applications.SortNewest},
		{"?sort=oldest", applications.SortOldest},
		{"?sort=status", applications.SortStatus},
		{"?sort=recent-activity", applications.SortRecentActivity},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			t.Parallel()
			apps := &fakeAppFinder{}
			mux := newNearbyMux(t, sortDeps(t, apps))

			rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications"+tc.query, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status: got %d, want 200", rec.Code)
			}
			if !apps.inZoneCalled {
				t.Fatal("a ?sort= request must use the sort-aware FindInZonePage path")
			}
			if apps.called {
				t.Error("a ?sort= request must NOT use the legacy FindNearbyPage path")
			}
			if apps.lastSort != tc.wantSort {
				t.Errorf("sort: got %q, want %q", apps.lastSort, tc.wantSort)
			}
			if apps.lastLimit != 150 {
				t.Errorf("sort-aware default limit: got %d, want 150", apps.lastLimit)
			}
		})
	}
}

// TestApplications_RecentActivityThreadsUserID proves a ?sort=recent-activity
// request is accepted (200), routes to the sort-aware finder, and threads the
// authenticated subject through to the store — recent-activity joins the caller's
// own notifications, so the wrong userID would surface another user's unread data.
func TestApplications_RecentActivityThreadsUserID(t *testing.T) {
	t.Parallel()
	apps := &fakeAppFinder{}
	mux := newNearbyMux(t, sortDeps(t, apps))

	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications?sort=recent-activity", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if !apps.inZoneCalled {
		t.Fatal("recent-activity must use the sort-aware FindInZonePage path")
	}
	if apps.lastSort != applications.SortRecentActivity {
		t.Errorf("sort: got %q, want recent-activity", apps.lastSort)
	}
	if apps.lastUserID != testUser {
		t.Errorf("userID threaded to store: got %q, want %q", apps.lastUserID, testUser)
	}
}

// TestApplications_RecentActivityCursorMismatchIs400 proves a recent-activity
// cursor replayed under another sort (store reports ErrCursorSortMismatch)
// surfaces as 400, never a mis-ordered page.
func TestApplications_RecentActivityCursorMismatchIs400(t *testing.T) {
	t.Parallel()
	apps := &fakeAppFinder{inZoneErr: applications.ErrCursorSortMismatch}
	mux := newNearbyMux(t, sortDeps(t, apps))

	cursor := base64.RawURLEncoding.EncodeToString([]byte("recent-activity-cursor"))
	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications?sort=newest&cursor="+cursor, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
}

// TestApplications_SortAwareLimitParsing proves the sort-aware path parses ?limit=
// with a 150 default and clamps to the shared 500 ceiling.
func TestApplications_SortAwareLimitParsing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		query     string
		wantLimit int
	}{
		{"explicit below ceiling", "?sort=newest&limit=50", 50},
		{"at ceiling", "?sort=newest&limit=500", 500},
		{"above ceiling clamps", "?sort=newest&limit=10000", 500},
		{"absent uses sort default", "?sort=newest", 150},
		{"zero uses sort default", "?sort=newest&limit=0", 150},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			apps := &fakeAppFinder{}
			mux := newNearbyMux(t, sortDeps(t, apps))
			rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications"+tc.query, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status: got %d, want 200", rec.Code)
			}
			if apps.lastLimit != tc.wantLimit {
				t.Errorf("limit: got %d, want %d", apps.lastLimit, tc.wantLimit)
			}
		})
	}
}

// TestApplications_UnknownSortIs400 proves any sort outside the supported set is
// rejected with 400 before any spatial query runs. recent-activity is now
// supported (slice 3), so it is not in this set.
func TestApplications_UnknownSortIs400(t *testing.T) {
	t.Parallel()
	for _, sortVal := range []string{"nonsense", "DISTANCE", "Newest", "recentactivity"} {
		t.Run(sortVal, func(t *testing.T) {
			t.Parallel()
			apps := &fakeAppFinder{}
			mux := newNearbyMux(t, sortDeps(t, apps))
			rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications?sort="+sortVal, "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("sort=%q: got status %d, want 400", sortVal, rec.Code)
			}
			if apps.called || apps.inZoneCalled {
				t.Error("must not run any spatial query for an unsupported sort")
			}
		})
	}
}

// TestApplications_CursorSortMismatchIs400 proves a cursor minted under a
// different sort (the store reports ErrCursorSortMismatch) surfaces as 400, never
// a mis-ordered page.
func TestApplications_CursorSortMismatchIs400(t *testing.T) {
	t.Parallel()
	apps := &fakeAppFinder{inZoneErr: applications.ErrCursorSortMismatch}
	mux := newNearbyMux(t, sortDeps(t, apps))

	cursor := base64.RawURLEncoding.EncodeToString([]byte("cursor-from-another-sort"))
	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications?sort=oldest&cursor="+cursor, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
}

// TestApplications_MalformedInnerCursorIs400 proves a cursor that decodes at the
// transport layer but is not a valid keyset token (store reports ErrCursorInvalid)
// surfaces as 400.
func TestApplications_MalformedInnerCursorIs400(t *testing.T) {
	t.Parallel()
	apps := &fakeAppFinder{inZoneErr: applications.ErrCursorInvalid}
	mux := newNearbyMux(t, sortDeps(t, apps))

	cursor := base64.RawURLEncoding.EncodeToString([]byte("not-a-real-keyset"))
	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications?sort=newest&cursor="+cursor, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
}

// TestApplications_SortAwareSetsNextCursorHeader proves the sort-aware path hands
// the store's continuation token back via X-Next-Cursor, base64url-wrapped like
// the legacy path.
func TestApplications_SortAwareSetsNextCursorHeader(t *testing.T) {
	t.Parallel()
	apps := &fakeAppFinder{apps: []applications.PlanningApplication{testApp("uid-1", "24/001")}, next: "sort-token-9"}
	mux := newNearbyMux(t, sortDeps(t, apps))

	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications?sort=newest", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	got := rec.Header().Get("X-Next-Cursor")
	want := base64.RawURLEncoding.EncodeToString([]byte("sort-token-9"))
	if got != want {
		t.Errorf("X-Next-Cursor: got %q, want %q", got, want)
	}
}

// TestApplications_ParamlessGoldenResponse pins the byte-identical backward-compat
// contract: the param-less response is the bare []Result array, unchanged by the
// sort surface. An in-review iOS build depends on these exact bytes.
func TestApplications_ParamlessGoldenResponse(t *testing.T) {
	t.Parallel()
	apps := &fakeAppFinder{apps: []applications.PlanningApplication{testApp("uid-1", "24/001")}}
	mux := newNearbyMux(t, sortDeps(t, apps))

	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	want := `[{"name":"24/001","uid":"uid-1","areaName":"City of London","areaId":471,` +
		`"address":"1 Test St","postcode":null,"description":"An extension",` +
		`"appType":"Full","appState":"Permitted","appSize":null,"startDate":null,` +
		`"decidedDate":null,"consultedDate":null,"longitude":null,"latitude":null,` +
		`"url":null,"link":null,"lastDifferent":"2026-06-14T09:00:00+00:00",` +
		`"latestUnreadEvent":null}]`
	if got := rec.Body.String(); got != want {
		t.Errorf("param-less response not byte-identical:\n got = %s\nwant = %s", got, want)
	}
}

// TestApplications_StatusFilterRoutesToSortAwarePath proves ?status= opts into the
// sort-aware path (even without ?sort=, since a filter needs the filterable
// finder), threads the exact app_state through, and defaults the page size to 150.
// "All" means no status filter, so ?status=All alone (no sort) stays on the legacy
// distance path.
func TestApplications_StatusFilterRoutesToSortAwarePath(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"Undecided", "Permitted", "Conditions", "Rejected", "Withdrawn", "Appealed"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			apps := &fakeAppFinder{}
			mux := newNearbyMux(t, sortDeps(t, apps))
			rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications?status="+status, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%q: got %d, want 200", status, rec.Code)
			}
			if !apps.inZoneCalled {
				t.Fatalf("status=%q must use the sort-aware FindInZonePage path", status)
			}
			if apps.lastStatus != status {
				t.Errorf("threaded status: got %q, want %q", apps.lastStatus, status)
			}
			if apps.lastSort != applications.SortDistance {
				t.Errorf("status filter without ?sort= defaults to distance: got %q", apps.lastSort)
			}
			if apps.lastLimit != 150 {
				t.Errorf("filtered default limit: got %d, want 150", apps.lastLimit)
			}
		})
	}
}

// TestApplications_StatusAllIsNoFilter proves ?status=All means "no status
// filter": with no sort it stays byte-identical on the legacy distance path, and
// combined with a sort it routes to the sort-aware path with an empty status.
func TestApplications_StatusAllIsNoFilter(t *testing.T) {
	t.Parallel()
	t.Run("All alone stays on the legacy path", func(t *testing.T) {
		t.Parallel()
		apps := &fakeAppFinder{}
		mux := newNearbyMux(t, sortDeps(t, apps))
		rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications?status=All", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status=All: got %d, want 200", rec.Code)
		}
		if !apps.called || apps.inZoneCalled {
			t.Errorf("status=All alone must use the legacy FindNearbyPage path: called=%v inZone=%v", apps.called, apps.inZoneCalled)
		}
	})
	t.Run("All with a sort carries no status filter", func(t *testing.T) {
		t.Parallel()
		apps := &fakeAppFinder{}
		mux := newNearbyMux(t, sortDeps(t, apps))
		rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications?sort=newest&status=All", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("sort=newest&status=All: got %d, want 200", rec.Code)
		}
		if !apps.inZoneCalled || apps.lastStatus != "" {
			t.Errorf("status=All must thread an empty status filter: inZone=%v status=%q", apps.inZoneCalled, apps.lastStatus)
		}
	})
}

// TestApplications_UnknownStatusIs400 proves any ?status= outside the app_state
// vocabulary (and absent "All") is rejected with 400 before any spatial query.
func TestApplications_UnknownStatusIs400(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"nonsense", "permitted", "REJECTED", "Approved", "all"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			apps := &fakeAppFinder{}
			mux := newNearbyMux(t, sortDeps(t, apps))
			rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications?status="+status, "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%q: got %d, want 400", status, rec.Code)
			}
			if apps.called || apps.inZoneCalled {
				t.Error("must not run any spatial query for an unrecognised status")
			}
		})
	}
}

// TestApplications_UnreadFilterRoutesToSortAwarePath proves ?unread=true opts into
// the sort-aware path (even without ?sort=), threads the flag through, and defaults
// the page size to 150. ?unread=false (like absent) means no unread filter, so it
// stays on the legacy distance path.
func TestApplications_UnreadFilterRoutesToSortAwarePath(t *testing.T) {
	t.Parallel()
	t.Run("true routes to the filtered path", func(t *testing.T) {
		t.Parallel()
		apps := &fakeAppFinder{}
		mux := newNearbyMux(t, sortDeps(t, apps))
		rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications?unread=true", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("unread=true: got %d, want 200", rec.Code)
		}
		if !apps.inZoneCalled || !apps.lastUnread {
			t.Errorf("unread=true must use the filtered path with the flag set: inZone=%v unread=%v", apps.inZoneCalled, apps.lastUnread)
		}
		if apps.lastSort != applications.SortDistance || apps.lastLimit != 150 {
			t.Errorf("unread without ?sort= defaults to distance/150: sort=%q limit=%d", apps.lastSort, apps.lastLimit)
		}
	})
	t.Run("true with a sort carries the flag", func(t *testing.T) {
		t.Parallel()
		apps := &fakeAppFinder{}
		mux := newNearbyMux(t, sortDeps(t, apps))
		rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications?sort=oldest&unread=true", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("sort=oldest&unread=true: got %d, want 200", rec.Code)
		}
		if !apps.inZoneCalled || !apps.lastUnread || apps.lastSort != applications.SortOldest {
			t.Errorf("got inZone=%v unread=%v sort=%q", apps.inZoneCalled, apps.lastUnread, apps.lastSort)
		}
	})
	t.Run("false alone stays on the legacy path", func(t *testing.T) {
		t.Parallel()
		apps := &fakeAppFinder{}
		mux := newNearbyMux(t, sortDeps(t, apps))
		rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications?unread=false", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("unread=false: got %d, want 200", rec.Code)
		}
		if !apps.called || apps.inZoneCalled {
			t.Errorf("unread=false alone must use the legacy path: called=%v inZone=%v", apps.called, apps.inZoneCalled)
		}
	})
}

// TestApplications_InvalidUnreadIs400 proves ?unread= that is neither true, false
// nor absent is a clean 400 before any spatial query — never silently treated as
// off.
func TestApplications_InvalidUnreadIs400(t *testing.T) {
	t.Parallel()
	for _, val := range []string{"yes", "1", "TRUE", "True", "nope"} {
		t.Run(val, func(t *testing.T) {
			t.Parallel()
			apps := &fakeAppFinder{}
			mux := newNearbyMux(t, sortDeps(t, apps))
			rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications?unread="+val, "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("unread=%q: got %d, want 400", val, rec.Code)
			}
			if apps.called || apps.inZoneCalled {
				t.Error("must not run any spatial query for an invalid unread value")
			}
		})
	}
}

// TestApplications_StatusAndUnreadTogetherIs400 proves a status filter and the
// unread filter together are rejected with 400 before any spatial query — they
// are mutually exclusive. "All" is not a real status filter, so All + unread is
// allowed.
func TestApplications_StatusAndUnreadTogetherIs400(t *testing.T) {
	t.Parallel()
	t.Run("real status + unread is 400", func(t *testing.T) {
		t.Parallel()
		apps := &fakeAppFinder{}
		mux := newNearbyMux(t, sortDeps(t, apps))
		rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications?status=Permitted&unread=true", "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=Permitted&unread=true: got %d, want 400", rec.Code)
		}
		if apps.called || apps.inZoneCalled {
			t.Error("must not run any spatial query when status and unread conflict")
		}
	})
	t.Run("All + unread is allowed", func(t *testing.T) {
		t.Parallel()
		apps := &fakeAppFinder{}
		mux := newNearbyMux(t, sortDeps(t, apps))
		rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications?status=All&unread=true", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status=All&unread=true: got %d, want 200", rec.Code)
		}
		if !apps.inZoneCalled || !apps.lastUnread || apps.lastStatus != "" {
			t.Errorf("All+unread must filter on unread only: inZone=%v unread=%v status=%q", apps.inZoneCalled, apps.lastUnread, apps.lastStatus)
		}
	})
}

// TestApplications_CursorFilterMismatchIs400 proves a cursor minted under one
// filter and replayed under another (the store reports ErrCursorFilterMismatch)
// surfaces as 400, never a gapped or overlapping page.
func TestApplications_CursorFilterMismatchIs400(t *testing.T) {
	t.Parallel()
	apps := &fakeAppFinder{inZoneErr: applications.ErrCursorFilterMismatch}
	mux := newNearbyMux(t, sortDeps(t, apps))

	cursor := base64.RawURLEncoding.EncodeToString([]byte("cursor-from-another-filter"))
	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications?sort=newest&status=Permitted&cursor="+cursor, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
}

// manyApps builds n distinct nearby applications, for asserting the bounded page
// caps the downstream unread UID set.
func manyApps(n int) []applications.PlanningApplication {
	apps := make([]applications.PlanningApplication, n)
	for i := range apps {
		id := strconv.Itoa(i)
		apps[i] = testApp("uid-"+id, "24/"+id)
	}
	return apps
}

func mustZone(t *testing.T, id string, authorityID int) WatchZone {
	t.Helper()
	z, err := NewWatchZone(id, testUser, "Zone", 51.5, -0.12, 1000, authorityID, nearbyNow, true, true)
	if err != nil {
		t.Fatalf("NewWatchZone: %v", err)
	}
	return z
}

// clusterDeps builds a standard dependency set for the clusters-endpoint tests:
// one seeded zone (lat 51.5, lng -0.12, radius 1000) and a configurable finder.
func clusterDeps(t *testing.T, apps *fakeAppFinder) nearbyDeps {
	t.Helper()
	return nearbyDeps{
		store:    &fakeZoneStore{zones: []WatchZone{mustZone(t, "zone-1", 471)}},
		apps:     apps,
		profiles: &fakeProfileReader{},
		resolver: &fakeResolver{},
		state:    &fakeWatermark{},
		unread:   &fakeUnread{},
	}
}

// validClusterQuery is a well-formed bbox+zoom query string for the clusters
// endpoint. bbox=west,south,east,north (WGS84 degrees) around the fixture zone.
const validClusterQuery = "?bbox=-0.2,51.4,-0.05,51.6&zoom=12"

// TestClusters_ZoneNotFoundIs404 proves an unowned (absent) zone is a 404, like
// the sibling applications route, before any spatial query runs.
func TestClusters_ZoneNotFoundIs404(t *testing.T) {
	t.Parallel()
	apps := &fakeAppFinder{}
	d := clusterDeps(t, apps)
	d.store = &fakeZoneStore{} // no zones
	mux := newNearbyMux(t, d)

	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/missing/applications/clusters"+validClusterQuery, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rec.Code)
	}
	if apps.clustersCalled {
		t.Error("must not run the cluster query for an unowned zone")
	}
}

// TestClusters_PassesZoneAndParamsToStore proves the handler loads the zone and
// threads its centre+radius, the parsed bbox, the zoom-derived grid size, and the
// status filter into the store query.
func TestClusters_PassesZoneAndParamsToStore(t *testing.T) {
	t.Parallel()
	apps := &fakeAppFinder{}
	mux := newNearbyMux(t, clusterDeps(t, apps))

	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications/clusters?bbox=-0.2,51.4,-0.05,51.6&zoom=12&status=Permitted", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !apps.clustersCalled {
		t.Fatal("the cluster query must run for an owned zone")
	}
	q := apps.lastClusterQuery
	if q.Latitude != 51.5 || q.Longitude != -0.12 || q.RadiusMetres != 1000 {
		t.Errorf("zone centre/radius: got (%v,%v,%v), want (51.5,-0.12,1000)", q.Latitude, q.Longitude, q.RadiusMetres)
	}
	if q.West != -0.2 || q.South != 51.4 || q.East != -0.05 || q.North != 51.6 {
		t.Errorf("bbox: got (%v,%v,%v,%v), want (-0.2,51.4,-0.05,51.6)", q.West, q.South, q.East, q.North)
	}
	if q.Status != "Permitted" {
		t.Errorf("status: got %q, want %q", q.Status, "Permitted")
	}
	wantGrid := 45.0 / float64(uint64(1)<<12) // 360/2^(12+3)
	if q.GridSizeDegrees != wantGrid {
		t.Errorf("grid size for zoom 12: got %v, want %v", q.GridSizeDegrees, wantGrid)
	}
	// The coalesce threshold is the finest (zoom-20) grid cell, independent of the
	// request's own zoom: it is the "zooming can never separate these" boundary
	// below which a multi-member cell gets an applicationIds member list.
	wantThreshold := 45.0 / float64(uint64(1)<<20) // 360/2^(20+3)
	if q.CoalesceThresholdDegrees != wantThreshold {
		t.Errorf("coalesce threshold: got %v, want the finest-grid size %v", q.CoalesceThresholdDegrees, wantThreshold)
	}
}

// TestClusters_CoalesceThresholdIsFinestGridRegardlessOfZoom proves the handler
// threads the same finest-grid coalesce threshold into the store query for every
// request zoom — it is the zoom-20 cell size, not the request's own grid size — so
// the member-list decision is stable across zoom levels.
func TestClusters_CoalesceThresholdIsFinestGridRegardlessOfZoom(t *testing.T) {
	t.Parallel()
	wantThreshold := 45.0 / float64(uint64(1)<<20)
	for _, zoom := range []int{0, 5, 12, 20} {
		t.Run(strconv.Itoa(zoom), func(t *testing.T) {
			t.Parallel()
			apps := &fakeAppFinder{}
			mux := newNearbyMux(t, clusterDeps(t, apps))
			rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications/clusters?bbox=-0.2,51.4,-0.05,51.6&zoom="+strconv.Itoa(zoom), "")
			if rec.Code != http.StatusOK {
				t.Fatalf("zoom=%d: got %d, want 200", zoom, rec.Code)
			}
			if got := apps.lastClusterQuery.CoalesceThresholdDegrees; got != wantThreshold {
				t.Errorf("zoom=%d coalesce threshold: got %v, want %v", zoom, got, wantThreshold)
			}
		})
	}
}

// TestClusters_WireShape_ApplicationIds pins the additive member-list contract: an
// unsplittable coincident cell carries applicationIds listing its members, while a
// splittable multi-member cell omits the field entirely (omitempty). The status
// filter is threaded straight through to the store (which restricts the member
// list at the SQL layer — see the integration suite).
func TestClusters_WireShape_ApplicationIds(t *testing.T) {
	t.Parallel()
	apps := &fakeAppFinder{clusters: []applications.Cluster{
		{ // splittable multi-member cell: no member list, client zooms in
			Latitude: 51.51, Longitude: -0.12, Count: 42,
			StatusCounts: map[string]int{"Permitted": 42},
			Members:      nil,
		},
		{ // unsplittable coincident cell: carries the disambiguation list
			Latitude: 51.52, Longitude: -0.13, Count: 3,
			StatusCounts: map[string]int{"Permitted": 2, "Rejected": 1},
			Members: []applications.PlanningApplicationID{
				{Authority: "471", Name: "24/001"},
				{Authority: "471", Name: "24/002"},
				{Authority: "471", Name: "24/003"},
			},
		},
	}}
	mux := newNearbyMux(t, clusterDeps(t, apps))

	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications/clusters"+validClusterQuery, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var got []struct {
		Count          int `json:"count"`
		ApplicationIDs *[]struct {
			Authority string `json:"authority"`
			Name      string `json:"name"`
		} `json:"applicationIds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v; raw=%s", err, rec.Body.String())
	}
	if len(got) != 2 {
		t.Fatalf("clusters: got %d, want 2", len(got))
	}
	if got[0].ApplicationIDs != nil {
		t.Errorf("splittable cell must omit applicationIds, got %+v", got[0].ApplicationIDs)
	}
	if got[1].ApplicationIDs == nil {
		t.Fatalf("coincident cell must carry applicationIds, got null")
	}
	if len(*got[1].ApplicationIDs) != 3 {
		t.Fatalf("applicationIds: got %d entries, want 3", len(*got[1].ApplicationIDs))
	}
	ids := *got[1].ApplicationIDs
	if ids[0].Authority != "471" || ids[0].Name != "24/001" {
		t.Errorf("applicationIds[0]: got %+v, want {471, 24/001}", ids[0])
	}
}

// TestClusters_ZoomToGridSizeLookup proves the server-owned zoom -> grid-cell-size
// table: each cell is 1/8 of a slippy tile (360/2^z), so the size halves per zoom
// level, and the size passed to the store matches the table for the request zoom.
func TestClusters_ZoomToGridSizeLookup(t *testing.T) {
	t.Parallel()
	for _, zoom := range []int{0, 1, 8, 12, 16, 20} {
		t.Run(strconv.Itoa(zoom), func(t *testing.T) {
			t.Parallel()
			apps := &fakeAppFinder{}
			mux := newNearbyMux(t, clusterDeps(t, apps))
			rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications/clusters?bbox=-0.2,51.4,-0.05,51.6&zoom="+strconv.Itoa(zoom), "")
			if rec.Code != http.StatusOK {
				t.Fatalf("zoom=%d: got %d, want 200", zoom, rec.Code)
			}
			want := 45.0 / float64(uint64(1)<<uint(zoom)) // 360/2^(z+3)
			if apps.lastClusterQuery.GridSizeDegrees != want {
				t.Errorf("zoom=%d grid size: got %v, want %v", zoom, apps.lastClusterQuery.GridSizeDegrees, want)
			}
		})
	}
}

// TestClusters_RequiresBBox proves an absent bbox is a clean 400 before any query.
func TestClusters_RequiresBBox(t *testing.T) {
	t.Parallel()
	apps := &fakeAppFinder{}
	mux := newNearbyMux(t, clusterDeps(t, apps))
	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications/clusters?zoom=12", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
	if apps.clustersCalled {
		t.Error("must not run the cluster query when bbox is absent")
	}
}

// TestClusters_MalformedBBoxIs400 proves a bbox that is not four in-range,
// well-ordered finite degrees is a clean 400 before any query.
func TestClusters_MalformedBBoxIs400(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"too few values":    "bbox=-0.2,51.4,-0.05&zoom=12",
		"too many values":   "bbox=-0.2,51.4,-0.05,51.6,1&zoom=12",
		"non-numeric":       "bbox=-0.2,abc,-0.05,51.6&zoom=12",
		"west >= east":      "bbox=-0.05,51.4,-0.2,51.6&zoom=12",
		"south >= north":    "bbox=-0.2,51.6,-0.05,51.4&zoom=12",
		"lng out of range":  "bbox=-181,51.4,-0.05,51.6&zoom=12",
		"lat out of range":  "bbox=-0.2,51.4,-0.05,91&zoom=12",
		"empty value":       "bbox=&zoom=12",
		"non-finite (NaN)":  "bbox=-0.2,NaN,-0.05,51.6&zoom=12",
		"non-finite (+Inf)": "bbox=-0.2,51.4,Inf,51.6&zoom=12",
	}
	for name, query := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			apps := &fakeAppFinder{}
			mux := newNearbyMux(t, clusterDeps(t, apps))
			rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications/clusters?"+query, "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s: got %d, want 400", name, rec.Code)
			}
			if apps.clustersCalled {
				t.Errorf("%s: must not run the cluster query for a malformed bbox", name)
			}
		})
	}
}

// TestClusters_RequiresZoom proves an absent zoom is a clean 400 before any query.
func TestClusters_RequiresZoom(t *testing.T) {
	t.Parallel()
	apps := &fakeAppFinder{}
	mux := newNearbyMux(t, clusterDeps(t, apps))
	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications/clusters?bbox=-0.2,51.4,-0.05,51.6", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
	if apps.clustersCalled {
		t.Error("must not run the cluster query when zoom is absent")
	}
}

// TestClusters_ZoomOutOfRangeIs400 proves a zoom outside the slippy range 0..20
// (or non-integer) is a clean 400 before any query.
func TestClusters_ZoomOutOfRangeIs400(t *testing.T) {
	t.Parallel()
	for _, zoom := range []string{"-1", "21", "100", "abc", "1.5", ""} {
		t.Run("zoom="+zoom, func(t *testing.T) {
			t.Parallel()
			apps := &fakeAppFinder{}
			mux := newNearbyMux(t, clusterDeps(t, apps))
			rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications/clusters?bbox=-0.2,51.4,-0.05,51.6&zoom="+zoom, "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("zoom=%q: got %d, want 400", zoom, rec.Code)
			}
			if apps.clustersCalled {
				t.Errorf("zoom=%q: must not run the cluster query for an out-of-range zoom", zoom)
			}
		})
	}
}

// TestClusters_UnknownStatusIs400 proves a status outside the app_state vocabulary
// (and not "All"/absent) is a clean 400 before any query, reusing the same
// vocabulary as the list endpoint.
func TestClusters_UnknownStatusIs400(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"nonsense", "permitted", "REJECTED", "all"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			apps := &fakeAppFinder{}
			mux := newNearbyMux(t, clusterDeps(t, apps))
			rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications/clusters?bbox=-0.2,51.4,-0.05,51.6&zoom=12&status="+status, "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%q: got %d, want 400", status, rec.Code)
			}
			if apps.clustersCalled {
				t.Errorf("status=%q: must not run the cluster query for an unrecognised status", status)
			}
		})
	}
}

// TestClusters_StatusAllIsNoFilter proves ?status=All threads an empty status
// filter (no filter), like the list endpoint.
func TestClusters_StatusAllIsNoFilter(t *testing.T) {
	t.Parallel()
	apps := &fakeAppFinder{}
	mux := newNearbyMux(t, clusterDeps(t, apps))
	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications/clusters?bbox=-0.2,51.4,-0.05,51.6&zoom=12&status=All", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if !apps.clustersCalled || apps.lastClusterQuery.Status != "" {
		t.Errorf("status=All must thread an empty status filter: called=%v status=%q", apps.clustersCalled, apps.lastClusterQuery.Status)
	}
}

// TestClusters_WireShape pins the response contract: an array of cluster objects
// with latitude/longitude/count/statusCounts, a null applicationId for a
// multi-member cell, and a {authority,name} applicationId for a single-member cell.
func TestClusters_WireShape(t *testing.T) {
	t.Parallel()
	apps := &fakeAppFinder{clusters: []applications.Cluster{
		{
			Latitude: 51.51, Longitude: -0.12, Count: 194,
			StatusCounts: map[string]int{"Permitted": 120, "Undecided": 60, "Rejected": 14},
			Member:       nil,
		},
		{
			Latitude: 51.52, Longitude: -0.13, Count: 1,
			StatusCounts: map[string]int{"Permitted": 1},
			Member:       &applications.PlanningApplicationID{Authority: "471", Name: "24/001"},
		},
	}}
	mux := newNearbyMux(t, clusterDeps(t, apps))

	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications/clusters"+validClusterQuery, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var got []struct {
		Latitude      float64        `json:"latitude"`
		Longitude     float64        `json:"longitude"`
		Count         int            `json:"count"`
		StatusCounts  map[string]int `json:"statusCounts"`
		ApplicationID *struct {
			Authority string `json:"authority"`
			Name      string `json:"name"`
		} `json:"applicationId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v; raw=%s", err, rec.Body.String())
	}
	if len(got) != 2 {
		t.Fatalf("clusters: got %d, want 2", len(got))
	}
	if got[0].Count != 194 || got[0].ApplicationID != nil {
		t.Errorf("multi-member cell: got count=%d applicationId=%+v, want count=194 applicationId=null", got[0].Count, got[0].ApplicationID)
	}
	if got[0].StatusCounts["Permitted"] != 120 || got[0].StatusCounts["Undecided"] != 60 || got[0].StatusCounts["Rejected"] != 14 {
		t.Errorf("multi-member statusCounts: got %v", got[0].StatusCounts)
	}
	if got[1].Count != 1 || got[1].ApplicationID == nil {
		t.Fatalf("single-member cell: got count=%d applicationId=%+v, want count=1 with a non-null id", got[1].Count, got[1].ApplicationID)
	}
	if got[1].ApplicationID.Authority != "471" || got[1].ApplicationID.Name != "24/001" {
		t.Errorf("single-member id: got %+v, want {authority:471, name:24/001}", *got[1].ApplicationID)
	}
}

// TestClusters_EmptyReturnsEmptyArray proves an empty result encodes as a bare
// JSON array, never null.
func TestClusters_EmptyReturnsEmptyArray(t *testing.T) {
	t.Parallel()
	apps := &fakeAppFinder{} // nil clusters
	mux := newNearbyMux(t, clusterDeps(t, apps))
	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications/clusters"+validClusterQuery, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "[]" {
		t.Errorf("empty clusters: got %s, want []", got)
	}
}

// TestClusters_StoreErrorIs500 proves a store failure surfaces as a 500.
func TestClusters_StoreErrorIs500(t *testing.T) {
	t.Parallel()
	apps := &fakeAppFinder{clusterErr: errors.New("postgis down")}
	mux := newNearbyMux(t, clusterDeps(t, apps))
	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones/zone-1/applications/clusters"+validClusterQuery, "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
}
