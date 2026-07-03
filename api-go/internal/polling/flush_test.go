package polling

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

// fakePushFlusher records Reset/Flush calls, standing in for the poll-cycle
// push coalescer (GH#784). onReset/onFlush are optional hooks a test can use to
// observe cycle state at the moment each method fires, proving ordering
// relative to the authority loop.
type fakePushFlusher struct {
	resetCalls int
	flushCalls int
	flushErr   error
	onReset    func()
	onFlush    func()
}

func (f *fakePushFlusher) Reset() {
	f.resetCalls++
	if f.onReset != nil {
		f.onReset()
	}
}

func (f *fakePushFlusher) Flush(_ context.Context) error {
	f.flushCalls++
	if f.onFlush != nil {
		f.onFlush()
	}
	return f.flushErr
}

func TestHandler_PushFlusher_ResetBeforeFetchAndFlushAfterLoop_NaturalEnd(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	pi.pages[pageKey{99, 1}] = planitPage(testApp("24/0001", 99, ld))
	apps := newFakeApps()
	state := newFakeStateStore()
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())

	var resetBeforeFetch, flushAfterUpsert bool
	flusher := &fakePushFlusher{}
	flusher.onReset = func() { resetBeforeFetch = pi.fetchCount() == 0 }
	flusher.onFlush = func() { flushAfterUpsert = len(apps.upserts) == 1 }
	h.WithPushFlusher(flusher)

	res, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if flusher.resetCalls != 1 {
		t.Errorf("Reset calls: got %d, want 1", flusher.resetCalls)
	}
	if flusher.flushCalls != 1 {
		t.Errorf("Flush calls: got %d, want 1", flusher.flushCalls)
	}
	if !resetBeforeFetch {
		t.Error("Reset must run before the authority loop starts fetching")
	}
	if !flushAfterUpsert {
		t.Error("Flush must run after the authority loop has upserted applications")
	}
	if res.TerminationReason != TerminationNatural {
		t.Errorf("termination: got %v, want Natural", res.TerminationReason)
	}
}

func TestHandler_PushFlusher_FlushesOn429(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	ra := 90 * time.Second
	pi.errs[99] = &planit.RateLimitError{RetryAfter: &ra}
	apps := newFakeApps()
	state := newFakeStateStore()
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())
	flusher := &fakePushFlusher{}
	h.WithPushFlusher(flusher)

	res, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if flusher.resetCalls != 1 || flusher.flushCalls != 1 {
		t.Errorf("reset/flush calls: reset=%d flush=%d, want 1/1", flusher.resetCalls, flusher.flushCalls)
	}
	if res.TerminationReason != TerminationRateLimited {
		t.Errorf("termination: got %v, want RateLimited", res.TerminationReason)
	}
}

func TestHandler_PushFlusher_FlushesOnCancelledContext(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	apps := newFakeApps()
	state := newFakeStateStore()
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99, 200}}, CycleSeed, defaultHandlerOpts())
	flusher := &fakePushFlusher{}
	h.WithPushFlusher(flusher)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before the loop starts — a budget/ctx-timeout cycle

	res, err := h.Handle(ctx)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if flusher.resetCalls != 1 || flusher.flushCalls != 1 {
		t.Errorf("reset/flush calls: reset=%d flush=%d, want 1/1", flusher.resetCalls, flusher.flushCalls)
	}
	if res.TerminationReason != TerminationTimeBounded {
		t.Errorf("termination: got %v, want TimeBounded", res.TerminationReason)
	}
}

func TestHandler_PushFlusher_FlushErrorIsSwallowed(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	pi.pages[pageKey{99, 1}] = planitPage(testApp("24/0001", 99, ld))
	apps := newFakeApps()
	state := newFakeStateStore()
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())
	flusher := &fakePushFlusher{flushErr: errors.New("apns down")}
	h.WithPushFlusher(flusher)

	res, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle must swallow a flush error, got %v", err)
	}
	if res.ApplicationCount != 1 || res.TerminationReason != TerminationNatural {
		t.Errorf("a flush error must not change the cycle result: %+v", res)
	}
}

func TestHandler_NoPushFlusherWired_RunsWithoutPanicOrFlush(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	pi.pages[pageKey{99, 1}] = planitPage(testApp("24/0001", 99, ld))
	apps := newFakeApps()
	state := newFakeStateStore()
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())

	if _, err := h.Handle(context.Background()); err != nil {
		t.Fatalf("ingestion-only mode (no flusher wired) must run without error: %v", err)
	}
}
