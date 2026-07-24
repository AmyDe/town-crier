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

// TestInverseMaskLane_ResumesActiveEpochWithOverlap proves the per-page
// checkpoint's resume story (GH#986): an active cursor resumes pagination at
// max(0, NextIndex-resumeOverlapRecords) within the SAME epoch bounds,
// mirroring Lane A/B's own resume overlap (nationallane.go), rather than
// either re-anchoring or resuming at the checkpointed index with no safety
// margin.
func TestInverseMaskLane_ResumesActiveEpochWithOverlap(t *testing.T) {
	t.Parallel()
	epochLower := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	epochUpper := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	fetcher := newFakeInverseMaskFetcher()
	// 300 - the 100-record resume overlap = 200.
	fetcher.pages[200] = planit.FetchPageResult{From: 200, Applications: nil, HasMorePages: false}
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
	if len(fetcher.queries) != 1 || fetcher.queries[0].StartIndex != 200 {
		t.Fatalf("expected exactly one fetch at StartIndex 200 (300 - the 100-record resume overlap), got %+v", fetcher.queries)
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

// TestInverseMaskLane_ResumeOverlapDedupesAlreadyProcessedRows proves the
// resume overlap's safety property (GH#986): rows the overlap window
// re-serves that are ALREADY correct in Postgres dedupe via
// GetByUID/inverseMaskDiffers and are never re-hydrated or re-notified,
// while a genuine straggler beyond the overlap zone still hydrates and
// ingests normally — the overlap costs a few redundant existence reads, not
// duplicate notifications.
func TestInverseMaskLane_ResumeOverlapDedupesAlreadyProcessedRows(t *testing.T) {
	t.Parallel()
	epochLower := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	epochUpper := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	ld := epochLower.Add(time.Hour)

	same := "Permitted"
	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[200] = planit.FetchPageResult{ // 300 - the 100-record resume overlap
		From: 200,
		Applications: []applications.PlanningApplication{
			lightApp("already/FUL", 99, same, ld), // in the overlap window: unchanged since last pass
			lightApp("genuine/FUL", 99, "Permitted", ld),
		},
		HasMorePages: false,
	}
	full := testApp("genuine", 99, ld)
	full.UID = "genuine/FUL"
	permitted := "Permitted"
	full.AppState = &permitted
	fetcher.hydrated["genuine/FUL"] = full

	apps := newFakeApps()
	apps.existing["already/FUL"] = applications.PlanningApplication{UID: "already/FUL", AreaID: 99, AppState: &same, LastDifferent: ld}
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
	if len(fetcher.hydrateCalls) != 1 || fetcher.hydrateCalls[0] != "genuine/FUL" {
		t.Errorf("expected only the genuine straggler to hydrate (the overlap-reprocessed row dedupes via GetByUID/inverseMaskDiffers): got %v", fetcher.hydrateCalls)
	}
	if out.recordsIngested != 1 {
		t.Errorf("recordsIngested: got %d, want 1 (the already-processed row must not be re-notified)", out.recordsIngested)
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

// TestInverseMaskLane_RateLimitedPageFetchPreservesCursorAdvancesLastPollTime
// is GH#986 acceptance criterion (d): a page-fetch 429 must never lose the
// existing checkpoint (the cursor's NextIndex is re-saved unchanged, exactly
// as loaded — nothing was actually fetched, so there is no new progress to
// record), but it MUST advance last_poll_time so the planner's LRU rotates
// off Lane C instead of freezing it at the front of the queue forever (the
// observed prod livelock).
func TestInverseMaskLane_RateLimitedPageFetchPreservesCursorAdvancesLastPollTime(t *testing.T) {
	t.Parallel()
	epochLower := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	epochUpper := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	retryAfter := 30 * time.Second
	wantNow := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) // newLaneCHandler's pinned clock

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
		t.Errorf("cursor: got %+v, want the preserved checkpoint (NextIndex=300) — the next cycle resumes here, no re-tread", got)
	}
	if lastPoll := state.states[sentinelLaneC].LastPollTime; !lastPoll.Equal(wantNow) {
		t.Errorf("LastPollTime: got %v, want %v (must advance so the planner LRU rotates off this lane)", lastPoll, wantNow)
	}
}

// TestInverseMaskLane_FreshAnchorPageFetch429PreservesUnanchoredState covers
// the page-fetch-429 checkpoint's OTHER shape: a 429 on the very first fetch
// of a freshly-anchoring epoch (no active cursor yet). The re-save must
// persist the state EXACTLY as it was loaded — the prior ceiling as
// HighWaterMark, cursor nil — not the in-memory epochUpper this call
// provisionally reset to now(); saving that would falsely mark a brand new
// epoch as already anchored-and-drained (nil cursor) without a single record
// ever having been walked, silently skipping its entire contents.
func TestInverseMaskLane_FreshAnchorPageFetch429PreservesUnanchoredState(t *testing.T) {
	t.Parallel()
	priorCeiling := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	retryAfter := 15 * time.Second
	wantNow := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) // newLaneCHandler's pinned clock

	fetcher := newFakeInverseMaskFetcher()
	fetcher.failNth[1] = &planit.RateLimitError{RetryAfter: &retryAfter}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: priorCeiling} // no cursor: about to anchor a new epoch

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if !out.rateLimited {
		t.Fatal("expected rateLimited=true")
	}
	got := state.states[sentinelLaneC]
	if !got.HighWaterMark.Equal(priorCeiling) {
		t.Errorf("HighWaterMark: got %v, want the unchanged prior ceiling %v (never falsely advanced to now on a failed anchor fetch)", got.HighWaterMark, priorCeiling)
	}
	if got.Cursor != nil {
		t.Errorf("cursor: got %+v, want nil (still no active cursor -- the epoch never actually anchored)", got.Cursor)
	}
	if !got.LastPollTime.Equal(wantNow) {
		t.Errorf("LastPollTime: got %v, want %v (must still advance so LRU rotates)", got.LastPollTime, wantNow)
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

// TestInverseMaskLane_PageFetchTimeoutSetsTimedOut proves a page-fetch
// client-side timeout (the real prod shape: a *url.Error wrapping
// context.DeadlineExceeded once PlanIt's HTTP client's retries are
// exhausted) is flagged on the outcome via timedOut, distinguishing it from
// a plain fetch error so NationalPollHandler.Handle can classify the cycle
// as TerminationTimeout rather than TerminationNatural (tc-pmh5y).
func TestInverseMaskLane_PageFetchTimeoutSetsTimedOut(t *testing.T) {
	t.Parallel()
	epochLower := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	epochUpper := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

	fetcher := newFakeInverseMaskFetcher()
	fetcher.failNth[1] = &url.Error{Op: "Get", URL: "https://www.planit.org.uk/api/applics/json", Err: context.DeadlineExceeded}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: epochUpper, Cursor: &PollCursor{DifferentStart: epochLower, NextIndex: 300}}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err == nil {
		t.Fatal("expected the timed-out fetch to surface as out.err")
	}
	if !out.timedOut {
		t.Error("timedOut: got false, want true (page-fetch client timeout)")
	}
}

// TestInverseMaskLane_HydrationTimeoutSetsTimedOut mirrors
// TestInverseMaskLane_PageFetchTimeoutSetsTimedOut for a HYDRATION
// sub-fetch timeout — the exact site that produced the real 2026-07-23 prod
// failure ("lane C: hydration fetch ... context deadline exceeded", via
// FetchByUID).
func TestInverseMaskLane_HydrationTimeoutSetsTimedOut(t *testing.T) {
	t.Parallel()
	epochLower := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	epochUpper := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	newLD := epochLower.Add(time.Hour)

	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{lightApp("first/FUL", 99, "Permitted", newLD)},
		HasMorePages: false,
	}
	fetcher.hydrateErr["first/FUL"] = &url.Error{Op: "Get", URL: "https://www.planit.org.uk/api/applics/json", Err: context.DeadlineExceeded}

	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: epochUpper, Cursor: &PollCursor{DifferentStart: epochLower, NextIndex: 0}}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err == nil {
		t.Fatal("expected the timed-out hydration fetch to surface as out.err")
	}
	if !out.timedOut {
		t.Error("timedOut: got false, want true (hydration client timeout)")
	}
}

