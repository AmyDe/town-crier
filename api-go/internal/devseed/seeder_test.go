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

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/polling"
)

// fakeAuthorityLister is a hand-written fake for authorityLister.
type fakeAuthorityLister struct {
	ids []int
	err error
}

func (f *fakeAuthorityLister) DistinctAuthorityIDs(ctx context.Context) ([]int, error) {
	return f.ids, f.err
}

// fakeProdReader is a hand-written fake for prodReader.
type fakeProdReader struct {
	apps []applications.PlanningApplication
	err  error

	calls           int
	gotAuthorityIDs []int
	gotLimit        int
}

func (f *fakeProdReader) RecentInAuthorities(ctx context.Context, authorityIDs []int, limit int) ([]applications.PlanningApplication, error) {
	f.calls++
	f.gotAuthorityIDs = authorityIDs
	f.gotLimit = limit
	return f.apps, f.err
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

	zones := &fakeAuthorityLister{ids: nil}
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
		t.Fatalf("RecentInAuthorities called %d times, want 0 (no-op before reaching prod)", prod.calls)
	}
	if push.resetCalls != 0 || push.flushCalls != 0 {
		t.Fatalf("push Reset/Flush = %d/%d, want 0/0 (no-op returns before touching push)", push.resetCalls, push.flushCalls)
	}
}

func TestSeeder_Run_ZonesListerError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("zones down")
	zones := &fakeAuthorityLister{err: wantErr}
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
		t.Fatalf("RecentInAuthorities called %d times, want 0", prod.calls)
	}
	if push.resetCalls != 0 || push.flushCalls != 0 {
		t.Fatalf("push Reset/Flush = %d/%d, want 0/0", push.resetCalls, push.flushCalls)
	}
}

func TestSeeder_Run_ProdReaderError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("prod down")
	zones := &fakeAuthorityLister{ids: []int{100}}
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

	zones := &fakeAuthorityLister{ids: []int{100}}
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
		t.Fatalf("push Reset/Flush = %d/%d, want 1/1 (non-zero authorities still flush, even with zero candidates)", push.resetCalls, push.flushCalls)
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

	zones := &fakeAuthorityLister{ids: []int{100, 200}}
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
	if got, want := prod.gotAuthorityIDs, []int{100, 200}; !reflect.DeepEqual(got, want) {
		t.Fatalf("RecentInAuthorities authorityIDs = %v, want %v", got, want)
	}
	if prod.gotLimit != 5 {
		t.Fatalf("RecentInAuthorities limit = %d, want 5", prod.gotLimit)
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

	zones := &fakeAuthorityLister{ids: []int{100}}
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

	zones := &fakeAuthorityLister{ids: []int{100}}
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
