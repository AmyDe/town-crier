package polling

import (
	"context"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// TestHandler_ProbeDoesNotRunWithoutActiveCursor covers the "iff cursor active"
// gate: a fresh authority (no PollState, so no cursor) must fetch only the
// ascending drain — no newest-first probe.
func TestHandler_ProbeDoesNotRunWithoutActiveCursor(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	pi.pages[pageKey{authority: 99, index: 0}] = planit.FetchPageResult{
		From: 0, Applications: []applications.PlanningApplication{testApp("a", 99, ld)}, HasMorePages: false,
	}
	apps := newFakeApps()
	state := newFakeStateStore() // no existing PollState -> no cursor
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())

	if _, err := h.Handle(context.Background()); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(pi.fetched) != 1 {
		t.Fatalf("fetched: got %+v, want 1 (no probe, one drain fetch)", pi.fetched)
	}
	if pi.fetched[0].descending {
		t.Errorf("no active cursor: probe must not run, got descending fetch %+v", pi.fetched[0])
	}
}

// TestHandler_ProbeRunsWithActiveCursorButNeverAdvancesHWM covers the probe's
// isolation from HWM/cursor bookkeeping: the probe's (newest-first) record must
// still be ingested (upserted, counted), but the persisted HWM at a natural end
// must reflect only the ascending drain's LastDifferent, never the probe's.
func TestHandler_ProbeRunsWithActiveCursorButNeverAdvancesHWM(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	hwm := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	// Probe (descending, index 0): a record far newer than anything the drain
	// will see. Must never surface in the persisted HWM.
	probeLD := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	pi.pages[pageKey{authority: 99, index: 0, descending: true}] = planit.FetchPageResult{
		From: 0, Applications: []applications.PlanningApplication{testApp("newest", 99, probeLD)}, HasMorePages: false,
	}
	// Drain resumes at max(0, 400-100)=300 and naturally ends.
	drainLD := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	pi.pages[pageKey{authority: 99, index: 300}] = planit.FetchPageResult{
		From: 300, Applications: []applications.PlanningApplication{testApp("drain", 99, drainLD)}, HasMorePages: false,
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[99] = PollState{
		LastPollTime: hwm, HighWaterMark: hwm,
		Cursor: &PollCursor{DifferentStart: hwm, NextIndex: 400},
	}
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())

	res, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if res.ApplicationCount != 2 {
		t.Errorf("ApplicationCount: got %d, want 2 (probe + drain both ingested)", res.ApplicationCount)
	}
	if len(apps.upserts) != 2 {
		t.Errorf("upserts: got %d, want 2", len(apps.upserts))
	}
	if len(state.saves) != 1 {
		t.Fatalf("state saves: got %d, want 1", len(state.saves))
	}
	if !state.saves[0].highWaterMark.Equal(drainLD) {
		t.Errorf("persisted HWM: got %v, want %v (drain's LastDifferent; the probe's %v must never surface)",
			state.saves[0].highWaterMark, drainLD, probeLD)
	}
}

// TestHandler_ProbeDoesNotConsumeDrainFetchCap proves the probe is one
// additional gated fetch, not a use of the drain-fetch cap: with MaxPages=1,
// the total fetch count is 2 (1 probe + 1 drain), and the persisted cursor
// reflects exactly one drain fetch's progress.
func TestHandler_ProbeDoesNotConsumeDrainFetchCap(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	hwm := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	pi.pages[pageKey{authority: 99, index: 0, descending: true}] = planit.FetchPageResult{From: 0, HasMorePages: false}
	full := make([]applications.PlanningApplication, 100)
	for i := range full {
		full[i] = testApp("app", 99, hwm)
	}
	pi.pages[pageKey{authority: 99, index: 300}] = planit.FetchPageResult{
		From: 300, Applications: full, HasMorePages: true, Total: platform.Ptr(1000),
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[99] = PollState{LastPollTime: hwm, HighWaterMark: hwm, Cursor: &PollCursor{DifferentStart: hwm, NextIndex: 400}}
	one := 1
	opts := HandlerOptions{MaxPagesPerAuthorityPerCycle: &one, HandlerBudget: 4 * time.Minute}
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, opts)

	if _, err := h.Handle(context.Background()); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if pi.fetchCount() != 2 {
		t.Errorf("fetchCount: got %d, want 2 (1 probe + 1 drain at cap)", pi.fetchCount())
	}
	if len(state.saves) != 1 {
		t.Fatalf("state saves: got %d, want 1", len(state.saves))
	}
	cur := state.saves[0].cursor
	if cur == nil || cur.NextIndex != 400 {
		t.Errorf("cursor: %+v, want NextIndex=400 (from=300 + records=100)", cur)
	}
}

// TestHandler_Probe429AbortsAuthorityWithRateLimitSemantics proves a 429 on the
// probe fetch aborts the authority exactly like any other rate-limited fetch —
// including never attempting the drain and never persisting state.
func TestHandler_Probe429AbortsAuthorityWithRateLimitSemantics(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	hwm := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	ra := 45 * time.Second
	pi.errs[99] = &planit.RateLimitError{RetryAfter: &ra}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[99] = PollState{LastPollTime: hwm, HighWaterMark: hwm, Cursor: &PollCursor{DifferentStart: hwm, NextIndex: 400}}
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())

	res, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !res.RateLimited || res.TerminationReason != TerminationRateLimited {
		t.Errorf("expected rate-limited termination, got %+v", res)
	}
	if res.RetryAfter == nil || *res.RetryAfter != ra {
		t.Errorf("RetryAfter: got %v, want %v", res.RetryAfter, ra)
	}
	if pi.fetchCount() != 1 {
		t.Errorf("fetchCount: got %d, want 1 (probe only; drain must not run after a probe 429)", pi.fetchCount())
	}
	if len(state.saves) != 0 {
		t.Errorf("no cursor/HWM should be persisted on a probe 429: %+v", state.saves)
	}
}
