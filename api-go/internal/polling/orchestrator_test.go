package polling

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

// --- hand-written fakes for the orchestrator's dependencies ---

// fakeLease records acquire/release calls and can be primed to be held by a peer
// or to fail transiently. It also records the ORDER of lease vs publish calls via
// a shared call log so tests can assert publish-after-consume + release-last.
type fakeLease struct {
	log       *callLog
	held      bool
	transient bool
	acquireCt int
	releaseCt int
	released  bool

	// confirmDenied makes Confirm report false (lease lost before publish); the
	// zero value confirms, matching held/transient's "zero value = success" style.
	confirmDenied bool
	confirmCt     int
}

func (f *fakeLease) TryAcquire(_ context.Context, _ time.Duration) (LeaseAcquireResult, error) {
	f.acquireCt++
	if f.transient {
		return LeaseAcquireResult{TransientErr: errors.New("cosmos blip")}, nil
	}
	if f.held {
		return LeaseAcquireResult{Held: true}, nil
	}
	f.log.add("acquire")
	return LeaseAcquireResult{Acquired: true, Handle: LeaseHandle{ETag: "e1"}}, nil
}

func (f *fakeLease) Release(_ context.Context, _ LeaseHandle) LeaseReleaseOutcome {
	f.releaseCt++
	f.released = true
	f.log.add("release")
	return LeaseReleased
}

func (f *fakeLease) Confirm(_ context.Context, _ LeaseHandle, _ time.Duration) bool {
	f.confirmCt++
	if f.confirmDenied {
		return false
	}
	f.log.add("confirm")
	return true
}

// fakeReceiver hands out one trigger (or none) in receive-and-delete mode. There
// is no settle method — the message is destructively consumed on receive.
type fakeReceiver struct {
	log       *callLog
	hasMsg    bool
	receiveCt int
	err       error
}

func (f *fakeReceiver) ReceiveTrigger(_ context.Context) (bool, error) {
	f.receiveCt++
	if f.err != nil {
		return false, f.err
	}
	if !f.hasMsg {
		return false, nil
	}
	f.log.add("receive")
	return true, nil
}

// fakePublisher records the scheduled enqueue time of the next trigger.
type fakePublisher struct {
	log         *callLog
	publishCt   int
	publishedAt time.Time
	err         error
}

func (f *fakePublisher) PublishAt(_ context.Context, at time.Time, _ []byte) error {
	f.publishCt++
	if f.err != nil {
		return f.err
	}
	f.publishedAt = at
	f.log.add("publish")
	return nil
}

// fakeCycleHandler stands in for the ingestion handler.
type fakeCycleHandler struct {
	log    *callLog
	result PollPlanItResult
	err    error
}

func (f *fakeCycleHandler) Handle(_ context.Context) (PollPlanItResult, error) {
	f.log.add("handle")
	return f.result, f.err
}

// callLog records the order of side-effecting calls across fakes.
type callLog struct{ events []string }

func (c *callLog) add(e string) { c.events = append(c.events, e) }

func newOrchestrator(t *testing.T, lease *fakeLease, recv *fakeReceiver, pub *fakePublisher, handler *fakeCycleHandler) *Orchestrator {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(discard{}, nil))
	clock := func() time.Time { return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC) }
	scheduler := NewNextRunScheduler(DefaultSchedulerOptions(), zeroJitter{})
	return NewOrchestrator(handler, recv, pub, lease, scheduler, OrchestratorOptions{LeaseTTL: 4*time.Minute + 30*time.Second}, clock, logger)
}

func newWiredFakes() (*callLog, *fakeLease, *fakeReceiver, *fakePublisher, *fakeCycleHandler) {
	log := &callLog{}
	return log,
		&fakeLease{log: log},
		&fakeReceiver{log: log, hasMsg: true},
		&fakePublisher{log: log},
		&fakeCycleHandler{log: log, result: PollPlanItResult{ApplicationCount: 5, TerminationReason: TerminationNatural}}
}

// --- tests ---

