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

// fakeInverseMaskFetcher serves pre-canned Lane C epoch pages keyed by the
// requested 0-based record offset, and hydration responses keyed by uid. It
// can be primed to fail a specific fetch ordinal (1-based, failNth) or a
// specific hydration uid (hydrateErr).
type fakeInverseMaskFetcher struct {
	pages        map[int]planit.FetchPageResult
	hydrated     map[string]applications.PlanningApplication
	hydrateErr   map[string]error
	failNth      map[int]error
	calls        int
	queries      []planit.NationalInverseMaskQuery
	hydrateCalls []string
}

func newFakeInverseMaskFetcher() *fakeInverseMaskFetcher {
	return &fakeInverseMaskFetcher{
		pages:      map[int]planit.FetchPageResult{},
		hydrated:   map[string]applications.PlanningApplication{},
		hydrateErr: map[string]error{},
		failNth:    map[int]error{},
	}
}

func (f *fakeInverseMaskFetcher) FetchInverseMaskPage(_ context.Context, q planit.NationalInverseMaskQuery) (planit.FetchPageResult, error) {
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

func (f *fakeInverseMaskFetcher) FetchByUID(_ context.Context, uid string) (planit.FetchPageResult, error) {
	f.hydrateCalls = append(f.hydrateCalls, uid)
	if err, ok := f.hydrateErr[uid]; ok {
		return planit.FetchPageResult{}, err
	}
	app, ok := f.hydrated[uid]
	if !ok {
		return planit.FetchPageResult{Applications: nil}, nil
	}
	return planit.FetchPageResult{Applications: []applications.PlanningApplication{app}}, nil
}

// lightApp builds a Lane C light-projection row: uid, area_id, app_state,
// last_different — the fields planit.inverseMaskSelectFields actually
// requests.
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

// newLaneCHandler wires an InverseMaskLaneHandler pinned to a fixed clock
// (2026-07-14T12:00:00Z).
func newLaneCHandler(t *testing.T, fetcher *fakeInverseMaskFetcher, apps applicationStore, state *fakeStateStore, opts InverseMaskOptions) *InverseMaskLaneHandler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(discard{}, nil))
	clock := func() time.Time { return time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) }
	return NewInverseMaskLaneHandler(fetcher, state, apps, opts, clock, logger)
}

func defaultInverseMaskOpts() InverseMaskOptions {
	return InverseMaskOptions{MaskWindow: 90 * 24 * time.Hour}
}

// TestInverseMaskLane_FirstRunSeedsWithNoRequest proves Lane C's seed is
// pure state initialisation: unlike Lane A/B (which fetch a real page-0 to
// discover PlanIt's head), Lane C's epoch is purely time-bound, so seeding
// makes ZERO PlanIt requests — a zero-width epoch cannot possibly contain a
// record.
func TestInverseMaskLane_FirstRunSeedsWithNoRequest(t *testing.T) {
	t.Parallel()
	fetcher := newFakeInverseMaskFetcher()
	apps := newFakeApps()
	state := newFakeStateStore() // never run

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if fetcher.calls != 0 {
		t.Errorf("expected zero PlanIt requests on the seed call, got %d", fetcher.calls)
	}
	wantNow := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	if !out.watermarkAfter.Equal(wantNow) {
		t.Errorf("watermarkAfter (epoch_upper): got %v, want now() %v", out.watermarkAfter, wantNow)
	}
	got := state.states[sentinelLaneC]
	if !got.HighWaterMark.Equal(wantNow) {
		t.Errorf("persisted HighWaterMark: got %v, want %v", got.HighWaterMark, wantNow)
	}
	if got.Cursor != nil {
		t.Errorf("persisted Cursor: got %+v, want nil", got.Cursor)
	}
}

