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

// fakeBackfillResponse is one pre-canned FetchBackfillPage call outcome: either
// a page result or an error, served in call order (not keyed by query) so
// tests can script a precise sequence across window slides without having to
// predict every intermediate window boundary.
type fakeBackfillResponse struct {
	result planit.FetchPageResult
	err    error
}

// backfillQuery records one FetchBackfillPage call's arguments for assertion.
type backfillQuery struct {
	windowStart time.Time
	windowEnd   time.Time
	startIndex  int
}

// fakeBackfillFetcher serves pre-canned backfill pages in call order. A call
// past the end of the scripted responses returns an empty, fully-drained page
// (mirrors PlanIt genuinely running out of matching records).
type fakeBackfillFetcher struct {
	responses []fakeBackfillResponse
	calls     int
	queries   []backfillQuery
}

func newFakeBackfillFetcher(responses ...fakeBackfillResponse) *fakeBackfillFetcher {
	return &fakeBackfillFetcher{responses: responses}
}

func (f *fakeBackfillFetcher) FetchBackfillPage(_ context.Context, windowStart, windowEnd time.Time, startIndex int) (planit.FetchPageResult, error) {
	f.queries = append(f.queries, backfillQuery{windowStart, windowEnd, startIndex})
	if f.calls >= len(f.responses) {
		f.calls++
		return planit.FetchPageResult{From: startIndex, HasMorePages: false}, nil
	}
	r := f.responses[f.calls]
	f.calls++
	return r.result, r.err
}

// fakeBackfillStateStore is a single-row in-memory stand-in for
// backfillStateAccess, mirroring the real singleton row (one Get, one Save,
// no candidate-id list).
type fakeBackfillStateStore struct {
	state   BackfillState
	saves   []BackfillState
	getErr  error
	saveErr error
}

func newFakeBackfillStateStore() *fakeBackfillStateStore {
	return &fakeBackfillStateStore{}
}

func (f *fakeBackfillStateStore) Get(context.Context) (BackfillState, error) {
	if f.getErr != nil {
		return BackfillState{}, f.getErr
	}
	return f.state, nil
}

func (f *fakeBackfillStateStore) Save(_ context.Context, s BackfillState) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.state = s
	f.saves = append(f.saves, s)
	return nil
}

// backfillClock pins the handler's clock to 2026-07-14T12:00:00Z, matching
// the rest of the polling package's test fixtures (nationallane_test.go).
func backfillClock() time.Time { return time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) }

func newBackfillHandler(t *testing.T, fetcher *fakeBackfillFetcher, apps *fakeApps, state *fakeBackfillStateStore, opts BackfillOptions) *BackfillHandler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(discard{}, nil))
	return NewBackfillHandler(fetcher, state, apps, opts, backfillClock, logger)
}

func defaultBackfillOpts() BackfillOptions {
	return BackfillOptions{WindowWidthDays: 90, MaxPagesPerCycle: 2, EmptyWindowsBeforeComplete: 12}
}

// TestNewBackfillHandler_NoFanOutCollaborators is the structural safety
// guarantee (GH#967): the ingester BackfillHandler builds is ALWAYS
// constructed with nil decision/enqueuer collaborators, and BackfillHandler
// has no method that could ever attach one (there is no WithFanOut in this
// file — verified by the absence of that method in the diff, not by a test,
// since Go cannot assert the non-existence of a method).
func TestNewBackfillHandler_NoFanOutCollaborators(t *testing.T) {
	t.Parallel()
	h := newBackfillHandler(t, newFakeBackfillFetcher(), newFakeApps(), newFakeBackfillStateStore(), defaultBackfillOpts())

	if h.ingester.decision != nil {
		t.Error("ingester.decision: got non-nil, want nil (backfill must never dispatch decision events)")
	}
	if h.ingester.enqueuer != nil {
		t.Error("ingester.enqueuer: got non-nil, want nil (backfill must never fan out watch-zone notifications)")
	}
}