// TestInverseMaskLane_MidPageHydrationRateLimitCheckpointsAtFailingOffset is
// GH#986 acceptance criterion (a): a mid-page hydration 429 must checkpoint
// the cursor at the FAILING record's own offset (startIndex + i, where i
// counts every record iterated this page including the exact-instant skip)
// and advance last_poll_time — previously this path (stoppedEarly) returned
// with no save at all, which froze the cursor on the same page forever (the
// observed 59x re-fetch of the same 300-record boundary in prod) and froze
// last_poll_time, starving Lane A/B via the planner's pure LRU.
func TestInverseMaskLane_MidPageHydrationRateLimitCheckpointsAtFailingOffset(t *testing.T) {
	t.Parallel()
	epochLower := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	epochUpper := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	newLD := epochLower.Add(time.Hour)
	retryAfter := 20 * time.Second
	wantNow := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) // newLaneCHandler's pinned clock

	full := testApp("ok", 99, newLD)
	full.UID = "ok/FUL"
	permitted := "Permitted"
	full.AppState = &permitted

	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{
		From: 0,
		Applications: []applications.PlanningApplication{
			lightApp("before/FUL", 99, "Permitted", epochLower), // index 0: <= epochLower, skipped -- still counts toward the offset
			lightApp("ok/FUL", 99, "Permitted", newLD),          // index 1: hydrates fine
			lightApp("fails/FUL", 99, "Permitted", newLD),       // index 2: hydration 429s here
			lightApp("never/FUL", 99, "Permitted", newLD),       // index 3: must never be reached
		},
		HasMorePages: false,
	}
	fetcher.hydrated["ok/FUL"] = full
	fetcher.hydrateErr["fails/FUL"] = &planit.RateLimitError{RetryAfter: &retryAfter}

	apps := newFakeApps() // every uid is new: every one would otherwise hydrate
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: epochUpper, Cursor: &PollCursor{DifferentStart: epochLower, NextIndex: 0}}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if !out.rateLimited {
		t.Fatal("expected rateLimited=true")
	}
	if len(fetcher.hydrateCalls) != 2 || fetcher.hydrateCalls[0] != "ok/FUL" || fetcher.hydrateCalls[1] != "fails/FUL" {
		t.Fatalf("expected hydration to stop right after the failing record, never reaching the fourth: got %v", fetcher.hydrateCalls)
	}
	got := state.states[sentinelLaneC].Cursor
	if got == nil || got.NextIndex != 2 {
		t.Errorf("cursor: got %+v, want NextIndex=2 (the failing record's own offset, so a retry re-attempts it)", got)
	}
	if lastPoll := state.states[sentinelLaneC].LastPollTime; !lastPoll.Equal(wantNow) {
		t.Errorf("LastPollTime: got %v, want %v (must advance so the planner LRU rotates off this lane)", lastPoll, wantNow)
	}
}

