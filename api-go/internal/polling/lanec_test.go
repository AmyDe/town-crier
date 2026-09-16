package polling

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

// laneCTestPageSize is the per-page record count fakeInverseMaskFetcher's
// virtual-result-set mode (f.rows) serves, mirroring planit.nationalPageSize.
const laneCTestPageSize = 300

// fakeInverseMaskFetcher serves Lane C rolling-window pages. Pages come from
// one of two modes: pre-canned pages keyed by the requested 0-based record
// offset (f.pages), or — when f.rows is set — real index-paginated slices of
// a single virtual result set (laneCTestPageSize per page, HasMorePages until
// the tail), the way PlanIt actually pages, so a resume test can prove the
// cursor walks PAST a head-of-window cluster rather than re-fetching a fixed
// canned page forever. It can be primed to fail a specific fetch ordinal
// (1-based, failNth). tc-hku56 / GH#1140 removed FetchByUID and its
// hydration-response fields (hydrated/hydratedMulti/hydrateErr/hydrateCalls)
// entirely: Lane C no longer makes a second PlanIt request per row, so a
// hydration call is now a compile error, not just untested.
type fakeInverseMaskFetcher struct {
	pages   map[int]planit.FetchPageResult
	rows    []applications.PlanningApplication
	failNth map[int]error
	calls   int
	queries []planit.NationalInverseMaskQuery
}

func newFakeInverseMaskFetcher() *fakeInverseMaskFetcher {
	return &fakeInverseMaskFetcher{
		pages:   map[int]planit.FetchPageResult{},
		failNth: map[int]error{},
	}
}

func (f *fakeInverseMaskFetcher) FetchInverseMaskPage(_ context.Context, q planit.NationalInverseMaskQuery) (planit.FetchPageResult, error) {
	f.calls++
	f.queries = append(f.queries, q)
	if err, ok := f.failNth[f.calls]; ok {
		return planit.FetchPageResult{}, err
	}
	if f.rows != nil {
		return f.pageFromRows(q.StartIndex), nil
	}
	res, ok := f.pages[q.StartIndex]
	if !ok {
		return planit.FetchPageResult{From: q.StartIndex, HasMorePages: false}, nil
	}
	return res, nil
}

// pageFromRows serves one index-paginated slice of f.rows, mirroring PlanIt's
// own paging: up to laneCTestPageSize records from start, HasMorePages set
// while the tail is unreached.
func (f *fakeInverseMaskFetcher) pageFromRows(start int) planit.FetchPageResult {
	if start > len(f.rows) {
		start = len(f.rows)
	}
	end := min(start+laneCTestPageSize, len(f.rows))
	return planit.FetchPageResult{
		From:         start,
		Applications: append([]applications.PlanningApplication(nil), f.rows[start:end]...),
		HasMorePages: end < len(f.rows),
	}
}

// lightApp builds a Lane C page row carrying the fields most tests need to
// exercise the diff/ingest gate: uid, area_id, app_state, last_different.
// tc-hku56: the page row is now the FULL ingestSelectFields projection (no
// separate hydration fetch), so a test proving a row's other fields survive
// ingestion (e.g. Description, OtherFields) builds its own row directly
// rather than going through this minimal helper.
func lightApp(uid string, areaID int, appState string, lastDifferent time.Time) applications.PlanningApplication {
	return applications.PlanningApplication{
		UID:           uid,
		AreaID:        areaID,
		AppState:      &appState,
		LastDifferent: lastDifferent,
	}
}

// fakeScopedApps wraps fakeApps to record the authorityCode each GetByUID
// call received, so a test can assert Lane C scopes its existence check by
// area_id rather than diffing by a bare uid that could collide across
// authorities (ADR 0044's national-query correctness fix).
type fakeScopedApps struct {
	*fakeApps
	authorityCodesSeen []string
}

func (f *fakeScopedApps) GetByUID(ctx context.Context, uid, authorityCode string) (applications.PlanningApplication, bool, error) {
	f.authorityCodesSeen = append(f.authorityCodesSeen, authorityCode)
	return f.fakeApps.GetByUID(ctx, uid, authorityCode)
}

// laneCNow is the fixed clock newLaneCHandler pins (2026-07-14T12:00:00Z);
// laneCToday is its calendar date at UTC midnight — the value an in-flight
// scan cursor's DifferentStart must carry to count as same-day/active.
var (
	laneCNow   = time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	laneCToday = time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
)

// newLaneCHandler wires an InverseMaskLaneHandler pinned to the laneCNow
// clock.
func newLaneCHandler(t *testing.T, fetcher *fakeInverseMaskFetcher, apps applicationStore, state *fakeStateStore, opts InverseMaskOptions) *InverseMaskLaneHandler {
	t.Helper()
	return newLaneCHandlerAt(t, fetcher, apps, state, opts, func() time.Time { return laneCNow })
}

// newLaneCHandlerAt is newLaneCHandler with a caller-supplied clock, for the
// cases that need "now" somewhere other than laneCNow (e.g. a scan resuming
// across a day rollover, or the pre-#1127 frozen epoch row).
func newLaneCHandlerAt(t *testing.T, fetcher *fakeInverseMaskFetcher, apps applicationStore, state *fakeStateStore, opts InverseMaskOptions, clock func() time.Time) *InverseMaskLaneHandler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(discard{}, nil))
	return NewInverseMaskLaneHandler(fetcher, state, apps, opts, clock, logger)
}

func defaultInverseMaskOpts() InverseMaskOptions {
	return InverseMaskOptions{MaskWindow: 90 * 24 * time.Hour}
}

// tc-hku56 deleted TestInverseMaskLane_ResumeOverlapSmallerThanHydrationCap:
// it pinned laneCResumeOverlapRecords staying strictly below the now-deleted
// maxHydrationsPerPass. With the id_match hydration fan-out gone there is no
// hydration cap left for the overlap to stay under — see
// laneCResumeOverlapRecords' rewritten doc comment.

