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

// fakeRecentSweepResponse is one pre-canned FetchRecentSweepPage outcome,
// served in call order (not keyed by query) so tests can script a precise
// sequence across window slides, mirroring fakeBackfillFetcher.
type fakeRecentSweepResponse struct {
	result planit.FetchPageResult
	err    error
}

// recentSweepQuery records one FetchRecentSweepPage call's arguments.
type recentSweepQuery struct {
	windowStart time.Time
	windowEnd   time.Time
	startIndex  int
}

// fakeRecentSweepFetcher serves pre-canned recent-sweep pages in call order and
// hydration responses keyed by uid. A call past the end of the scripted
// responses returns an empty, fully-drained page.
type fakeRecentSweepFetcher struct {
	responses     []fakeRecentSweepResponse
	calls         int
	queries       []recentSweepQuery
	hydrated      map[string]applications.PlanningApplication
	hydratedMulti map[string][]applications.PlanningApplication
	hydrateErr    map[string]error
	hydrateCalls  []string
}

func newFakeRecentSweepFetcher(responses ...fakeRecentSweepResponse) *fakeRecentSweepFetcher {
	return &fakeRecentSweepFetcher{
		responses:     responses,
		hydrated:      map[string]applications.PlanningApplication{},
		hydratedMulti: map[string][]applications.PlanningApplication{},
		hydrateErr:    map[string]error{},
	}
}

func (f *fakeRecentSweepFetcher) FetchRecentSweepPage(_ context.Context, windowStart, windowEnd time.Time, startIndex int) (planit.FetchPageResult, error) {
	f.queries = append(f.queries, recentSweepQuery{windowStart, windowEnd, startIndex})
	if f.calls >= len(f.responses) {
		f.calls++
		return planit.FetchPageResult{From: startIndex, HasMorePages: false}, nil
	}
	r := f.responses[f.calls]
	f.calls++
	return r.result, r.err
}

func (f *fakeRecentSweepFetcher) FetchByUID(_ context.Context, uid string) (planit.FetchPageResult, error) {
	f.hydrateCalls = append(f.hydrateCalls, uid)
	if err, ok := f.hydrateErr[uid]; ok {
		return planit.FetchPageResult{}, err
	}
	if apps, ok := f.hydratedMulti[uid]; ok {
		return planit.FetchPageResult{Applications: append([]applications.PlanningApplication(nil), apps...)}, nil
	}
	app, ok := f.hydrated[uid]
	if !ok {
		return planit.FetchPageResult{Applications: nil}, nil
	}
	return planit.FetchPageResult{Applications: []applications.PlanningApplication{app}}, nil
}

// fakeRecentSweepStateStore is a single-row in-memory stand-in for
// recentSweepStateAccess, mirroring fakeBackfillStateStore.
type fakeRecentSweepStateStore struct {
	state   RecentSweepState
	saves   []RecentSweepState
	getErr  error
	saveErr error
}

func newFakeRecentSweepStateStore() *fakeRecentSweepStateStore {
	return &fakeRecentSweepStateStore{}
}

func (f *fakeRecentSweepStateStore) Get(context.Context) (RecentSweepState, error) {
	if f.getErr != nil {
		return RecentSweepState{}, f.getErr
	}
	return f.state, nil
}

func (f *fakeRecentSweepStateStore) Save(_ context.Context, s RecentSweepState) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.state = s
	f.saves = append(f.saves, s)
	return nil
}

// recentSweepNow pins the handler's clock to 2026-09-07T12:00:00Z (ADR 0047's
// measurement date); recentSweepToday is its calendar date at UTC midnight.
var (
	recentSweepNow   = time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	recentSweepToday = time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
)

func recentSweepClock() time.Time { return recentSweepNow }

func newRecentSweepHandler(t *testing.T, fetcher *fakeRecentSweepFetcher, apps *fakeApps, state *fakeRecentSweepStateStore, opts RecentSweepOptions) *RecentSweepHandler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(discard{}, nil))
	return NewRecentSweepHandler(fetcher, state, apps, opts, recentSweepClock, logger)
}

