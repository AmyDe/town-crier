// Package devseed mirrors a small slice of recently-changed prod planning
// applications into dev so a TestFlight build pointed at dev gets real push
// notifications to test against (bd tc-grvu.4, GH#808).
package devseed

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/polling"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// fakeZoneLister is a hand-written fake for zoneLister.
type fakeZoneLister struct {
	zones []watchzones.WatchZone
	err   error
}

func (f *fakeZoneLister) All(ctx context.Context) ([]watchzones.WatchZone, error) {
	return f.zones, f.err
}

// fakeProdReader is a hand-written fake for prodReader.
type fakeProdReader struct {
	apps []applications.PlanningApplication
	err  error

	calls    int
	gotZones []applications.ZoneGeometry
	gotLimit int
}

func (f *fakeProdReader) RecentNearZones(ctx context.Context, zones []applications.ZoneGeometry, limit int) ([]applications.PlanningApplication, error) {
	f.calls++
	f.gotZones = zones
	f.gotLimit = limit
	return f.apps, f.err
}

// testZone builds a validated watch zone at the given coordinates. authorityID
// is arbitrary and fixed at 1 -- devseed no longer reads it, but
// watchzones.NewWatchZone still requires it positive (that invariant is out
// of this bead's scope; see tc-9nbs4.2).
func testZone(t *testing.T, id string, lat, lon, radius float64) watchzones.WatchZone {
	t.Helper()
	z, err := watchzones.NewWatchZone(id, "user-1", "zone-"+id, lat, lon, radius, 1,
		time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC), true, false)
	if err != nil {
		t.Fatalf("NewWatchZone(%s): %v", id, err)
	}
	return z
}

// TestZoneGeometryOf_CustomShape exercises zoneGeometryOf's polygon branch
// directly, using a real WithBoundary zone rather than a circle -- PR #1077
// review feedback: the only prior coverage of a custom-shape ZoneGeometry was
// at the applications.RecentNearZones store level, via a directly-constructed
// ZoneGeometry that bypasses this exact translation, so a regression here
// (e.g. a swapped Longitude/Latitude field, or a boundary-shaped zone losing
// its ring) would have slipped through both suites.
func TestZoneGeometryOf_CustomShape(t *testing.T) {
	t.Parallel()

	base := testZone(t, "z1", 51.5, -0.12, 500)
	polygon, err := base.WithBoundary([]watchzones.Coordinate{
		{Longitude: -0.130, Latitude: 51.510},
		{Longitude: -0.110, Latitude: 51.510},
		{Longitude: -0.110, Latitude: 51.490},
	})
	if err != nil {
		t.Fatalf("WithBoundary: %v", err)
	}
	if !polygon.IsCustomShape() {
		t.Fatal("test setup: WithBoundary must produce a custom-shape zone")
	}

	got := zoneGeometryOf(polygon)

	if len(got.Boundary) != len(polygon.Boundary) {
		t.Fatalf("zoneGeometryOf Boundary length = %d, want %d (source ring, closed by NewBoundary)",
			len(got.Boundary), len(polygon.Boundary))
	}
	for i, v := range polygon.Boundary {
		want := applications.Coordinate{Longitude: v.Longitude, Latitude: v.Latitude}
		if got.Boundary[i] != want {
			t.Fatalf("zoneGeometryOf Boundary[%d] = %+v, want %+v (Longitude/Latitude must not be swapped)",
				i, got.Boundary[i], want)
		}
	}
	if got.Latitude != polygon.Latitude || got.Longitude != polygon.Longitude || got.RadiusMetres != polygon.RadiusMetres {
		t.Fatalf("zoneGeometryOf circle-fallback fields = {%v,%v,%v}, want {%v,%v,%v} (still populated from the derived centroid/enclosing radius, even for a custom-shape zone)",
			got.Latitude, got.Longitude, got.RadiusMetres, polygon.Latitude, polygon.Longitude, polygon.RadiusMetres)
	}
}