// TestRunOnePage_WindowDaysClampedToRange pins #1127's window-width rule:
// N = clamp(days_since(last_clean_scan_at) + 1, 2, maxInverseMaskWindowDays),
// measured on truncate-to-date values, with a zero last_clean_scan_at
// yielding the cap.
func TestRunOnePage_WindowDaysClampedToRange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		lastCleanScanAt time.Time
		want            int
	}{
		{"clean scan today: N=2", laneCNow, 2},
		{"clean scan yesterday: N=2", laneCNow.AddDate(0, 0, -1), 2},
		{"clean scan two days ago: N=3 (cap)", laneCNow.AddDate(0, 0, -2), 3},
		{"clean scan 30 days ago: N=3 (cap)", laneCNow.AddDate(0, 0, -30), 3},
		{"never cleanly scanned (zero): N=3 (cap)", time.Time{}, 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fetcher := newFakeInverseMaskFetcher()
			fetcher.pages[0] = planit.FetchPageResult{From: 0, Applications: nil, HasMorePages: false}
			apps := newFakeApps()
			state := newFakeStateStore()
			state.states[sentinelLaneC] = PollState{HighWaterMark: tc.lastCleanScanAt}

			h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
			out := h.RunOnePage(context.Background())
			if out.err != nil {
				t.Fatalf("RunOnePage: %v", out.err)
			}
			if len(fetcher.queries) != 1 {
				t.Fatalf("expected exactly one fetch, got %d", len(fetcher.queries))
			}
			if got := fetcher.queries[0].WindowDays; got != tc.want {
				t.Errorf("WindowDays: got %d, want %d", got, tc.want)
			}
		})
	}
}

// TestRunOnePage_CleanScanStampsLastCleanScanAtAndClearsCursor pins the
// clean-scan completion path (#1127): a scan that reaches the last page with
// no 429 and no straggler-error bail stamps last_clean_scan_at = now and
// clears the cursor — that is what resets N to 2 next cycle. See
// TestInverseMaskLane_LastPageStampsLastCleanScanAt for the same completion
// path when the last page ALSO ingests genuine stragglers.
func TestRunOnePage_CleanScanStampsLastCleanScanAtAndClearsCursor(t *testing.T) {
	t.Parallel()
	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{From: 0, Applications: nil, HasMorePages: false}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: laneCNow.AddDate(0, 0, -5)}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())
	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	got := state.states[sentinelLaneC]
	if !got.HighWaterMark.Equal(laneCNow) {
		t.Errorf("last_clean_scan_at (HighWaterMark): got %v, want now %v", got.HighWaterMark, laneCNow)
	}
	if got.Cursor != nil {
		t.Errorf("cursor: got %+v, want nil (cleared on a clean scan)", got.Cursor)
	}
}

// tc-hku56 deleted TestRunOnePage_RateLimitMidScanKeepsLastCleanScanAt: its
// 429 came from a hydration sub-fetch, which no longer exists. The
// page-fetch 429 case (the only PlanIt request left in RunOnePage) is already
// covered by TestInverseMaskLane_RateLimitedPageFetchPreservesCursorAdvancesLastPollTime.

// TestRunOnePage_DayRolloverDiscardsStaleCursor pins the calendar-day
// staleness guard (#1127): a cursor whose DifferentStart is yesterday is
// meaningless once the rolling window has shifted a day, so the scan restarts
// at index 0 with a freshly recomputed N.
func TestRunOnePage_DayRolloverDiscardsStaleCursor(t *testing.T) {
	t.Parallel()
	yesterday := laneCToday.AddDate(0, 0, -1)

	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{From: 0, Applications: nil, HasMorePages: false}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{
		HighWaterMark: yesterday, // last clean scan was yesterday: N = clamp(1+1, 2, 3) = 2
		Cursor:        &PollCursor{DifferentStart: yesterday, NextIndex: 900},
	}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())
	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if len(fetcher.queries) != 1 {
		t.Fatalf("expected exactly one fetch, got %d", len(fetcher.queries))
	}
	if fetcher.queries[0].StartIndex != 0 {
		t.Errorf("StartIndex: got %d, want 0 (the stale cross-day cursor is discarded)", fetcher.queries[0].StartIndex)
	}
	if fetcher.queries[0].WindowDays != 2 {
		t.Errorf("WindowDays: got %d, want 2 (recomputed from last_clean_scan_at = yesterday)", fetcher.queries[0].WindowDays)
	}
}

// TestRunOnePage_StaleFrozenEpochRowIsColdStart pins #1127's compatibility
// requirement: the frozen pre-#1127 poll_state row -3 (epoch semantics:
// HighWaterMark 2026-07-20, Cursor{DifferentStart: 2026-07-19, NextIndex: 33})
// must not panic or replay history when the fix runs. The cross-day cursor is
// discarded (index 0) and the weeks-old HighWaterMark yields the window cap —
// a normal, bounded different=3 scan.
func TestRunOnePage_StaleFrozenEpochRowIsColdStart(t *testing.T) {
	t.Parallel()
	// "now" is the day the fix ships, well after the row froze in 2026-07.
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	frozenHWM := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	frozenCursorDate := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)

	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{From: 0, Applications: nil, HasMorePages: false}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{
		HighWaterMark: frozenHWM,
		Cursor:        &PollCursor{DifferentStart: frozenCursorDate, NextIndex: 33},
	}

	h := newLaneCHandlerAt(t, fetcher, apps, state, defaultInverseMaskOpts(), func() time.Time { return now })
	out := h.RunOnePage(context.Background())
	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if len(fetcher.queries) != 1 {
		t.Fatalf("expected exactly one fetch, got %d", len(fetcher.queries))
	}
	if got := fetcher.queries[0]; got.WindowDays != 3 || got.StartIndex != 0 {
		t.Errorf("first fetch: got WindowDays=%d StartIndex=%d, want 3 / 0 (bounded cold start, no historical replay)", got.WindowDays, got.StartIndex)
	}
}

