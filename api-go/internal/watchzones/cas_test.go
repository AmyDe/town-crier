package watchzones

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// fakeProfileCAS is a hand-written fake for profileCAS. It serialises CAS
// operations under a mutex so the concurrent-create test runs safely under
// -race. The first concurrent caller wins; subsequent callers see
// ErrCASPreconditionFailed on their first attempt, then succeed on retry.
type fakeProfileCAS struct {
	mu      sync.Mutex
	profile *profiles.UserProfile
	etag    string
	getErr  error
	casErr  error // injected once then cleared
}

func newFakeProfileCAS(p *profiles.UserProfile) *fakeProfileCAS {
	return &fakeProfileCAS{profile: p, etag: "etag-1"}
}

func (f *fakeProfileCAS) GetWithETag(_ context.Context, _ string) (*profiles.UserProfile, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, "", f.getErr
	}
	if f.profile == nil {
		return nil, "", nil
	}
	cp := *f.profile
	return &cp, f.etag, nil
}

func (f *fakeProfileCAS) UpdateZoneCountWithCAS(_ context.Context, _ string, p *profiles.UserProfile, etag string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.casErr != nil {
		err := f.casErr
		f.casErr = nil // consume once
		return err
	}
	if etag != f.etag {
		return platform.ErrCASPreconditionFailed
	}
	cp := *p
	f.profile = &cp
	f.etag = "etag-2"
	return nil
}

// concurrentFakeProfileCAS lets the first N callers see ErrCASPreconditionFailed
// on their first attempt; the (N+1)th caller wins. Used to simulate N concurrent
// creates where only one can succeed.
type concurrentFakeProfileCAS struct {
	mu       sync.Mutex
	profile  *profiles.UserProfile
	etag     string
	winners  int // how many CAS wins are allowed
	attempts int // total UpdateZoneCountWithCAS calls so far
}

func newConcurrentFakeProfileCAS(p *profiles.UserProfile, maxWinners int) *concurrentFakeProfileCAS {
	return &concurrentFakeProfileCAS{profile: p, etag: "etag-0", winners: maxWinners}
}

func (f *concurrentFakeProfileCAS) GetWithETag(_ context.Context, _ string) (*profiles.UserProfile, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.profile == nil {
		return nil, "", nil
	}
	cp := *f.profile
	return &cp, f.etag, nil
}

func (f *concurrentFakeProfileCAS) UpdateZoneCountWithCAS(_ context.Context, _ string, p *profiles.UserProfile, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attempts++
	if f.winners <= 0 {
		return platform.ErrCASPreconditionFailed
	}
	f.winners--
	cp := *p
	f.profile = &cp
	f.etag = "etag-won"
	return nil
}

// TestCreate_ConcurrentCreatesRespectQuota drives two concurrent POST
// /v1/me/watch-zones by a Free (limit-1) user. The CAS fake allows exactly
// one winner. The test asserts that exactly one request returns 201 and the
// other returns 403.
func TestCreate_ConcurrentCreatesRespectQuota(t *testing.T) {
	t.Parallel()

	// Free tier limit = 1. User starts with 0 zones.
	p, err := profiles.NewProfile(testUser, "", nearbyNow)
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	casFake := newConcurrentFakeProfileCAS(p, 1 /* only one winner allowed */)

	d := nearbyDeps{
		store:    &fakeZoneStore{},
		profiles: &fakeProfileReader{profile: p},
		resolver: &fakeResolver{},
		apps:     &fakeAppFinder{},
		unread:   &fakeUnread{},
	}
	mux := newNearbyMuxWithCAS(t, d, casFake)

	body := `{"name":"My Zone","latitude":51.5,"longitude":-0.12,"radiusMetres":1000,"authorityId":471}`

	type result struct{ code int }
	results := make(chan result, 2)

	var wg sync.WaitGroup
	wg.Add(2)
	for range 2 {
		go func() {
			defer wg.Done()
			rec := doReq(t, mux, http.MethodPost, "/v1/me/watch-zones", body)
			results <- result{code: rec.Code}
		}()
	}
	wg.Wait()
	close(results)

	var created, forbidden int
	for r := range results {
		switch r.code {
		case http.StatusCreated:
			created++
		case http.StatusForbidden:
			forbidden++
		default:
			t.Errorf("unexpected status %d", r.code)
		}
	}
	if created != 1 || forbidden != 1 {
		t.Errorf("concurrent creates: got %d created, %d forbidden; want 1 and 1", created, forbidden)
	}
}