func recentSweepOpts(maxPages int) RecentSweepOptions {
	return RecentSweepOptions{
		DepthDays:           90,
		WindowWidthDays:     15,
		MaxPagesPerCycle:    maxPages,
		NotifyRecencyWindow: 30 * 24 * time.Hour,
	}
}

// recentLightRow builds a Lane E light-projection row: uid, area_id, app_state
// — the divergence-relevant fields recentSweepSelectFields requests. No
// last_different (deliberately absent from Lane E's projection).
func recentLightRow(uid string, areaID int, appState string) applications.PlanningApplication {
	s := appState
	return applications.PlanningApplication{UID: uid, AreaID: areaID, AppState: &s}
}

// --- construction ---

// TestNewRecentSweepHandler_IngestionOnlyUntilWithFanOut proves the handler
// starts with nil fan-out collaborators (ingestion-only), exactly like every
// other lane before WithFanOut is called.
func TestNewRecentSweepHandler_IngestionOnlyUntilWithFanOut(t *testing.T) {
	t.Parallel()
	h := newRecentSweepHandler(t, newFakeRecentSweepFetcher(), newFakeApps(), newFakeRecentSweepStateStore(), recentSweepOpts(6))
	if h.ingester.decision != nil || h.ingester.enqueuer != nil {
		t.Error("a freshly constructed RecentSweepHandler must have no fan-out collaborators")
	}
}

// TestNewRecentSweepHandler_ClampsWindowWidthToMax pins ADR 0047's hard cap: a
// misconfigured wide window is clamped to maxRecentSweepWindowWidthDays so
// nobody can set 90 and walk back toward PlanIt's 45s first-page cliff.
func TestNewRecentSweepHandler_ClampsWindowWidthToMax(t *testing.T) {
	t.Parallel()
	h := newRecentSweepHandler(t, newFakeRecentSweepFetcher(), newFakeApps(), newFakeRecentSweepStateStore(),
		RecentSweepOptions{DepthDays: 90, WindowWidthDays: 90, MaxPagesPerCycle: 6, NotifyRecencyWindow: 30 * 24 * time.Hour})
	if h.opts.WindowWidthDays != maxRecentSweepWindowWidthDays {
		t.Errorf("WindowWidthDays: got %d, want clamped to %d", h.opts.WindowWidthDays, maxRecentSweepWindowWidthDays)
	}
}

