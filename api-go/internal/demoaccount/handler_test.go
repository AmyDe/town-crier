package demoaccount

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

var fixedNow = time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)

type fakeProfileStore struct {
	profile *profiles.UserProfile
	getErr  error
	saved   *profiles.UserProfile
	saveErr error
}

func (f *fakeProfileStore) Get(_ context.Context, _ string) (*profiles.UserProfile, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.profile == nil {
		return nil, profiles.ErrNotFound
	}
	return f.profile, nil
}

func (f *fakeProfileStore) Save(_ context.Context, p *profiles.UserProfile) error {
	f.saved = p
	if f.saveErr != nil {
		return f.saveErr
	}
	f.profile = p
	return nil
}

type fakeZoneStore struct {
	saved   []watchzones.WatchZone
	saveErr error
}

func (f *fakeZoneStore) Save(_ context.Context, z watchzones.WatchZone) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, z)
	return nil
}

type fakeAppStore struct {
	upserted   []applications.PlanningApplication
	nearby     []applications.PlanningApplication
	upsertErr  error
	findErr    error
	findCalled bool
}

func (f *fakeAppStore) Upsert(_ context.Context, a applications.PlanningApplication) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.upserted = append(f.upserted, a)
	return nil
}

func (f *fakeAppStore) FindNearby(_ context.Context, _, _, _ float64) ([]applications.PlanningApplication, error) {
	f.findCalled = true
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.nearby, nil
}

// serve wires the route and issues GET /v1/demo-account, returning the recorder.
func serve(t *testing.T, p profileStore, z zoneStore, a appStore) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	Routes(mux, p, z, a, func() time.Time { return fixedNow }, slog.New(slog.DiscardHandler))
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/demo-account", nil)
	mux.ServeHTTP(rec, req)
	return rec
}

func TestGetDemoAccount_FirstCall_SeedsAndReturnsDemoAccount(t *testing.T) {
	t.Parallel()
	p := &fakeProfileStore{}
	z := &fakeZoneStore{}
	a := &fakeAppStore{nearby: seedApplications(fixedNow)}

	rec := serve(t, p, z, a)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	// Seed side effects: a Pro profile, one watch zone, and the five fixed apps.
	if p.saved == nil || p.saved.Tier != profiles.TierPro {
		t.Errorf("seeded profile: got %+v, want Pro tier", p.saved)
	}
	if p.saved != nil && p.saved.SubscriptionExpiry != nil {
		if got := *p.saved.SubscriptionExpiry; !got.Equal(fixedNow.AddDate(demoSubscriptionYears, 0, 0)) {
			t.Errorf("subscription expiry: got %v, want +10y", got)
		}
	}
	if len(z.saved) != 1 {
		t.Fatalf("watch zones saved: got %d, want 1", len(z.saved))
	}
	if zone := z.saved[0]; zone.ID != demoZoneID || zone.AuthorityID != seedAuthorityID || !zone.CreatedAt.IsZero() {
		t.Errorf("seeded zone: got %+v", zone)
	}
	if len(a.upserted) != 5 {
		t.Errorf("seeded applications: got %d, want 5", len(a.upserted))
	}
	// FindNearby now runs cross-partition (no authority partition key), so we
	// assert it ran and returned the seeded apps rather than a partition key.
	if !a.findCalled {
		t.Error("FindNearby must run to populate the demo applications")
	}

	var got demoAccountResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v; raw=%s", err, rec.Body.String())
	}
	if got.UserID != demoUserID {
		t.Errorf("userId: got %q, want %q", got.UserID, demoUserID)
	}
	if got.Tier != "Pro" {
		t.Errorf("tier: got %q, want \"Pro\"", got.Tier)
	}
	wantZone := demoWatchZoneResult{ZoneID: demoZoneID, AuthorityName: seedAuthorityName, Latitude: demoLatitude, Longitude: demoLongitude, RadiusMetres: demoRadiusMetres}
	if got.WatchZone != wantZone {
		t.Errorf("watchZone: got %+v, want %+v", got.WatchZone, wantZone)
	}
	if len(got.Applications) != 5 {
		t.Fatalf("applications: got %d, want 5", len(got.Applications))
	}
	first := got.Applications[0]
	if first.UID != "demo-app-001" || first.Name != "24/05678/FULL" || first.AppType == nil || *first.AppType != "Full" {
		t.Errorf("first application: got %+v", first)
	}
}

func TestGetDemoAccount_SecondCall_DoesNotReseed(t *testing.T) {
	t.Parallel()
	existing, err := profiles.NewProfile(demoUserID, "", fixedNow)
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	existing.ActivateSubscription(profiles.TierPro, fixedNow.AddDate(demoSubscriptionYears, 0, 0))
	p := &fakeProfileStore{profile: existing}
	z := &fakeZoneStore{}
	a := &fakeAppStore{nearby: seedApplications(fixedNow)}

	rec := serve(t, p, z, a)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if p.saved != nil {
		t.Error("expected no profile re-save on second call")
	}
	if len(z.saved) != 0 {
		t.Errorf("expected no zone re-seed, got %d", len(z.saved))
	}
	if len(a.upserted) != 0 {
		t.Errorf("expected no application re-seed, got %d", len(a.upserted))
	}
	if !a.findCalled {
		t.Error("FindNearby must still run on a repeat call")
	}
}

func TestGetDemoAccount_EmptyNearby_SerialisesEmptyArray(t *testing.T) {
	t.Parallel()
	p := &fakeProfileStore{}
	rec := serve(t, p, &fakeZoneStore{}, &fakeAppStore{})

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	// The applications array must serialise as [] (not null) when empty.
	var raw struct {
		Applications json.RawMessage `json:"applications"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if string(raw.Applications) != "[]" {
		t.Errorf("applications: got %s, want []", raw.Applications)
	}
}

func TestGetDemoAccount_ProfileLoadError_Returns500(t *testing.T) {
	t.Parallel()
	p := &fakeProfileStore{getErr: errors.New("cosmos unavailable")}
	rec := serve(t, p, &fakeZoneStore{}, &fakeAppStore{})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
}

func TestGetDemoAccount_FindNearbyError_Returns500(t *testing.T) {
	t.Parallel()
	p := &fakeProfileStore{}
	a := &fakeAppStore{findErr: errors.New("query failed")}
	rec := serve(t, p, &fakeZoneStore{}, a)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
}

func TestGetDemoAccount_SeedSaveError_Returns500(t *testing.T) {
	t.Parallel()
	p := &fakeProfileStore{saveErr: errors.New("save failed")}
	rec := serve(t, p, &fakeZoneStore{}, &fakeAppStore{})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
}