func TestOrchestrator_HappyPath_AcquireReceiveHandlePublishRelease(t *testing.T) {
	t.Parallel()
	log, lease, recv, pub, handler := newWiredFakes()
	o := newOrchestrator(t, lease, recv, pub, handler)

	res, err := o.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !res.MessageReceived || !res.PublishedNext {
		t.Errorf("result: %+v", res)
	}
	// Ordering MUST be acquire -> receive -> handle -> confirm -> publish -> release.
	want := []string{"acquire", "receive", "handle", "confirm", "publish", "release"}
	if got := log.events; !equalStrings(got, want) {
		t.Errorf("call order: got %v, want %v", got, want)
	}
}

func TestOrchestrator_PublishHappensBeforeRelease(t *testing.T) {
	t.Parallel()
	// Crash-safety per ADR 0024: publish-after-consume, and the lease is released
	// only after the next trigger is published. Assert publish precedes release.
	log, lease, recv, pub, handler := newWiredFakes()
	o := newOrchestrator(t, lease, recv, pub, handler)

	if _, err := o.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	pubIdx, relIdx := indexOf(log.events, "publish"), indexOf(log.events, "release")
	if pubIdx == -1 || relIdx == -1 || pubIdx > relIdx {
		t.Errorf("publish must precede release: events=%v", log.events)
	}
}

func TestOrchestrator_LeaseUnavailableExitsWithoutReceiveOrPublish(t *testing.T) {
	t.Parallel()
	// A peer holds the lease (after the in-method retry) → exit cleanly without
	// touching the queue. This is what prevents two pollers double-polling PlanIt.
	log, lease, recv, pub, handler := newWiredFakes()
	lease.held = true
	o := newOrchestrator(t, lease, recv, pub, handler)

	res, err := o.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !res.LeaseUnavailable {
		t.Error("expected LeaseUnavailable=true")
	}
	if res.MessageReceived || res.PublishedNext {
		t.Errorf("must not receive/publish when lease unavailable: %+v", res)
	}
	if recv.receiveCt != 0 || pub.publishCt != 0 {
		t.Errorf("no queue ops when lease unavailable: receive=%d publish=%d", recv.receiveCt, pub.publishCt)
	}
	if len(log.events) != 0 {
		t.Errorf("no side effects when lease unavailable: %v", log.events)
	}
}

func TestOrchestrator_EmptyQueueExitsCleanlyAndReleasesLease(t *testing.T) {
	t.Parallel()
	// No trigger message → run nothing, publish nothing, but still release the
	// lease (bootstrap will re-seed).
	log, lease, recv, pub, handler := newWiredFakes()
	recv.hasMsg = false
	o := newOrchestrator(t, lease, recv, pub, handler)

	res, err := o.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if res.MessageReceived || res.PublishedNext {
		t.Errorf("empty queue should not receive/publish: %+v", res)
	}
	if handler.log != nil && indexOf(log.events, "handle") != -1 {
		t.Error("handler must not run on an empty queue")
	}
	if !lease.released {
		t.Error("lease must be released even on an empty queue")
	}
}

func TestOrchestrator_RateLimitedResultDrivesScheduledEnqueue(t *testing.T) {
	t.Parallel()
	log, lease, recv, pub, _ := newWiredFakes()
	ra := 90 * time.Second
	handler := &fakeCycleHandler{
		log:    log,
		result: PollPlanItResult{ApplicationCount: 2, RateLimited: true, TerminationReason: TerminationRateLimited, RetryAfter: &ra},
	}
	o := newOrchestrator(t, lease, recv, pub, handler)

	if _, err := o.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	// Next enqueue = now + retryAfter (zero jitter): 12:00:00 + 90s = 12:01:30.
	want := time.Date(2026, 6, 14, 12, 1, 30, 0, time.UTC)
	if !pub.publishedAt.Equal(want) {
		t.Errorf("scheduled enqueue: got %v, want %v", pub.publishedAt, want)
	}
}