// TestRecentSweepHandler_WithFanOut_WrapsBothCollaborators is acceptance
// criterion: WithFanOut wraps its arguments in the recency decorators itself,
// so the handler's Ingester never holds the raw collaborators — the wiring
// site cannot pass Lane E an ungated notifier.
func TestRecentSweepHandler_WithFanOut_WrapsBothCollaborators(t *testing.T) {
	t.Parallel()
	h := newRecentSweepHandler(t, newFakeRecentSweepFetcher(), newFakeApps(), newFakeRecentSweepStateStore(), recentSweepOpts(6))

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

// --- Run: lap / window mechanics ---

// TestRecentSweepHandler_Run_FirstRunAnchorsToday proves the first-ever run
// cuts LapAnchor and WindowEnd to today and fetches window [today-width, today]
// at index 0.
func TestRecentSweepHandler_Run_FirstRunAnchorsToday(t *testing.T) {
	t.Parallel()
	row := recentLightRow("a/FUL", 300, "Undecided")
	fetcher := newFakeRecentSweepFetcher(fakeRecentSweepResponse{
		result: planit.FetchPageResult{Applications: []applications.PlanningApplication{row}, HasMorePages: true},
	})
	apps := newFakeApps()
	apps.existing["a/FUL"] = row // found + non-diverging: no hydration noise
	state := newFakeRecentSweepStateStore()

	h := newRecentSweepHandler(t, fetcher, apps, state, recentSweepOpts(1))
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	if len(fetcher.queries) != 1 {
		t.Fatalf("expected exactly one fetch, got %d", len(fetcher.queries))
	}
	wantEnd := recentSweepToday
	wantStart := wantEnd.AddDate(0, 0, -15)
	if !fetcher.queries[0].windowEnd.Equal(wantEnd) {
		t.Errorf("windowEnd: got %v, want %v", fetcher.queries[0].windowEnd, wantEnd)
	}
	if !fetcher.queries[0].windowStart.Equal(wantStart) {
		t.Errorf("windowStart: got %v, want %v", fetcher.queries[0].windowStart, wantStart)
	}
	if fetcher.queries[0].startIndex != 0 {
		t.Errorf("startIndex: got %d, want 0", fetcher.queries[0].startIndex)
	}
	if !state.state.LapAnchor.Equal(wantEnd) {
		t.Errorf("persisted LapAnchor: got %v, want %v", state.state.LapAnchor, wantEnd)
	}
	if !state.state.WindowEnd.Equal(wantEnd) {
		t.Errorf("persisted WindowEnd: got %v, want %v", state.state.WindowEnd, wantEnd)
	}
	if state.state.CursorNextIndex != 1 {
		t.Errorf("persisted CursorNextIndex: got %d, want 1", state.state.CursorNextIndex)
	}
}

// TestRecentSweepHandler_Run_SlidesWindowOnFullDrain proves a drained window
// (HasMorePages false) moves WindowEnd back by exactly WindowWidthDays and
// resets CursorNextIndex to 0, with no lap reset while the cursor is still
// above the floor.
func TestRecentSweepHandler_Run_SlidesWindowOnFullDrain(t *testing.T) {
	t.Parallel()
	fetcher := newFakeRecentSweepFetcher(fakeRecentSweepResponse{
		result: planit.FetchPageResult{Applications: nil, HasMorePages: false},
	})
	apps := newFakeApps()
	state := newFakeRecentSweepStateStore()
	state.state = RecentSweepState{LapAnchor: recentSweepToday, WindowEnd: recentSweepToday, CursorNextIndex: 40}

	h := newRecentSweepHandler(t, fetcher, apps, state, recentSweepOpts(1))
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	wantSlid := recentSweepToday.AddDate(0, 0, -15)
	if !state.state.WindowEnd.Equal(wantSlid) {
		t.Errorf("WindowEnd: got %v, want %v (slid back by exactly 15 days)", state.state.WindowEnd, wantSlid)
	}
	if state.state.CursorNextIndex != 0 {
		t.Errorf("CursorNextIndex: got %d, want 0 (reset for the new window)", state.state.CursorNextIndex)
	}
	if state.state.LapsCompleted != 0 {
		t.Errorf("LapsCompleted: got %d, want 0 (cursor still well above the floor)", state.state.LapsCompleted)
	}
}

// TestRecentSweepHandler_Run_ResetsLapWhenCursorCrossesFloor proves the lap
// reset: when the slid WindowEnd reaches LapAnchor-DepthDays, LapsCompleted
// increments, LastLapCompletedAt is stamped, LapAnchor is re-cut to today (a
// fresh anchor, days later than the old one), and WindowEnd equals the new
// anchor.
func TestRecentSweepHandler_Run_ResetsLapWhenCursorCrossesFloor(t *testing.T) {
	t.Parallel()
	oldAnchor := recentSweepToday.AddDate(0, 0, -3) // a lap started 3 days ago
	// windowStart after this drain = oldAnchor-75d-15d = oldAnchor-90d = the floor.
	fetcher := newFakeRecentSweepFetcher(fakeRecentSweepResponse{
		result: planit.FetchPageResult{Applications: nil, HasMorePages: false},
	})
	apps := newFakeApps()
	state := newFakeRecentSweepStateStore()
	state.state = RecentSweepState{
		LapAnchor:     oldAnchor,
		WindowEnd:     oldAnchor.AddDate(0, 0, -75),
		LapsCompleted: 2,
	}

	h := newRecentSweepHandler(t, fetcher, apps, state, recentSweepOpts(1))
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	if state.state.LapsCompleted != 3 {
		t.Errorf("LapsCompleted: got %d, want 3", state.state.LapsCompleted)
	}
	if !state.state.LastLapCompletedAt.Equal(recentSweepNow) {
		t.Errorf("LastLapCompletedAt: got %v, want %v", state.state.LastLapCompletedAt, recentSweepNow)
	}
	if !state.state.LapAnchor.Equal(recentSweepToday) {
		t.Errorf("LapAnchor: got %v, want %v (re-cut to today)", state.state.LapAnchor, recentSweepToday)
	}
	if !state.state.WindowEnd.Equal(recentSweepToday) {
		t.Errorf("WindowEnd: got %v, want %v (== the new anchor)", state.state.WindowEnd, recentSweepToday)
	}
	if state.state.CursorNextIndex != 0 {
		t.Errorf("CursorNextIndex: got %d, want 0", state.state.CursorNextIndex)
	}
}

// TestRecentSweepHandler_Run_HasNoCompleteConcept proves Lane E never
// finishes: it always keeps re-anchoring and sweeping, lap after lap, with no
// terminal state.
func TestRecentSweepHandler_Run_HasNoCompleteConcept(t *testing.T) {
	t.Parallel()
	// A lap that is already at its floor: draining it must roll straight into
	// a fresh lap and immediately fetch the new window, not stop.
	oldAnchor := recentSweepToday.AddDate(0, 0, -3)
	fetcher := newFakeRecentSweepFetcher(
		fakeRecentSweepResponse{result: planit.FetchPageResult{HasMorePages: false}},
		fakeRecentSweepResponse{result: planit.FetchPageResult{
			Applications: []applications.PlanningApplication{recentLightRow("fresh/FUL", 300, "Undecided")},
			HasMorePages: true,
		}},
	)
	apps := newFakeApps()
	apps.existing["fresh/FUL"] = recentLightRow("fresh/FUL", 300, "Undecided")
	state := newFakeRecentSweepStateStore()
	state.state = RecentSweepState{LapAnchor: oldAnchor, WindowEnd: oldAnchor.AddDate(0, 0, -75)}

	h := newRecentSweepHandler(t, fetcher, apps, state, recentSweepOpts(4))
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	if state.state.LapsCompleted != 1 {
		t.Fatalf("LapsCompleted: got %d, want 1", state.state.LapsCompleted)
	}
	// The second fetch is the fresh lap's first window.
	if len(fetcher.queries) < 2 {
		t.Fatalf("expected the fresh lap to fetch immediately, got %d fetches", len(fetcher.queries))
	}
	if !fetcher.queries[1].windowEnd.Equal(recentSweepToday) {
		t.Errorf("fresh lap window end: got %v, want %v", fetcher.queries[1].windowEnd, recentSweepToday)
	}
}

// --- Run: checkpointing and error handling ---

// TestRecentSweepHandler_Run_PersistsAfterEveryPageAndHoldsOnFailure proves
// state is saved after every successful page, and a page whose fetch fails
// leaves the previous page's checkpoint standing (no save for the failed
// page), with prior pages in the same turn still persisted.
func TestRecentSweepHandler_Run_PersistsAfterEveryPageAndHoldsOnFailure(t *testing.T) {
	t.Parallel()
	rows := []applications.PlanningApplication{
		recentLightRow("p0a/FUL", 300, "Undecided"),
		recentLightRow("p0b/FUL", 300, "Undecided"),
	}
	apps := newFakeApps()
	for _, r := range rows {
		apps.existing[r.UID] = r
	}
	apps.existing["p1a/FUL"] = recentLightRow("p1a/FUL", 300, "Undecided")
	apps.existing["p1b/FUL"] = recentLightRow("p1b/FUL", 300, "Undecided")

	fetcher := newFakeRecentSweepFetcher(
		fakeRecentSweepResponse{result: planit.FetchPageResult{Applications: rows, HasMorePages: true}},
		fakeRecentSweepResponse{result: planit.FetchPageResult{Applications: []applications.PlanningApplication{
			recentLightRow("p1a/FUL", 300, "Undecided"), recentLightRow("p1b/FUL", 300, "Undecided"),
		}, HasMorePages: true}},
		fakeRecentSweepResponse{err: errors.New("planit: transport blew up")},
	)
	state := newFakeRecentSweepStateStore()
	state.state = RecentSweepState{LapAnchor: recentSweepToday, WindowEnd: recentSweepToday}

	h := newRecentSweepHandler(t, fetcher, apps, state, recentSweepOpts(6))
	out := h.Run(context.Background())

	if out.err == nil {
		t.Fatal("expected the third page's fetch error to surface")
	}
	if !out.planitOrigin {
		t.Error("planitOrigin: got false, want true (error straight from FetchRecentSweepPage)")
	}
	if fetcher.calls != 3 {
		t.Errorf("fetcher.calls: got %d, want 3 (loop stops after the failed page)", fetcher.calls)
	}
	if len(state.saves) != 2 {
		t.Errorf("saves: got %d, want 2 (one per successful page)", len(state.saves))
	}
	if state.state.CursorNextIndex != 4 {
		t.Errorf("CursorNextIndex: got %d, want 4 (only the two good pages persisted)", state.state.CursorNextIndex)
	}
}

// TestRecentSweepHandler_Run_IngestErrorHoldsPreviousCheckpoint proves an
// ingest (upsert) error mid-page stops the turn without persisting that page,
// and is NOT classified as planitOrigin (it is a Postgres failure).
func TestRecentSweepHandler_Run_IngestErrorHoldsPreviousCheckpoint(t *testing.T) {
	t.Parallel()
	apps := newFakeApps()
	apps.upsertErr = errors.New("postgres: connection refused")

	fetcher := newFakeRecentSweepFetcher(fakeRecentSweepResponse{
		result: planit.FetchPageResult{
			Applications: []applications.PlanningApplication{recentLightRow("gap/FUL", 300, "Undecided")},
			HasMorePages: true,
		},
	})
	fetcher.hydrated["gap/FUL"] = applications.PlanningApplication{UID: "gap/FUL", AreaID: 300}
	state := newFakeRecentSweepStateStore()
	state.state = RecentSweepState{LapAnchor: recentSweepToday, WindowEnd: recentSweepToday}

	h := newRecentSweepHandler(t, fetcher, apps, state, recentSweepOpts(6))
	out := h.Run(context.Background())

	if out.err == nil {
		t.Fatal("expected the ingest error to surface")
	}
	if out.planitOrigin {
		t.Error("planitOrigin: got true, want false (the failure is a Postgres upsert, not a PlanIt fetch)")
	}
	if len(state.saves) != 0 {
		t.Errorf("saves: got %d, want 0 (the failed page's checkpoint must not stand)", len(state.saves))
	}
}

// TestRecentSweepHandler_Run_StateGetErrorIsNeverPlanitOrigin proves a
// state-read failure is surfaced as an error but never misclassified as
// PlanIt-origin, and no fetch is attempted.
func TestRecentSweepHandler_Run_StateGetErrorIsNeverPlanitOrigin(t *testing.T) {
	t.Parallel()
	fetcher := newFakeRecentSweepFetcher()
	state := newFakeRecentSweepStateStore()
	state.getErr = errors.New("postgres: connection refused")

	h := newRecentSweepHandler(t, fetcher, newFakeApps(), state, recentSweepOpts(6))
	out := h.Run(context.Background())

	if out.err == nil {
		t.Fatal("expected the state-read error to surface")
	}
	if out.planitOrigin {
		t.Error("planitOrigin: got true, want false")
	}
	if fetcher.calls != 0 {
		t.Errorf("fetcher.calls: got %d, want 0", fetcher.calls)
	}
}

// TestRecentSweepHandler_Run_RateLimitedStopsTurnWithoutError proves a 429
// records rateLimited + retryAfter, does NOT set err, and does not persist the
// rate-limited page (only the prior good page's checkpoint stands).
func TestRecentSweepHandler_Run_RateLimitedStopsTurnWithoutError(t *testing.T) {
	t.Parallel()
	retryAfter := 45 * time.Second
	good := recentLightRow("ok/FUL", 300, "Undecided")
	apps := newFakeApps()
	apps.existing["ok/FUL"] = good

	fetcher := newFakeRecentSweepFetcher(
		fakeRecentSweepResponse{result: planit.FetchPageResult{Applications: []applications.PlanningApplication{good}, HasMorePages: true}},
		fakeRecentSweepResponse{err: &planit.RateLimitError{RetryAfter: &retryAfter}},
	)
	state := newFakeRecentSweepStateStore()
	state.state = RecentSweepState{LapAnchor: recentSweepToday, WindowEnd: recentSweepToday}

	h := newRecentSweepHandler(t, fetcher, apps, state, recentSweepOpts(6))
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: a 429 must not surface as an error: %v", out.err)
	}
	if !out.rateLimited {
		t.Error("rateLimited: got false, want true")
	}
	if out.retryAfter == nil || *out.retryAfter != retryAfter {
		t.Errorf("retryAfter: got %v, want %v", out.retryAfter, retryAfter)
	}
	if state.state.CursorNextIndex != 1 {
		t.Errorf("CursorNextIndex: got %d, want 1 (only the good page persisted)", state.state.CursorNextIndex)
	}
}

