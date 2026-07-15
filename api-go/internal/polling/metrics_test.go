package polling

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

// fakePollMetrics records every metrics call the handler makes so a test can
// assert on the recorded business metrics without a real OTel pipeline. It
// satisfies the polling package's consumer-side metricsRecorder interface.
// The *Tags slices capture the cycleType/lane tag passed on each call (in
// call order), which the ADR 0041 lane handlers' tests need to assert on —
// unlike the older per-authority tags, a national lane's tag IS the fact
// under test (which lane produced the metric).
type fakePollMetrics struct {
	authoritiesPolled      int
	authoritiesSkipped     int
	applicationsIngest     int
	applicationsIngestTags []string
	rateLimited            int
	rateLimitedTags        []string
	retryAfter             []float64
	retryAfterTags         []string
	authorityProcMs        int
	authorityTotals        []int
	cyclesCompleted        []string // termination per completed cycle
	cyclesCompletedTags    []string // cycleType per completed cycle
	cursorAdvanced         int
	cursorCleared          int
	oldestHwmCalls         int
	neverPolledCalls       int
}

func (f *fakePollMetrics) AuthorityPolled(context.Context, string)  { f.authoritiesPolled++ }
func (f *fakePollMetrics) AuthoritySkipped(context.Context, string) { f.authoritiesSkipped++ }
func (f *fakePollMetrics) ApplicationsIngested(_ context.Context, n int, cycleType string) {
	f.applicationsIngest += n
	f.applicationsIngestTags = append(f.applicationsIngestTags, cycleType)
}
func (f *fakePollMetrics) RateLimited(_ context.Context, cycleType string) {
	f.rateLimited++
	f.rateLimitedTags = append(f.rateLimitedTags, cycleType)
}
func (f *fakePollMetrics) RetryAfterSeconds(_ context.Context, s float64, cycleType string, _ int, _ bool) {
	f.retryAfter = append(f.retryAfter, s)
	f.retryAfterTags = append(f.retryAfterTags, cycleType)
}
func (f *fakePollMetrics) AuthorityProcessingMillis(context.Context, float64, string) {
	f.authorityProcMs++
}
func (f *fakePollMetrics) AuthorityTotal(_ context.Context, total int, _ string, _ int) {
	f.authorityTotals = append(f.authorityTotals, total)
}
func (f *fakePollMetrics) CycleCompleted(_ context.Context, cycleType string, termination string) {
	f.cyclesCompleted = append(f.cyclesCompleted, termination)
	f.cyclesCompletedTags = append(f.cyclesCompletedTags, cycleType)
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

// TestNationalLaneRun_RecordsApplicationsIngestedWithLaneTag covers the ADR
// 0041 lane wiring (tc-7ef9g): a national lane's Run records
// ApplicationsIngested tagged with its own lane letter ("A"/"B") — there is
// no Watched/Seed cycle concept on this code path, so the lane IS the tag.
// The oldest-HWM staleness gauge should fire too (asserted via the existing
// call-count field; the value/tag detail isn't asserted here).
func TestNationalLaneRun_RecordsApplicationsIngestedWithLaneTag(t *testing.T) {
	t.Parallel()
	watermark := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	newer := watermark.Add(1 * time.Hour)

	fetcher := newFakeNationalFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{testApp("newer", 300, newer)},
		HasMorePages: false,
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: watermark}

	rec := &fakePollMetrics{}
	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())
	h.WithMetrics(rec)

	out := h.Run(context.Background())
	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	if rec.applicationsIngest != 1 {
		t.Errorf("ApplicationsIngested total = %d, want 1", rec.applicationsIngest)
	}
	if len(rec.applicationsIngestTags) != 1 || rec.applicationsIngestTags[0] != "A" {
		t.Errorf("ApplicationsIngested tags = %v, want [A]", rec.applicationsIngestTags)
	}
	if rec.oldestHwmCalls != 1 {
		t.Errorf("OldestHighWaterMarkAge calls = %d, want 1 (a clean run always ends with a non-zero watermark)", rec.oldestHwmCalls)
	}
}

