package polling

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

// fakeNationalFetcher serves pre-canned national-delta pages keyed by the
// requested 0-based record offset, and can be primed to fail on a specific
// call ordinal (1-based, failNth) to model a mid-walk transport error or 429.
type fakeNationalFetcher struct {
	pages   map[int]planit.FetchPageResult
	failNth map[int]error
	calls   int
	queries []planit.NationalDeltaQuery
}

func newFakeNationalFetcher() *fakeNationalFetcher {
	return &fakeNationalFetcher{pages: map[int]planit.FetchPageResult{}, failNth: map[int]error{}}
}

func (f *fakeNationalFetcher) FetchNationalDeltaPage(_ context.Context, q planit.NationalDeltaQuery) (planit.FetchPageResult, error) {
	f.calls++
	f.queries = append(f.queries, q)
	if err, ok := f.failNth[f.calls]; ok {
		return planit.FetchPageResult{}, err
	}
	res, ok := f.pages[q.StartIndex]
	if !ok {
		return planit.FetchPageResult{From: q.StartIndex, HasMorePages: false}, nil
	}
	return res, nil
}

// manyApps builds n synthetic applications with strictly descending
// LastDifferent values spaced step apart, starting at head. Used to push a
// walk's cursor.NextIndex past the 100-record resume overlap
// (handler.go's resumeOverlapRecords) in a multi-page test without needing
// a real PlanIt page.
func manyApps(n int, head time.Time, step time.Duration) []applications.PlanningApplication {
	apps := make([]applications.PlanningApplication, n)
	for i := range n {
		apps[i] = testApp(fmt.Sprintf("bulk-%d", i), 300, head.Add(-time.Duration(i)*step))
	}
	return apps
}

// newLaneHandler wires a NationalLaneHandler pinned to a fixed clock
// (2026-07-14T12:00:00Z), for deterministic mask-cutoff math.
func newLaneHandler(t *testing.T, fetcher *fakeNationalFetcher, apps *fakeApps, state *fakeStateStore, opts NationalLaneOptions) *NationalLaneHandler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(discard{}, nil))
	clock := func() time.Time { return time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) }
	return NewNationalLaneHandler(fetcher, state, apps, opts, clock, logger)
}

func laneAOpts() NationalLaneOptions {
	return NationalLaneOptions{Lane: LaneA, Mask: planit.MaskStartDate, MaskWindow: 90 * 24 * time.Hour}
}

func laneBOpts(maxPages int) NationalLaneOptions {
	return NationalLaneOptions{Lane: LaneB, Mask: planit.MaskDecidedStart, MaskWindow: 90 * 24 * time.Hour, MaxPages: &maxPages}
}

// TestNationalLane_IngestsNewerRecordsAndStopsAtExactBoundary pins the
// central boundary decision (ADR 0041 / GH#962): a record whose
// LastDifferent equals the watermark stops the walk without being
// re-ingested, but a strictly-newer record that shares the SAME page as the
// boundary record must still be ingested — descending order means it is
// encountered first. One page suffices to reach the boundary, so one
// RunOnePage call completes the whole walk (ADR 0044).
func TestNationalLane_IngestsNewerRecordsAndStopsAtExactBoundary(t *testing.T) {
	t.Parallel()
	watermark := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	newer := watermark.Add(1 * time.Hour)
	older := watermark.Add(-1 * time.Hour)

	fetcher := newFakeNationalFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From: 0,
		Applications: []applications.PlanningApplication{
			testApp("newer", 300, newer),        // strictly newer: must be ingested
			testApp("boundary", 300, watermark), // == watermark: must stop here, not ingested
			testApp("older", 300, older),        // must never be reached
		},
		HasMorePages: false,
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: watermark}

	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if out.recordsSeen != 3 {
		t.Errorf("recordsSeen: got %d, want 3 (every record fetched, including the ones at/after the boundary)", out.recordsSeen)
	}
	if out.recordsIngested != 1 {
		t.Errorf("recordsIngested: got %d, want 1 (only the strictly-newer record)", out.recordsIngested)
	}
	if len(apps.upserts) != 1 || apps.upserts[0].Name != "newer" {
		t.Fatalf("upserts: got %+v, want exactly the 'newer' record", apps.upserts)
	}
	if !out.watermarkAfter.Equal(newer) {
		t.Errorf("watermarkAfter: got %v, want %v (max ingested this page)", out.watermarkAfter, newer)
	}
	if state.states[sentinelLaneA].Cursor != nil {
		t.Errorf("cursor: got %+v, want nil (a clean completion clears the cursor)", state.states[sentinelLaneA].Cursor)
	}
}

// TestNationalLane_HandlesEmptyPage covers a page with zero records: no
// ingestion, watermark unchanged, no panic on the empty slice.
func TestNationalLane_HandlesEmptyPage(t *testing.T) {
	t.Parallel()
	watermark := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	fetcher := newFakeNationalFetcher()
	fetcher.pages[0] = planit.FetchPageResult{From: 0, Applications: nil, HasMorePages: false}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: watermark}

	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if out.recordsSeen != 0 || out.recordsIngested != 0 {
		t.Errorf("expected no records seen/ingested, got seen=%d ingested=%d", out.recordsSeen, out.recordsIngested)
	}
	if !out.watermarkAfter.Equal(watermark) {
		t.Errorf("watermarkAfter: got %v, want unchanged %v", out.watermarkAfter, watermark)
	}
}

// TestNationalLane_HandlesTotalAbsent covers a response that omits total: the
// outcome must not carry a misleading planitTotal, and the caller (span
// attributes) must be able to distinguish "PlanIt omitted total" from
// "total is zero".
func TestNationalLane_HandlesTotalAbsent(t *testing.T) {
	t.Parallel()
	fetcher := newFakeNationalFetcher()
	fetcher.pages[0] = planit.FetchPageResult{From: 0, Applications: nil, HasMorePages: false, Total: nil}
	apps := newFakeApps()
	state := newFakeStateStore()

	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())
	out := h.RunOnePage(context.Background())

	if out.planitTotal != nil {
		t.Errorf("planitTotal: got %v, want nil (PlanIt omitted total)", out.planitTotal)
	}
}

// findLogRecord parses each newline-delimited JSON record in buf and returns
// the first whose "msg" field equals want, decoded into a generic map so
// tests can assert on individual keys without coupling to slog's exact
// encoding. Fails the test if no matching record is found.
func findLogRecord(t *testing.T, buf []byte, want string) map[string]any {
	t.Helper()
	for _, line := range bytes.Split(buf, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		if rec["msg"] == want {
			return rec
		}
	}
	t.Fatalf("no log record with msg=%q found in:\n%s", want, buf)
	return nil
}