// TestInverseMaskLane_ResumesActiveScanWithOverlap proves the within-scan
// checkpoint's resume story (GH#986): a same-day cursor resumes pagination at
// max(0, NextIndex-laneCResumeOverlapRecords) — Lane C's own, deliberately
// smaller overlap (tc-nkvil), not the shared resumeOverlapRecords Lane A/B
// use — rather than either restarting or resuming at the checkpointed index
// with no safety margin.
func TestInverseMaskLane_ResumesActiveScanWithOverlap(t *testing.T) {
	t.Parallel()
	fetcher := newFakeInverseMaskFetcher()
	// 300 - the 10-record Lane C resume overlap = 290.
	fetcher.pages[290] = planit.FetchPageResult{From: 290, Applications: nil, HasMorePages: false}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{
		HighWaterMark: laneCNow.AddDate(0, 0, -1), // last clean scan yesterday
		Cursor:        &PollCursor{DifferentStart: laneCToday, NextIndex: 300},
	}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if len(fetcher.queries) != 1 || fetcher.queries[0].StartIndex != 290 {
		t.Fatalf("expected exactly one fetch at StartIndex 290 (300 - the 10-record Lane C resume overlap), got %+v", fetcher.queries)
	}
	if fetcher.queries[0].WindowDays != 2 {
		t.Errorf("WindowDays: got %d, want 2", fetcher.queries[0].WindowDays)
	}
	if got := state.states[sentinelLaneC].Cursor; got != nil {
		t.Errorf("cursor: got %+v, want nil (scan completed on an empty final page)", got)
	}
	if !state.states[sentinelLaneC].HighWaterMark.Equal(laneCNow) {
		t.Errorf("last_clean_scan_at: got %v, want now %v (clean scan)", state.states[sentinelLaneC].HighWaterMark, laneCNow)
	}
}

// TestInverseMaskLane_ResumeOverlapDedupesAlreadyProcessedRows proves the
// resume overlap's safety property (GH#986): rows the overlap window
// re-serves that are ALREADY correct in Postgres dedupe via
// GetByUID/inverseMaskDiffers and are never re-ingested, while a genuine
// straggler beyond the overlap zone still ingests normally — the overlap
// costs a few redundant existence reads, not duplicate notifications.
func TestInverseMaskLane_ResumeOverlapDedupesAlreadyProcessedRows(t *testing.T) {
	t.Parallel()
	ld := laneCNow.Add(-time.Hour)

	same := "Permitted"
	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[290] = planit.FetchPageResult{ // 300 - the 10-record Lane C resume overlap
		From: 290,
		Applications: []applications.PlanningApplication{
			lightApp("already/FUL", 99, same, ld), // in the overlap window: unchanged since last pass
			lightApp("genuine/FUL", 99, "Permitted", ld),
		},
		HasMorePages: false,
	}

	apps := newFakeApps()
	apps.existing["already/FUL"] = applications.PlanningApplication{UID: "already/FUL", AreaID: 99, AppState: &same, LastDifferent: ld}
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{
		HighWaterMark: laneCNow.AddDate(0, 0, -1),
		Cursor:        &PollCursor{DifferentStart: laneCToday, NextIndex: 300},
	}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if len(apps.upserts) != 1 || apps.upserts[0].UID != "genuine/FUL" {
		t.Errorf("expected only the genuine straggler to be ingested (the overlap-reprocessed row dedupes via GetByUID/inverseMaskDiffers): got %+v", apps.upserts)
	}
	if out.recordsIngested != 1 {
		t.Errorf("recordsIngested: got %d, want 1 (the already-processed row must not be re-notified)", out.recordsIngested)
	}
}

// #1127 deleted TestInverseMaskLane_PinnedCeilingStopsTheEpoch and
// TestInverseMaskLane_SkipsRecordsAtOrBeforeEpochLower: with a rolling
// different=N window there is no pinned epoch_upper to stop at and no
// epoch_lower to skip below — every row the query returns is within the last
// N days by construction, and every one is processed (dedup still bounds the
// work). TestRunOnePage_WindowDaysClampedToRange and the resume/dedupe tests
// cover the replacement model.

// TestInverseMaskLane_LastDifferentOnlyChurnDoesNotIngest (renamed from
// ...DoesNotHydrate, tc-hku56) is the ADR 0044 §4 anti-amplification test: a
// row whose app_state and decided_date both still match Postgres, but whose
// last_different has moved (a re-index bump), must NOT be treated as a
// straggler — this is the exact bug the old per-authority
// ReconciliationHandler hit.
func TestInverseMaskLane_LastDifferentOnlyChurnDoesNotIngest(t *testing.T) {
	t.Parallel()
	windowStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	lastCleanScanAt := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	oldLD := windowStart.Add(-24 * time.Hour) // the persisted record's own last_different — irrelevant to the diff
	newLD := windowStart.Add(time.Hour)       // only last_different changed (a re-index bump)

	same := "Undecided"
	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{lightApp("24/0001/FUL", 99, same, newLD)},
		HasMorePages: false,
	}
	apps := newFakeApps()
	apps.existing["24/0001/FUL"] = applications.PlanningApplication{UID: "24/0001/FUL", AreaID: 99, AppState: &same, LastDifferent: oldLD}
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: lastCleanScanAt, Cursor: &PollCursor{DifferentStart: laneCToday, NextIndex: 0}}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if len(apps.upserts) != 0 {
		t.Errorf("a last_different-only churned row must NOT ingest: upserts=%+v", apps.upserts)
	}
	if out.recordsIngested != 0 {
		t.Errorf("recordsIngested: got %d, want 0", out.recordsIngested)
	}
}

// TestInverseMaskLane_AppStateDriftIngests (renamed from ...Hydrates,
// tc-hku56) is the positive case alongside the anti-amplification test: a
// genuine app_state change DOES ingest, directly from the page row (no
// separate hydration fetch).
func TestInverseMaskLane_AppStateDriftIngests(t *testing.T) {
	t.Parallel()
	windowStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	lastCleanScanAt := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	newLD := windowStart.Add(time.Hour)

	existingState := "Undecided"
	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{lightApp("24/0001/FUL", 99, "Permitted", newLD)},
		HasMorePages: false,
	}

	apps := newFakeApps()
	apps.existing["24/0001/FUL"] = applications.PlanningApplication{UID: "24/0001/FUL", AreaID: 99, AppState: &existingState, LastDifferent: windowStart.Add(-time.Hour)}
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: lastCleanScanAt, Cursor: &PollCursor{DifferentStart: laneCToday, NextIndex: 0}}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if out.recordsIngested != 1 {
		t.Errorf("recordsIngested: got %d, want 1", out.recordsIngested)
	}
	if len(apps.upserts) != 1 || apps.upserts[0].UID != "24/0001/FUL" {
		t.Fatalf("upserts: got %+v", apps.upserts)
	}
}