// TestNationalLaneRun_RecordsRateLimitMetrics covers a lane hitting a 429
// mid-walk: RateLimited and RetryAfterSeconds must both fire, tagged with the
// lane letter, mirroring PollPlanItHandler.recordRetryAfter.
func TestNationalLaneRun_RecordsRateLimitMetrics(t *testing.T) {
	t.Parallel()
	watermark := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	page1LD := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	retryAfter := 30 * time.Second

	fetcher := newFakeNationalFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{testApp("page1", 300, page1LD)},
		HasMorePages: true,
	}
	fetcher.failNth[2] = &planit.RateLimitError{RetryAfter: &retryAfter}

	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: watermark}

	rec := &fakePollMetrics{}
	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())
	h.WithMetrics(rec)

	out := h.Run(context.Background())
	if !out.rateLimited {
		t.Fatal("expected rateLimited=true")
	}
	if rec.rateLimited != 1 {
		t.Errorf("RateLimited calls = %d, want 1", rec.rateLimited)
	}
	if len(rec.rateLimitedTags) != 1 || rec.rateLimitedTags[0] != "A" {
		t.Errorf("RateLimited tags = %v, want [A]", rec.rateLimitedTags)
	}
	if len(rec.retryAfter) != 1 || rec.retryAfter[0] != 30 {
		t.Errorf("RetryAfterSeconds = %v, want [30]", rec.retryAfter)
	}
	if len(rec.retryAfterTags) != 1 || rec.retryAfterTags[0] != "A" {
		t.Errorf("RetryAfterSeconds tags = %v, want [A]", rec.retryAfterTags)
	}
}

// TestNationalLane_NilMetricsRecorderIsNoOp mirrors
// TestHandler_NilMetricsRecorderIsNoOp for a national lane: no WithMetrics
// call must mean no panic (the noopMetrics fallback).
func TestNationalLane_NilMetricsRecorderIsNoOp(t *testing.T) {
	t.Parallel()
	watermark := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	fetcher := newFakeNationalFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{testApp("a", 300, watermark.Add(time.Hour))},
		HasMorePages: false,
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: watermark}

	// No WithMetrics call: Run must not panic.
	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())
	if out := h.Run(context.Background()); out.err != nil {
		t.Fatalf("Run without metrics recorder: %v", out.err)
	}
}

// TestReconciliationRun_RecordsApplicationsIngestedWithHydratedCount covers
// Lane C: Run records ApplicationsIngested tagged "C" with the count of
// stragglers actually hydrated this sweep.
func TestReconciliationRun_RecordsApplicationsIngestedWithHydratedCount(t *testing.T) {
	t.Parallel()
	ld := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	fetcher := newFakeReconciliationFetcher()
	fetcher.pages[99] = planit.FetchPageResult{
		Applications: []applications.PlanningApplication{lightApp("24/0001/FUL", 99, "Permitted", ld)},
	}
	full := testApp("24/0001", 99, ld)
	full.UID = "24/0001/FUL"
	permitted := "Permitted"
	full.AppState = &permitted
	fetcher.hydrated["24/0001/FUL"] = full

	apps := newFakeApps()
	undecided := "Undecided"
	apps.existing["24/0001/FUL"] = applications.PlanningApplication{UID: "24/0001/FUL", AreaID: 99, AppState: &undecided, LastDifferent: ld.Add(-time.Hour)}
	state := newFakeStateStore()

	rec := &fakePollMetrics{}
	h := newReconciliationHandler(t, fetcher, apps, state, []int{99}, defaultReconciliationOpts())
	h.WithMetrics(rec)

	out := h.Run(context.Background())
	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	if rec.applicationsIngest != 1 {
		t.Errorf("ApplicationsIngested total = %d, want 1", rec.applicationsIngest)
	}
	if len(rec.applicationsIngestTags) != 1 || rec.applicationsIngestTags[0] != "C" {
		t.Errorf("ApplicationsIngested tags = %v, want [C]", rec.applicationsIngestTags)
	}
}