// TestInverseMaskLane_AnchorsNewEpochFromPriorCeiling proves epoch tiling's
// contiguous-floor rule (ADR 0044 §5): once a prior epoch has drained (no
// active cursor, a pinned ceiling persisted), the NEXT call anchors a new
// epoch whose floor is EXACTLY that prior ceiling — no gap.
func TestInverseMaskLane_AnchorsNewEpochFromPriorCeiling(t *testing.T) {
	t.Parallel()
	priorCeiling := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{From: 0, Applications: nil, HasMorePages: false}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: priorCeiling} // no cursor: the previous epoch already drained

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if len(fetcher.queries) != 1 {
		t.Fatalf("expected exactly one fetch, got %d", len(fetcher.queries))
	}
	if !fetcher.queries[0].EpochLower.Equal(priorCeiling) {
		t.Errorf("EpochLower: got %v, want the prior epoch's ceiling %v (contiguous tiling, no gap)", fetcher.queries[0].EpochLower, priorCeiling)
	}
	wantNewCeiling := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) // the pinned test clock
	if !out.watermarkAfter.Equal(wantNewCeiling) {
		t.Errorf("watermarkAfter (new epoch_upper): got %v, want %v", out.watermarkAfter, wantNewCeiling)
	}
}

// TestInverseMaskLane_ResumesActiveEpochAtCheckpointedIndex proves the
// per-page checkpoint's whole point: an active cursor resumes pagination at
// its persisted NextIndex within the SAME epoch bounds, rather than
// re-anchoring.
func TestInverseMaskLane_ResumesActiveEpochAtCheckpointedIndex(t *testing.T) {
	t.Parallel()
	epochLower := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	epochUpper := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[300] = planit.FetchPageResult{From: 300, Applications: nil, HasMorePages: false}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{
		HighWaterMark: epochUpper,
		Cursor:        &PollCursor{DifferentStart: epochLower, NextIndex: 300},
	}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if len(fetcher.queries) != 1 || fetcher.queries[0].StartIndex != 300 {
		t.Fatalf("expected exactly one fetch at StartIndex 300, got %+v", fetcher.queries)
	}
	if !fetcher.queries[0].EpochLower.Equal(epochLower) {
		t.Errorf("EpochLower: got %v, want the active epoch's floor %v", fetcher.queries[0].EpochLower, epochLower)
	}
	if got := state.states[sentinelLaneC].Cursor; got != nil {
		t.Errorf("cursor: got %+v, want nil (epoch drained on an empty final page)", got)
	}
	if !out.watermarkAfter.Equal(epochUpper) {
		t.Errorf("watermarkAfter: got %v, want the unchanged pinned ceiling %v", out.watermarkAfter, epochUpper)
	}
}

// TestInverseMaskLane_PinnedCeilingStopsTheEpoch proves the pinned
// epoch_upper is enforced by the executor reading returned records, not by
// PlanIt (which has no different_end/ceiling param): a record past the
// ceiling stops the walk without being ingested, and — because ascending
// order means everything after it also exceeds the ceiling — the whole
// epoch is marked drained, not just this page.
func TestInverseMaskLane_PinnedCeilingStopsTheEpoch(t *testing.T) {
	t.Parallel()
	epochLower := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	epochUpper := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	withinCeiling := epochUpper.Add(-time.Hour)
	pastCeiling := epochUpper.Add(time.Hour)

	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From: 0,
		Applications: []applications.PlanningApplication{
			lightApp("in/FUL", 99, "Permitted", withinCeiling),
			lightApp("out/FUL", 99, "Permitted", pastCeiling), // belongs to a future epoch
		},
		HasMorePages: false,
	}
	apps := newFakeApps() // no existing records: both would otherwise be new stragglers
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{
		HighWaterMark: epochUpper,
		Cursor:        &PollCursor{DifferentStart: epochLower, NextIndex: 0},
	}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if len(fetcher.hydrateCalls) != 1 || fetcher.hydrateCalls[0] != "in/FUL" {
		t.Errorf("expected only the within-ceiling record to be processed: got %v", fetcher.hydrateCalls)
	}
	if got := state.states[sentinelLaneC].Cursor; got != nil {
		t.Errorf("cursor: got %+v, want nil (the ceiling hit marks the whole epoch drained)", got)
	}
	if !out.watermarkAfter.Equal(epochUpper) {
		t.Errorf("watermarkAfter: got %v, want the unchanged pinned ceiling %v", out.watermarkAfter, epochUpper)
	}
}