// tc-hku56 deleted TestInverseMaskLane_HydratesCollidingUIDByAreaID: it
// proved the id_match hydration lookup's cross-authority uid-collision guard
// (Croydon vs Bassetlaw sharing a bare uid). With FetchByUID gone from Lane
// C's interface there is no id_match lookup left to guard — every row the
// national page returns already carries its own area_id, and
// TestInverseMaskLane_UsesAreaIDForAuthorityScopedExistenceCheck below still
// covers scoping the existence check by it.

// TestInverseMaskLane_UsesAreaIDForAuthorityScopedExistenceCheck pins the
// deliberate ADR 0044 deviation from the issue's literal query string: a
// NATIONAL query's uid alone is not enough to scope the existence check
// (PlanIt's uid is only unique within one authority) — the light row's
// area_id must build the authorityCode GetByUID is called with. tc-hku56:
// GetByUID is now called TWICE for a genuinely-differing row — once by
// processStraggler's own diff check, once more inside the Ingester's Ingest
// — both scoped by the same area_id-derived authorityCode. This doubled read
// volume is the accepted trade-off the issue's "Postgres read volume rises"
// research note calls out explicitly, not a regression to chase down.
func TestInverseMaskLane_UsesAreaIDForAuthorityScopedExistenceCheck(t *testing.T) {
	t.Parallel()
	windowStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	lastCleanScanAt := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	newLD := windowStart.Add(time.Hour)

	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{lightApp("24/0001/FUL", 300, "Undecided", newLD)},
		HasMorePages: false,
	}
	apps := &fakeScopedApps{fakeApps: newFakeApps()}
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: lastCleanScanAt, Cursor: &PollCursor{DifferentStart: laneCToday, NextIndex: 0}}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	for i, code := range apps.authorityCodesSeen {
		if code != "300" {
			t.Errorf("authorityCode call %d: got %q, want %q (built from the light row's area_id)", i, code, "300")
		}
	}
	if len(apps.authorityCodesSeen) != 2 {
		t.Errorf("GetByUID calls: got %d, want 2 (processStraggler's own diff check plus the Ingester's internal read)", len(apps.authorityCodesSeen))
	}
}

// TestInverseMaskLane_RateLimitedPageFetchPreservesCursorAdvancesLastPollTime
// is GH#986 acceptance criterion (d): a page-fetch 429 must never lose the
// existing checkpoint (the cursor's NextIndex is re-saved unchanged, exactly
// as loaded — nothing was actually fetched, so there is no new progress to
// record), but it MUST advance last_poll_time so the planner's LRU rotates
// off Lane C instead of freezing it at the front of the queue forever (the
// observed prod livelock).
func TestInverseMaskLane_RateLimitedPageFetchPreservesCursorAdvancesLastPollTime(t *testing.T) {
	t.Parallel()
	lastCleanScanAt := laneCNow.AddDate(0, 0, -2)
	retryAfter := 30 * time.Second
	wantNow := laneCNow

	fetcher := newFakeInverseMaskFetcher()
	fetcher.failNth[1] = &planit.RateLimitError{RetryAfter: &retryAfter}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: lastCleanScanAt, Cursor: &PollCursor{DifferentStart: laneCToday, NextIndex: 300}}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if !out.rateLimited {
		t.Fatal("expected rateLimited=true")
	}
	if out.retryAfter == nil || *out.retryAfter != retryAfter {
		t.Errorf("retryAfter: got %v, want %v", out.retryAfter, retryAfter)
	}
	got := state.states[sentinelLaneC].Cursor
	if got == nil || got.NextIndex != 300 {
		t.Errorf("cursor: got %+v, want the preserved checkpoint (NextIndex=300) — the next cycle resumes here, no re-tread", got)
	}
	if lastPoll := state.states[sentinelLaneC].LastPollTime; !lastPoll.Equal(wantNow) {
		t.Errorf("LastPollTime: got %v, want %v (must advance so the planner LRU rotates off this lane)", lastPoll, wantNow)
	}
}

// TestInverseMaskLane_FreshScanPageFetch429PreservesLastCleanScanAt covers
// the page-fetch-429 checkpoint's OTHER shape: a 429 on the very first fetch
// of a fresh scan (no active cursor). The re-save must persist the state
// EXACTLY as loaded — last_clean_scan_at unchanged, cursor still nil — and
// only advance last_poll_time so the LRU rotates.
func TestInverseMaskLane_FreshScanPageFetch429PreservesLastCleanScanAt(t *testing.T) {
	t.Parallel()
	lastCleanScanAt := laneCNow.AddDate(0, 0, -4)
	retryAfter := 15 * time.Second
	wantNow := laneCNow

	fetcher := newFakeInverseMaskFetcher()
	fetcher.failNth[1] = &planit.RateLimitError{RetryAfter: &retryAfter}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: lastCleanScanAt} // no cursor: a fresh scan

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if !out.rateLimited {
		t.Fatal("expected rateLimited=true")
	}
	got := state.states[sentinelLaneC]
	if !got.HighWaterMark.Equal(lastCleanScanAt) {
		t.Errorf("last_clean_scan_at: got %v, want the unchanged loaded value %v (never stamped by a failed fetch)", got.HighWaterMark, lastCleanScanAt)
	}
	if got.Cursor != nil {
		t.Errorf("cursor: got %+v, want nil (the scan never started)", got.Cursor)
	}
	if !got.LastPollTime.Equal(wantNow) {
		t.Errorf("LastPollTime: got %v, want %v (must still advance so LRU rotates)", got.LastPollTime, wantNow)
	}
}

// tc-hku56 deleted TestInverseMaskLane_HydrationRateLimitStopsTheWholePage:
// its 429 came from a hydration sub-fetch, which no longer exists. A 429 can
// now only ever come from the page fetch, which is covered by
// TestInverseMaskLane_RateLimitedPageFetchPreservesCursorAdvancesLastPollTime.