// TestNationalLane_RunOnePage_LogsPageFetchDiagnostics pins the ONE
// diagnostic log line added for the frozen-watermark investigation
// (tc-h2tcx): every successful fetch logs enough to tell, per cycle, what
// PlanIt actually returned for this query shape (recordsSeen plus the
// first/last record's UID+LastDifferent) alongside the query parameters
// that produced it (watermarkBefore, differentStart, maskCutoff,
// startIndex) — all without touching recordsIngested, the watermark-advance
// decision, or any span attribute.
func TestNationalLane_RunOnePage_LogsPageFetchDiagnostics(t *testing.T) {
	t.Parallel()
	watermark := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	newer := watermark.Add(2 * time.Hour)
	newest := watermark.Add(3 * time.Hour)

	fetcher := newFakeNationalFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From: 0,
		Applications: []applications.PlanningApplication{
			testApp("newest", 300, newest),
			testApp("newer", 300, newer),
		},
		HasMorePages: false,
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: watermark}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	clock := func() time.Time { return time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) }
	h := NewNationalLaneHandler(fetcher, state, apps, laneAOpts(), clock, logger)

	if out := h.RunOnePage(context.Background()); out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}

	rec := findLogRecord(t, buf.Bytes(), "lane delta page fetched")
	if got := rec["lane"]; got != "A" {
		t.Errorf("lane: got %v, want A", got)
	}
	if got := rec["recordsSeen"]; got != float64(2) {
		t.Errorf("recordsSeen: got %v, want 2", got)
	}
	if got := rec["startIndex"]; got != float64(0) {
		t.Errorf("startIndex: got %v, want 0", got)
	}
	if got := rec["planitTotal"]; got != float64(0) {
		t.Errorf("planitTotal: got %v, want 0 (nil-safe default when PlanIt omits total)", got)
	}
	if got := rec["firstUID"]; got != "newest/FUL" {
		t.Errorf("firstUID: got %v, want newest/FUL (descending order: newest record fetched first)", got)
	}
	if got := rec["lastUID"]; got != "newer/FUL" {
		t.Errorf("lastUID: got %v, want newer/FUL", got)
	}
	for _, key := range []string{"watermarkBefore", "differentStart", "maskCutoff", "firstLastDifferent", "lastLastDifferent"} {
		if _, ok := rec[key]; !ok {
			t.Errorf("expected key %q present in log record, got %+v", key, rec)
		}
	}
}

// TestNationalLane_RunOnePage_LogsEmptyPageWithoutFirstLastKeys guards the
// empty-Applications edge case explicitly: no out-of-range index on
// res.Applications[0]/[len-1], and no firstUID/lastUID keys logged when
// there is nothing to report.
func TestNationalLane_RunOnePage_LogsEmptyPageWithoutFirstLastKeys(t *testing.T) {
	t.Parallel()
	watermark := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	fetcher := newFakeNationalFetcher()
	fetcher.pages[0] = planit.FetchPageResult{From: 0, Applications: nil, HasMorePages: false}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: watermark}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	clock := func() time.Time { return time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) }
	h := NewNationalLaneHandler(fetcher, state, apps, laneAOpts(), clock, logger)

	if out := h.RunOnePage(context.Background()); out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}

	rec := findLogRecord(t, buf.Bytes(), "lane delta page fetched")
	if got := rec["recordsSeen"]; got != float64(0) {
		t.Errorf("recordsSeen: got %v, want 0", got)
	}
	if _, ok := rec["firstUID"]; ok {
		t.Errorf("firstUID should be absent on an empty page, got %v", rec["firstUID"])
	}
	if _, ok := rec["lastUID"]; ok {
		t.Errorf("lastUID should be absent on an empty page, got %v", rec["lastUID"])
	}
}

// TestNationalLane_NeverAdvancesWatermarkPastAnErroredPage is the safety
// invariant under ADR 0044's per-page checkpointing: a page that ingests
// successfully and persists a resume cursor, followed by a SEPARATE
// RunOnePage call whose fetch fails, must leave the watermark exactly where
// it was before either call — never partway advanced to the first page's
// max — so nothing between the old watermark and the failed page is
// silently skipped on the next call. The first page's already-persisted
// cursor must also survive the second call's failure untouched.
func TestNationalLane_NeverAdvancesWatermarkPastAnErroredPage(t *testing.T) {
	t.Parallel()
	watermark := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	page1LD := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

	fetcher := newFakeNationalFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{testApp("page1", 300, page1LD)},
		HasMorePages: true, // more pages follow -> the next RunOnePage call fetches index 1
	}
	fetcher.failNth[2] = errors.New("planit: transport blew up")

	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: watermark}

	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())

	firstOut := h.RunOnePage(context.Background())
	if firstOut.err != nil {
		t.Fatalf("first RunOnePage: %v", firstOut.err)
	}
	if len(apps.upserts) != 1 {
		t.Fatalf("page 1's record should have been ingested: got %d upserts", len(apps.upserts))
	}
	persistedCursor := state.states[sentinelLaneA].Cursor
	if persistedCursor == nil || persistedCursor.NextIndex != 1 {
		t.Fatalf("expected a resume cursor at index 1 after page 1, got %+v", persistedCursor)
	}
	if !firstOut.watermarkAfter.Equal(watermark) {
		t.Errorf("watermarkAfter after page 1: got %v, want unchanged %v (mid-drain)", firstOut.watermarkAfter, watermark)
	}

	secondOut := h.RunOnePage(context.Background())
	if secondOut.err == nil {
		t.Fatal("expected the second page's fetch error to surface on the outcome")
	}
	if !secondOut.watermarkAfter.Equal(watermark) {
		t.Errorf("watermarkAfter after the failed page: got %v, want unchanged %v (never advance past an errored page)", secondOut.watermarkAfter, watermark)
	}
	if got := state.states[sentinelLaneA].Cursor; got == nil || got.NextIndex != 1 {
		t.Errorf("cursor after the failed page: got %+v, want the untouched page-1 checkpoint (NextIndex=1)", got)
	}
}

// TestNationalLane_NeverAdvancesWatermarkOn429 mirrors the errored-page test
// for a 429: the watermark must not advance even though the walk stopped for
// an "expected" reason rather than a hard error.
func TestNationalLane_NeverAdvancesWatermarkOn429(t *testing.T) {
	t.Parallel()
	watermark := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	retryAfter := 30 * time.Second

	fetcher := newFakeNationalFetcher()
	fetcher.failNth[1] = &planit.RateLimitError{RetryAfter: &retryAfter}

	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: watermark}

	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())
	out := h.RunOnePage(context.Background())

	if !out.rateLimited {
		t.Fatal("expected rateLimited=true")
	}
	if out.retryAfter == nil || *out.retryAfter != retryAfter {
		t.Errorf("retryAfter: got %v, want %v", out.retryAfter, retryAfter)
	}
	if !out.watermarkAfter.Equal(watermark) {
		t.Errorf("watermarkAfter: got %v, want unchanged %v (never advance past a 429)", out.watermarkAfter, watermark)
	}
}

