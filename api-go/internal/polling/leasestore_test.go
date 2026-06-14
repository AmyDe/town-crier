package polling

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// mustLeaseDoc builds a marshalled lease document expiring at the given instant,
// for priming the fake's stored document in acquire-race tests.
func mustLeaseDoc(t *testing.T, expiresAt time.Time) []byte {
	t.Helper()
	body, err := json.Marshal(leaseDocument{
		ID:            leaseDocumentID,
		HolderID:      "peer",
		AcquiredAtUTC: expiresAt.Add(-time.Minute).UTC().Format(dotNetRoundTrip),
		ExpiresAtUTC:  expiresAt.UTC().Format(dotNetRoundTrip),
	})
	if err != nil {
		t.Fatalf("marshal lease doc: %v", err)
	}
	return body
}

// fakeLeaseItems is a hand-written double for the lease store's CAS-aware Cosmos
// accessor. It simulates a single lease document with an etag, and can be primed
// to force conflicts on create/replace/delete so the tests can exercise the
// "held by peer" and "precondition failed" branches without a live Cosmos.
type fakeLeaseItems struct {
	doc  []byte
	etag string

	createConflict  bool // CreateItem returns ErrConflict (a peer created first)
	replaceMismatch bool // ReplaceItemWithETag returns ErrPreconditionFailed
	deleteMismatch  bool // DeleteItemWithETag returns ErrPreconditionFailed
	deleteNotFound  bool // DeleteItemWithETag returns ErrLeaseNotFound
	createErr       error
	nextETag        int
	deleteCalls     int
	createCalls     int
	replaceCalls    int
}

func newFakeLeaseItems() *fakeLeaseItems { return &fakeLeaseItems{} }

func (f *fakeLeaseItems) bump() string {
	f.nextETag++
	return "etag-" + time.Duration(f.nextETag).String()
}

func (f *fakeLeaseItems) ReadLeaseWithETag(_ context.Context, _ string) ([]byte, string, bool, error) {
	if f.doc == nil {
		return nil, "", false, nil
	}
	return f.doc, f.etag, true, nil
}

func (f *fakeLeaseItems) CreateLease(_ context.Context, _ string, item []byte) (string, error) {
	f.createCalls++
	if f.createErr != nil {
		return "", f.createErr
	}
	if f.createConflict {
		return "", ErrLeaseConflict
	}
	f.doc = item
	f.etag = f.bump()
	return f.etag, nil
}

func (f *fakeLeaseItems) ReplaceLeaseWithETag(_ context.Context, _ string, item []byte, _ string) (string, error) {
	f.replaceCalls++
	if f.replaceMismatch {
		return "", ErrLeasePreconditionFailed
	}
	f.doc = item
	f.etag = f.bump()
	return f.etag, nil
}

func (f *fakeLeaseItems) DeleteLeaseWithETag(_ context.Context, _, _ string) error {
	f.deleteCalls++
	if f.deleteNotFound {
		return ErrLeaseNotFound
	}
	if f.deleteMismatch {
		return ErrLeasePreconditionFailed
	}
	f.doc = nil
	return nil
}

func newLeaseStore(items leaseItems) *LeaseStore {
	clock := func() time.Time { return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC) }
	return NewLeaseStore(items, clock)
}

func TestLeaseStore_AcquireCreatesWhenAbsent(t *testing.T) {
	t.Parallel()
	items := newFakeLeaseItems()
	store := newLeaseStore(items)

	res, err := store.TryAcquire(context.Background(), 4*time.Minute)
	if err != nil {
		t.Fatalf("TryAcquire: %v", err)
	}
	if !res.Acquired {
		t.Fatalf("expected acquired, got %+v", res)
	}
	if res.Handle.ETag == "" {
		t.Error("expected a non-empty handle etag")
	}
	if items.createCalls != 1 {
		t.Errorf("create calls: got %d, want 1", items.createCalls)
	}
}

func TestLeaseStore_AcquireHeldWhenCreateConflicts(t *testing.T) {
	t.Parallel()
	// A peer created the lease first → CreateItem 409 → Held (not acquired).
	items := newFakeLeaseItems()
	items.createConflict = true
	store := newLeaseStore(items)

	res, err := store.TryAcquire(context.Background(), 4*time.Minute)
	if err != nil {
		t.Fatalf("TryAcquire: %v", err)
	}
	if res.Acquired {
		t.Error("expected not acquired (peer holds the lease)")
	}
	if !res.Held {
		t.Error("expected Held=true")
	}
}

