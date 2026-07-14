package polling

import (
	"context"
	"errors"
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
// encountered first.
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
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
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
		t.Errorf("watermarkAfter: got %v, want %v (max ingested this run)", out.watermarkAfter, newer)
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
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
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
	out := h.Run(context.Background())

	if out.planitTotal != nil {
		t.Errorf("planitTotal: got %v, want nil (PlanIt omitted total)", out.planitTotal)
	}
}

// TestNationalLane_NeverAdvancesWatermarkPastAnErroredPage is the safety
// invariant: a page that ingests successfully, followed by a page that
// fails, must leave the watermark exactly where it was before the run —
// never partway advanced to the first page's max — so nothing between the
// old watermark and the failed page is silently skipped on the next run.
func TestNationalLane_NeverAdvancesWatermarkPastAnErroredPage(t *testing.T) {
	t.Parallel()
	watermark := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	page1LD := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

	fetcher := newFakeNationalFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{testApp("page1", 300, page1LD)},
		HasMorePages: true, // more pages follow -> the loop will fetch index 1
	}
	fetcher.failNth[2] = errors.New("planit: transport blew up")

	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneA] = PollState{HighWaterMark: watermark, LastPollTime: watermark}

	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())
	out := h.Run(context.Background())

	if out.err == nil {
		t.Fatal("expected the second page's fetch error to surface on the outcome")
	}
	if len(apps.upserts) != 1 {
		t.Errorf("page 1's record should still have been ingested: got %d upserts", len(apps.upserts))
	}
	if !out.watermarkAfter.Equal(watermark) {
		t.Errorf("watermarkAfter: got %v, want unchanged %v (never advance past an errored page)", out.watermarkAfter, watermark)
	}
}

// TestNationalLane_NeverAdvancesWatermarkOn429 mirrors the errored-page test
// for a 429: the watermark must not advance even though the walk stopped for
// an "expected" reason rather than a hard error.
func TestNationalLane_NeverAdvancesWatermarkOn429(t *testing.T) {
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

	h := newLaneHandler(t, fetcher, apps, state, laneAOpts())
	out := h.Run(context.Background())

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

// TestNationalLane_LaneBPageCapStopsWalkAndFreezesWatermark proves Lane B's
// hard page cap: the walk stops after MaxPages even though more pages remain,
// and — because a cap-hit is an incomplete run — the watermark freezes rather
// than advancing to whatever the capped walk saw.
func TestNationalLane_LaneBPageCapStopsWalkAndFreezesWatermark(t *testing.T) {
	t.Parallel()
	watermark := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	ld := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

	fetcher := newFakeNationalFetcher()
	for i := range 3 {
		fetcher.pages[i] = planit.FetchPageResult{
			From:         i,
			Applications: []applications.PlanningApplication{testApp("d", 300, ld)},
			HasMorePages: true, // every page claims more follow
		}
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneB] = PollState{HighWaterMark: watermark, LastPollTime: watermark}

	h := newLaneHandler(t, fetcher, apps, state, laneBOpts(2))
	out := h.Run(context.Background())

	if out.pages != 2 {
		t.Errorf("pages: got %d, want 2 (capped)", out.pages)
	}
	if !out.capHit {
		t.Error("capHit: got false, want true")
	}
	if !out.watermarkAfter.Equal(watermark) {
		t.Errorf("watermarkAfter: got %v, want unchanged %v (a cap hit is an incomplete run)", out.watermarkAfter, watermark)
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
	h.Run(context.Background())

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
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
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
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
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
	out := h.Run(context.Background())

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
	out := h.Run(context.Background())

	if !out.rateLimited {
		t.Fatal("expected rateLimited=true")
	}
	if len(state.saves) != 0 {
		t.Errorf("expected NO Save call when the seed fetch is rate-limited, got %+v", state.saves)
	}
}

// TestNationalLane_SeedThenForwardFlow proves seeding does not break steady
// state: after a seeded run, a second run from that watermark ingests only
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

	seedOut := h.Run(context.Background())
	if seedOut.recordsIngested != 0 {
		t.Fatalf("seed run must not ingest: got %d", seedOut.recordsIngested)
	}

	// Second run: a fresh descending walk (a new fetcher, as a new PlanIt
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
	out2 := h2.Run(context.Background())

	if out2.err != nil {
		t.Fatalf("Run: %v", out2.err)
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
	h.Run(context.Background())

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
		h.Run(context.Background())
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
		h.Run(context.Background())
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

// TestNationalPollHandler_Handle_AggregatesBothLanes covers the top-level
// cycleHandler: both lanes run, their ingested counts sum, and CycleType is
// stamped "National" so runPollSB's existing polling.cycle_type tag keeps
// working with a value that reflects the new design.
func TestNationalPollHandler_Handle_AggregatesBothLanes(t *testing.T) {
	t.Parallel()
	// Both lanes already seeded (a watermark row exists), so this exercises
	// the steady-state delta walk, not the first-run seed path.
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
}

// TestNationalPollHandler_Handle_CountsLaneErrorsWithoutFailingTheCycle
// proves a lane error is counted, not fatal: the cycle still returns a nil
// error so the orchestrator schedules the next run (self-healing — no
// per-authority state to strand).
func TestNationalPollHandler_Handle_CountsLaneErrorsWithoutFailingTheCycle(t *testing.T) {
	t.Parallel()
	fetcherA := newFakeNationalFetcher()
	fetcherA.failNth[1] = errors.New("planit: transport blew up")
	fetcherB := newFakeNationalFetcher()

	apps := newFakeApps()
	state := newFakeStateStore()
	logger := slog.New(slog.NewTextHandler(discard{}, nil))
	clock := func() time.Time { return time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) }

	laneAHandler := NewNationalLaneHandler(fetcherA, state, apps, laneAOpts(), clock, logger)
	laneBHandler := NewNationalLaneHandler(fetcherB, state, apps, laneBOpts(20), clock, logger)
	handler := NewNationalPollHandler(laneAHandler, laneBHandler, nil, clock, logger)

	res, err := handler.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle must not fail the cycle on a lane error: %v", err)
	}
	if res.AuthorityErrors != 1 {
		t.Errorf("AuthorityErrors: got %d, want 1", res.AuthorityErrors)
	}
}
