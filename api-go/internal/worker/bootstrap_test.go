package worker

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/polling"
	"github.com/AmyDe/town-crier/api-go/internal/servicebus"
)

// fakeTriggerQueue is a hand-written double for the bootstrapper's consumer-side
// triggerQueue interface. It records publish calls and can be primed with a
// queue depth and/or errors.
type fakeTriggerQueue struct {
	depth        servicebus.QueueDepth
	depthErr     error
	publishErr   error
	publishCalls int
	publishedAt  time.Time
	publishedBig []byte

	// peeked/peekErr prime PeekMessages, used by the reconciler to decide which
	// trigger to keep when the queue has forked (GH#938 PR2).
	peeked    []servicebus.PeekedMessage
	peekErr   error
	peekCalls int

	// cancelErr primes CancelScheduled; cancelledSeqs records every sequence
	// number the reconciler asked to cancel, across all calls.
	cancelErr     error
	cancelCalls   int
	cancelledSeqs []int64

	// receiveResult/receiveErr prime ReceiveTrigger for the reconciler's
	// discard-surplus-active loop. The loop's iteration count is bounded by the
	// peeked snapshot, not by this fake counting down, so a fixed result per
	// call is enough to drive the tests.
	receiveResult bool
	receiveErr    error
	receiveCalls  int

	// dlqDrained/dlqErr prime DrainDeadLetters.
	dlqDrained int
	dlqErr     error
	dlqCalls   int
}

func (f *fakeTriggerQueue) QueueDepth(context.Context) (servicebus.QueueDepth, error) {
	if f.depthErr != nil {
		return servicebus.QueueDepth{}, f.depthErr
	}
	return f.depth, nil
}

func (f *fakeTriggerQueue) PublishAt(_ context.Context, at time.Time, body []byte) error {
	f.publishCalls++
	f.publishedAt = at
	f.publishedBig = body
	return f.publishErr
}

func (f *fakeTriggerQueue) PeekMessages(context.Context) ([]servicebus.PeekedMessage, error) {
	f.peekCalls++
	if f.peekErr != nil {
		return nil, f.peekErr
	}
	return f.peeked, nil
}

func (f *fakeTriggerQueue) CancelScheduled(_ context.Context, sequenceNumbers []int64) error {
	f.cancelCalls++
	f.cancelledSeqs = append(f.cancelledSeqs, sequenceNumbers...)
	return f.cancelErr
}

func (f *fakeTriggerQueue) ReceiveTrigger(context.Context) (bool, error) {
	f.receiveCalls++
	if f.receiveErr != nil {
		return false, f.receiveErr
	}
	return f.receiveResult, nil
}

func (f *fakeTriggerQueue) DrainDeadLetters(context.Context) (int, error) {
	f.dlqCalls++
	return f.dlqDrained, f.dlqErr
}

// fakeLeaseAccess is a hand-written double for the bootstrapper's consumer-side
// leaseAccess interface. It records acquire/release calls so tests can prove the
// lease-guard ordering (acquire before probe, release after publish) and the
// TTL requested on acquire.
type fakeLeaseAccess struct {
	acquireResult polling.LeaseAcquireResult
	acquireErr    error
	acquireCalls  int
	lastTTL       time.Duration

	releaseCalls   int
	releaseHandle  polling.LeaseHandle
	releaseOutcome polling.LeaseReleaseOutcome
}

func (f *fakeLeaseAccess) TryAcquire(_ context.Context, ttl time.Duration) (polling.LeaseAcquireResult, error) {
	f.acquireCalls++
	f.lastTTL = ttl
	return f.acquireResult, f.acquireErr
}

func (f *fakeLeaseAccess) Release(_ context.Context, handle polling.LeaseHandle) polling.LeaseReleaseOutcome {
	f.releaseCalls++
	f.releaseHandle = handle
	return f.releaseOutcome
}

// newAcquiredLeaseFake returns a fakeLeaseAccess primed to win the acquire
// immediately, the default every queue-behaviour test (predating the lease
// guard) implicitly relies on.
func newAcquiredLeaseFake() *fakeLeaseAccess {
	return &fakeLeaseAccess{
		acquireResult: polling.LeaseAcquireResult{Acquired: true, Handle: polling.LeaseHandle{ETag: "test-holder"}},
	}
}

func newTestBootstrapper(t *testing.T, q *fakeTriggerQueue) *Bootstrapper {
	t.Helper()
	return newTestBootstrapperWithLease(t, q, newAcquiredLeaseFake())
}

func newTestBootstrapperWithLease(t *testing.T, q *fakeTriggerQueue, lease *fakeLeaseAccess) *Bootstrapper {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	return NewBootstrapper(q, lease, logger, func() time.Time { return now })
}