// TestNationalLane_MidDrainResumesAtCheckpointedIndex proves the per-page
// checkpoint's whole point (ADR 0044 §2, GH#955/tc-nlvpz): a second
// RunOnePage call resumes pagination at the persisted (minus the resume
// overlap) NextIndex rather than re-fetching from index 0.
//
// It doubles as the GH#983 acceptance-criterion-4 regression test: the
// pre-seeded cursor here never sets WalkHead (its zero value, matching a
// row read back from before the cursor_walk_head column existed), so
// completion must fall back to this page's own maxIngested -- exactly
// today's behaviour -- rather than erroring or leaving the watermark
// stuck. See TestNationalLane_ResumeWithCarriedWalkHead_CompletionUsesCarriedHead
// for the counterpart proving a genuinely carried WalkHead wins instead.
func TestNationalLane_MidDrainResumesAtCheckpointedIndex(t *testing.T) {
	t.Parallel()
	watermark := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	ld := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	prefilterDate := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	// A pre-seeded cursor at a nontrivial index, so resumeOverlapRecords
	// (100) genuinely clamps to a non-zero StartIndex — 300 - 100 = 200 —
	// rather than degenerating to 0 the way a tiny (single-record) fake page
	// would.
	fetcher := newFakeNationalFetcher()
	fetcher.pages[200] = planit.FetchPageResult{
		From:         200,
		Applications: []applications.PlanningApplication{testApp("resumed", 300, ld)},
		HasMorePages: false,
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneA] = PollState{
		HighWaterMark: watermark,
		Cursor:        &PollCursor{DifferentStart: prefilterDate, NextIndex: 300},
	}

	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if len(fetcher.queries) != 1 || fetcher.queries[0].StartIndex != 200 {
		t.Fatalf("expected exactly one fetch at StartIndex 200 (300 - the 100-record resume overlap), got %+v", fetcher.queries)
	}
	if len(apps.upserts) != 1 || apps.upserts[0].Name != "resumed" {
		t.Fatalf("upserts: got %+v", apps.upserts)
	}
	if !out.watermarkAfter.Equal(ld) {
		t.Errorf("watermarkAfter: got %v, want %v (walk completed on this page)", out.watermarkAfter, ld)
	}
	if state.states[sentinelLaneA].Cursor != nil {
		t.Errorf("cursor: got %+v, want nil (walk completed)", state.states[sentinelLaneA].Cursor)
	}
}

// TestNationalLane_MultiPageWalk_WatermarkUsesPageZeroHead is the primary
// regression test for GH#983: a two-page walk driven the way
// NationalPollHandler.Handle's planner loop drives it (RunOnePage called
// repeatedly, feeding the persisted cursor forward) must land the final
// watermark on page 0's head -- the walk's true maximum for a descending
// walk -- never the boundary (oldest) page's own max.
func TestNationalLane_MultiPageWalk_WatermarkUsesPageZeroHead(t *testing.T) {
	t.Parallel()
	watermark := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	head := watermark.Add(48 * time.Hour)

	fetcher := newFakeNationalFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: manyApps(110, head, time.Minute),
		HasMorePages: true, // walk continues past this page
	}
	fetcher.pages[10] = planit.FetchPageResult{
		From: 10,
		Applications: []applications.PlanningApplication{
			testApp("boundary-page-newer", 300, watermark.Add(2*time.Hour)), // ingested, but far below the true walk head
			testApp("boundary-page-stop", 300, watermark),                   // == watermark: stops the walk here
		},
		HasMorePages: false,
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: watermark}

	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())

	firstOut := h.RunOnePage(context.Background())
	if firstOut.err != nil {
		t.Fatalf("page 0 RunOnePage: %v", firstOut.err)
	}
	if !firstOut.watermarkAfter.Equal(watermark) {
		t.Errorf("watermarkAfter after page 0: got %v, want unchanged %v (mid-walk)", firstOut.watermarkAfter, watermark)
	}

	secondOut := h.RunOnePage(context.Background())
	if secondOut.err != nil {
		t.Fatalf("page 1 RunOnePage: %v", secondOut.err)
	}
	if !secondOut.watermarkAfter.Equal(head) {
		t.Errorf("watermarkAfter after completion: got %v, want the WHOLE walk's head %v, not the boundary page's own max", secondOut.watermarkAfter, head)
	}
	if state.states[sentinelLaneA].Cursor != nil {
		t.Errorf("cursor: got %+v, want nil (walk completed)", state.states[sentinelLaneA].Cursor)
	}
}

// TestNationalLane_InterruptedWalk_CursorCarriesWalkHead proves an
// interrupted (more-pages-remain, boundary not reached) walk captures page
// 0's head into the persisted cursor's WalkHead even though it goes unused
// this call -- it is the value a LATER RunOnePage call (this cycle or a
// future one) needs to complete the walk correctly (GH#983).
func TestNationalLane_InterruptedWalk_CursorCarriesWalkHead(t *testing.T) {
	t.Parallel()
	watermark := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	head := watermark.Add(10 * time.Hour)

	fetcher := newFakeNationalFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: manyApps(110, head, time.Minute),
		HasMorePages: true, // more pages remain: the walk does not complete this call
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: watermark}

	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if !out.watermarkAfter.Equal(watermark) {
		t.Errorf("watermarkAfter: got %v, want unchanged %v (walk interrupted, not complete)", out.watermarkAfter, watermark)
	}
	cursor := state.states[sentinelLaneA].Cursor
	if cursor == nil {
		t.Fatal("expected a resume cursor to be persisted")
	}
	if !cursor.WalkHead.Equal(head) {
		t.Errorf("cursor.WalkHead: got %v, want %v (page 0's first record, captured even though unused this call)", cursor.WalkHead, head)
	}
}

// TestNationalLane_ResumeWithCarriedWalkHead_CompletionUsesCarriedHead is
// the core regression test for GH#983: when a resume completes the walk,
// the persisted watermark must come from the CARRIED cursor.WalkHead --
// never a comparison against or blend with this page's own maxIngested,
// even when maxIngested happens to be numerically newer than the carried
// head. That proves the fix genuinely replaces the per-page-scoped
// variable as the source of truth rather than just widening its scope
// with a max().
func TestNationalLane_ResumeWithCarriedWalkHead_CompletionUsesCarriedHead(t *testing.T) {
	t.Parallel()
	watermark := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	prefilterDate := watermark
	carriedHead := watermark.Add(3 * time.Hour) // the walk's true head, captured on page 0
	pageMax := watermark.Add(10 * time.Hour)    // THIS resumed page's own max -- numerically newer, must be ignored

	fetcher := newFakeNationalFetcher()
	fetcher.pages[200] = planit.FetchPageResult{
		From: 200,
		Applications: []applications.PlanningApplication{
			testApp("resumed-max", 300, pageMax),
			testApp("resumed-boundary", 300, watermark),
		},
		HasMorePages: false,
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneA] = PollState{
		HighWaterMark: watermark,
		Cursor:        &PollCursor{DifferentStart: prefilterDate, NextIndex: 300, WalkHead: carriedHead},
	}

	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if !out.watermarkAfter.Equal(carriedHead) {
		t.Errorf("watermarkAfter: got %v, want the carried WalkHead %v, not this page's own max %v", out.watermarkAfter, carriedHead, pageMax)
	}
	if state.states[sentinelLaneA].Cursor != nil {
		t.Errorf("cursor: got %+v, want nil (walk completed)", state.states[sentinelLaneA].Cursor)
	}
}