// --- Run: hydration decision ---

// TestRecentSweepHandler_Run_NoHydrationWhenFoundAndUnchanged proves a light
// row that is present in Postgres and does not diverge triggers no hydration.
func TestRecentSweepHandler_Run_NoHydrationWhenFoundAndUnchanged(t *testing.T) {
	t.Parallel()
	row := recentLightRow("same/FUL", 300, "Undecided")
	apps := newFakeApps()
	apps.existing["same/FUL"] = row

	fetcher := newFakeRecentSweepFetcher(fakeRecentSweepResponse{
		result: planit.FetchPageResult{Applications: []applications.PlanningApplication{row}, HasMorePages: false},
	})
	state := newFakeRecentSweepStateStore()
	state.state = RecentSweepState{LapAnchor: recentSweepToday, WindowEnd: recentSweepToday}

	h := newRecentSweepHandler(t, fetcher, apps, state, recentSweepOpts(1))
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	if len(fetcher.hydrateCalls) != 0 {
		t.Errorf("hydrateCalls: got %v, want none", fetcher.hydrateCalls)
	}
	if len(apps.upserts) != 0 {
		t.Errorf("upserts: got %d, want 0", len(apps.upserts))
	}
	if out.recordsIngested != 0 {
		t.Errorf("recordsIngested: got %d, want 0", out.recordsIngested)
	}
}