// TestBackfillHandler_Run_InsertsGapWithoutFanOut proves the gap-fill path: a
// first-seen application PlanIt returns gets inserted even though it is
// already in a decision state (app_state=Permitted — under the ordinary
// Ingester this would trigger both a decision dispatch AND a watch-zone
// fan-out). Because BackfillHandler's ingester carries nil collaborators,
// neither is reachable; the only observable effect is the upsert.
func TestBackfillHandler_Run_InsertsGapWithoutFanOut(t *testing.T) {
	t.Parallel()
	windowEnd := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	app := testApp("gap", 300, time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC))
	permitted := "Permitted"
	app.AppState = &permitted

	fetcher := newFakeBackfillFetcher(fakeBackfillResponse{
		result: planit.FetchPageResult{Applications: []applications.PlanningApplication{app}, HasMorePages: false},
	})
	apps := newFakeApps()
	state := newFakeBackfillStateStore()
	state.state = BackfillState{WindowEnd: windowEnd}

	h := newBackfillHandler(t, fetcher, apps, state, BackfillOptions{WindowWidthDays: 90, MaxPagesPerCycle: 1, EmptyWindowsBeforeComplete: 12})
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	if out.recordsIngested != 1 {
		t.Errorf("recordsIngested: got %d, want 1", out.recordsIngested)
	}
	if len(apps.upserts) != 1 || apps.upserts[0].UID != "gap/FUL" {
		t.Fatalf("upserts: got %+v, want exactly the gap-fill record", apps.upserts)
	}
	if apps.upserts[0].AppState == nil || *apps.upserts[0].AppState != "Permitted" {
		t.Errorf("AppState: got %v, want Permitted", apps.upserts[0].AppState)
	}
}

// TestBackfillHandler_Run_EnrichesNullFieldsWithoutFanOut proves the
// enrichment path (GH#935 staleness): an existing application whose silent
// fields (Reference, Altid, ...) are NULL gets them populated by a re-ingest
// through the backfill path, with no business-field change (so no fan-out
// would fire even with a wired Ingester).
func TestBackfillHandler_Run_EnrichesNullFieldsWithoutFanOut(t *testing.T) {
	t.Parallel()
	windowEnd := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	t0 := time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2019, 3, 2, 0, 0, 0, 0, time.UTC)

	existing := testApp("enrich", 300, t0) // Reference/Altid/etc left nil, mirroring a pre-GH#935 row
	apps := newFakeApps()
	apps.existing["enrich/FUL"] = existing

	fetched := testApp("enrich", 300, t1) // identical business fields, LastDifferent bumped
	ref := "24/00099/FUL"
	fetched.Reference = &ref
	fetched.Altid = []byte(`"ALT1"`)

	fetcher := newFakeBackfillFetcher(fakeBackfillResponse{
		result: planit.FetchPageResult{Applications: []applications.PlanningApplication{fetched}, HasMorePages: false},
	})
	state := newFakeBackfillStateStore()
	state.state = BackfillState{WindowEnd: windowEnd}

	h := newBackfillHandler(t, fetcher, apps, state, BackfillOptions{WindowWidthDays: 90, MaxPagesPerCycle: 1, EmptyWindowsBeforeComplete: 12})
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	if len(apps.upserts) != 1 {
		t.Fatalf("upserts: got %d, want 1 (silent-field change must still upsert)", len(apps.upserts))
	}
	if apps.upserts[0].Reference == nil || *apps.upserts[0].Reference != ref {
		t.Errorf("Reference: got %v, want %q", apps.upserts[0].Reference, ref)
	}
}

// TestBackfillHandler_Run_BookkeepingOnlyChangeSkipsWrite proves the
// reindex-flood guard still applies through the backfill path: a record that
// differs only in LastDifferent (identical business AND silent fields) causes
// no upsert at all.
func TestBackfillHandler_Run_BookkeepingOnlyChangeSkipsWrite(t *testing.T) {
	t.Parallel()
	windowEnd := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	t0 := time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2019, 3, 2, 0, 0, 0, 0, time.UTC)

	existing := testApp("noop", 300, t0)
	apps := newFakeApps()
	apps.existing["noop/FUL"] = existing

	fetched := testApp("noop", 300, t1) // only LastDifferent differs

	fetcher := newFakeBackfillFetcher(fakeBackfillResponse{
		result: planit.FetchPageResult{Applications: []applications.PlanningApplication{fetched}, HasMorePages: false},
	})
	state := newFakeBackfillStateStore()
	state.state = BackfillState{WindowEnd: windowEnd}

	h := newBackfillHandler(t, fetcher, apps, state, BackfillOptions{WindowWidthDays: 90, MaxPagesPerCycle: 1, EmptyWindowsBeforeComplete: 12})
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	if len(apps.upserts) != 0 {
		t.Errorf("upserts: got %d, want 0 (bookkeeping-only touch must not write)", len(apps.upserts))
	}
}