// TestInverseMaskLane_PageFetchTimeoutSetsTimedOut proves a page-fetch
// client-side timeout (the real prod shape: a *url.Error wrapping
// context.DeadlineExceeded once PlanIt's HTTP client's retries are
// exhausted) is flagged on the outcome via timedOut, distinguishing it from
// a plain fetch error so NationalPollHandler.Handle can classify the cycle
// as TerminationTimeout rather than TerminationNatural (tc-pmh5y).
func TestInverseMaskLane_PageFetchTimeoutSetsTimedOut(t *testing.T) {
	t.Parallel()
	lastCleanScanAt := laneCNow.AddDate(0, 0, -2)

	fetcher := newFakeInverseMaskFetcher()
	fetcher.failNth[1] = &url.Error{Op: "Get", URL: "https://www.planit.org.uk/api/applics/json", Err: context.DeadlineExceeded}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: lastCleanScanAt, Cursor: &PollCursor{DifferentStart: laneCToday, NextIndex: 300}}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err == nil {
		t.Fatal("expected the timed-out fetch to surface as out.err")
	}
	if !out.timedOut {
		t.Error("timedOut: got false, want true (page-fetch client timeout)")
	}
	if !out.planitOrigin {
		t.Error("planitOrigin: got false, want true (page-fetch error, tc-uitxr)")
	}
}

// tc-hku56 deleted TestInverseMaskLane_HydrationTimeoutSetsTimedOut: it
// pinned a timeout on the FetchByUID hydration sub-fetch (the exact site that
// produced the real 2026-07-23 prod failure), which no longer exists.
// TestInverseMaskLane_PageFetchTimeoutSetsTimedOut above covers the one
// PlanIt request that remains.

// TestInverseMaskLane_PageFetchErrorPlusWatermarkSaveFailureClearsPlanitOrigin
// covers the gap CodeRabbit flagged as a follow-up on tc-uitxr: the page-fetch
// error path sets planitOrigin=true, then unconditionally re-saves the
// watermark (GH#986, so LastPollTime still advances and the lane rotates off
// the LRU). If THAT save also fails, the failure is a genuine Postgres/
// state-store problem -- never PlanIt's fault -- so it must surface on
// out.err rather than being silently dropped, and it must clear
// planitOrigin so NationalPollHandler.Handle never misclassifies the cycle
// as self-healing PlanIt noise and lets runPollSB exit 0 on a real
// persistence failure.
func TestInverseMaskLane_PageFetchErrorPlusWatermarkSaveFailureClearsPlanitOrigin(t *testing.T) {
	t.Parallel()
	fetchErr := errors.New("planit: page fetch failed")
	saveErr := errors.New("postgres: save failed")

	fetcher := newFakeInverseMaskFetcher()
	fetcher.failNth[1] = fetchErr
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: laneCNow.AddDate(0, 0, -2), Cursor: &PollCursor{DifferentStart: laneCToday, NextIndex: 300}}
	state.saveErr = saveErr

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.planitOrigin {
		t.Error("planitOrigin: got true, want false (watermark save also failed, so this is a genuine non-PlanIt failure)")
	}
	if out.err == nil || !errors.Is(out.err, fetchErr) {
		t.Errorf("out.err: got %v, want it to wrap the original page-fetch error %v", out.err, fetchErr)
	}
	if out.err == nil || !errors.Is(out.err, saveErr) {
		t.Errorf("out.err: got %v, want it to wrap the watermark save error %v", out.err, saveErr)
	}
}

// tc-hku56 deleted TestInverseMaskLane_HydrationErrorPlusWatermarkSaveFailureClearsPlanitOrigin:
// it covered a hydration fetch error stacked with a watermark-save failure.
// The hydration sub-fetch no longer exists; the page-fetch equivalent above
// (TestInverseMaskLane_PageFetchErrorPlusWatermarkSaveFailureClearsPlanitOrigin)
// still covers the one PlanIt request that remains, and
// TestInverseMaskLane_ClampHoldsCursorOnEarlyIngestError below covers a
// processStraggler (Postgres-origin) error stacked with the tc-6u4da clamp.

// TestInverseMaskLane_GetByUIDTimeoutDoesNotSetTimedOut proves
// processStraggler's error source is never misclassified as PlanIt-origin
// (tc-c5tmz, a CodeRabbit follow-up on tc-pmh5y): a Postgres GetByUID read
// failure -- even one that happens to satisfy net.Error/Timeout(),
// deliberately atypical for a real Postgres fake, but that is exactly the
// point -- must never be run through isTimeoutError, and must never set
// planitOrigin. tc-hku56: with the id_match hydration fan-out gone this is
// the only PlanIt-shaped error source left inside processStraggler's
// Postgres calls, so it stays classified as TerminationNatural (1h cadence),
// never misclassified as TerminationTimeout (2h cadence).
func TestInverseMaskLane_GetByUIDTimeoutDoesNotSetTimedOut(t *testing.T) {
	t.Parallel()
	windowStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	lastCleanScanAt := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	newLD := windowStart.Add(time.Hour)

	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{lightApp("first/FUL", 99, "Permitted", newLD)},
		HasMorePages: false,
	}

	apps := newFakeApps()
	// Deliberately atypical for a Postgres fake: a *url.Error satisfying
	// net.Error/Timeout()==true, to prove isTimeoutError is never even
	// consulted on the GetByUID path -- it is Postgres, never PlanIt.
	apps.getErr = &url.Error{Op: "Get", URL: "postgres://irrelevant", Err: context.DeadlineExceeded}
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: lastCleanScanAt, Cursor: &PollCursor{DifferentStart: laneCToday, NextIndex: 0}}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err == nil {
		t.Fatal("expected the GetByUID failure to surface as out.err")
	}
	if out.timedOut {
		t.Error("timedOut: got true, want false (GetByUID is Postgres, never PlanIt -- isTimeoutError must never be consulted on this path)")
	}
	if out.planitOrigin {
		t.Error("planitOrigin: got true, want false (GetByUID is Postgres, never PlanIt, tc-uitxr)")
	}
	if len(apps.upserts) != 0 {
		t.Errorf("expected Ingest never called when GetByUID fails, got %+v", apps.upserts)
	}
}

// tc-hku56 deleted TestInverseMaskLane_MidPageHydrationRateLimitCheckpointsAtFailingOffset:
// its 429 came from a hydration sub-fetch, which no longer exists — a 429 can
// now only come from the page fetch (which stops the WHOLE page, never a
// mid-page record) and is covered by
// TestInverseMaskLane_RateLimitedPageFetchPreservesCursorAdvancesLastPollTime.