// TestInverseMaskLane_HydrationCapStopsPassAndCheckpoints is GH#986
// acceptance criterion (c): once a single RunOnePage call has attempted
// maxHydrationsPerPass hydrations, it stops the walk as a CLEAN early stop
// (out.err is nil, not an error) and checkpoints at the offset reached, so
// the next pass resumes past what this one already hydrated. Bounds the
// FetchByUID burst a page of many clustered genuine stragglers can trigger.
func TestInverseMaskLane_HydrationCapStopsPassAndCheckpoints(t *testing.T) {
	t.Parallel()
	epochLower := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	epochUpper := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	ld := epochLower.Add(time.Hour)
	wantNow := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) // newLaneCHandler's pinned clock

	const recordCount = maxHydrationsPerPass + 5
	fetcher := newFakeInverseMaskFetcher()
	apps := newFakeApps() // every uid is new: every one is a genuine straggler
	lightRows := make([]applications.PlanningApplication, 0, recordCount)
	for i := range recordCount {
		uid := fmt.Sprintf("straggler-%02d/FUL", i)
		lightRows = append(lightRows, lightApp(uid, 99, "Permitted", ld))
		full := testApp(fmt.Sprintf("straggler-%02d", i), 99, ld)
		full.UID = uid
		fetcher.hydrated[uid] = full
	}
	fetcher.pages[0] = planit.FetchPageResult{From: 0, Applications: lightRows, HasMorePages: false}

	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{HighWaterMark: epochUpper, Cursor: &PollCursor{DifferentStart: epochLower, NextIndex: 0}}

	h := newLaneCHandler(t, fetcher, apps, state, defaultInverseMaskOpts())
	out := h.RunOnePage(context.Background())

	if out.err != nil {
		t.Fatalf("RunOnePage: %v (the hydration cap is a clean early stop, not an error)", out.err)
	}
	if len(fetcher.hydrateCalls) != maxHydrationsPerPass {
		t.Fatalf("hydrateCalls: got %d, want %d (the per-pass cap)", len(fetcher.hydrateCalls), maxHydrationsPerPass)
	}
	got := state.states[sentinelLaneC].Cursor
	if got == nil || got.NextIndex != maxHydrationsPerPass {
		t.Errorf("cursor: got %+v, want NextIndex=%d (checkpointed right after the capped hydration run)", got, maxHydrationsPerPass)
	}
	if lastPoll := state.states[sentinelLaneC].LastPollTime; !lastPoll.Equal(wantNow) {
		t.Errorf("LastPollTime: got %v, want %v (LRU must still rotate on a clean cap stop)", lastPoll, wantNow)
	}
}

// TestInverseMaskLane_IngestErrorIsAHardStop proves a hydrated Ingest
// failure checkpoints at the failing record's offset (GH#986) exactly like a
// mid-page hydration 429 does, and advances last_poll_time, even though the
// Ingest failure itself surfaces as out.err — the checkpoint and the error
// are independent: a retry re-fetches from this exact offset (the resume
// overlap covers any residual doubt), rather than either re-walking the
// whole page from scratch or freezing the LRU clock.
func TestInverseMaskLane_IngestErrorIsAHardStop(t *testing.T) {
	t.Parallel()
	epochLower := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	epochUpper := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	newLD := epochLower.Add(time.Hour)

	wantNow := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) // newLaneCHandler's pinned clock

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