// TestRecentSweepHandler_Run_HydratesRowMissingFromPostgres proves the primary
// backstop path: a light row not found in Postgres is hydrated to the full
// record and ingested.
func TestRecentSweepHandler_Run_HydratesRowMissingFromPostgres(t *testing.T) {
	t.Parallel()
	fetcher := newFakeRecentSweepFetcher(fakeRecentSweepResponse{
		result: planit.FetchPageResult{
			Applications: []applications.PlanningApplication{recentLightRow("missing/FUL", 300, "Undecided")},
			HasMorePages: false,
		},
	})
	full := testApp("missing", 300, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	fetcher.hydrated["missing/FUL"] = full
	apps := newFakeApps()
	state := newFakeRecentSweepStateStore()
	state.state = RecentSweepState{LapAnchor: recentSweepToday, WindowEnd: recentSweepToday}

	h := newRecentSweepHandler(t, fetcher, apps, state, recentSweepOpts(1))
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	if len(fetcher.hydrateCalls) != 1 || fetcher.hydrateCalls[0] != "missing/FUL" {
		t.Errorf("hydrateCalls: got %v, want [missing/FUL]", fetcher.hydrateCalls)
	}
	if len(apps.upserts) != 1 || apps.upserts[0].UID != "missing/FUL" {
		t.Fatalf("upserts: got %+v", apps.upserts)
	}
	if out.recordsIngested != 1 {
		t.Errorf("recordsIngested: got %d, want 1", out.recordsIngested)
	}
	if out.hydrations != 1 {
		t.Errorf("hydrations: got %d, want 1", out.hydrations)
	}
}

// TestRecentSweepHandler_Run_HydratesRowThatDiverges proves a light row whose
// app_state or decided_date differs from Postgres is hydrated.
func TestRecentSweepHandler_Run_HydratesRowThatDiverges(t *testing.T) {
	t.Parallel()
	apps := newFakeApps()
	apps.existing["drift/FUL"] = recentLightRow("drift/FUL", 300, "Undecided")

	fetcher := newFakeRecentSweepFetcher(fakeRecentSweepResponse{
		result: planit.FetchPageResult{
			Applications: []applications.PlanningApplication{recentLightRow("drift/FUL", 300, "Permitted")},
			HasMorePages: false,
		},
	})
	full := testApp("drift", 300, time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC))
	permitted := "Permitted"
	full.AppState = &permitted
	fetcher.hydrated["drift/FUL"] = full
	state := newFakeRecentSweepStateStore()
	state.state = RecentSweepState{LapAnchor: recentSweepToday, WindowEnd: recentSweepToday}

	h := newRecentSweepHandler(t, fetcher, apps, state, recentSweepOpts(1))
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	if len(fetcher.hydrateCalls) != 1 {
		t.Errorf("hydrateCalls: got %v, want one", fetcher.hydrateCalls)
	}
	if len(apps.upserts) != 1 || apps.upserts[0].AppState == nil || *apps.upserts[0].AppState != "Permitted" {
		t.Fatalf("upserts: got %+v", apps.upserts)
	}
}

