// Package devseed mirrors a small slice of recently-changed prod planning
// applications into dev so a TestFlight build pointed at dev gets real push
// notifications to test against (bd tc-grvu.4, GH#808).
package devseed

import (
	"context"
	"errors"
	"io"
	"log/slog"
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
	return slog.New(slog.NewTextHandler(io.Discard, nil))
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