// tc-hku56 deleted TestInverseMaskLane_HydrationCapStopsPassAndCheckpoints: it
// pinned the now-deleted maxHydrationsPerPass cap on the FetchByUID fan-out.
// There is no hydration burst left to bound — every genuinely-differing row's
// ingest is a plain Postgres GetByUID + Ingest — so InverseMaskOptions.MaxPages
// (a per-CYCLE page cap, enforced by NationalPollHandler.loadPlannerState) is
// the whole budget story now; see nationallane_test.go's
// TestNationalPollHandler_Handle_ExcludesCappedLaneCForRestOfCycle.

// tc-nkvil replaced TestInverseMaskLane_HydrationCapNeverRegressesCursor and
// TestInverseMaskLane_HydrationCapFlatlinesAcrossRepeatedPasses with
// TestInverseMaskLane_PhantomClusterDoesNotPinCursor and
// TestInverseMaskLane_CleanScanCompletesPastPhantomCluster (both since
// deleted, tc-hku56 below) plus TestInverseMaskLane_ClampHoldsCursorOnEarlyIngestError,
// which keeps coverage of the tc-6u4da clamp itself for the residual case it
// still fires on (an early processStraggler error within the first
// laneCResumeOverlapRecords rows of a resume).
//
// tc-hku56 deleted TestInverseMaskLane_PhantomClusterDoesNotPinCursor and
// TestInverseMaskLane_CleanScanCompletesPastPhantomCluster: both proved the
// scan could walk PAST a cluster of permanently-unhydratable cross-authority
// uid collisions instead of pinning the cursor forever. With FetchByUID gone
// from Lane C there is no such thing as an "unhydratable" row any more —
// every page row is ingestable directly — so the whole failure class the
// phantom-cluster tests guarded against cannot arise in the new model.

// TestInverseMaskLane_ClampHoldsCursorOnEarlyIngestError (renamed from
// ...OnEarlyHydrationError, tc-hku56) keeps coverage of the tc-6u4da
// monotonic-NextIndex clamp for the one case it still fires on: a
// processStraggler hard error (a Postgres GetByUID read or an Ingest
// failure — here, an Ingest failure) that breaks the loop within the first
// laneCResumeOverlapRecords rows of a resume, so startIndex+i lands below the
// loaded cursor. The clamp must hold the persisted cursor at its prior value
// rather than retreat it.
func TestInverseMaskLane_ClampHoldsCursorOnEarlyIngestError(t *testing.T) {
	t.Parallel()
	lastCleanScanAt := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	ld := time.Date(2026, 7, 1, 1, 0, 0, 0, time.UTC)
	ingestErr := errors.New("postgres: upsert failed")

	const priorNextIndex = 50
	const startIndex = priorNextIndex - laneCResumeOverlapRecords // 40

	same := "Undecided"
	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[startIndex] = planit.FetchPageResult{
		From: startIndex,
		Applications: []applications.PlanningApplication{
			lightApp("clamp-a/FUL", 99, same, ld),          // i=0: dedupes
			lightApp("clamp-b/FUL", 99, same, ld),          // i=1: dedupes
			lightApp("clamp-c/FUL", 99, same, ld),          // i=2: dedupes
			lightApp("clamp-err/FUL", 99, "Permitted", ld), // i=3 (< the 10-record overlap): Ingest hard-errors
			lightApp("clamp-never/FUL", 99, "Permitted", ld),
		},
		HasMorePages: false,
	}

	apps := newFakeApps()
	for _, uid := range []string{"clamp-a/FUL", "clamp-b/FUL", "clamp-c/FUL"} {
		apps.existing[uid] = applications.PlanningApplication{UID: uid, AreaID: 99, AppState: &same}
	}
	apps.upsertErr = ingestErr // the first non-deduping row (clamp-err/FUL) hard-errors on Ingest
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: lastCleanScanAt, Cursor: &PollCursor{DifferentStart: laneCToday, NextIndex: priorNextIndex}}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err == nil || !errors.Is(out.err, ingestErr) {
		t.Fatalf("out.err: got %v, want it to wrap the Ingest error %v", out.err, ingestErr)
	}
	got := state.states[sentinelLaneC].Cursor
	if got == nil || got.NextIndex != priorNextIndex {
		t.Errorf("cursor.NextIndex: got %+v, want held at the prior checkpoint %d (tc-6u4da: raw startIndex+i would have been %d)", got, priorNextIndex, startIndex+3)
	}
	if lastPoll := state.states[sentinelLaneC].LastPollTime; !lastPoll.Equal(laneCNow) {
		t.Errorf("LastPollTime: got %v, want %v (must advance so the planner LRU rotates)", lastPoll, laneCNow)
	}
}

// TestInverseMaskLane_IngestErrorIsAHardStop proves an Ingest failure on a
// page row checkpoints at the failing record's offset (GH#986) exactly like a
// page-fetch 429 does, and advances last_poll_time, even though the Ingest
// failure itself surfaces as out.err — the checkpoint and the error are
// independent: a retry re-fetches from this exact offset (the resume overlap
// covers any residual doubt), rather than either re-walking the whole page
// from scratch or freezing the LRU clock.
func TestInverseMaskLane_IngestErrorIsAHardStop(t *testing.T) {
	t.Parallel()
	windowStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	lastCleanScanAt := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	newLD := windowStart.Add(time.Hour)

	wantNow := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) // newLaneCHandler's pinned clock

	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{lightApp("24/0001/FUL", 99, "Permitted", newLD)},
		HasMorePages: false,
	}

	apps := newFakeApps()
	apps.upsertErr = errors.New("db write failed")
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: lastCleanScanAt, Cursor: &PollCursor{DifferentStart: laneCToday, NextIndex: 0}}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err == nil {
		t.Fatal("expected the Ingest failure to surface as out.err")
	}
	if out.timedOut {
		t.Error("timedOut: got true, want false (a plain persistence/ingest error must not be misclassified as a timeout)")
	}
	got := state.states[sentinelLaneC].Cursor
	if got == nil || got.NextIndex != 0 {
		t.Errorf("cursor: got %+v, want the checkpoint at the failing record's own offset (NextIndex=0)", got)
	}
	if lastPoll := state.states[sentinelLaneC].LastPollTime; !lastPoll.Equal(wantNow) {
		t.Errorf("LastPollTime: got %v, want %v (must advance even on an Ingest-error bail)", lastPoll, wantNow)
	}
}

