package polling

import (
	"context"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

// fakePollMetrics records every metrics call the handler makes so a test can
// assert on the recorded business metrics without a real OTel pipeline. It
// satisfies the polling package's consumer-side metricsRecorder interface.
type fakePollMetrics struct {
	authoritiesPolled  int
	authoritiesSkipped int
	applicationsIngest int
	rateLimited        int
	retryAfter         []float64
	authorityProcMs    int
	authorityTotals    []int
	cyclesCompleted    []string // termination per completed cycle
	cursorAdvanced     int
	cursorCleared      int
	oldestHwmCalls     int
	neverPolledCalls   int
}

func (f *fakePollMetrics) AuthorityPolled(context.Context, string)  { f.authoritiesPolled++ }
func (f *fakePollMetrics) AuthoritySkipped(context.Context, string) { f.authoritiesSkipped++ }
func (f *fakePollMetrics) ApplicationsIngested(_ context.Context, n int, _ string) {
	f.applicationsIngest += n
}
func (f *fakePollMetrics) RateLimited(context.Context, string) { f.rateLimited++ }
func (f *fakePollMetrics) RetryAfterSeconds(_ context.Context, s float64, _ string, _ int, _ bool) {
	f.retryAfter = append(f.retryAfter, s)
}
func (f *fakePollMetrics) AuthorityProcessingMillis(context.Context, float64, string) {
	f.authorityProcMs++
}
func (f *fakePollMetrics) AuthorityTotal(_ context.Context, total int, _ string, _ int) {
	f.authorityTotals = append(f.authorityTotals, total)
}
func (f *fakePollMetrics) CycleCompleted(_ context.Context, _ string, termination string) {
	f.cyclesCompleted = append(f.cyclesCompleted, termination)
}
func (f *fakePollMetrics) CursorAdvanced(context.Context, string) { f.cursorAdvanced++ }
func (f *fakePollMetrics) CursorCleared(context.Context, string)  { f.cursorCleared++ }
func (f *fakePollMetrics) OldestHighWaterMarkAge(context.Context, float64, string, int, bool) {
	f.oldestHwmCalls++
}
func (f *fakePollMetrics) NeverPolledCount(context.Context, int, string) { f.neverPolledCalls++ }

func TestHandler_RecordsNaturalCycleMetrics(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	apps := newFakeApps()
	state := newFakeStateStore()
	total := 2
	pi.pages[pageKey{authority: 99, index: 0}] = planit.FetchPageResult{
		From:         0,
		Total:        &total,
		Applications: []applications.PlanningApplication{testApp("App", 99, time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC))},
		HasMorePages: false,
	}

	rec := &fakePollMetrics{}
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())
	h.WithMetrics(rec)

	res, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if res.ApplicationCount != 1 {
		t.Fatalf("ApplicationCount = %d, want 1", res.ApplicationCount)
	}

	if rec.authoritiesPolled != 1 {
		t.Errorf("AuthorityPolled = %d, want 1", rec.authoritiesPolled)
	}
	if rec.applicationsIngest != 1 {
		t.Errorf("ApplicationsIngested = %d, want 1", rec.applicationsIngest)
	}
	if rec.authorityProcMs != 1 {
		t.Errorf("AuthorityProcessingMillis calls = %d, want 1", rec.authorityProcMs)
	}
	if len(rec.authorityTotals) != 1 || rec.authorityTotals[0] != 2 {
		t.Errorf("AuthorityTotal = %v, want [2]", rec.authorityTotals)
	}
	if len(rec.cyclesCompleted) != 1 || rec.cyclesCompleted[0] != "Natural" {
		t.Errorf("CycleCompleted = %v, want [Natural]", rec.cyclesCompleted)
	}
	if rec.oldestHwmCalls != 1 {
		t.Errorf("OldestHighWaterMarkAge calls = %d, want 1", rec.oldestHwmCalls)
	}
	if rec.neverPolledCalls != 1 {
		t.Errorf("NeverPolledCount calls = %d, want 1", rec.neverPolledCalls)
	}
}