// fakePushFlusher is a hand-written fake for pushFlusher.
type fakePushFlusher struct {
	resetCalls int
	flushCalls int
	flushErr   error
}

func (f *fakePushFlusher) Reset() { f.resetCalls++ }

func (f *fakePushFlusher) Flush(ctx context.Context) error {
	f.flushCalls++
	return f.flushErr
}

// fakeAppStore is a hand-written fake for the Ingester's own applicationStore
// collaborator (polling.applicationStore), keyed by uid|authorityCode so
// GetByUID/Upsert honour the real (uid, authority) identity.
type fakeAppStore struct {
	existing       map[string]applications.PlanningApplication
	getErrByKey    map[string]error
	upserted       []applications.PlanningApplication
	upsertErrByKey map[string]error
}

func (f *fakeAppStore) GetByUID(ctx context.Context, uid, authorityCode string) (applications.PlanningApplication, bool, error) {
	k := uid + "|" + authorityCode
	if err, ok := f.getErrByKey[k]; ok {
		return applications.PlanningApplication{}, false, err
	}
	a, found := f.existing[k]
	return a, found, nil
}

func (f *fakeAppStore) Upsert(ctx context.Context, a applications.PlanningApplication) error {
	k := storeKey(a.UID, a.AreaID)
	if err, ok := f.upsertErrByKey[k]; ok {
		return err
	}
	if f.existing == nil {
		f.existing = map[string]applications.PlanningApplication{}
	}
	f.existing[k] = a
	f.upserted = append(f.upserted, a)
	return nil
}

// fakeDecisionDispatcher is a hand-written fake for polling.DecisionDispatcher.
type fakeDecisionDispatcher struct {
	calls []applications.PlanningApplication
	err   error
}

func (f *fakeDecisionDispatcher) Dispatch(ctx context.Context, app applications.PlanningApplication) error {
	f.calls = append(f.calls, app)
	return f.err
}

// fakeEnqueuer is a hand-written fake for polling.NotificationEnqueuer.
type fakeEnqueuer struct {
	calls []applications.PlanningApplication
	err   error
}

func (f *fakeEnqueuer) EnqueueForApplication(ctx context.Context, app applications.PlanningApplication) error {
	f.calls = append(f.calls, app)
	return f.err
}

// storeKey mirrors the real (uid, authority-code) identity used by
// applicationStore.GetByUID/Upsert in the polling package.
func storeKey(uid string, areaID int) string {
	return uid + "|" + strconv.Itoa(areaID)
}

func testLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func testApp(uid string, areaID int, description string) applications.PlanningApplication {
	return applications.PlanningApplication{
		Name:        "Test Application",
		UID:         uid,
		AreaName:    "Test Authority",
		AreaID:      areaID,
		Address:     "1 Test Street",
		Description: description,
	}
}

func TestSeeder_Run_NoWatchZones_NoOp(t *testing.T) {
	t.Parallel()

	zones := &fakeZoneLister{zones: nil}
	prod := &fakeProdReader{}
	push := &fakePushFlusher{}
	ingester := polling.NewIngester(&fakeAppStore{}, &fakeDecisionDispatcher{}, &fakeEnqueuer{})

	s := NewSeeder(zones, prod, ingester, push, 5, testLogger())

	count, err := s.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if count != 0 {
		t.Fatalf("Run() count = %d, want 0", count)
	}
	if prod.calls != 0 {
		t.Fatalf("RecentNearZones called %d times, want 0 (no-op before reaching prod)", prod.calls)
	}
	if push.resetCalls != 0 || push.flushCalls != 0 {
		t.Fatalf("push Reset/Flush = %d/%d, want 0/0 (no-op returns before touching push)", push.resetCalls, push.flushCalls)
	}
}