func TestBootstrapper_PublishesSeedWhenQueueEmpty(t *testing.T) {
	t.Parallel()
	q := &fakeTriggerQueue{depth: servicebus.QueueDepth{}}
	b := newTestBootstrapper(t, q)

	res, err := b.TryBootstrap(context.Background())
	if err != nil {
		t.Fatalf("TryBootstrap: %v", err)
	}
	if !res.Published {
		t.Error("Published: got false, want true (empty queue should seed)")
	}
	if res.ProbeFailed {
		t.Error("ProbeFailed: got true, want false")
	}
	if q.publishCalls != 1 {
		t.Fatalf("publish calls: got %d, want exactly 1", q.publishCalls)
	}
	// The seed is scheduled in the future (jittered natural cadence), never
	// enqueued immediately.
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	if !q.publishedAt.After(now) {
		t.Errorf("publishedAt: got %v, want strictly after %v", q.publishedAt, now)
	}
	if len(q.publishedBig) == 0 {
		t.Error("published body is empty; want a diagnostic payload")
	}
}

func TestBootstrapper_SkipsWhenQueueNotEmpty(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		depth servicebus.QueueDepth
	}{
		{"active message present", servicebus.QueueDepth{ActiveMessageCount: 1}},
		{"scheduled message present", servicebus.QueueDepth{ScheduledMessageCount: 1}},
		{"both present", servicebus.QueueDepth{ActiveMessageCount: 2, ScheduledMessageCount: 3}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q := &fakeTriggerQueue{depth: tc.depth}
			b := newTestBootstrapper(t, q)

			res, err := b.TryBootstrap(context.Background())
			if err != nil {
				t.Fatalf("TryBootstrap: %v", err)
			}
			if res.Published {
				t.Error("Published: got true, want false (chain is alive)")
			}
			if q.publishCalls != 0 {
				t.Errorf("publish calls: got %d, want 0 (must not reseed a live chain)", q.publishCalls)
			}
		})
	}
}

// TestTryBootstrap_SkipsWhenLeaseHeld proves the fork-guard (GH#938 PR1): when
// the polling lease is held by a peer (the chain owner is alive), TryBootstrap
// must not probe the queue depth or publish a seed trigger — a bootstrap tick
// landing mid-cycle must never reseed a live chain. The result reports the skip
// via LeaseUnavailable.
func TestTryBootstrap_SkipsWhenLeaseHeld(t *testing.T) {
	t.Parallel()
	q := &fakeTriggerQueue{depth: servicebus.QueueDepth{}} // would otherwise seed: queue looks empty
	lease := &fakeLeaseAccess{acquireResult: polling.LeaseAcquireResult{Held: true}}
	b := newTestBootstrapperWithLease(t, q, lease)

	res, err := b.TryBootstrap(context.Background())
	if err != nil {
		t.Fatalf("TryBootstrap: %v", err)
	}
	if !res.LeaseUnavailable {
		t.Error("LeaseUnavailable: got false, want true (lease held by a peer)")
	}
	if res.Published || res.ProbeFailed {
		t.Errorf("result must report a clean skip, got %+v", res)
	}
	if q.publishCalls != 0 {
		t.Errorf("publish calls: got %d, want 0 (must not touch the queue while the lease is held)", q.publishCalls)
	}
	if lease.releaseCalls != 0 {
		t.Errorf("release calls: got %d, want 0 (never acquired, nothing to release)", lease.releaseCalls)
	}
}

// TestTryBootstrap_AcquiresAndReleasesLease proves the happy path: the lease is
// acquired before the depth probe runs at all (a fake that returns Held would
// otherwise make QueueDepth/PublishAt calls meaningless), the queue is only
// seeded when genuinely empty, and the lease is released exactly once
// afterwards regardless of whether a publish happened.
func TestTryBootstrap_AcquiresAndReleasesLease(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		depth       servicebus.QueueDepth
		wantPublish bool
	}{
		{"empty queue: acquires, publishes, releases", servicebus.QueueDepth{}, true},
		{"live chain: acquires, skips publish, still releases", servicebus.QueueDepth{ActiveMessageCount: 1}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q := &fakeTriggerQueue{depth: tc.depth}
			lease := newAcquiredLeaseFake()
			b := newTestBootstrapperWithLease(t, q, lease)

			res, err := b.TryBootstrap(context.Background())
			if err != nil {
				t.Fatalf("TryBootstrap: %v", err)
			}
			if res.Published != tc.wantPublish {
				t.Errorf("Published: got %v, want %v", res.Published, tc.wantPublish)
			}
			if lease.acquireCalls != 1 {
				t.Errorf("acquire calls: got %d, want 1", lease.acquireCalls)
			}
			if lease.lastTTL != bootstrapLeaseTTL {
				t.Errorf("acquire TTL: got %v, want %v (short TTL suffices for probe+publish)", lease.lastTTL, bootstrapLeaseTTL)
			}
			if lease.releaseCalls != 1 {
				t.Errorf("release calls: got %d, want 1 (must always release after acquiring)", lease.releaseCalls)
			}
			if lease.releaseHandle != lease.acquireResult.Handle {
				t.Errorf("release handle: got %+v, want the acquired handle %+v", lease.releaseHandle, lease.acquireResult.Handle)
			}
		})
	}
}