// TestInverseMaskLane_SkipsRecordsAtOrBeforeEpochLower proves the ascending
// exact-instant skip: the date-granular different_start prefilter can
// re-serve records from the boundary day the PREVIOUS epoch already
// handled, and this epoch must not re-process them.
func TestInverseMaskLane_SkipsRecordsAtOrBeforeEpochLower(t *testing.T) {
	t.Parallel()
	epochLower := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	epochUpper := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From: 0,
		Applications: []applications.PlanningApplication{
			lightApp("boundary/FUL", 99, "Permitted", epochLower),               // == epochLower: already handled by the previous epoch
			lightApp("before/FUL", 99, "Permitted", epochLower.Add(-time.Hour)), // < epochLower
		},
		HasMorePages: false,
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: epochUpper, Cursor: &PollCursor{DifferentStart: epochLower, NextIndex: 0}}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if len(fetcher.hydrateCalls) != 0 {
		t.Errorf("expected no hydration attempts (both records at/before epochLower): got %v", fetcher.hydrateCalls)
	}
}

// TestInverseMaskLane_LastDifferentOnlyChurnDoesNotHydrate is the ADR
// 0044 §4 anti-amplification test: a row whose app_state and decided_date
// both still match Postgres, but whose last_different has moved (a re-index
// bump), must NOT be treated as a straggler — this is the exact bug the old
// per-authority ReconciliationHandler hit.
func TestInverseMaskLane_LastDifferentOnlyChurnDoesNotHydrate(t *testing.T) {
	t.Parallel()
	epochLower := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	epochUpper := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	oldLD := epochLower.Add(-24 * time.Hour) // the persisted record's own last_different — irrelevant to the diff
	newLD := epochLower.Add(time.Hour)       // only last_different changed (a re-index bump)

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
	state.states[sentinelLaneC] = PollState{HighWaterMark: epochUpper, Cursor: &PollCursor{DifferentStart: epochLower, NextIndex: 0}}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if len(fetcher.hydrateCalls) != 0 {
		t.Errorf("a last_different-only churned row must NOT hydrate: hydrateCalls=%v", fetcher.hydrateCalls)
	}
	if out.recordsIngested != 0 {
		t.Errorf("recordsIngested: got %d, want 0", out.recordsIngested)
	}
}

// TestInverseMaskLane_AppStateDriftHydrates is the positive case alongside
// the anti-amplification test: a genuine app_state change DOES hydrate and
// ingest.
func TestInverseMaskLane_AppStateDriftHydrates(t *testing.T) {
	t.Parallel()
	epochLower := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	epochUpper := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	newLD := epochLower.Add(time.Hour)

	existingState := "Undecided"
	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{lightApp("24/0001/FUL", 99, "Permitted", newLD)},
		HasMorePages: false,
	}
	full := testApp("24/0001", 99, newLD)
	full.UID = "24/0001/FUL"
	permitted := "Permitted"
	full.AppState = &permitted
	fetcher.hydrated["24/0001/FUL"] = full

	apps := newFakeApps()
	apps.existing["24/0001/FUL"] = applications.PlanningApplication{UID: "24/0001/FUL", AreaID: 99, AppState: &existingState, LastDifferent: epochLower.Add(-time.Hour)}
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: epochUpper, Cursor: &PollCursor{DifferentStart: epochLower, NextIndex: 0}}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if len(fetcher.hydrateCalls) != 1 || fetcher.hydrateCalls[0] != "24/0001/FUL" {
		t.Errorf("expected exactly one hydration attempt for 24/0001/FUL: got %v", fetcher.hydrateCalls)
	}
	if out.recordsIngested != 1 {
		t.Errorf("recordsIngested: got %d, want 1", out.recordsIngested)
	}
	if len(apps.upserts) != 1 || apps.upserts[0].UID != "24/0001/FUL" {
		t.Fatalf("upserts: got %+v", apps.upserts)
	}
}