func TestOrchestrator_NaturalResultUsesNaturalCadence(t *testing.T) {
	t.Parallel()
	_, lease, recv, pub, handler := newWiredFakes()
	o := newOrchestrator(t, lease, recv, pub, handler)

	if _, err := o.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	// Natural cadence (5m): 12:00:00 + 5m = 12:05:00.
	want := time.Date(2026, 6, 14, 12, 5, 0, 0, time.UTC)
	if !pub.publishedAt.Equal(want) {
		t.Errorf("scheduled enqueue: got %v, want %v", pub.publishedAt, want)
	}
}

func TestOrchestrator_ReleasesLeaseEvenWhenHandlerFails(t *testing.T) {
	t.Parallel()
	log, lease, recv, pub, _ := newWiredFakes()
	handler := &fakeCycleHandler{log: log, err: errors.New("ingestion blew up")}
	o := newOrchestrator(t, lease, recv, pub, handler)

	_, err := o.RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected handler error to surface")
	}
	if !lease.released {
		t.Error("lease must be released even when the handler errors")
	}
	// A handler failure must NOT publish a next trigger (no scheduled enqueue from
	// a failed cycle) — the bootstrap recovers the chain.
	if pub.publishCt != 0 {
		t.Errorf("must not publish after a handler failure: publish=%d", pub.publishCt)
	}
}

// TestRunOnce_SkipsPublishWhenLeaseLost proves the fork-guard (GH#938 PR1): when
// Confirm reports the lease is no longer held (expired mid-cycle and taken by a
// peer), RunOnce must never call PublishAt — publishing here would fork the
// trigger chain. Losing the chain instead is safe (the bootstrap reseeds).
func TestRunOnce_SkipsPublishWhenLeaseLost(t *testing.T) {
	t.Parallel()
	log, lease, recv, pub, handler := newWiredFakes()
	lease.confirmDenied = true
	o := newOrchestrator(t, lease, recv, pub, handler)

	res, err := o.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce must not error when the lease is lost before publish: %v", err)
	}
	if res.PublishedNext {
		t.Error("PublishedNext: got true, want false (lease lost; must not publish)")
	}
	if !res.MessageReceived {
		t.Error("MessageReceived: got false, want true (the trigger was still consumed)")
	}
	if pub.publishCt != 0 {
		t.Errorf("publish calls: got %d, want 0 (a lost lease must skip publish entirely)", pub.publishCt)
	}
	if lease.confirmCt != 1 {
		t.Errorf("confirm calls: got %d, want 1", lease.confirmCt)
	}
	if idx := indexOf(log.events, "publish"); idx != -1 {
		t.Errorf("publish must not appear in the call log: %v", log.events)
	}
	// The lease is still released even though publish was skipped — TTL is a
	// backstop, not the only release path.
	if !lease.released {
		t.Error("lease must still be released after a skipped publish")
	}
}

// TestRunOnce_PublishesWhenLeaseConfirmed proves the companion happy path: when
// Confirm reports the lease is still held (and extends it), RunOnce publishes
// the next trigger exactly once.
func TestRunOnce_PublishesWhenLeaseConfirmed(t *testing.T) {
	t.Parallel()
	_, lease, recv, pub, handler := newWiredFakes()
	o := newOrchestrator(t, lease, recv, pub, handler)

	res, err := o.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !res.PublishedNext {
		t.Error("PublishedNext: got false, want true (lease confirmed)")
	}
	if pub.publishCt != 1 {
		t.Errorf("publish calls: got %d, want exactly 1", pub.publishCt)
	}
	if lease.confirmCt != 1 {
		t.Errorf("confirm calls: got %d, want 1", lease.confirmCt)
	}
}

func TestOrchestrator_LeaseTransientErrorExitsCleanly(t *testing.T) {
	t.Parallel()
	_, lease, recv, pub, handler := newWiredFakes()
	lease.transient = true
	o := newOrchestrator(t, lease, recv, pub, handler)

	res, err := o.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if res.MessageReceived || res.PublishedNext {
		t.Errorf("transient lease error must not touch the queue: %+v", res)
	}
	if recv.receiveCt != 0 {
		t.Errorf("no receive on transient lease error: %d", recv.receiveCt)
	}
}

// --- helpers ---

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func indexOf(s []string, v string) int {
	for i, e := range s {
		if e == v {
			return i
		}
	}
	return -1
}