// TestRecentSweepHandler_Run_HydrationStopsAtCap proves the burst bound: once
// maxHydrationsPerSweepTurn hydrations have run, further divergent rows are
// left alone, the cap-hit flag is set, the page is checkpointed, and the turn
// returns with NO error.
func TestRecentSweepHandler_Run_HydrationStopsAtCap(t *testing.T) {
	t.Parallel()
	rows := make([]applications.PlanningApplication, 0, maxHydrationsPerSweepTurn+3)
	fetcher := newFakeRecentSweepFetcher()
	for i := range maxHydrationsPerSweepTurn + 3 {
		uid := "cap-" + string(rune('a'+i)) + "/FUL"
		rows = append(rows, recentLightRow(uid, 300, "Undecided"))
		fetcher.hydrated[uid] = applications.PlanningApplication{UID: uid, AreaID: 300}
	}
	fetcher.responses = []fakeRecentSweepResponse{{
		result: planit.FetchPageResult{Applications: rows, HasMorePages: true},
	}}
	apps := newFakeApps() // none found: every row wants hydration
	state := newFakeRecentSweepStateStore()
	state.state = RecentSweepState{LapAnchor: recentSweepToday, WindowEnd: recentSweepToday}

	h := newRecentSweepHandler(t, fetcher, apps, state, recentSweepOpts(6))
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: a hydration cap hit must not error: %v", out.err)
	}
	if !out.hydrationCapHit {
		t.Error("hydrationCapHit: got false, want true")
	}
	if out.hydrations != maxHydrationsPerSweepTurn {
		t.Errorf("hydrations: got %d, want %d", out.hydrations, maxHydrationsPerSweepTurn)
	}
	if len(fetcher.hydrateCalls) != maxHydrationsPerSweepTurn {
		t.Errorf("hydrateCalls: got %d, want %d", len(fetcher.hydrateCalls), maxHydrationsPerSweepTurn)
	}
	if len(state.saves) != 1 {
		t.Errorf("saves: got %d, want 1 (the page is still checkpointed on a cap hit)", len(state.saves))
	}
	if fetcher.calls != 1 {
		t.Errorf("fetcher.calls: got %d, want 1 (the turn stops after the cap)", fetcher.calls)
	}
}