// TestReconciliation_NilMetricsRecorderIsNoOp mirrors
// TestHandler_NilMetricsRecorderIsNoOp for Lane C.
func TestReconciliation_NilMetricsRecorderIsNoOp(t *testing.T) {
	t.Parallel()
	ld := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	fetcher := newFakeReconciliationFetcher()
	fetcher.pages[99] = planit.FetchPageResult{
		Applications: []applications.PlanningApplication{lightApp("24/0001/FUL", 99, "Permitted", ld)},
	}
	apps := newFakeApps()
	state := newFakeStateStore()

	// No WithMetrics call: Run must not panic.
	h := newReconciliationHandler(t, fetcher, apps, state, []int{99}, defaultReconciliationOpts())
	if out := h.Run(context.Background()); out.err != nil {
		t.Fatalf("Run without metrics recorder: %v", out.err)
	}
}

// TestNationalPollHandler_RecordsCycleCompletedOnce covers the top-level
// orchestration handler: exactly one CycleCompleted("National", <termination>)
// per Handle call, regardless of how many lanes ran underneath it.
func TestNationalPollHandler_RecordsCycleCompletedOnce(t *testing.T) {
	t.Parallel()
	watermark := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	ldA := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	ldB := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)

	fetcherA := newFakeNationalFetcher()
	fetcherA.pages[0] = planit.FetchPageResult{From: 0, Applications: []applications.PlanningApplication{testApp("a", 300, ldA)}}
	fetcherB := newFakeNationalFetcher()
	fetcherB.pages[0] = planit.FetchPageResult{From: 0, Applications: []applications.PlanningApplication{testApp("b", 300, ldB)}}

	appsA := newFakeApps()
	appsB := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: watermark}
	state.states[sentinelLaneB] = PollState{HighWaterMark: watermark, LastPollTime: watermark}
	logger := slog.New(slog.NewTextHandler(discard{}, nil))
	clock := func() time.Time { return time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) }

	laneAHandler := NewNationalLaneHandler(fetcherA, state, appsA, laneAOpts(), clock, logger)
	laneBHandler := NewNationalLaneHandler(fetcherB, state, appsB, laneBOpts(20), clock, logger)
	handler := NewNationalPollHandler(laneAHandler, laneBHandler, nil, clock, logger)

	rec := &fakePollMetrics{}
	handler.WithMetrics(rec)

	res, err := handler.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(rec.cyclesCompleted) != 1 || rec.cyclesCompleted[0] != res.TerminationReason.TelemetryValue() {
		t.Errorf("CycleCompleted terminations = %v, want [%s]", rec.cyclesCompleted, res.TerminationReason.TelemetryValue())
	}
	if len(rec.cyclesCompletedTags) != 1 || rec.cyclesCompletedTags[0] != "National" {
		t.Errorf("CycleCompleted cycleType tags = %v, want [National]", rec.cyclesCompletedTags)
	}
}

// TestNationalPollHandler_NilMetricsRecorderIsNoOp mirrors
// TestHandler_NilMetricsRecorderIsNoOp for the top-level handler.
func TestNationalPollHandler_NilMetricsRecorderIsNoOp(t *testing.T) {
	t.Parallel()
	fetcherA := newFakeNationalFetcher()
	fetcherB := newFakeNationalFetcher()
	apps := newFakeApps()
	state := newFakeStateStore()
	logger := slog.New(slog.NewTextHandler(discard{}, nil))
	clock := func() time.Time { return time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) }

	laneAHandler := NewNationalLaneHandler(fetcherA, state, apps, laneAOpts(), clock, logger)
	laneBHandler := NewNationalLaneHandler(fetcherB, state, apps, laneBOpts(20), clock, logger)
	handler := NewNationalPollHandler(laneAHandler, laneBHandler, nil, clock, logger)

	// No WithMetrics call: Handle must not panic.
	if _, err := handler.Handle(context.Background()); err != nil {
		t.Fatalf("Handle without metrics recorder: %v", err)
	}
}