func TestHandler_RecordsRateLimitMetrics(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	apps := newFakeApps()
	state := newFakeStateStore()
	retryAfter := 30 * time.Second
	pi.errs[99] = &planit.RateLimitError{RetryAfter: &retryAfter}

	rec := &fakePollMetrics{}
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleWatched, defaultHandlerOpts())
	h.WithMetrics(rec)

	res, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !res.RateLimited {
		t.Fatal("expected RateLimited result")
	}
	if rec.rateLimited != 1 {
		t.Errorf("RateLimited = %d, want 1", rec.rateLimited)
	}
	if len(rec.retryAfter) != 1 || rec.retryAfter[0] != 30 {
		t.Errorf("RetryAfterSeconds = %v, want [30]", rec.retryAfter)
	}
	if len(rec.cyclesCompleted) != 1 || rec.cyclesCompleted[0] != "RateLimited" {
		t.Errorf("CycleCompleted = %v, want [RateLimited]", rec.cyclesCompleted)
	}
}

func TestHandler_RecordsCursorAdvancedOnCapHit(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	apps := newFakeApps()
	state := newFakeStateStore()
	// Two pages with more remaining, but a cap of 1 forces a cursor save.
	pi.pages[pageKey{authority: 99, index: 0}] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{testApp("A", 99, time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC))},
		HasMorePages: true,
	}
	one := 1
	opts := HandlerOptions{MaxPagesPerAuthorityPerCycle: &one, HandlerBudget: 4 * time.Minute}

	rec := &fakePollMetrics{}
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, opts)
	h.WithMetrics(rec)

	if _, err := h.Handle(context.Background()); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if rec.cursorAdvanced != 1 {
		t.Errorf("CursorAdvanced = %d, want 1", rec.cursorAdvanced)
	}
}

// fakeLeaseMetrics records lease-acquired calls; it satisfies the orchestrator's
// consumer-side leaseMetricsRecorder interface.
type fakeLeaseMetrics struct {
	acquiredCallers []string
}

func (f *fakeLeaseMetrics) LeaseAcquired(_ context.Context, caller string) {
	f.acquiredCallers = append(f.acquiredCallers, caller)
}

func TestOrchestrator_RecordsLeaseAcquired(t *testing.T) {
	t.Parallel()
	_, lease, recv, pub, handler := newWiredFakes()
	o := newOrchestrator(t, lease, recv, pub, handler)
	rec := &fakeLeaseMetrics{}
	o.WithLeaseMetrics(rec)

	if _, err := o.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if len(rec.acquiredCallers) != 1 || rec.acquiredCallers[0] != "orchestrator" {
		t.Errorf("LeaseAcquired callers = %v, want [orchestrator]", rec.acquiredCallers)
	}
}

func TestOrchestrator_DoesNotRecordLeaseAcquiredWhenHeld(t *testing.T) {
	t.Parallel()
	_, lease, recv, pub, handler := newWiredFakes()
	lease.held = true
	o := newOrchestrator(t, lease, recv, pub, handler)
	rec := &fakeLeaseMetrics{}
	o.WithLeaseMetrics(rec)

	if _, err := o.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(rec.acquiredCallers) != 0 {
		t.Errorf("LeaseAcquired must not fire when the lease is held: %v", rec.acquiredCallers)
	}
}

func TestHandler_NilMetricsRecorderIsNoOp(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	apps := newFakeApps()
	state := newFakeStateStore()
	pi.pages[pageKey{authority: 99, index: 0}] = planitPage(testApp("App", 99, time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC)))

	// No WithMetrics call: the handler must record nothing and not panic.
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())
	if _, err := h.Handle(context.Background()); err != nil {
		t.Fatalf("Handle without metrics recorder: %v", err)
	}
}