// TestNationalLane_BoundaryHitAtIndexZero_WatermarkUnchanged pins the
// healthy-quiet path from the 07-18->19 PlanIt outage (GH#983 "NOT the
// bug"): when the walk's very FIRST record already sits at the watermark,
// the walk completes on iteration 0 having ingested nothing. Because
// walkHead is captured from that same first record, newWatermark ==
// walkHead == watermarkBefore -- numerically a no-op -- so the fix must
// not regress this previously-verified freeze-safe behaviour.
func TestNationalLane_BoundaryHitAtIndexZero_WatermarkUnchanged(t *testing.T) {
	t.Parallel()
	watermark := time.Date(2026, 7, 18, 7, 1, 28, 0, time.UTC)
	older := watermark.Add(-1 * time.Hour)

	fetcher := newFakeNationalFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From: 0,
		Applications: []applications.PlanningApplication{
			testApp("boundary", 300, watermark), // == watermark: stops the walk immediately
			testApp("older", 300, older),        // must never be reached
		},
		HasMorePages: false,
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: watermark}

	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if out.recordsIngested != 0 {
		t.Errorf("recordsIngested: got %d, want 0 (boundary hit on the very first record)", out.recordsIngested)
	}
	if !out.watermarkAfter.Equal(watermark) {
		t.Errorf("watermarkAfter: got %v, want unchanged %v (healthy-quiet path, GH#983 'NOT the bug')", out.watermarkAfter, watermark)
	}
	if state.states[sentinelLaneA].Cursor != nil {
		t.Errorf("cursor: got %+v, want nil (clean completion)", state.states[sentinelLaneA].Cursor)
	}
}

// TestNationalLane_FirstRunPrefiltersOnMaskCutoff covers a lane that has
// never run (no sentinel poll_state row): the seeding fetch's different_start
// falls back to the mask cutoff date — never an unprefiltered national query
// (ADR 0041's guardrail) — and requests only page 0.
func TestNationalLane_FirstRunPrefiltersOnMaskCutoff(t *testing.T) {
	t.Parallel()
	fetcher := newFakeNationalFetcher()
	apps := newFakeApps()
	state := newFakeStateStore() // no sentinel row: never run

	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())
	h.RunOnePage(context.Background())

	if len(fetcher.queries) != 1 {
		t.Fatalf("expected exactly one fetch, got %d", len(fetcher.queries))
	}
	q := fetcher.queries[0]
	wantCutoff := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC) // 2026-07-14 - 90 days
	if !q.DifferentStart.Equal(wantCutoff) {
		t.Errorf("DifferentStart prefilter: got %v, want the mask cutoff %v", q.DifferentStart, wantCutoff)
	}
	if !q.MaskCutoff.Equal(wantCutoff) {
		t.Errorf("MaskCutoff: got %v, want %v", q.MaskCutoff, wantCutoff)
	}
	if q.Mask != planit.MaskStartDate {
		t.Errorf("Mask: got %v, want MaskStartDate", q.Mask)
	}
}

// TestNationalLane_SeedsWatermarkFromHeadOnFirstRun is the fix for the
// first-run backfill bug: a lane with no prior watermark must fetch page 0
// ONLY — never walking further pages even when PlanIt reports more follow —
// ingest nothing, and persist the watermark as the max last_different seen on
// that single page. Forward-flow only: no historical sweep (ADR 0041 rule 2).
func TestNationalLane_SeedsWatermarkFromHeadOnFirstRun(t *testing.T) {
	t.Parallel()
	head := time.Date(2026, 7, 14, 5, 14, 58, 0, time.UTC)
	older := head.Add(-2 * time.Hour)
	total := 1717

	fetcher := newFakeNationalFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From: 0,
		Applications: []applications.PlanningApplication{
			testApp("newest", 300, head),
			testApp("older", 300, older),
		},
		HasMorePages: true, // PlanIt claims more pages follow; seeding must never fetch them
		Total:        &total,
	}
	apps := newFakeApps()
	state := newFakeStateStore() // no sentinel row: never run

	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if fetcher.calls != 1 {
		t.Fatalf("expected exactly one fetch (page 0 only, never walking further pages), got %d", fetcher.calls)
	}
	if out.recordsIngested != 0 {
		t.Errorf("recordsIngested: got %d, want 0 (a seed must never ingest — that would be a backfill)", out.recordsIngested)
	}
	if len(apps.upserts) != 0 {
		t.Errorf("expected zero upserts on a seed, got %d", len(apps.upserts))
	}
	if !out.watermarkAfter.Equal(head) {
		t.Errorf("watermarkAfter: got %v, want the page's max last_different %v", out.watermarkAfter, head)
	}
	if len(state.saves) != 1 || !state.saves[0].highWaterMark.Equal(head) {
		t.Fatalf("expected exactly one Save persisting the seeded watermark: got %+v", state.saves)
	}
}

// TestNationalLane_SeedsToNowOnEmptyFirstPage proves a lane whose masked
// window currently matches nothing still seeds (to now()) rather than
// staying permanently unseeded and re-attempting the seed forever.
func TestNationalLane_SeedsToNowOnEmptyFirstPage(t *testing.T) {
	t.Parallel()
	fetcher := newFakeNationalFetcher()
	fetcher.pages[0] = planit.FetchPageResult{From: 0, Applications: nil, HasMorePages: false}
	apps := newFakeApps()
	state := newFakeStateStore()

	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	wantNow := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) // newLaneHandler's pinned clock
	if !out.watermarkAfter.Equal(wantNow) {
		t.Errorf("watermarkAfter: got %v, want now() %v", out.watermarkAfter, wantNow)
	}
	if out.recordsIngested != 0 {
		t.Errorf("recordsIngested: got %d, want 0", out.recordsIngested)
	}
	if len(state.saves) != 1 {
		t.Fatalf("expected exactly one Save, got %+v", state.saves)
	}
}