// TestInverseMaskLane_UsesAreaIDForAuthorityScopedExistenceCheck pins the
// deliberate ADR 0044 deviation from the issue's literal query string: a
// NATIONAL query's uid alone is not enough to scope the existence check
// (PlanIt's uid is only unique within one authority) — the light row's
// area_id must build the authorityCode GetByUID is called with.
func TestInverseMaskLane_UsesAreaIDForAuthorityScopedExistenceCheck(t *testing.T) {
	t.Parallel()
	epochLower := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	epochUpper := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	newLD := epochLower.Add(time.Hour)

	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{lightApp("24/0001/FUL", 300, "Undecided", newLD)},
		HasMorePages: false,
	}
	apps := &fakeScopedApps{fakeApps: newFakeApps()}
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: epochUpper, Cursor: &PollCursor{DifferentStart: epochLower, NextIndex: 0}}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if len(apps.authorityCodesSeen) != 1 || apps.authorityCodesSeen[0] != "300" {
		t.Errorf("authorityCode passed to GetByUID: got %v, want [\"300\"] (built from the light row's area_id)", apps.authorityCodesSeen)
	}
}

// TestInverseMaskLane_RateLimitedPageFetchLeavesCursorUntouched mirrors Lane
// A/B's "never advance past a 429" invariant for Lane C's epoch cursor.
func TestInverseMaskLane_RateLimitedPageFetchLeavesCursorUntouched(t *testing.T) {
	t.Parallel()
	epochLower := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	epochUpper := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	retryAfter := 30 * time.Second

	fetcher := newFakeInverseMaskFetcher()
	fetcher.failNth[1] = &planit.RateLimitError{RetryAfter: &retryAfter}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: epochUpper, Cursor: &PollCursor{DifferentStart: epochLower, NextIndex: 300}}

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
		t.Errorf("cursor: got %+v, want the untouched checkpoint (NextIndex=300) — the next cycle resumes here, no re-tread", got)
	}
}

// TestInverseMaskLane_HydrationRateLimitStopsTheWholePage proves a 429 on a
// HYDRATION sub-fetch trips the same "stop everything" rule as a page-fetch
// 429 (ADR 0044: one break on the first 429 from ANY lane) — the second
// straggler on the same page must never be attempted.
func TestInverseMaskLane_HydrationRateLimitStopsTheWholePage(t *testing.T) {
	t.Parallel()
	epochLower := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	epochUpper := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	newLD := epochLower.Add(time.Hour)
	retryAfter := 20 * time.Second

	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From: 0,
		Applications: []applications.PlanningApplication{
			lightApp("first/FUL", 99, "Permitted", newLD),
			lightApp("second/FUL", 99, "Permitted", newLD),
		},
		HasMorePages: false,
	}
	fetcher.hydrateErr["first/FUL"] = &planit.RateLimitError{RetryAfter: &retryAfter}

	apps := newFakeApps() // both new: both would otherwise hydrate
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: epochUpper, Cursor: &PollCursor{DifferentStart: epochLower, NextIndex: 0}}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if !out.rateLimited {
		t.Fatal("expected rateLimited=true")
	}
	if len(fetcher.hydrateCalls) != 1 || fetcher.hydrateCalls[0] != "first/FUL" {
		t.Errorf("expected hydration to stop after the first 429, never attempting the second straggler: got %v", fetcher.hydrateCalls)
	}
}