// TestBackfillHandler_Run_FirstRunFixesWindowEndToNow proves the first-ever
// run's window fixing: WindowEnd starts zero (never run) and is set to the
// injected clock's date on the first Run, with the fetched window bounded
// [now-WindowWidthDays, now].
func TestBackfillHandler_Run_FirstRunFixesWindowEndToNow(t *testing.T) {
	t.Parallel()
	fetcher := newFakeBackfillFetcher(fakeBackfillResponse{
		result: planit.FetchPageResult{
			Applications: []applications.PlanningApplication{testApp("a", 300, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))},
			HasMorePages: true, // mid-window: no slide, isolates the window-fixing behaviour
		},
	})
	apps := newFakeApps()
	state := newFakeBackfillStateStore() // zero value: never run

	h := newBackfillHandler(t, fetcher, apps, state, BackfillOptions{WindowWidthDays: 90, MaxPagesPerCycle: 1, EmptyWindowsBeforeComplete: 12})
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	if len(fetcher.queries) != 1 {
		t.Fatalf("expected exactly one fetch, got %d", len(fetcher.queries))
	}
	wantEnd := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	wantStart := wantEnd.AddDate(0, 0, -90)
	if !fetcher.queries[0].windowEnd.Equal(wantEnd) {
		t.Errorf("windowEnd: got %v, want %v (today's date)", fetcher.queries[0].windowEnd, wantEnd)
	}
	if !fetcher.queries[0].windowStart.Equal(wantStart) {
		t.Errorf("windowStart: got %v, want %v", fetcher.queries[0].windowStart, wantStart)
	}
	if !state.state.WindowEnd.Equal(wantEnd) {
		t.Errorf("persisted WindowEnd: got %v, want %v", state.state.WindowEnd, wantEnd)
	}
	if state.state.CursorNextIndex != 1 {
		t.Errorf("persisted CursorNextIndex: got %d, want 1", state.state.CursorNextIndex)
	}
}

// TestBackfillHandler_Run_SlidesWindowOnFullDrain proves window sliding: a
// fetch that fully drains a window (HasMorePages=false) with non-zero records
// seen resets ConsecutiveEmptyWindows to 0 and moves WindowEnd back by
// WindowWidthDays, resetting the cursor and per-window counter for the new
// window.
func TestBackfillHandler_Run_SlidesWindowOnFullDrain(t *testing.T) {
	t.Parallel()
	windowEnd := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	fetcher := newFakeBackfillFetcher(fakeBackfillResponse{
		result: planit.FetchPageResult{
			Applications: []applications.PlanningApplication{
				testApp("a", 300, time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)),
				testApp("b", 300, time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC)),
			},
			HasMorePages: false,
		},
	})
	apps := newFakeApps()
	state := newFakeBackfillStateStore()
	state.state = BackfillState{WindowEnd: windowEnd, ConsecutiveEmptyWindows: 3}

	h := newBackfillHandler(t, fetcher, apps, state, BackfillOptions{WindowWidthDays: 90, MaxPagesPerCycle: 1, EmptyWindowsBeforeComplete: 12})
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	wantSlid := windowEnd.AddDate(0, 0, -90)
	if !state.state.WindowEnd.Equal(wantSlid) {
		t.Errorf("WindowEnd: got %v, want %v", state.state.WindowEnd, wantSlid)
	}
	if state.state.CursorNextIndex != 0 {
		t.Errorf("CursorNextIndex: got %d, want 0 (reset for new window)", state.state.CursorNextIndex)
	}
	if state.state.WindowRecordsSeen != 0 {
		t.Errorf("WindowRecordsSeen: got %d, want 0 (reset for new window)", state.state.WindowRecordsSeen)
	}
	if state.state.ConsecutiveEmptyWindows != 0 {
		t.Errorf("ConsecutiveEmptyWindows: got %d, want 0 (this window was not empty)", state.state.ConsecutiveEmptyWindows)
	}
	if state.state.Complete {
		t.Error("Complete: got true, want false")
	}
}