// #1127 deleted TestInverseMaskLane_ContiguousEpochTilingAcrossAStall: there
// is no epoch to tile any more. TestInverseMaskLane_MultiPageScanResumesWithinADay
// covers the replacement — a scan that spans more than one page inside a
// single calendar day resumes at its checkpoint and only stamps
// last_clean_scan_at once the final page lands.
func TestInverseMaskLane_MultiPageScanResumesWithinADay(t *testing.T) {
	t.Parallel()
	fetcher := newFakeInverseMaskFetcher()
	apps := newFakeApps()
	state := newFakeStateStore()
	lastCleanScanAt := laneCNow.AddDate(0, 0, -3)
	state.states[sentinelLaneC] = PollState{HighWaterMark: lastCleanScanAt}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())

	// Page 1: more pages remain — checkpoint a same-day cursor, do NOT stamp
	// last_clean_scan_at.
	page1 := make([]applications.PlanningApplication, 300)
	for i := range page1 {
		page1[i] = lightApp(fmt.Sprintf("p1-%03d/FUL", i), 99, "Undecided", laneCNow.Add(-time.Hour))
		apps.existing[page1[i].UID] = applications.PlanningApplication{UID: page1[i].UID, AreaID: 99, AppState: page1[i].AppState}
	}
	fetcher.pages[0] = planit.FetchPageResult{From: 0, Applications: page1, HasMorePages: true}

	if out := h.RunOnePage(context.Background()); out.err != nil {
		t.Fatalf("page 1: %v", out.err)
	}
	mid := state.states[sentinelLaneC]
	if mid.Cursor == nil || mid.Cursor.NextIndex != 300 {
		t.Fatalf("page 1 cursor: got %+v, want NextIndex=300", mid.Cursor)
	}
	if !sameDate(mid.Cursor.DifferentStart, laneCNow) {
		t.Errorf("cursor.DifferentStart: got %v, want today's date", mid.Cursor.DifferentStart)
	}
	if !mid.HighWaterMark.Equal(lastCleanScanAt) {
		t.Errorf("last_clean_scan_at: got %v, want unchanged %v (scan not finished)", mid.HighWaterMark, lastCleanScanAt)
	}

	// Page 2: resumes at 300 - the 10-record Lane C overlap = 290, finishes.
	fetcher.pages[290] = planit.FetchPageResult{From: 290, Applications: nil, HasMorePages: false}
	if out := h.RunOnePage(context.Background()); out.err != nil {
		t.Fatalf("page 2: %v", out.err)
	}
	if got := fetcher.queries[len(fetcher.queries)-1].StartIndex; got != 290 {
		t.Errorf("page 2 StartIndex: got %d, want 290 (Lane C resume overlap)", got)
	}
	done := state.states[sentinelLaneC]
	if done.Cursor != nil {
		t.Errorf("cursor: got %+v, want nil (clean scan)", done.Cursor)
	}
	if !done.HighWaterMark.Equal(laneCNow) {
		t.Errorf("last_clean_scan_at: got %v, want now %v (clean scan)", done.HighWaterMark, laneCNow)
	}
}

// --- tc-hku56 / GH#1140: no-hydration page-row ingest ---

// TestInverseMaskLane_IngestsFromPageRowWithoutHydration is the core
// acceptance criterion of tc-hku56: a page where every row differs is
// ingested directly from the page's own full ingestSelectFields projection —
// no second PlanIt request. The proof is structural as well as behavioural:
// fakeInverseMaskFetcher no longer has a FetchByUID method or the
// hydrated/hydratedMulti/hydrateErr/hydrateCalls fields, so a hydration call
// cannot compile; fetcher.calls == 1 confirms only the page fetch itself ran.
func TestInverseMaskLane_IngestsFromPageRowWithoutHydration(t *testing.T) {
	t.Parallel()
	ld := laneCNow.Add(-time.Hour)

	rowA := testApp("24/0001", 99, ld)
	permitted := "Permitted"
	rowA.AppState = &permitted
	rowA.Description = "Two storey side extension"
	rowA.OtherFields = map[string]any{"comment_url": "https://example.test/comments"}

	rowB := testApp("24/0002", 99, ld)
	rejected := "Rejected"
	rowB.AppState = &rejected

	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{rowA, rowB},
		HasMorePages: false,
	}

	apps := newFakeApps() // both uids absent from Postgres: absence counts as a difference
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: laneCNow.AddDate(0, 0, -2), Cursor: &PollCursor{DifferentStart: laneCToday, NextIndex: 0}}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if out.recordsIngested != 2 {
		t.Errorf("recordsIngested: got %d, want 2 (both rows differ from Postgres)", out.recordsIngested)
	}
	if fetcher.calls != 1 {
		t.Errorf("fetcher calls: got %d, want exactly 1 (the page fetch is the ONLY PlanIt request RunOnePage can make)", fetcher.calls)
	}
	if len(apps.upserts) != 2 {
		t.Fatalf("upserts: got %d, want 2: %+v", len(apps.upserts), apps.upserts)
	}
	var got *applications.PlanningApplication
	for i := range apps.upserts {
		if apps.upserts[i].UID == rowA.UID {
			got = &apps.upserts[i]
		}
	}
	if got == nil {
		t.Fatalf("expected %q to be upserted: %+v", rowA.UID, apps.upserts)
	}
	if got.Description != "Two storey side extension" {
		t.Errorf("Description: got %q, want %q (the ingested record must carry the page row's own full fields, not a re-fetched one)", got.Description, "Two storey side extension")
	}
	if got.OtherFields == nil {
		t.Error("OtherFields: got nil, want the page row's map (tc-hku56: no separate hydration fetch to source it from)")
	}
}