// TestRecentSweepHandler_Run_SkipsHydratedRecordWithWrongAreaID proves the
// area_id guard: a hydrated record whose AreaID does not match the light row
// that flagged it (a cross-authority uid collision) is logged and skipped, not
// ingested.
func TestRecentSweepHandler_Run_SkipsHydratedRecordWithWrongAreaID(t *testing.T) {
	t.Parallel()
	fetcher := newFakeRecentSweepFetcher(fakeRecentSweepResponse{
		result: planit.FetchPageResult{
			Applications: []applications.PlanningApplication{recentLightRow("coll/FUL", 301, "Undecided")},
			HasMorePages: false,
		},
	})
	fetcher.hydratedMulti["coll/FUL"] = []applications.PlanningApplication{
		{UID: "coll/FUL", AreaID: 999}, // wrong authority (light row flagged area 301)
	}
	apps := newFakeApps()
	state := newFakeRecentSweepStateStore()
	state.state = RecentSweepState{LapAnchor: recentSweepToday, WindowEnd: recentSweepToday}

	h := newRecentSweepHandler(t, fetcher, apps, state, recentSweepOpts(1))
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	if len(apps.upserts) != 0 {
		t.Errorf("upserts: got %d, want 0 (wrong-area record must be skipped)", len(apps.upserts))
	}
	if out.recordsIngested != 0 {
		t.Errorf("recordsIngested: got %d, want 0", out.recordsIngested)
	}
}