func TestBootstrapper_ProbeFailureIsAbsorbed(t *testing.T) {
	t.Parallel()
	q := &fakeTriggerQueue{depthErr: errors.New("network down")}
	b := newTestBootstrapper(t, q)

	res, err := b.TryBootstrap(context.Background())
	if err != nil {
		t.Fatalf("TryBootstrap should absorb probe failures, got err=%v", err)
	}
	if res.Published {
		t.Error("Published: got true, want false (probe failed)")
	}
	if !res.ProbeFailed {
		t.Error("ProbeFailed: got false, want true")
	}
	if q.publishCalls != 0 {
		t.Errorf("publish calls: got %d, want 0 (no publish after probe failure)", q.publishCalls)
	}
}

func TestBootstrapper_PublishFailureIsAbsorbed(t *testing.T) {
	t.Parallel()
	q := &fakeTriggerQueue{depth: servicebus.QueueDepth{}, publishErr: errors.New("send rejected")}
	b := newTestBootstrapper(t, q)

	res, err := b.TryBootstrap(context.Background())
	if err != nil {
		t.Fatalf("TryBootstrap should absorb publish failures, got err=%v", err)
	}
	if res.Published {
		t.Error("Published: got true, want false (publish failed)")
	}
	if !res.ProbeFailed {
		t.Error("ProbeFailed: got false, want true (a failed publish is recorded as a probe failure for telemetry parity)")
	}
}

// TestTryBootstrap_ReconcilesForkToSingleTrigger proves the GH#938 PR2
// reconciler: when the queue holds more than one live (active+scheduled)
// trigger, TryBootstrap collapses it back to exactly one, preferring the
// latest-activating scheduled message (Pre-Resolved Design Decision — it
// carries the most recent Retry-After knowledge), cancelling every other
// scheduled message and destructively discarding every surplus active one. It
// must never publish a new seed while reconciling a live fork.
func TestTryBootstrap_ReconcilesForkToSingleTrigger(t *testing.T) {
	t.Parallel()
	earlier := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	latest := time.Date(2026, 7, 12, 9, 5, 0, 0, time.UTC)
	q := &fakeTriggerQueue{
		depth: servicebus.QueueDepth{ActiveMessageCount: 1, ScheduledMessageCount: 2},
		peeked: []servicebus.PeekedMessage{
			{SequenceNumber: 10, State: servicebus.MessageStateActive},
			{SequenceNumber: 20, State: servicebus.MessageStateScheduled, ScheduledEnqueueTime: earlier},
			{SequenceNumber: 21, State: servicebus.MessageStateScheduled, ScheduledEnqueueTime: latest},
		},
		receiveResult: true,
	}
	b := newTestBootstrapper(t, q)

	res, err := b.TryBootstrap(context.Background())
	if err != nil {
		t.Fatalf("TryBootstrap: %v", err)
	}
	if !res.Reconciled {
		t.Error("Reconciled: got false, want true (queue held 3 live triggers)")
	}
	if res.Published {
		t.Error("Published: got true, want false (must never seed while reconciling a live fork)")
	}
	if q.publishCalls != 0 {
		t.Errorf("publish calls: got %d, want 0", q.publishCalls)
	}
	if res.ScheduledCancelled != 1 {
		t.Errorf("ScheduledCancelled: got %d, want 1 (the earlier of the two scheduled triggers)", res.ScheduledCancelled)
	}
	if len(q.cancelledSeqs) != 1 || q.cancelledSeqs[0] != 20 {
		t.Errorf("cancelledSeqs: got %v, want [20] (the earlier-activating scheduled message)", q.cancelledSeqs)
	}
	if res.ActiveDiscarded != 1 {
		t.Errorf("ActiveDiscarded: got %d, want 1 (the sole active message)", res.ActiveDiscarded)
	}
	if q.receiveCalls != 1 {
		t.Errorf("ReceiveTrigger calls: got %d, want 1", q.receiveCalls)
	}
}