// TestBackfillHandler_Run_IncrementsEmptyWindowCounter proves a window that
// drains with zero records increments ConsecutiveEmptyWindows and, while
// still below the threshold, keeps the lane going (window slides back).
func TestBackfillHandler_Run_IncrementsEmptyWindowCounter(t *testing.T) {
	t.Parallel()
	windowEnd := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	fetcher := newFakeBackfillFetcher(fakeBackfillResponse{
		result: planit.FetchPageResult{Applications: nil, HasMorePages: false},
	})
	apps := newFakeApps()
	state := newFakeBackfillStateStore()
	state.state = BackfillState{WindowEnd: windowEnd, ConsecutiveEmptyWindows: 2}

	h := newBackfillHandler(t, fetcher, apps, state, BackfillOptions{WindowWidthDays: 90, MaxPagesPerCycle: 1, EmptyWindowsBeforeComplete: 12})
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	if state.state.ConsecutiveEmptyWindows != 3 {
		t.Errorf("ConsecutiveEmptyWindows: got %d, want 3", state.state.ConsecutiveEmptyWindows)
	}
	if state.state.Complete {
		t.Error("Complete: got true, want false (below threshold)")
	}
	wantSlid := windowEnd.AddDate(0, 0, -90)
	if !state.state.WindowEnd.Equal(wantSlid) {
		t.Errorf("WindowEnd: got %v, want %v (still creeping backward)", state.state.WindowEnd, wantSlid)
	}
}

// TestBackfillHandler_Run_MarksCompleteAtThreshold proves the terminal
// transition: the EmptyWindowsBeforeComplete-th consecutive empty window sets
// Complete=true.
func TestBackfillHandler_Run_MarksCompleteAtThreshold(t *testing.T) {
	t.Parallel()
	windowEnd := time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)
	fetcher := newFakeBackfillFetcher(fakeBackfillResponse{
		result: planit.FetchPageResult{Applications: nil, HasMorePages: false},
	})
	apps := newFakeApps()
	state := newFakeBackfillStateStore()
	state.state = BackfillState{WindowEnd: windowEnd, ConsecutiveEmptyWindows: 11}

	h := newBackfillHandler(t, fetcher, apps, state, BackfillOptions{WindowWidthDays: 90, MaxPagesPerCycle: 1, EmptyWindowsBeforeComplete: 12})
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	if !out.complete {
		t.Error("outcome complete: got false, want true")
	}
	if state.state.ConsecutiveEmptyWindows != 12 {
		t.Errorf("ConsecutiveEmptyWindows: got %d, want 12", state.state.ConsecutiveEmptyWindows)
	}
	if !state.state.Complete {
		t.Error("persisted Complete: got false, want true")
	}
}

// TestBackfillHandler_Run_NoOpWhenComplete proves completion is terminal:
// once Complete is true, Run makes no fetch calls at all, forever.
func TestBackfillHandler_Run_NoOpWhenComplete(t *testing.T) {
	t.Parallel()
	fetcher := newFakeBackfillFetcher()
	apps := newFakeApps()
	state := newFakeBackfillStateStore()
	state.state = BackfillState{Complete: true}

	h := newBackfillHandler(t, fetcher, apps, state, defaultBackfillOpts())
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	if !out.complete {
		t.Error("outcome complete: got false, want true")
	}
	if fetcher.calls != 0 {
		t.Errorf("fetcher.calls: got %d, want 0 (a complete lane must never fetch)", fetcher.calls)
	}
	if len(state.saves) != 0 {
		t.Errorf("saves: got %d, want 0 (nothing changed)", len(state.saves))
	}
}

// TestBackfillHandler_Run_RespectsMaxPagesPerCycle proves the page cap:
// exactly MaxPagesPerCycle pages are fetched per Run, with CursorNextIndex
// correctly reflecting partial progress mid-window for the next cycle to
// resume from.
func TestBackfillHandler_Run_RespectsMaxPagesPerCycle(t *testing.T) {
	t.Parallel()
	windowEnd := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	ld := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	fetcher := newFakeBackfillFetcher(
		fakeBackfillResponse{result: planit.FetchPageResult{
			Applications: []applications.PlanningApplication{testApp("p0a", 300, ld), testApp("p0b", 300, ld), testApp("p0c", 300, ld)},
			HasMorePages: true,
		}},
		fakeBackfillResponse{result: planit.FetchPageResult{
			Applications: []applications.PlanningApplication{testApp("p1a", 300, ld), testApp("p1b", 300, ld)},
			HasMorePages: true,
		}},
	)
	apps := newFakeApps()
	state := newFakeBackfillStateStore()
	state.state = BackfillState{WindowEnd: windowEnd}

	h := newBackfillHandler(t, fetcher, apps, state, BackfillOptions{WindowWidthDays: 90, MaxPagesPerCycle: 2, EmptyWindowsBeforeComplete: 12})
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	if out.pages != 2 {
		t.Errorf("pages: got %d, want 2 (capped)", out.pages)
	}
	if fetcher.calls != 2 {
		t.Errorf("fetcher.calls: got %d, want 2", fetcher.calls)
	}
	if state.state.CursorNextIndex != 5 {
		t.Errorf("CursorNextIndex: got %d, want 5 (3+2, ready for next cycle to resume)", state.state.CursorNextIndex)
	}
	if state.state.WindowEnd != windowEnd {
		t.Errorf("WindowEnd: got %v, want unchanged %v (mid-window, no slide)", state.state.WindowEnd, windowEnd)
	}
}