// TestNationalLane_SeedFetchErrorLeavesLaneUnseeded proves rule 5 of the
// seeding fix: a failed page-0 fetch must not persist ANY watermark — the
// lane stays unseeded and the next cycle simply retries the seed (bounded to
// one extra request, harmless).
func TestNationalLane_SeedFetchErrorLeavesLaneUnseeded(t *testing.T) {
	t.Parallel()
	fetcher := newFakeNationalFetcher()
	fetcher.failNth[1] = errors.New("planit: transport blew up")
	apps := newFakeApps()
	state := newFakeStateStore()

	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())
	out := h.RunOnePage(context.Background())

	if out.err == nil {
		t.Fatal("expected the page-0 fetch error to surface on the outcome")
	}
	if len(state.saves) != 0 {
		t.Errorf("expected NO Save call when the seed fetch fails, got %+v", state.saves)
	}
}

// TestNationalLane_SeedRateLimitedLeavesLaneUnseeded mirrors the error case
// for a 429 on the seeding fetch.
func TestNationalLane_SeedRateLimitedLeavesLaneUnseeded(t *testing.T) {
	t.Parallel()
	retryAfter := 30 * time.Second
	fetcher := newFakeNationalFetcher()
	fetcher.failNth[1] = &planit.RateLimitError{RetryAfter: &retryAfter}
	apps := newFakeApps()
	state := newFakeStateStore()

	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())
	out := h.RunOnePage(context.Background())

	if !out.rateLimited {
		t.Fatal("expected rateLimited=true")
	}
	if len(state.saves) != 0 {
		t.Errorf("expected NO Save call when the seed fetch is rate-limited, got %+v", state.saves)
	}
}

// TestNationalLane_SeedThenForwardFlow proves seeding does not break steady
// state: after a seeded call, a second call from that watermark ingests only
// the record strictly newer than the seed and stops normally at the
// (re-encountered) boundary.
func TestNationalLane_SeedThenForwardFlow(t *testing.T) {
	t.Parallel()
	seedHead := time.Date(2026, 7, 14, 5, 0, 0, 0, time.UTC)
	fetcher := newFakeNationalFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{testApp("seed-head", 300, seedHead)},
		HasMorePages: false,
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())

	seedOut := h.RunOnePage(context.Background())
	if seedOut.recordsIngested != 0 {
		t.Fatalf("seed call must not ingest: got %d", seedOut.recordsIngested)
	}

	// Second call: a fresh descending walk (a new fetcher, as a new PlanIt
	// request would be) finds one genuinely new record ahead of the seeded
	// watermark, plus the seed-head record again at the exact boundary.
	newRecord := seedHead.Add(1 * time.Hour)
	fetcher2 := newFakeNationalFetcher()
	fetcher2.pages[0] = planit.FetchPageResult{
		From: 0,
		Applications: []applications.PlanningApplication{
			testApp("forward", 300, newRecord),
			testApp("seed-head-again", 300, seedHead), // == watermark: boundary, must stop here
		},
		HasMorePages: false,
	}
	h2 := newLaneHandler(t, fetcher2, apps, state, laneAOpts())
	out2 := h2.RunOnePage(context.Background())

	if out2.err != nil {
		t.Fatalf("RunOnePage: %v", out2.err)
	}
	if out2.recordsIngested != 1 {
		t.Errorf("recordsIngested: got %d, want 1 (only the record strictly newer than the seed)", out2.recordsIngested)
	}
	if !out2.watermarkAfter.Equal(newRecord) {
		t.Errorf("watermarkAfter: got %v, want %v", out2.watermarkAfter, newRecord)
	}
}

// TestNationalLane_SubsequentRunPrefiltersOnWatermarkDate covers the normal
// case: the different_start prefilter tracks the watermark's calendar date,
// not the mask cutoff.
func TestNationalLane_SubsequentRunPrefiltersOnWatermarkDate(t *testing.T) {
	t.Parallel()
	watermark := time.Date(2026, 7, 10, 15, 30, 0, 0, time.UTC)
	fetcher := newFakeNationalFetcher()
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneB] = PollState{HighWaterMark: watermark, LastPollTime: watermark}

	h := newLaneHandler(t, fetcher, apps, state, laneBOpts(20))
	h.RunOnePage(context.Background())

	q := fetcher.queries[0]
	wantPrefilter := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	if !q.DifferentStart.Equal(wantPrefilter) {
		t.Errorf("DifferentStart prefilter: got %v, want the watermark's calendar date %v", q.DifferentStart, wantPrefilter)
	}
	if q.Mask != planit.MaskDecidedStart {
		t.Errorf("Mask: got %v, want MaskDecidedStart", q.Mask)
	}
}

// TestNationalLaneRun_EmitsSpanWithFullAttributeSet pins the "PlanIt national
// lane poll" span's attribute set — the telemetry that IS the cutover's
// safety mechanism (ADR 0041): planit.total, poll.records_seen,
// poll.records_ingested, poll.pages, poll.watermark_before/after, poll.lane.
func TestNationalLaneRun_EmitsSpanWithFullAttributeSet(t *testing.T) {
	watermark := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	ld := watermark.Add(1 * time.Hour)
	total := 1717

	fetcher := newFakeNationalFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{testApp("a", 300, ld)},
		HasMorePages: false,
		Total:        &total,
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: watermark}

	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())

	spans := recordSpans(t, func() {
		h.RunOnePage(context.Background())
	})
	span, ok := spanNamed(spans, "PlanIt national lane poll")
	if !ok {
		t.Fatalf("expected a %q span among %d recorded spans", "PlanIt national lane poll", len(spans))
	}

	if v, ok := attrValue(span, "poll.lane"); !ok || v.AsString() != "A" {
		t.Errorf("poll.lane: got %v (ok=%v), want A", v, ok)
	}
	if v, ok := attrValue(span, "poll.records_seen"); !ok || v.AsInt64() != 1 {
		t.Errorf("poll.records_seen: got %v (ok=%v), want 1", v, ok)
	}
	if v, ok := attrValue(span, "poll.records_ingested"); !ok || v.AsInt64() != 1 {
		t.Errorf("poll.records_ingested: got %v (ok=%v), want 1", v, ok)
	}
	if v, ok := attrValue(span, "poll.pages"); !ok || v.AsInt64() != 1 {
		t.Errorf("poll.pages: got %v (ok=%v), want 1", v, ok)
	}
	if v, ok := attrValue(span, "planit.total"); !ok || v.AsInt64() != 1717 {
		t.Errorf("planit.total: got %v (ok=%v), want 1717", v, ok)
	}
	if v, ok := attrValue(span, "poll.watermark_before"); !ok || v.AsString() != watermark.Format(time.RFC3339) {
		t.Errorf("poll.watermark_before: got %v (ok=%v)", v, ok)
	}
	if v, ok := attrValue(span, "poll.watermark_after"); !ok || v.AsString() != ld.Format(time.RFC3339) {
		t.Errorf("poll.watermark_after: got %v (ok=%v), want %v", v, ok, ld.Format(time.RFC3339))
	}
	if v, ok := attrValue(span, "poll.rate_limited"); !ok || v.AsBool() {
		t.Errorf("poll.rate_limited: got %v (ok=%v), want false", v, ok)
	}
	if v, ok := attrValue(span, "poll.cap_hit"); !ok || v.AsBool() {
		t.Errorf("poll.cap_hit: got %v (ok=%v), want false", v, ok)
	}
	if v, ok := attrValue(span, "poll.seeded"); !ok || v.AsBool() {
		t.Errorf("poll.seeded: got %v (ok=%v), want false (this is a steady-state walk, not a seed)", v, ok)
	}
	if v, ok := attrValue(span, "poll.different_start"); !ok || v.AsString() != watermark.Format("2006-01-02") {
		t.Errorf("poll.different_start: got %v (ok=%v), want %v", v, ok, watermark.Format("2006-01-02"))
	}
}