// TestCreate_DeleteFreesQuota creates a zone to the limit (1 for Free tier),
// deletes it, then asserts a subsequent create succeeds.
func TestCreate_DeleteFreesQuota(t *testing.T) {
	t.Parallel()

	// Free profile already at limit (counter = 1).
	count := 1
	p, err := profiles.NewProfile(testUser, "", nearbyNow)
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	p.WatchZoneCount = &count

	zone := mustZone(t, "zone-1", 471)
	store := &fakeZoneStore{zones: []WatchZone{zone}}
	casFake := newFakeProfileCAS(p)

	// Delete the zone: DELETE /v1/me/watch-zones/zone-1 must decrement the counter.
	deleteMux := newDeleteMuxWithCAS(t, store, casFake)
	delRec := doReq(t, deleteMux, http.MethodDelete, "/v1/me/watch-zones/zone-1", "")
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("delete: got %d, want 204 (body=%s)", delRec.Code, delRec.Body)
	}
	// Counter should now be 0.
	if casFake.profile.WatchZoneCount == nil || *casFake.profile.WatchZoneCount != 0 {
		t.Fatalf("after delete: WatchZoneCount = %v, want 0", casFake.profile.WatchZoneCount)
	}

	// Now create should succeed (counter 0 < limit 1).
	d := nearbyDeps{
		store:    &fakeZoneStore{}, // empty after the conceptual delete
		profiles: &fakeProfileReader{profile: casFake.profile},
		resolver: &fakeResolver{},
		apps:     &fakeAppFinder{},
		unread:   &fakeUnread{},
	}
	createMux := newNearbyMuxWithCAS(t, d, casFake)
	createBody := `{"name":"New Zone","latitude":51.5,"longitude":-0.12,"radiusMetres":1000,"authorityId":471}`
	createRec := doReq(t, createMux, http.MethodPost, "/v1/me/watch-zones", createBody)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create after delete: got %d, want 201 (body=%s)", createRec.Code, createRec.Body)
	}
}

// TestCreate_LegacyProfileLazyInit verifies that a legacy profile (nil
// WatchZoneCount) that already has 1 zone cannot create a 2nd zone under the
// Free tier limit of 1. The lazy-init path reads the live zone count from the
// store, initialises the counter, and then enforces the quota.
func TestCreate_LegacyProfileLazyInit(t *testing.T) {
	t.Parallel()

	// Legacy Free profile: WatchZoneCount is nil (written before the field existed).
	p, err := profiles.NewProfile(testUser, "", nearbyNow)
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	// p.WatchZoneCount is nil — legacy profile

	// Store already has 1 zone (at the free limit).
	existingZone := mustZone(t, "zone-existing", 326)
	store := &fakeZoneStore{zones: []WatchZone{existingZone}}
	casFake := newFakeProfileCAS(p)

	d := nearbyDeps{
		store:    store,
		profiles: &fakeProfileReader{profile: p},
		resolver: &fakeResolver{},
		apps:     &fakeAppFinder{},
		unread:   &fakeUnread{},
	}
	mux := newNearbyMuxWithCAS(t, d, casFake)

	body := `{"name":"Second Zone","latitude":51.5,"longitude":-0.12,"radiusMetres":1000,"authorityId":471}`
	rec := doReq(t, mux, http.MethodPost, "/v1/me/watch-zones", body)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("legacy profile at limit: got %d, want 403 (body=%s)", rec.Code, rec.Body)
	}
}

// TestCreate_FailsClosedWhenCASNotWired proves the create path fails closed
// (500) rather than silently running an unprotected quota check when the CAS
// store is absent. This guards against a wiring regression reintroducing the
// TOCTOU race: there is no non-atomic fallback create path.
func TestCreate_FailsClosedWhenCASNotWired(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	// NearbyRoutes WITHOUT WithProfileCAS — profileCAS is nil.
	NearbyRoutes(mux, &fakeZoneStore{}, &fakeProfileReader{profile: freeProfile(t)},
		&fakeResolver{}, &fakeAppFinder{}, &fakeUnread{},
		func() string { return "zone-x" }, func() time.Time { return nearbyNow },
		slog.New(slog.DiscardHandler))

	body := `{"name":"Z","latitude":51.5,"longitude":-0.12,"radiusMetres":1000,"authorityId":471}`
	rec := doReq(t, mux, http.MethodPost, "/v1/me/watch-zones", body)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("nil CAS must fail closed: got %d, want 500 (body=%s)", rec.Code, rec.Body)
	}
}

// newNearbyMuxWithCAS registers NearbyRoutes with the WithProfileCAS option.
func newNearbyMuxWithCAS(t *testing.T, d nearbyDeps, cas profileCAS) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	NearbyRoutes(mux, d.store, d.profiles, d.resolver, d.apps, d.unread,
		func() string { return "zone-cas-" + time.Now().Format("150405.000000000") },
		func() time.Time { return nearbyNow },
		slog.New(slog.DiscardHandler),
		WithProfileCAS(cas))
	return mux
}

// newDeleteMuxWithCAS registers Routes (delete path) with the WithProfileCAS option.
func newDeleteMuxWithCAS(t *testing.T, store zoneStore, cas profileCAS) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	Routes(mux, store, slog.New(slog.DiscardHandler), WithProfileCAS(cas))
	return mux
}