// TestInverseMaskLane_IngestErrorIsAHardStop proves a hydrated Ingest
// failure freezes state exactly like a page-fetch error: nothing new is
// persisted this call, so the previous checkpoint stands and a retry simply
// re-fetches this page.
func TestInverseMaskLane_IngestErrorIsAHardStop(t *testing.T) {
	t.Parallel()
	epochLower := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	epochUpper := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	newLD := epochLower.Add(time.Hour)

	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{lightApp("24/0001/FUL", 99, "Permitted", newLD)},
		HasMorePages: false,
	}
	full := testApp("24/0001", 99, newLD)
	full.UID = "24/0001/FUL"
	fetcher.hydrated["24/0001/FUL"] = full

	apps := newFakeApps()
	apps.upsertErr = errors.New("db write failed")
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: epochUpper, Cursor: &PollCursor{DifferentStart: epochLower, NextIndex: 0}}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err == nil {
		t.Fatal("expected the Ingest failure to surface as out.err")
	}
	got := state.states[sentinelLaneC].Cursor
	if got == nil || got.NextIndex != 0 {
		t.Errorf("cursor: got %+v, want the untouched checkpoint (NextIndex=0, nothing persisted this call)", got)
	}
}

// TestInverseMaskLane_ContiguousEpochTilingAcrossAStall drives Lane C
// through seed -> anchor -> a simulated mid-epoch stall (a page reports more
// follow, then the epoch drains on a LATER call, mirroring a storm that
// stalls the walk and resumes once it passes) -> a second anchor, and
// asserts the second epoch's floor is EXACTLY the first epoch's ceiling —
// contiguous tiling, no gap, regardless of the stall (ADR 0044 §5).
func TestInverseMaskLane_ContiguousEpochTilingAcrossAStall(t *testing.T) {
	t.Parallel()
	fetcher := newFakeInverseMaskFetcher()
	apps := newFakeApps()
	state := newFakeStateStore()
	logger := slog.New(slog.NewTextHandler(discard{}, nil))

	// The clock genuinely advances between calls (like production), so
	// successive epoch ceilings are distinguishable instants.
	callTime := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time {
		now := callTime
		callTime = callTime.Add(time.Hour)
		return now
	}
	h := NewInverseMaskLaneHandler(fetcher, state, apps, defaultInverseMaskOpts(), clock, logger)

	if out := h.RunOnePage(context.Background()); out.err != nil {
		t.Fatalf("seed: %v", out.err)
	}

	// Anchor epoch 1 for real: a page that reports more pages remain leaves
	// the epoch mid-drain (the simulated stall point).
	fetcher.pages[0] = planit.FetchPageResult{From: 0, Applications: nil, HasMorePages: true}
	anchorOut := h.RunOnePage(context.Background())
	if anchorOut.err != nil {
		t.Fatalf("anchor: %v", anchorOut.err)
	}
	epoch1Ceiling := anchorOut.watermarkAfter
	midCursor := state.states[sentinelLaneC].Cursor
	if midCursor == nil {
		t.Fatal("expected an active mid-epoch cursor after a HasMorePages=true page")
	}

	// The stall passes: the resumed page is empty with no more pages, so the
	// epoch fully drains on this call.
	fetcher.pages[midCursor.NextIndex] = planit.FetchPageResult{From: midCursor.NextIndex, Applications: nil, HasMorePages: false}
	if out := h.RunOnePage(context.Background()); out.err != nil {
		t.Fatalf("drain: %v", out.err)
	}
	if state.states[sentinelLaneC].Cursor != nil {
		t.Fatal("expected the cursor to clear once the epoch drains")
	}

	// Epoch 2 anchors: its floor must be EXACTLY epoch 1's ceiling.
	fetcher.pages[0] = planit.FetchPageResult{From: 0, Applications: nil, HasMorePages: false}
	if out := h.RunOnePage(context.Background()); out.err != nil {
		t.Fatalf("epoch 2 anchor: %v", out.err)
	}
	lastQuery := fetcher.queries[len(fetcher.queries)-1]
	if !lastQuery.EpochLower.Equal(truncateToDate(epoch1Ceiling)) {
		t.Errorf("epoch 2's EpochLower: got %v, want epoch 1's ceiling %v (contiguous tiling, no gap)", lastQuery.EpochLower, epoch1Ceiling)
	}
}