// TestNationalLaneRun_SeedingSpanTagsSeededAndDifferentStart proves the two
// telemetry additions requested on top of the seeding fix: poll.seeded=true
// on a seeding run, and poll.different_start present (the mask cutoff, since
// there is no watermark yet to prefilter on).
func TestNationalLaneRun_SeedingSpanTagsSeededAndDifferentStart(t *testing.T) {
	fetcher := newFakeNationalFetcher()
	total := 1717
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{testApp("a", 300, time.Date(2026, 7, 14, 5, 0, 0, 0, time.UTC))},
		Total:        &total,
	}
	apps := newFakeApps()
	state := newFakeStateStore() // no sentinel row: never run

	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())

	spans := recordSpans(t, func() {
		h.RunOnePage(context.Background())
	})
	span, ok := spanNamed(spans, "PlanIt national lane poll")
	if !ok {
		t.Fatalf("expected a %q span", "PlanIt national lane poll")
	}
	if v, ok := attrValue(span, "poll.seeded"); !ok || !v.AsBool() {
		t.Errorf("poll.seeded: got %v (ok=%v), want true", v, ok)
	}
	// 2026-07-14 minus the 90-day Lane A mask window.
	if v, ok := attrValue(span, "poll.different_start"); !ok || v.AsString() != "2026-04-15" {
		t.Errorf("poll.different_start: got %v (ok=%v), want 2026-04-15", v, ok)
	}
	if v, ok := attrValue(span, "planit.total"); !ok || v.AsInt64() != 1717 {
		t.Errorf("planit.total: got %v (ok=%v), want 1717 (a seed still stamps PlanIt's reported total)", v, ok)
	}
}

// --- NationalPollHandler.Handle: the ADR 0044 planner/executor loop ---

// newTestPlannerOpts pins a Planner over the same clock newNationalPollTestHandler uses.
func newTestPlannerOpts() PlannerOptions {
	dayStart, _ := ParseCivilTime("07:00")
	dayEnd, _ := ParseCivilTime("19:00")
	loc, _ := time.LoadLocation("Europe/London")
	return PlannerOptions{FreshnessInterval: 15 * time.Minute, DayStart: dayStart, DayEnd: dayEnd, Location: loc}
}

// TestNationalPollHandler_Handle_RunsBothLanesToCompletion covers the
// top-level planner/executor loop end to end: both lanes are due (mid-day
// UTC clock, both eligible 24/7), each has exactly one page of work, the
// loop runs both to completion and then finds nothing left to do (nil ->
// Natural).
func TestNationalPollHandler_Handle_RunsBothLanesToCompletion(t *testing.T) {
	t.Parallel()
	// Both lanes already seeded (a watermark row exists) and last polled
	// long enough ago to be due, so this exercises the steady-state delta
	// walk, not the first-run seed path.
	clockTime := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	watermark := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	longAgo := clockTime.Add(-time.Hour)
	ldA := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	ldB := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)

	fetcherA := newFakeNationalFetcher()
	fetcherA.pages[0] = planit.FetchPageResult{From: 0, Applications: []applications.PlanningApplication{testApp("a", 300, ldA)}}
	fetcherB := newFakeNationalFetcher()
	fetcherB.pages[0] = planit.FetchPageResult{From: 0, Applications: []applications.PlanningApplication{testApp("b", 300, ldB)}}

	appsA := newFakeApps()
	appsB := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: longAgo}
	state.states[sentinelLaneB] = PollState{HighWaterMark: watermark, LastPollTime: longAgo}
	logger := slog.New(slog.NewTextHandler(discard{}, nil))
	clock := func() time.Time { return clockTime }

	laneAHandler := NewNationalLaneHandler(fetcherA, state, appsA, laneAOpts(), clock, logger)
	laneBHandler := NewNationalLaneHandler(fetcherB, state, appsB, laneBOpts(20), clock, logger)
	planner := NewPlanner(newTestPlannerOpts())

	handler := NewNationalPollHandler(laneAHandler, laneBHandler, nil, planner, NationalPollOptions{HandlerBudget: 4 * time.Minute}, clock, logger)

	res, err := handler.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if res.ApplicationCount != 2 {
		t.Errorf("ApplicationCount: got %d, want 2", res.ApplicationCount)
	}
	if res.CycleType != "National" {
		t.Errorf("CycleType: got %q, want National", res.CycleType)
	}
	if res.TerminationReason != TerminationNatural {
		t.Errorf("TerminationReason: got %v, want TerminationNatural", res.TerminationReason)
	}
	if res.AuthorityErrors != 0 {
		t.Errorf("AuthorityErrors: got %d, want 0", res.AuthorityErrors)
	}
	if fetcherA.calls != 1 || fetcherB.calls != 1 {
		t.Errorf("expected exactly one fetch per lane, got A=%d B=%d", fetcherA.calls, fetcherB.calls)
	}
}