func TestSeeder_Run_ZonesListerError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("zones down")
	zones := &fakeZoneLister{err: wantErr}
	prod := &fakeProdReader{}
	push := &fakePushFlusher{}
	ingester := polling.NewIngester(&fakeAppStore{}, &fakeDecisionDispatcher{}, &fakeEnqueuer{})

	s := NewSeeder(zones, prod, ingester, push, 5, testLogger())

	count, err := s.Run(context.Background())

	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want wrapping %v", err, wantErr)
	}
	if count != 0 {
		t.Fatalf("Run() count = %d, want 0", count)
	}
	if prod.calls != 0 {
		t.Fatalf("RecentNearZones called %d times, want 0", prod.calls)
	}
	if push.resetCalls != 0 || push.flushCalls != 0 {
		t.Fatalf("push Reset/Flush = %d/%d, want 0/0", push.resetCalls, push.flushCalls)
	}
}

func TestSeeder_Run_ProdReaderError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("prod down")
	zones := &fakeZoneLister{zones: []watchzones.WatchZone{testZone(t, "z1", 51.5, -0.12, 500)}}
	prod := &fakeProdReader{err: wantErr}
	push := &fakePushFlusher{}
	ingester := polling.NewIngester(&fakeAppStore{}, &fakeDecisionDispatcher{}, &fakeEnqueuer{})

	s := NewSeeder(zones, prod, ingester, push, 5, testLogger())

	count, err := s.Run(context.Background())

	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want wrapping %v", err, wantErr)
	}
	if count != 0 {
		t.Fatalf("Run() count = %d, want 0", count)
	}
	if push.resetCalls != 0 || push.flushCalls != 0 {
		t.Fatalf("push Reset/Flush = %d/%d, want 0/0 (error occurs before Reset)", push.resetCalls, push.flushCalls)
	}
}

func TestSeeder_Run_ZeroAppsReturned_StillFlushesOnce(t *testing.T) {
	t.Parallel()

	zones := &fakeZoneLister{zones: []watchzones.WatchZone{testZone(t, "z1", 51.5, -0.12, 500)}}
	prod := &fakeProdReader{apps: nil}
	push := &fakePushFlusher{}
	ingester := polling.NewIngester(&fakeAppStore{}, &fakeDecisionDispatcher{}, &fakeEnqueuer{})

	s := NewSeeder(zones, prod, ingester, push, 5, testLogger())

	count, err := s.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if count != 0 {
		t.Fatalf("Run() count = %d, want 0", count)
	}
	if push.resetCalls != 1 || push.flushCalls != 1 {
		t.Fatalf("push Reset/Flush = %d/%d, want 1/1 (non-zero zones still flush, even with zero candidates)", push.resetCalls, push.flushCalls)
	}
}