func TestLeaseStore_AcquireHeldWhenLiveLeaseExists(t *testing.T) {
	t.Parallel()
	// An unexpired lease held by a peer → not acquired, no replace attempted.
	items := newFakeLeaseItems()
	store := newLeaseStore(items)
	// Lease expires 1 minute in the future relative to the fixed clock.
	items.doc = mustLeaseDoc(t, time.Date(2026, 6, 14, 12, 1, 0, 0, time.UTC))
	items.etag = "live"

	res, err := store.TryAcquire(context.Background(), 4*time.Minute)
	if err != nil {
		t.Fatalf("TryAcquire: %v", err)
	}
	if res.Acquired {
		t.Error("expected not acquired (live lease held by peer)")
	}
	if !res.Held {
		t.Error("expected Held=true")
	}
	if items.replaceCalls != 0 {
		t.Errorf("must not attempt replace on a live lease: replace calls=%d", items.replaceCalls)
	}
}

func TestLeaseStore_AcquireReplacesExpiredLease(t *testing.T) {
	t.Parallel()
	// An expired lease → replace with If-Match etag → acquired.
	items := newFakeLeaseItems()
	store := newLeaseStore(items)
	items.doc = mustLeaseDoc(t, time.Date(2026, 6, 14, 11, 59, 0, 0, time.UTC)) // expired
	items.etag = "stale"

	res, err := store.TryAcquire(context.Background(), 4*time.Minute)
	if err != nil {
		t.Fatalf("TryAcquire: %v", err)
	}
	if !res.Acquired {
		t.Errorf("expected acquired (expired lease replaced), got %+v", res)
	}
	if items.replaceCalls != 1 {
		t.Errorf("replace calls: got %d, want 1", items.replaceCalls)
	}
}

func TestLeaseStore_AcquireHeldWhenReplaceRacesLost(t *testing.T) {
	t.Parallel()
	// Expired lease, but a peer replaced it first → If-Match 412 → Held.
	items := newFakeLeaseItems()
	store := newLeaseStore(items)
	items.doc = mustLeaseDoc(t, time.Date(2026, 6, 14, 11, 59, 0, 0, time.UTC))
	items.etag = "stale"
	items.replaceMismatch = true

	res, err := store.TryAcquire(context.Background(), 4*time.Minute)
	if err != nil {
		t.Fatalf("TryAcquire: %v", err)
	}
	if res.Acquired {
		t.Error("expected not acquired (lost the replace race)")
	}
	if !res.Held {
		t.Error("expected Held=true")
	}
}

func TestLeaseStore_AcquireTransientErrorSurfacesAsResult(t *testing.T) {
	t.Parallel()
	items := newFakeLeaseItems()
	items.createErr = errors.New("network blip")
	store := newLeaseStore(items)

	res, err := store.TryAcquire(context.Background(), 4*time.Minute)
	if err != nil {
		t.Fatalf("TryAcquire must not return an error for a transient: %v", err)
	}
	if res.Acquired || res.Held {
		t.Errorf("transient should be neither acquired nor held: %+v", res)
	}
	if res.TransientErr == nil {
		t.Error("expected TransientErr to be set")
	}
}

func TestLeaseStore_ReleaseOutcomes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		setup func(*fakeLeaseItems)
		want  LeaseReleaseOutcome
	}{
		{"released", func(*fakeLeaseItems) {}, LeaseReleased},
		{"already gone", func(f *fakeLeaseItems) { f.deleteNotFound = true }, LeaseAlreadyGone},
		{"precondition failed", func(f *fakeLeaseItems) { f.deleteMismatch = true }, LeasePreconditionFailed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			items := newFakeLeaseItems()
			tc.setup(items)
			store := newLeaseStore(items)
			got := store.Release(context.Background(), LeaseHandle{ETag: "etag-x"})
			if got != tc.want {
				t.Errorf("Release outcome: got %v, want %v", got, tc.want)
			}
		})
	}
}