// TestNationalPollHandler_Handle_StopsOnFirstLaneError proves a lane error
// stops the WHOLE loop immediately (ADR 0044's single break, replacing the
// old per-lane fold): the cycle still returns a nil error (self-healing —
// the last checkpoint holds) but AuthorityErrors reports the stop.
func TestNationalPollHandler_Handle_StopsOnFirstLaneError(t *testing.T) {
	t.Parallel()
	clockTime := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	longAgo := clockTime.Add(-time.Hour)

	fetcherA := newFakeNationalFetcher()
	fetcherA.failNth[1] = errors.New("planit: transport blew up")
	fetcherB := newFakeNationalFetcher()
	fetcherB.pages[0] = planit.FetchPageResult{From: 0, Applications: nil, HasMorePages: false}

	apps := newFakeApps()
	state := newFakeStateStore()
	watermark := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: longAgo}
	state.states[sentinelLaneB] = PollState{HighWaterMark: watermark, LastPollTime: longAgo}
	logger := slog.New(slog.NewTextHandler(discard{}, nil))
	clock := func() time.Time { return clockTime }

	laneAHandler := NewNationalLaneHandler(fetcherA, state, apps, laneAOpts(), clock, logger)
	laneBHandler := NewNationalLaneHandler(fetcherB, state, apps, laneBOpts(20), clock, logger)
	planner := NewPlanner(newTestPlannerOpts())
	handler := NewNationalPollHandler(laneAHandler, laneBHandler, nil, planner, NationalPollOptions{HandlerBudget: 4 * time.Minute}, clock, logger)

	res, err := handler.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle must not fail the cycle on a lane error: %v", err)
	}
	if res.AuthorityErrors != 1 {
		t.Errorf("AuthorityErrors: got %d, want 1", res.AuthorityErrors)
	}
	// Lane A is the older (longAgo) and therefore LRU-first candidate, so it
	// runs (and fails) before Lane B ever gets a turn — the single break
	// covers whichever lane errors, without a per-lane fold to omit one.
	if fetcherB.calls != 0 {
		t.Errorf("lane B fetcher: got %d calls, want 0 (the loop stopped on lane A's error before lane B's turn)", fetcherB.calls)
	}
}

// TestNationalPollHandler_Handle_SkipsLaneCWhenNil pins the contract when
// Lane C is not wired (a test convenience / the current production shape
// before this PR is deployed): NextWork must never pick it, so Lane A/B run
// to completion and the cycle returns cleanly.
func TestNationalPollHandler_Handle_SkipsLaneCWhenNil(t *testing.T) {
	t.Parallel()
	clockTime := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	longAgo := clockTime.Add(-time.Hour)
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
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: longAgo}
	state.states[sentinelLaneB] = PollState{HighWaterMark: watermark, LastPollTime: longAgo}
	logger := slog.New(slog.NewTextHandler(discard{}, nil))
	clock := func() time.Time { return clockTime }

	laneAHandler := NewNationalLaneHandler(fetcherA, state, appsA, laneAOpts(), clock, logger)
	laneBHandler := NewNationalLaneHandler(fetcherB, state, appsB, laneBOpts(20), clock, logger)
	planner := NewPlanner(newTestPlannerOpts())
	handler := NewNationalPollHandler(laneAHandler, laneBHandler, nil, planner, NationalPollOptions{HandlerBudget: 4 * time.Minute}, clock, logger)

	res, err := handler.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if fetcherA.calls == 0 {
		t.Error("Lane A fetcher: got 0 calls, want Lane A to have run")
	}
	if fetcherB.calls == 0 {
		t.Error("Lane B fetcher: got 0 calls, want Lane B to have run")
	}
	if res.ApplicationCount != 2 {
		t.Errorf("ApplicationCount: got %d, want 2 (both lanes ingested, nil Lane C contributed nothing)", res.ApplicationCount)
	}
}

// TestNationalPollHandler_WithBackfillNilNeverPanics pins the wiring safety
// contract for Lane D (GH#967): WithBackfill(nil) — the shape cmd/worker's
// buildPollOrchestrator produces when POLLING_BACKFILL_ENABLED is off (its
// default) — must never panic, and Handle must complete normally with no
// backfill work attempted.
func TestNationalPollHandler_WithBackfillNilNeverPanics(t *testing.T) {
	t.Parallel()
	clockTime := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	fetcherA := newFakeNationalFetcher()
	fetcherB := newFakeNationalFetcher()
	apps := newFakeApps()
	state := newFakeStateStore()
	logger := slog.New(slog.NewTextHandler(discard{}, nil))
	clock := func() time.Time { return clockTime }

	laneAHandler := NewNationalLaneHandler(fetcherA, state, apps, laneAOpts(), clock, logger)
	laneBHandler := NewNationalLaneHandler(fetcherB, state, apps, laneBOpts(20), clock, logger)
	planner := NewPlanner(newTestPlannerOpts())
	handler := NewNationalPollHandler(laneAHandler, laneBHandler, nil, planner, NationalPollOptions{HandlerBudget: 4 * time.Minute}, clock, logger)

	handler.WithBackfill(nil)

	res, err := handler.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if res.CycleType != "National" {
		t.Errorf("CycleType: got %q, want National", res.CycleType)
	}
}

// TestNationalPollHandler_Handle_RunsBackfillLaneOutOfHours proves Lane D
// runs once the planner picks it: eligible only out-of-hours (night, no
// active window for A/B beyond their own due check), incomplete, and the
// only lane with work once A/B have run their single due page.
func TestNationalPollHandler_Handle_RunsBackfillLaneOutOfHours(t *testing.T) {
	t.Parallel()
	night := time.Date(2026, 7, 14, 3, 0, 0, 0, time.UTC) // 03:00 UTC = 03:00 GMT, out-of-hours
	fetcherA := newFakeNationalFetcher()
	fetcherB := newFakeNationalFetcher()
	apps := newFakeApps()
	state := newFakeStateStore()
	// A/B just polled: not due, so they never contend with Lane D's turn.
	watermark := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: night}
	state.states[sentinelLaneB] = PollState{HighWaterMark: watermark, LastPollTime: night}
	logger := slog.New(slog.NewTextHandler(discard{}, nil))
	clock := func() time.Time { return night }

	laneAHandler := NewNationalLaneHandler(fetcherA, state, apps, laneAOpts(), clock, logger)
	laneBHandler := NewNationalLaneHandler(fetcherB, state, apps, laneBOpts(20), clock, logger)
	planner := NewPlanner(newTestPlannerOpts())
	handler := NewNationalPollHandler(laneAHandler, laneBHandler, nil, planner, NationalPollOptions{HandlerBudget: 4 * time.Minute}, clock, logger)

	backfillFetcher := newFakeBackfillFetcher()
	backfillState := newFakeBackfillStateStore()
	backfillHandler := NewBackfillHandler(backfillFetcher, backfillState, apps, defaultBackfillOpts(), clock, logger)
	handler.WithBackfill(backfillHandler)

	if _, err := handler.Handle(context.Background()); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if backfillFetcher.calls == 0 {
		t.Error("backfill fetcher: got 0 calls, want Lane D to have run")
	}
}