func TestSeeder_Run_HappyPath_MixOfNewUnchangedChanged(t *testing.T) {
	t.Parallel()

	unchanged := testApp("unchanged-1", 100, "same description")
	changedOld := testApp("changed-1", 100, "old description")
	changedNew := testApp("changed-1", 100, "new description")
	newApp := testApp("new-1", 200, "brand new")

	appStore := &fakeAppStore{
		existing: map[string]applications.PlanningApplication{
			storeKey("unchanged-1", 100): unchanged,
			storeKey("changed-1", 100):   changedOld,
		},
	}
	enqueuer := &fakeEnqueuer{}
	decision := &fakeDecisionDispatcher{}
	ingester := polling.NewIngester(appStore, decision, enqueuer)

	zone1 := testZone(t, "z1", 51.5, -0.12, 500)
	zone2 := testZone(t, "z2", 52.2, -1.3, 750)
	zones := &fakeZoneLister{zones: []watchzones.WatchZone{zone1, zone2}}
	prod := &fakeProdReader{apps: []applications.PlanningApplication{unchanged, newApp, changedNew}}
	push := &fakePushFlusher{}

	s := NewSeeder(zones, prod, ingester, push, 5, testLogger())

	count, err := s.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if count != 3 {
		t.Fatalf("Run() count = %d, want 3 (all three processed without error)", count)
	}
	wantGeometries := []applications.ZoneGeometry{zoneGeometryOf(zone1), zoneGeometryOf(zone2)}
	if got := prod.gotZones; !reflect.DeepEqual(got, wantGeometries) {
		t.Fatalf("RecentNearZones zones = %+v, want %+v", got, wantGeometries)
	}
	if prod.gotLimit != 5 {
		t.Fatalf("RecentNearZones limit = %d, want 5", prod.gotLimit)
	}
	if push.resetCalls != 1 || push.flushCalls != 1 {
		t.Fatalf("push Reset/Flush = %d/%d, want 1/1", push.resetCalls, push.flushCalls)
	}
	if len(appStore.upserted) != 2 {
		t.Fatalf("upserted %d apps, want 2 (new + changed only; unchanged is a dedup no-op)", len(appStore.upserted))
	}
	if len(enqueuer.calls) != 2 {
		t.Fatalf("enqueuer called %d times, want 2 (new + changed only)", len(enqueuer.calls))
	}
	if len(decision.calls) != 0 {
		t.Fatalf("decision dispatcher called %d times, want 0 (no decision-state transitions in this fixture)", len(decision.calls))
	}
}

func TestSeeder_Run_IngestErrorDoesNotAbortBatch(t *testing.T) {
	t.Parallel()

	failing := testApp("fail-1", 100, "d")
	ok1 := testApp("ok-1", 100, "d1")
	ok2 := testApp("ok-2", 100, "d2")

	appStore := &fakeAppStore{
		getErrByKey: map[string]error{
			storeKey("fail-1", 100): errors.New("boom"),
		},
	}
	enqueuer := &fakeEnqueuer{}
	ingester := polling.NewIngester(appStore, &fakeDecisionDispatcher{}, enqueuer)

	zones := &fakeZoneLister{zones: []watchzones.WatchZone{testZone(t, "z1", 51.5, -0.12, 500)}}
	prod := &fakeProdReader{apps: []applications.PlanningApplication{ok1, failing, ok2}}
	push := &fakePushFlusher{}

	s := NewSeeder(zones, prod, ingester, push, 5, testLogger())

	count, err := s.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v, want nil (a single app's ingest error must not fail the cycle)", err)
	}
	if count != 2 {
		t.Fatalf("Run() count = %d, want 2 (one of three apps errored)", count)
	}
	if len(enqueuer.calls) != 2 {
		t.Fatalf("enqueuer called %d times, want 2 (the other two apps still processed)", len(enqueuer.calls))
	}
	if push.resetCalls != 1 || push.flushCalls != 1 {
		t.Fatalf("push Reset/Flush = %d/%d, want 1/1 (still flushed once despite the mid-batch error)", push.resetCalls, push.flushCalls)
	}
}

func TestSeeder_Run_FlushErrorIsSwallowed(t *testing.T) {
	t.Parallel()

	zones := &fakeZoneLister{zones: []watchzones.WatchZone{testZone(t, "z1", 51.5, -0.12, 500)}}
	prod := &fakeProdReader{apps: []applications.PlanningApplication{testApp("a1", 100, "d")}}
	push := &fakePushFlusher{flushErr: errors.New("push down")}
	ingester := polling.NewIngester(&fakeAppStore{}, &fakeDecisionDispatcher{}, &fakeEnqueuer{})

	s := NewSeeder(zones, prod, ingester, push, 5, testLogger())

	count, err := s.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v, want nil (a push flush problem must never fail the cycle)", err)
	}
	if count != 1 {
		t.Fatalf("Run() count = %d, want 1", count)
	}
	if push.flushCalls != 1 {
		t.Fatalf("push.flushCalls = %d, want 1", push.flushCalls)
	}
}