// TestInverseMaskLane_FullPageAdvancesCursorByPageSize asserts a fully
// consumed page (every row deduping, none genuinely differing) checkpoints
// NextIndex = startIndex + the page size.
func TestInverseMaskLane_FullPageAdvancesCursorByPageSize(t *testing.T) {
	t.Parallel()
	ld := laneCNow.Add(-time.Hour)
	same := "Undecided"

	fetcher := newFakeInverseMaskFetcher()
	apps := newFakeApps()
	rows := make([]applications.PlanningApplication, laneCTestPageSize)
	for i := range rows {
		uid := fmt.Sprintf("full-%03d/FUL", i)
		rows[i] = lightApp(uid, 99, same, ld)
		apps.existing[uid] = applications.PlanningApplication{UID: uid, AreaID: 99, AppState: &same}
	}
	fetcher.pages[0] = planit.FetchPageResult{From: 0, Applications: rows, HasMorePages: true}

	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: laneCNow.AddDate(0, 0, -1)}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	got := state.states[sentinelLaneC].Cursor
	if got == nil || got.NextIndex != laneCTestPageSize {
		t.Errorf("cursor.NextIndex: got %+v, want %d (a fully consumed page checkpoints startIndex + the page size)", got, laneCTestPageSize)
	}
	if out.recordsIngested != 0 {
		t.Errorf("recordsIngested: got %d, want 0 (every row dedupes)", out.recordsIngested)
	}
}

// TestInverseMaskLane_LastPageStampsLastCleanScanAt extends
// TestRunOnePage_CleanScanStampsLastCleanScanAtAndClearsCursor's trivial
// empty-page case: reaching HasMorePages == false stamps last_clean_scan_at
// and clears the cursor even when the SAME page also ingests a genuine
// straggler — scan completion and record ingestion are independent outcomes
// of the same call.
func TestInverseMaskLane_LastPageStampsLastCleanScanAt(t *testing.T) {
	t.Parallel()
	ld := laneCNow.Add(-time.Hour)
	row := testApp("24/0009", 99, ld)
	permitted := "Permitted"
	row.AppState = &permitted

	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{row},
		HasMorePages: false,
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: laneCNow.AddDate(0, 0, -5)}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if out.recordsIngested != 1 {
		t.Errorf("recordsIngested: got %d, want 1", out.recordsIngested)
	}
	got := state.states[sentinelLaneC]
	if !got.HighWaterMark.Equal(laneCNow) {
		t.Errorf("last_clean_scan_at: got %v, want now %v (the last page still stamps even though it also ingested)", got.HighWaterMark, laneCNow)
	}
	if got.Cursor != nil {
		t.Errorf("cursor: got %+v, want nil (cleared on a clean scan)", got.Cursor)
	}
}

// --- tc-hku56 / ADR 0047: Lane C's own recency-gated fan-out ---

// TestInverseMaskLane_WithFanOut_WrapsBothCollaborators mirrors
// TestRecentSweepHandler_WithFanOut_WrapsBothCollaborators
// (recentsweep_test.go): WithFanOut wraps its arguments in Lane C's own
// recency decorators itself, so the handler's Ingester never holds the raw
// collaborators — the wiring site cannot pass Lane C an ungated notifier.
func TestInverseMaskLane_WithFanOut_WrapsBothCollaborators(t *testing.T) {
	t.Parallel()
	opts := defaultInverseMaskOpts()
	opts.NotifyRecencyWindow = 45 * 24 * time.Hour
	h := newLaneCHandler(t, newFakeInverseMaskFetcher(), newFakeApps(), newFakeStateStore(), opts)

	disp := &fakeDecisionDispatcher{}
	enq := &fakeEnqueuer{}
	h.WithFanOut(disp, enq)

	gd, ok := h.ingester.decision.(recencyGatedDispatcher)
	if !ok {
		t.Fatalf("ingester.decision: got %T, want recencyGatedDispatcher", h.ingester.decision)
	}
	if gd.inner != DecisionDispatcher(disp) {
		t.Error("gated dispatcher must wrap the raw dispatcher passed to WithFanOut")
	}
	if gd.window != h.opts.NotifyRecencyWindow {
		t.Errorf("gated dispatcher window: got %v, want %v", gd.window, h.opts.NotifyRecencyWindow)
	}

	ge, ok := h.ingester.enqueuer.(recencyGatedEnqueuer)
	if !ok {
		t.Fatalf("ingester.enqueuer: got %T, want recencyGatedEnqueuer", h.ingester.enqueuer)
	}
	if ge.inner != NotificationEnqueuer(enq) {
		t.Error("gated enqueuer must wrap the raw enqueuer passed to WithFanOut")
	}
}

// TestInverseMaskLane_WithFanOutGatesOldApplicationNotifications proves the
// consequence ADR 0047/tc-hku56 call out as intended: Lane C's band is
// start_date <= today-90d by construction, so gating on recency suppresses
// ALL NewApplication fan-out from Lane C (a 200-day-old start_date never
// passes), while a genuinely recent decision on an old application (a
// decided_date within the window) still reaches the dispatcher — the
// backstop role Lane C exists for.
func TestInverseMaskLane_WithFanOutGatesOldApplicationNotifications(t *testing.T) {
	t.Parallel()
	oldStart := laneCNow.Add(-200 * 24 * time.Hour)
	recentDecided := laneCNow.Add(-2 * 24 * time.Hour)

	oldNew := testApp("old-new", 99, laneCNow.Add(-time.Hour))
	oldNew.StartDate = &oldStart

	oldDecision := testApp("old-decision", 99, laneCNow.Add(-time.Hour))
	permitted := "Permitted"
	oldDecision.AppState = &permitted
	oldDecision.DecidedDate = &recentDecided

	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{oldNew, oldDecision},
		HasMorePages: false,
	}

	apps := newFakeApps()
	// oldDecision already exists with a non-decision state, so this is a
	// genuine non-decision -> decision transition (isNewDecision == true).
	undecided := "Undecided"
	apps.existing[oldDecision.UID] = applications.PlanningApplication{UID: oldDecision.UID, AreaID: 99, AppState: &undecided}

	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: laneCNow.AddDate(0, 0, -2), Cursor: &PollCursor{DifferentStart: laneCToday, NextIndex: 0}}

	opts := defaultInverseMaskOpts()
	opts.NotifyRecencyWindow = 30 * 24 * time.Hour
	h := newLaneCHandler(t, fetcher, apps, state, opts)
	disp := &fakeDecisionDispatcher{}
	enq := &fakeEnqueuer{}
	h.WithFanOut(disp, enq)

	out := h.RunOnePage(context.Background())
	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if enq.count() != 0 {
		t.Errorf("enqueuer calls: got %d, want 0 (an application with start_date 200 days old must never produce a NewApplication fan-out)", enq.count())
	}
	if disp.count() != 1 {
		t.Errorf("dispatcher calls: got %d, want 1 (a genuinely recent decision on an old application must still dispatch)", disp.count())
	}
}