// TestNationalPollHandler_Handle_LaneDRateLimitBubblesToNextCycle proves a
// 429 hitting Lane D alone still bubbles into the cycle's
// RateLimited/TerminationReason/RetryAfter fields, so Orchestrator.RunOnce
// -> NextRunScheduler.ComputeNextRun backs off the next poll cycle exactly
// as it would for an A/B rate limit (tc-hew73).
func TestNationalPollHandler_Handle_LaneDRateLimitBubblesToNextCycle(t *testing.T) {
	t.Parallel()
	night := time.Date(2026, 7, 14, 3, 0, 0, 0, time.UTC)
	fetcherA := newFakeNationalFetcher()
	fetcherB := newFakeNationalFetcher()
	apps := newFakeApps()
	state := newFakeStateStore()
	watermark := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: night}
	state.states[sentinelLaneB] = PollState{HighWaterMark: watermark, LastPollTime: night}
	logger := slog.New(slog.NewTextHandler(discard{}, nil))
	clock := func() time.Time { return night }

	laneAHandler := NewNationalLaneHandler(fetcherA, state, apps, laneAOpts(), clock, logger)
	laneBHandler := NewNationalLaneHandler(fetcherB, state, apps, laneBOpts(20), clock, logger)
	planner := NewPlanner(newTestPlannerOpts())
	handler := NewNationalPollHandler(laneAHandler, laneBHandler, nil, planner, NationalPollOptions{HandlerBudget: 4 * time.Minute}, clock, logger)

	retryAfter := 45 * time.Second
	backfillFetcher := newFakeBackfillFetcher(fakeBackfillResponse{err: &planit.RateLimitError{RetryAfter: &retryAfter}})
	backfillState := newFakeBackfillStateStore()
	backfillHandler := NewBackfillHandler(backfillFetcher, backfillState, apps, defaultBackfillOpts(), clock, logger)
	handler.WithBackfill(backfillHandler)

	res, err := handler.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !res.RateLimited {
		t.Error("RateLimited: got false, want true (Lane D was rate-limited)")
	}
	if res.TerminationReason != TerminationRateLimited {
		t.Errorf("TerminationReason: got %v, want %v", res.TerminationReason, TerminationRateLimited)
	}
	if res.RetryAfter == nil || *res.RetryAfter != retryAfter {
		t.Errorf("RetryAfter: got %v, want %v", res.RetryAfter, retryAfter)
	}
}

// TestNationalPollHandler_Handle_BudgetExhaustedIsTimeBounded proves the
// budget check happens BEFORE calling the planner: once wall-clock time has
// advanced past the deadline computed at the start of Handle, the loop stops
// immediately with TerminationTimeBounded and no lane ever runs. The clock
// double returns the deadline-computation instant on its FIRST call, then a
// later instant (already past a small budget) on every subsequent call —
// modelling genuine wall-clock advancement between Handle's setup and its
// first budget check, since a literal zero/negative HandlerBudget instead
// disables the check entirely (hasDeadline's documented "zero disables"
// contract, mirroring the legacy handler.go).
func TestNationalPollHandler_Handle_BudgetExhaustedIsTimeBounded(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	fetcherA := newFakeNationalFetcher()
	fetcherB := newFakeNationalFetcher()
	apps := newFakeApps()
	state := newFakeStateStore()
	logger := slog.New(slog.NewTextHandler(discard{}, nil))

	calls := 0
	clock := func() time.Time {
		calls++
		if calls == 1 {
			return start
		}
		return start.Add(10 * time.Minute)
	}

	laneAHandler := NewNationalLaneHandler(fetcherA, state, apps, laneAOpts(), clock, logger)
	laneBHandler := NewNationalLaneHandler(fetcherB, state, apps, laneBOpts(20), clock, logger)
	planner := NewPlanner(newTestPlannerOpts())
	handler := NewNationalPollHandler(laneAHandler, laneBHandler, nil, planner, NationalPollOptions{HandlerBudget: time.Minute}, clock, logger)

	res, err := handler.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if res.TerminationReason != TerminationTimeBounded {
		t.Errorf("TerminationReason: got %v, want TerminationTimeBounded", res.TerminationReason)
	}
	if fetcherA.calls != 0 || fetcherB.calls != 0 {
		t.Errorf("expected no lane to run when the budget is already exhausted, got A=%d B=%d", fetcherA.calls, fetcherB.calls)
	}
}

// TestNationalPollHandler_Handle_ExcludesCappedLaneForRestOfCycle proves
// NationalLaneOptions.MaxPages is enforced per-cycle (ADR 0044): Lane B hits
// its cap after 2 pages and is excluded from planner candidacy for the rest
// of THIS cycle (its real persisted cursor is untouched, so it resumes
// normally next cycle) — Lane A, uncapped, keeps draining past 2 pages in
// the same cycle.
func TestNationalPollHandler_Handle_ExcludesCappedLaneForRestOfCycle(t *testing.T) {
	t.Parallel()
	clockTime := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	longAgo := clockTime.Add(-time.Hour)
	watermark := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	ld := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

	fetcherB := newFakeNationalFetcher()
	for i := range 5 {
		fetcherB.pages[i] = planit.FetchPageResult{
			From:         i,
			Applications: []applications.PlanningApplication{testApp("d", 300, ld)},
			HasMorePages: true, // every page claims more follow, so B never completes on its own
		}
	}
	fetcherA := newFakeNationalFetcher()
	fetcherA.pages[0] = planit.FetchPageResult{From: 0, Applications: nil, HasMorePages: false}

	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: longAgo}
	state.states[sentinelLaneB] = PollState{HighWaterMark: watermark, LastPollTime: longAgo}
	logger := slog.New(slog.NewTextHandler(discard{}, nil))
	clock := func() time.Time { return clockTime }

	laneAHandler := NewNationalLaneHandler(fetcherA, state, apps, laneAOpts(), clock, logger)
	laneBHandler := NewNationalLaneHandler(fetcherB, state, apps, laneBOpts(2), clock, logger) // cap: 2 pages/cycle
	planner := NewPlanner(newTestPlannerOpts())
	handler := NewNationalPollHandler(laneAHandler, laneBHandler, nil, planner, NationalPollOptions{HandlerBudget: 4 * time.Minute}, clock, logger)

	res, err := handler.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if fetcherB.calls != 2 {
		t.Errorf("lane B fetches: got %d, want exactly 2 (the per-cycle cap)", fetcherB.calls)
	}
	if res.TerminationReason != TerminationNatural {
		t.Errorf("TerminationReason: got %v, want TerminationNatural (A completed, B capped — nothing left eligible-with-work)", res.TerminationReason)
	}
	// Both capped calls land on StartIndex 0 (the 100-record resume overlap
	// swamps these 1-record fake pages' tiny NextIndex), so the persisted
	// cursor's exact NextIndex isn't the interesting assertion here — what
	// matters is that it is still ACTIVE (mid-drain), proving the cap never
	// touched Lane B's real persisted state, only the in-memory candidacy
	// this cycle.
	cursor := state.states[sentinelLaneB].Cursor
	if cursor == nil {
		t.Fatal("lane B's persisted cursor: got nil, want an active mid-drain cursor (untouched by the in-cycle cap, ready to resume next cycle)")
	}
}