// TestRecentSweepHandler_Run_HydrationFetchErrorIsPlanitOrigin proves a
// hydration fetch failure stops the turn, surfaces the error, and is
// classified as PlanIt-origin.
func TestRecentSweepHandler_Run_HydrationFetchErrorIsPlanitOrigin(t *testing.T) {
	t.Parallel()
	fetcher := newFakeRecentSweepFetcher(fakeRecentSweepResponse{
		result: planit.FetchPageResult{
			Applications: []applications.PlanningApplication{recentLightRow("boom/FUL", 300, "Undecided")},
			HasMorePages: true,
		},
	})
	fetcher.hydrateErr["boom/FUL"] = errors.New("planit: transport blew up")
	apps := newFakeApps()
	state := newFakeRecentSweepStateStore()
	state.state = RecentSweepState{LapAnchor: recentSweepToday, WindowEnd: recentSweepToday}

	h := newRecentSweepHandler(t, fetcher, apps, state, recentSweepOpts(6))
	out := h.Run(context.Background())

	if out.err == nil {
		t.Fatal("expected the hydration fetch error to surface")
	}
	if !out.planitOrigin {
		t.Error("planitOrigin: got false, want true")
	}
	if len(state.saves) != 0 {
		t.Errorf("saves: got %d, want 0", len(state.saves))
	}
}

// TestRecentSweepHandler_Run_Hydration429StopsTurnWithoutError proves a 429
// during hydration is handled exactly like a page-fetch 429: rateLimited set,
// no error, no save for the in-flight page.
func TestRecentSweepHandler_Run_Hydration429StopsTurnWithoutError(t *testing.T) {
	t.Parallel()
	retryAfter := 30 * time.Second
	fetcher := newFakeRecentSweepFetcher(fakeRecentSweepResponse{
		result: planit.FetchPageResult{
			Applications: []applications.PlanningApplication{recentLightRow("rl/FUL", 300, "Undecided")},
			HasMorePages: true,
		},
	})
	fetcher.hydrateErr["rl/FUL"] = &planit.RateLimitError{RetryAfter: &retryAfter}
	apps := newFakeApps()
	state := newFakeRecentSweepStateStore()
	state.state = RecentSweepState{LapAnchor: recentSweepToday, WindowEnd: recentSweepToday}

	h := newRecentSweepHandler(t, fetcher, apps, state, recentSweepOpts(6))
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	if !out.rateLimited || out.retryAfter == nil || *out.retryAfter != retryAfter {
		t.Errorf("rate limit not surfaced: rateLimited=%v retryAfter=%v", out.rateLimited, out.retryAfter)
	}
	if len(state.saves) != 0 {
		t.Errorf("saves: got %d, want 0", len(state.saves))
	}
}