// TestBackfillHandler_Run_FetchErrorDoesNotAdvanceCursor proves the error
// handling contract: a fetch error mid-cycle stops further pages without
// advancing CursorNextIndex past the failed page, while whatever earlier
// pages in the same Run call succeeded stays persisted.
func TestBackfillHandler_Run_FetchErrorDoesNotAdvanceCursor(t *testing.T) {
	t.Parallel()
	windowEnd := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	ld := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	fetcher := newFakeBackfillFetcher(
		fakeBackfillResponse{result: planit.FetchPageResult{
			Applications: []applications.PlanningApplication{testApp("p0a", 300, ld), testApp("p0b", 300, ld)},
			HasMorePages: true,
		}},
		fakeBackfillResponse{err: errors.New("planit: transport blew up")},
	)
	apps := newFakeApps()
	state := newFakeBackfillStateStore()
	state.state = BackfillState{WindowEnd: windowEnd}

	h := newBackfillHandler(t, fetcher, apps, state, BackfillOptions{WindowWidthDays: 90, MaxPagesPerCycle: 3, EmptyWindowsBeforeComplete: 12})
	out := h.Run(context.Background())

	if out.err == nil {
		t.Fatal("expected the second page's fetch error to surface on the outcome")
	}
	if fetcher.calls != 2 {
		t.Errorf("fetcher.calls: got %d, want 2 (loop stops after the failed page)", fetcher.calls)
	}
	if state.state.CursorNextIndex != 2 {
		t.Errorf("CursorNextIndex: got %d, want 2 (only page 0's advance persisted)", state.state.CursorNextIndex)
	}
	if len(state.saves) != 1 {
		t.Errorf("saves: got %d, want 1 (exactly the successful page)", len(state.saves))
	}
	if len(apps.upserts) != 2 {
		t.Errorf("upserts: got %d, want 2 (page 0's records were still ingested)", len(apps.upserts))
	}
}

// TestBackfillHandler_Run_RateLimitedDoesNotAdvanceCursor mirrors the fetch-
// error test for a 429: the cursor freezes at the last successful page and
// the outcome carries the parsed Retry-After hint.
func TestBackfillHandler_Run_RateLimitedDoesNotAdvanceCursor(t *testing.T) {
	t.Parallel()
	windowEnd := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	ld := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	retryAfter := 30 * time.Second
	fetcher := newFakeBackfillFetcher(
		fakeBackfillResponse{result: planit.FetchPageResult{
			Applications: []applications.PlanningApplication{testApp("p0a", 300, ld)},
			HasMorePages: true,
		}},
		fakeBackfillResponse{err: &planit.RateLimitError{RetryAfter: &retryAfter}},
	)
	apps := newFakeApps()
	state := newFakeBackfillStateStore()
	state.state = BackfillState{WindowEnd: windowEnd}

	h := newBackfillHandler(t, fetcher, apps, state, BackfillOptions{WindowWidthDays: 90, MaxPagesPerCycle: 3, EmptyWindowsBeforeComplete: 12})
	out := h.Run(context.Background())

	if !out.rateLimited {
		t.Fatal("expected rateLimited=true")
	}
	if out.retryAfter == nil || *out.retryAfter != retryAfter {
		t.Errorf("retryAfter: got %v, want %v", out.retryAfter, retryAfter)
	}
	if state.state.CursorNextIndex != 1 {
		t.Errorf("CursorNextIndex: got %d, want 1 (only page 0's advance persisted)", state.state.CursorNextIndex)
	}
}