// TestTryBootstrap_DrainsDeadLetterQueue proves TryBootstrap drains the
// trigger queue's dead-letter sub-queue on every cycle it runs (lease held),
// regardless of the active/scheduled outcome — dead-lettered corpses never
// self-clear (GH#938 PR2).
func TestTryBootstrap_DrainsDeadLetterQueue(t *testing.T) {
	t.Parallel()
	q := &fakeTriggerQueue{
		depth:      servicebus.QueueDepth{ActiveMessageCount: 1}, // healthy single trigger: no reseed, no reconcile
		dlqDrained: 4,
	}
	b := newTestBootstrapper(t, q)

	res, err := b.TryBootstrap(context.Background())
	if err != nil {
		t.Fatalf("TryBootstrap: %v", err)
	}
	if res.DeadLettered != 4 {
		t.Errorf("DeadLettered: got %d, want 4", res.DeadLettered)
	}
	if q.dlqCalls != 1 {
		t.Errorf("DrainDeadLetters calls: got %d, want 1", q.dlqCalls)
	}
	if res.Published || res.Reconciled {
		t.Errorf("a healthy single-trigger queue must not seed or reconcile, got %+v", res)
	}
}

// TestTryBootstrap_DeadLetterDrainFailureIsAbsorbed proves a DLQ drain failure
// never escalates to a caller-visible error — the next cron tick retries, same
// as every other absorbed failure in TryBootstrap.
func TestTryBootstrap_DeadLetterDrainFailureIsAbsorbed(t *testing.T) {
	t.Parallel()
	q := &fakeTriggerQueue{
		depth:  servicebus.QueueDepth{ActiveMessageCount: 1},
		dlqErr: errors.New("dlq receive failed"),
	}
	b := newTestBootstrapper(t, q)

	res, err := b.TryBootstrap(context.Background())
	if err != nil {
		t.Fatalf("TryBootstrap should absorb dead-letter drain failures, got err=%v", err)
	}
	if res.DeadLettered != 0 {
		t.Errorf("DeadLettered: got %d, want 0 (drain failed)", res.DeadLettered)
	}
}

// TestPickKeeper covers the pure reconciliation decision — which trigger
// survives a fork — in isolation from the queue and lease plumbing.
func TestPickKeeper(t *testing.T) {
	t.Parallel()
	earlier := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	latest := time.Date(2026, 7, 12, 9, 5, 0, 0, time.UTC)

	tests := []struct {
		name             string
		peeked           []servicebus.PeekedMessage
		wantKeepSeq      int64
		wantCancelSeqs   []int64
		wantDiscardCount int
	}{
		{
			name: "two scheduled plus one active: keeps latest-activating scheduled",
			peeked: []servicebus.PeekedMessage{
				{SequenceNumber: 10, State: servicebus.MessageStateActive},
				{SequenceNumber: 20, State: servicebus.MessageStateScheduled, ScheduledEnqueueTime: earlier},
				{SequenceNumber: 21, State: servicebus.MessageStateScheduled, ScheduledEnqueueTime: latest},
			},
			wantKeepSeq:      21,
			wantCancelSeqs:   []int64{20},
			wantDiscardCount: 1,
		},
		{
			name: "no scheduled: keeps the lowest-sequence active, discards the rest",
			peeked: []servicebus.PeekedMessage{
				{SequenceNumber: 30, State: servicebus.MessageStateActive},
				{SequenceNumber: 15, State: servicebus.MessageStateActive},
				{SequenceNumber: 22, State: servicebus.MessageStateActive},
			},
			wantKeepSeq:      15,
			wantCancelSeqs:   nil,
			wantDiscardCount: 2,
		},
		{
			name: "single scheduled: kept outright, nothing to cancel or discard",
			peeked: []servicebus.PeekedMessage{
				{SequenceNumber: 5, State: servicebus.MessageStateScheduled, ScheduledEnqueueTime: earlier},
			},
			wantKeepSeq:      5,
			wantCancelSeqs:   nil,
			wantDiscardCount: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			keep, cancelSeqs, discardCount := pickKeeper(tc.peeked)
			if keep.SequenceNumber != tc.wantKeepSeq {
				t.Errorf("keep.SequenceNumber: got %d, want %d", keep.SequenceNumber, tc.wantKeepSeq)
			}
			if !slices.Equal(cancelSeqs, tc.wantCancelSeqs) {
				t.Errorf("cancelSeqs: got %v, want %v", cancelSeqs, tc.wantCancelSeqs)
			}
			if discardCount != tc.wantDiscardCount {
				t.Errorf("discardCount: got %d, want %d", discardCount, tc.wantDiscardCount)
			}
		})
	}
}

func TestNextSeedDelay_JitteredWithinBounds(t *testing.T) {
	t.Parallel()
	// The natural cadence is 5 minutes; the seed is jittered up to the bound on
	// either side, never producing a non-positive delay.
	for range 200 {
		d := nextSeedDelay()
		if d < naturalCadence-jitterBound || d > naturalCadence+jitterBound {
			t.Fatalf("nextSeedDelay %v out of [%v, %v]", d, naturalCadence-jitterBound, naturalCadence+jitterBound)
		}
		if d <= 0 {
			t.Fatalf("nextSeedDelay must be positive, got %v", d)
		}
	}
}
