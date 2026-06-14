package watchzones

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
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
	apps   []applications.PlanningApplication
	err    error
	lastPK string
}

func (f *fakeAppFinder) FindNearby(_ context.Context, authorityCode string, _, _, _ float64) ([]applications.PlanningApplication, error) {
	f.lastPK = authorityCode
	return f.apps, f.err
}

type fakeWatermark struct {
	state *notificationstate.State
	err   error
}

func (f *fakeWatermark) Get(_ context.Context, _ string) (*notificationstate.State, error) {
	return f.state, f.err
}

type fakeUnread struct {
	result map[string]notifications.LatestUnread
	err    error
	called bool
}

func (f *fakeUnread) GetLatestUnreadByApplications(_ context.Context, _ string, _ []string, _ time.Time) (map[string]notifications.LatestUnread, error) {
	f.called = true
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
	NearbyRoutes(mux, d.store, d.profiles, d.resolver, d.apps, d.state, d.unread,
		func() string { return "zone-123" }, func() time.Time { return nearbyNow },
		slog.New(slog.DiscardHandler))
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

func TestCreate_PersistsZoneAndReturnsNearbyApplications(t *testing.T) {
	t.Parallel()
	d := nearbyDeps{
		store:    &fakeZoneStore{},
		profiles: &fakeProfileReader{profile: proProfile(t)},
		resolver: &fakeResolver{},
		apps:     &fakeAppFinder{apps: []applications.PlanningApplication{testApp("uid-1", "24/001")}},
		state:    &fakeWatermark{},
		unread:   &fakeUnread{},
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
	if d.apps.lastPK != "471" {
		t.Errorf("FindNearby authority partition: got %q", d.apps.lastPK)
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
	if len(got.NearbyApplications) != 1 || got.NearbyApplications[0].UID != "uid-1" {
		t.Fatalf("nearbyApplications: got %+v", got.NearbyApplications)
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

func mustZone(t *testing.T, id string, authorityID int) WatchZone {
	t.Helper()
	z, err := NewWatchZone(id, testUser, "Zone", 51.5, -0.12, 1000, authorityID, nearbyNow, true, true)
	if err != nil {
		t.Fatalf("NewWatchZone: %v", err)
	}
	return z
}
