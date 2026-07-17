package polling

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

// fakeReconciliationFetcher serves pre-canned light per-authority pages and
// per-uid hydration records.
type fakeReconciliationFetcher struct {
	pages        map[int]planit.FetchPageResult // authorityID -> light page
	pageErrs     map[int]error                  // authorityID -> fetch error
	hydrated     map[string]applications.PlanningApplication
	hydrateErrs  map[string]error
	hydrateCalls []string
	// pageCalls records the authority ids FetchReconciliationPage was called
	// with, in call order -- proves a bounded sweep only touches the
	// requested slice, and a resumed Run picks up where the last one left
	// off (a fake serving a handful of ids can't catch a resume bug; the
	// caller must supply more ids than AuthoritiesPerCycle).
	pageCalls []int
	// differentStarts records the differentStart cutoff FetchReconciliationPage
	// was called with, in call order -- proves Run computes the
	// LookbackDays-derived cutoff once per sweep and threads it down to every
	// fetch (tc-tuge8/GH#971).
	differentStarts []time.Time
	// cancelAfter, when > 0, calls cancel once len(pageCalls) reaches it --
	// simulates a budget/ctx cut-off arriving mid-sweep, so a test can assert
	// the persisted cursor reflects authorities ACTUALLY attempted, not the
	// planned slice end.
	cancelAfter int
	cancel      context.CancelFunc
}

func newFakeReconciliationFetcher() *fakeReconciliationFetcher {
	return &fakeReconciliationFetcher{
		pages:       map[int]planit.FetchPageResult{},
		pageErrs:    map[int]error{},
		hydrated:    map[string]applications.PlanningApplication{},
		hydrateErrs: map[string]error{},
	}
}

func (f *fakeReconciliationFetcher) FetchReconciliationPage(_ context.Context, authorityID, _ int, differentStart time.Time) (planit.FetchPageResult, error) {
	f.pageCalls = append(f.pageCalls, authorityID)
	f.differentStarts = append(f.differentStarts, differentStart)
	if f.cancelAfter > 0 && len(f.pageCalls) == f.cancelAfter && f.cancel != nil {
		f.cancel()
	}
	if err, ok := f.pageErrs[authorityID]; ok {
		return planit.FetchPageResult{}, err
	}
	return f.pages[authorityID], nil
}

func (f *fakeReconciliationFetcher) FetchByUID(_ context.Context, uid string) (planit.FetchPageResult, error) {
	f.hydrateCalls = append(f.hydrateCalls, uid)
	if err, ok := f.hydrateErrs[uid]; ok {
		return planit.FetchPageResult{}, err
	}
	app, ok := f.hydrated[uid]
	if !ok {
		return planit.FetchPageResult{Applications: nil}, nil
	}
	return planit.FetchPageResult{Applications: []applications.PlanningApplication{app}}, nil
}

// lightApp builds a light-projection record: only uid, area_id, app_state,
// decided_date and last_different are populated, mirroring what
// reconciliationSelectFields actually returns.
func lightApp(uid string, areaID int, state string, lastDifferent time.Time) applications.PlanningApplication {
	s := state
	return applications.PlanningApplication{UID: uid, AreaID: areaID, AppState: &s, LastDifferent: lastDifferent}
}

func newReconciliationHandler(t *testing.T, fetcher *fakeReconciliationFetcher, apps *fakeApps, state *fakeStateStore, authorityIDs []int, opts ReconciliationOptions) *ReconciliationHandler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(discard{}, nil))
	clock := func() time.Time { return time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) }
	return NewReconciliationHandler(fetcher, state, apps, fakeAuthorities{ids: authorityIDs}, opts, clock, logger)
}

// defaultReconciliationOpts sets AuthoritiesPerCycle generously (100) so the
// many tests exercising small (1-3 authority) id lists still sweep the whole
// set in a single Run call, as they did before the cursor existed. Tests that
// specifically exercise the cursor use a small AuthoritiesPerCycle of their
// own with a deliberately larger id list (see the fake-design note on
// pageCalls above).
func defaultReconciliationOpts() ReconciliationOptions {
	return ReconciliationOptions{Interval: 7 * 24 * time.Hour, MaxStragglersPerAuthority: 10, AuthoritiesPerCycle: 100, LookbackDays: 365}
}

// TestReconciliation_HydratesGenuinelyDifferingRow covers the straggler path:
// a light row whose app_state differs from Postgres is hydrated (full-record
// fetch by uid) and ingested.
func TestReconciliation_HydratesGenuinelyDifferingRow(t *testing.T) {
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

	h := newReconciliationHandler(t, fetcher, apps, state, []int{99}, defaultReconciliationOpts())
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("Run: %v", out.err)
	}
	if out.stragglers != 1 || out.hydrated != 1 {
		t.Errorf("stragglers=%d hydrated=%d, want 1/1", out.stragglers, out.hydrated)
	}
	if len(fetcher.hydrateCalls) != 1 || fetcher.hydrateCalls[0] != "24/0001/FUL" {
		t.Errorf("hydrateCalls: got %v", fetcher.hydrateCalls)
	}
	if len(apps.upserts) != 1 || apps.upserts[0].UID != "24/0001/FUL" {
		t.Fatalf("upserts: got %+v", apps.upserts)
	}
}

// TestReconciliation_SkipsRowThatMatchesPostgres proves the light-projection
// diff genuinely gates hydration: an identical row costs no hydration
// request and no ingest.
func TestReconciliation_SkipsRowThatMatchesPostgres(t *testing.T) {
	t.Parallel()
	ld := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	fetcher := newFakeReconciliationFetcher()
	fetcher.pages[99] = planit.FetchPageResult{
		Applications: []applications.PlanningApplication{lightApp("24/0001/FUL", 99, "Undecided", ld)},
	}
	apps := newFakeApps()
	undecided := "Undecided"
	apps.existing["24/0001/FUL"] = applications.PlanningApplication{UID: "24/0001/FUL", AreaID: 99, AppState: &undecided, LastDifferent: ld}
	state := newFakeStateStore()

	h := newReconciliationHandler(t, fetcher, apps, state, []int{99}, defaultReconciliationOpts())
	out := h.Run(context.Background())

	if out.stragglers != 0 || out.hydrated != 0 {
		t.Errorf("stragglers=%d hydrated=%d, want 0/0 (unchanged row must not hydrate)", out.stragglers, out.hydrated)
	}
	if len(fetcher.hydrateCalls) != 0 {
		t.Errorf("hydrateCalls: got %v, want none", fetcher.hydrateCalls)
	}
}

// TestReconciliation_MissingApplicationIsAStraggler covers a uid Postgres has
// never seen: found=false must hydrate unconditionally.
func TestReconciliation_MissingApplicationIsAStraggler(t *testing.T) {
	t.Parallel()
	ld := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	fetcher := newFakeReconciliationFetcher()
	fetcher.pages[99] = planit.FetchPageResult{
		Applications: []applications.PlanningApplication{lightApp("24/0002/FUL", 99, "Undecided", ld)},
	}
	full := testApp("24/0002", 99, ld)
	full.UID = "24/0002/FUL"
	fetcher.hydrated["24/0002/FUL"] = full

	apps := newFakeApps()
	state := newFakeStateStore()

	h := newReconciliationHandler(t, fetcher, apps, state, []int{99}, defaultReconciliationOpts())
	out := h.Run(context.Background())

	if out.stragglers != 1 || out.hydrated != 1 {
		t.Errorf("stragglers=%d hydrated=%d, want 1/1 (never-seen uid must hydrate)", out.stragglers, out.hydrated)
	}
}

// TestReconciliation_PassesLookbackCutoffToFetchReconciliationPage pins
// tc-tuge8/GH#971: Run computes the LookbackDays-derived different_start
// cutoff ONCE from its own injected clock (mirroring how
// NationalLaneHandler.Run computes MaskCutoff once per run) and threads the
// SAME cutoff down to every authority's FetchReconciliationPage call.
func TestReconciliation_PassesLookbackCutoffToFetchReconciliationPage(t *testing.T) {
	t.Parallel()
	fetcher := newFakeReconciliationFetcher()
	fetcher.pages[1] = planit.FetchPageResult{}
	fetcher.pages[2] = planit.FetchPageResult{}
	apps := newFakeApps()
	state := newFakeStateStore()
	opts := ReconciliationOptions{Interval: 7 * 24 * time.Hour, MaxStragglersPerAuthority: 10, AuthoritiesPerCycle: 100, LookbackDays: 365}
	h := newReconciliationHandler(t, fetcher, apps, state, []int{1, 2}, opts)

	h.Run(context.Background())

	// newReconciliationHandler pins the clock to 2026-07-14T12:00:00Z; 365
	// days earlier is 2025-07-14T12:00:00Z (no leap day falls in between).
	wantCutoff := time.Date(2025, 7, 14, 12, 0, 0, 0, time.UTC)
	if len(fetcher.differentStarts) != 2 {
		t.Fatalf("differentStarts: got %d calls, want 2", len(fetcher.differentStarts))
	}
	for i, got := range fetcher.differentStarts {
		if !got.Equal(wantCutoff) {
			t.Errorf("differentStarts[%d]: got %v, want %v (now - LookbackDays)", i, got, wantCutoff)
		}
	}
}

// TestReconciliation_AuthorityFetchErrorSkipsAuthorityNotSweep proves a
// single authority's page-fetch error does not fail the whole sweep: the
// next authority is still swept.
func TestReconciliation_AuthorityFetchErrorSkipsAuthorityNotSweep(t *testing.T) {
	t.Parallel()
	ld := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	fetcher := newFakeReconciliationFetcher()
	fetcher.pageErrs[1] = errors.New("planit: transport blew up")
	fetcher.pages[2] = planit.FetchPageResult{
		Applications: []applications.PlanningApplication{lightApp("24/0003/FUL", 2, "Undecided", ld)},
	}
	apps := newFakeApps()
	state := newFakeStateStore()

	h := newReconciliationHandler(t, fetcher, apps, state, []int{1, 2}, defaultReconciliationOpts())
	out := h.Run(context.Background())

	if out.err != nil {
		t.Fatalf("a per-authority fetch error must not fail the sweep: %v", out.err)
	}
	if out.authoritiesSwept != 1 {
		t.Errorf("authoritiesSwept: got %d, want 1 (authority 1 skipped, authority 2 swept)", out.authoritiesSwept)
	}
}

// TestReconciliation_MaxStragglersPerAuthorityBoundsHydration proves the
// per-authority hydration fan-out is capped.
func TestReconciliation_MaxStragglersPerAuthorityBoundsHydration(t *testing.T) {
	t.Parallel()
	ld := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	fetcher := newFakeReconciliationFetcher()
	fetcher.pages[99] = planit.FetchPageResult{
		Applications: []applications.PlanningApplication{
			lightApp("a/1", 99, "Permitted", ld),
			lightApp("a/2", 99, "Permitted", ld),
			lightApp("a/3", 99, "Permitted", ld),
		},
	}
	apps := newFakeApps()
	state := newFakeStateStore()

	h := newReconciliationHandler(t, fetcher, apps, state, []int{99}, ReconciliationOptions{Interval: 7 * 24 * time.Hour, MaxStragglersPerAuthority: 2, AuthoritiesPerCycle: 100})
	out := h.Run(context.Background())

	if out.stragglers != 2 {
		t.Errorf("stragglers: got %d, want 2 (capped)", out.stragglers)
	}
}

// TestReconciliation_StampsSampleErrorBodyAndCountOnSpan pins tc-tuge8/GH#971
// telemetry: AppTraces is Basic-tier (slog is invisible to KQL), so a
// captured PlanIt error body must land on the Lane C sweep span, not a log
// line. The FIRST non-empty body wins even though a later authority also
// errors (a second sample adds no diagnostic value once the reason is known),
// and reconciliation.error_count counts 400s specifically -- the 500 must not
// inflate it.
func TestReconciliation_StampsSampleErrorBodyAndCountOnSpan(t *testing.T) {
	// Not t.Parallel(): recordSpans swaps the global TracerProvider (see its
	// doc comment in span_test.go).
	fetcher := newFakeReconciliationFetcher()
	fetcher.pageErrs[1] = &planit.HTTPError{StatusCode: http.StatusBadRequest, Body: `{"error":"42703: column does not exist"}`}
	fetcher.pageErrs[2] = &planit.HTTPError{StatusCode: http.StatusBadRequest, Body: `{"error":"second authority's body"}`}
	fetcher.pageErrs[3] = &planit.HTTPError{StatusCode: http.StatusInternalServerError, Body: "boom"}
	apps := newFakeApps()
	state := newFakeStateStore()
	h := newReconciliationHandler(t, fetcher, apps, state, []int{1, 2, 3}, defaultReconciliationOpts())

	spans := recordSpans(t, func() {
		h.Run(context.Background())
	})

	span, ok := spanNamed(spans, "PlanIt reconciliation sweep")
	if !ok {
		t.Fatalf("expected a %q span", "PlanIt reconciliation sweep")
	}
	wantBody := `{"error":"42703: column does not exist"}`
	if v, ok := attrValue(span, "reconciliation.sample_error_body"); !ok || v.AsString() != wantBody {
		t.Errorf("reconciliation.sample_error_body: got %v (ok=%v), want the FIRST authority's body %q", v, ok, wantBody)
	}
	if v, ok := attrValue(span, "reconciliation.error_count"); !ok || v.AsInt64() != 2 {
		t.Errorf("reconciliation.error_count: got %v (ok=%v), want 2 (two 400s; the 500 must not count)", v, ok)
	}
}

// TestReconciliation_NonHTTPErrorLeavesSampleErrorBodyEmpty proves a
// non-PlanIt fetch error (transport failure, not a typed *planit.HTTPError)
// leaves the span's diagnostic attributes at their empty defaults rather than
// panicking or fabricating a body.
func TestReconciliation_NonHTTPErrorLeavesSampleErrorBodyEmpty(t *testing.T) {
	// Not t.Parallel(): recordSpans swaps the global TracerProvider (see its
	// doc comment in span_test.go).
	fetcher := newFakeReconciliationFetcher()
	fetcher.pageErrs[1] = errors.New("planit: transport blew up")
	apps := newFakeApps()
	state := newFakeStateStore()
	h := newReconciliationHandler(t, fetcher, apps, state, []int{1}, defaultReconciliationOpts())

	spans := recordSpans(t, func() {
		h.Run(context.Background())
	})

	span, ok := spanNamed(spans, "PlanIt reconciliation sweep")
	if !ok {
		t.Fatalf("expected a %q span", "PlanIt reconciliation sweep")
	}
	if v, ok := attrValue(span, "reconciliation.sample_error_body"); !ok || v.AsString() != "" {
		t.Errorf("reconciliation.sample_error_body: got %v (ok=%v), want empty", v, ok)
	}
	if v, ok := attrValue(span, "reconciliation.error_count"); !ok || v.AsInt64() != 0 {
		t.Errorf("reconciliation.error_count: got %v (ok=%v), want 0", v, ok)
	}
}

// TestReconciliation_PersistsLastRunDespiteCancelledCtx pins tc-tuge8/GH#971
// root cause 2: the request ctx can already be cancelled (a budget cut-off)
// by the time Run reaches its state save. The save must still land --
// fakeStateStore.Save errors on a cancelled ctx, so this only passes when the
// production code detaches the write via context.WithoutCancel.
func TestReconciliation_PersistsLastRunDespiteCancelledCtx(t *testing.T) {
	t.Parallel()
	fetcher := newFakeReconciliationFetcher()
	apps := newFakeApps()
	state := newFakeStateStore()
	h := newReconciliationHandler(t, fetcher, apps, state, []int{1, 2, 3}, defaultReconciliationOpts())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before Run starts -- a budget cut-off cycle

	h.Run(ctx)

	if len(state.saves) != 1 {
		t.Fatalf("expected the last-run save to persist despite the cancelled request ctx, got %d saves", len(state.saves))
	}
}

// TestReconciliation_HydrationRateLimitStopsHydratingFurtherStragglersInAuthority
// pins tc-mc0hf: once a straggler hydration fetch (FetchByUID) returns a
// *planit.RateLimitError, sweepAuthority must stop hydrating further
// stragglers for the REST of that same authority -- mirroring
// NationalLaneHandler's "stop on first 429" circuit breaker (Lane A/B). The
// fake page carries three never-seen (unconditionally-straggler) uids so the
// break can be distinguished from merely running out of rows: if the loop
// didn't actually stop, uids 2 and 3 would still be hydrated.
func TestReconciliation_HydrationRateLimitStopsHydratingFurtherStragglersInAuthority(t *testing.T) {
	t.Parallel()
	ld := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	fetcher := newFakeReconciliationFetcher()
	fetcher.pages[99] = planit.FetchPageResult{
		Applications: []applications.PlanningApplication{
			lightApp("a/1", 99, "Permitted", ld),
			lightApp("a/2", 99, "Permitted", ld),
			lightApp("a/3", 99, "Permitted", ld),
		},
	}
	fetcher.hydrateErrs["a/1"] = &planit.RateLimitError{}
	apps := newFakeApps()
	state := newFakeStateStore()

	opts := ReconciliationOptions{Interval: 7 * 24 * time.Hour, MaxStragglersPerAuthority: 10, AuthoritiesPerCycle: 100}
	h := newReconciliationHandler(t, fetcher, apps, state, []int{99}, opts)
	out := h.Run(context.Background())

	if !out.rateLimited {
		t.Error("out.rateLimited: got false, want true (FetchByUID returned a *planit.RateLimitError)")
	}
	if len(fetcher.hydrateCalls) != 1 || fetcher.hydrateCalls[0] != "a/1" {
		t.Errorf("hydrateCalls: got %v, want [a/1] only (a/2 and a/3 must not be attempted after the 429)", fetcher.hydrateCalls)
	}
	if out.stragglers != 1 {
		t.Errorf("stragglers: got %d, want 1 (a/2 and a/3 never reach the straggler count once rate-limited)", out.stragglers)
	}
}

// intsEqual compares two int slices for the pageCalls assertions below.
func intsEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestReconciliation_SweepsAtMostAuthoritiesPerCycleAndPersistsNextIndex pins
// the resumable cursor's basic shape: a Run call touches only
// AuthoritiesPerCycle authorities (a fake serving MORE ids than that is the
// only way to prove the bound, not just that every supplied id happened to be
// swept), and persists the next unfetched index -- not lastPollTime, since
// the pass is still mid-flight.
func TestReconciliation_SweepsAtMostAuthoritiesPerCycleAndPersistsNextIndex(t *testing.T) {
	t.Parallel()
	ids := make([]int, 12)
	fetcher := newFakeReconciliationFetcher()
	for i := range ids {
		ids[i] = i + 1
		fetcher.pages[ids[i]] = planit.FetchPageResult{}
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	opts := ReconciliationOptions{Interval: 7 * 24 * time.Hour, MaxStragglersPerAuthority: 10, AuthoritiesPerCycle: 5}
	h := newReconciliationHandler(t, fetcher, apps, state, ids, opts)

	out := h.Run(context.Background())

	if out.authoritiesSwept != 5 {
		t.Errorf("authoritiesSwept: got %d, want 5 (bounded by AuthoritiesPerCycle, 12 ids supplied)", out.authoritiesSwept)
	}
	if !intsEqual(fetcher.pageCalls, []int{1, 2, 3, 4, 5}) {
		t.Errorf("pageCalls: got %v, want [1 2 3 4 5]", fetcher.pageCalls)
	}
	if len(state.saves) != 1 {
		t.Fatalf("expected exactly one state save, got %d", len(state.saves))
	}
	saved := state.saves[0]
	if saved.cursor == nil || saved.cursor.NextIndex != 5 {
		t.Errorf("cursor: got %+v, want NextIndex=5 (mid-pass, 12 authorities > 5 per cycle)", saved.cursor)
	}
	if saved.cursor != nil && (!saved.cursor.DifferentStart.IsZero() || saved.cursor.KnownTotal != nil) {
		t.Errorf("cursor: got DifferentStart=%v KnownTotal=%v, want both zero/nil (Lane A/B pagination concepts, not applicable here)", saved.cursor.DifferentStart, saved.cursor.KnownTotal)
	}
	if !saved.lastPollTime.IsZero() {
		t.Errorf("lastPollTime: got %v, want zero (pass not yet complete, so last-run must not advance)", saved.lastPollTime)
	}
}

// TestReconciliation_ResumesFromPersistedCursorOnSecondRun proves the second
// Run call picks up exactly where the first left off, rather than restarting
// at authority index 0 -- the starvation bug the cursor exists to fix.
func TestReconciliation_ResumesFromPersistedCursorOnSecondRun(t *testing.T) {
	t.Parallel()
	ids := make([]int, 12)
	fetcher := newFakeReconciliationFetcher()
	for i := range ids {
		ids[i] = i + 1
		fetcher.pages[ids[i]] = planit.FetchPageResult{}
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	opts := ReconciliationOptions{Interval: 7 * 24 * time.Hour, MaxStragglersPerAuthority: 10, AuthoritiesPerCycle: 5}
	h := newReconciliationHandler(t, fetcher, apps, state, ids, opts)

	h.Run(context.Background())
	out2 := h.Run(context.Background())

	if out2.authoritiesSwept != 5 {
		t.Errorf("second Run authoritiesSwept: got %d, want 5", out2.authoritiesSwept)
	}
	if !intsEqual(fetcher.pageCalls, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}) {
		t.Errorf("pageCalls across both runs: got %v, want [1..5] then [6..10] (resume, not restart)", fetcher.pageCalls)
	}
	if len(state.saves) != 2 {
		t.Fatalf("expected two state saves, got %d", len(state.saves))
	}
	if c := state.saves[1].cursor; c == nil || c.NextIndex != 10 {
		t.Errorf("second save cursor: got %+v, want NextIndex=10", c)
	}
}

// TestReconciliation_EarlyCtxCancelPersistsActuallyAttemptedCountNotPlannedEnd
// pins the correctness-critical edge case: an early ctx-cancel break inside
// the sweep loop must persist how many authorities were ACTUALLY attempted,
// not the planned AuthoritiesPerCycle slice end -- otherwise the un-attempted
// tail between "actually swept" and "planned end" is skipped forever
// (exactly the starvation bug this cursor exists to fix).
func TestReconciliation_EarlyCtxCancelPersistsActuallyAttemptedCountNotPlannedEnd(t *testing.T) {
	t.Parallel()
	ids := []int{1, 2, 3, 4, 5}
	fetcher := newFakeReconciliationFetcher()
	for _, id := range ids {
		fetcher.pages[id] = planit.FetchPageResult{}
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	opts := ReconciliationOptions{Interval: 7 * 24 * time.Hour, MaxStragglersPerAuthority: 10, AuthoritiesPerCycle: 5} // planned to sweep all 5 this cycle
	h := newReconciliationHandler(t, fetcher, apps, state, ids, opts)

	ctx, cancel := context.WithCancel(context.Background())
	fetcher.cancelAfter = 2
	fetcher.cancel = cancel // simulates a budget cut-off arriving after the 2nd authority

	out := h.Run(ctx)

	if out.authoritiesSwept != 2 {
		t.Fatalf("authoritiesSwept: got %d, want 2 (broke early on ctx cancel)", out.authoritiesSwept)
	}
	if len(state.saves) != 1 {
		t.Fatalf("expected the cursor save to survive the cancelled ctx (context.WithoutCancel), got %d saves", len(state.saves))
	}
	saved := state.saves[0]
	if saved.cursor == nil || saved.cursor.NextIndex != 2 {
		t.Errorf("cursor: got %+v, want NextIndex=2 (actually attempted, NOT the planned slice end of 5)", saved.cursor)
	}
}

// TestReconciliation_RateLimitStopsSweepingFurtherAuthorities pins tc-mc0hf:
// once a per-authority sweep fetch (FetchReconciliationPage) returns a
// *planit.RateLimitError, Run must stop sweeping further authorities for the
// rest of this cycle -- mirroring NationalLaneHandler's "stop on first 429"
// behavior. The fake authority list supplies MORE ids than the point where
// the 429 hits, so a broken loop is distinguishable from one that merely ran
// out of authorities to sweep.
func TestReconciliation_RateLimitStopsSweepingFurtherAuthorities(t *testing.T) {
	t.Parallel()
	ids := []int{1, 2, 3, 4, 5}
	fetcher := newFakeReconciliationFetcher()
	for _, id := range ids {
		fetcher.pages[id] = planit.FetchPageResult{}
	}
	fetcher.pageErrs[2] = &planit.RateLimitError{}
	apps := newFakeApps()
	state := newFakeStateStore()
	opts := ReconciliationOptions{Interval: 7 * 24 * time.Hour, MaxStragglersPerAuthority: 10, AuthoritiesPerCycle: 100}
	h := newReconciliationHandler(t, fetcher, apps, state, ids, opts)

	out := h.Run(context.Background())

	if !out.rateLimited {
		t.Error("out.rateLimited: got false, want true (authority 2's sweep fetch returned a *planit.RateLimitError)")
	}
	if !intsEqual(fetcher.pageCalls, []int{1, 2}) {
		t.Errorf("pageCalls: got %v, want [1 2] (authorities 3, 4, 5 must not be swept after the 429)", fetcher.pageCalls)
	}
	if out.authoritiesSwept != 1 {
		t.Errorf("authoritiesSwept: got %d, want 1 (authority 1 succeeded; authority 2's fetch errored, so it never increments)", out.authoritiesSwept)
	}
}

// TestReconciliation_RateLimitedEarlyExitPersistsActuallyAttemptedCount pins
// the correctness-critical edge case for the rate-limit circuit breaker,
// mirroring TestReconciliation_EarlyCtxCancelPersistsActuallyAttemptedCountNotPlannedEnd
// with a 429 as the trigger instead of ctx-cancel: the persisted cursor must
// reflect how many authorities were ACTUALLY attempted (including the one
// that got rate-limited -- it WAS attempted, just rejected), not the planned
// AuthoritiesPerCycle slice end. This reuses the exact same "persist what was
// actually attempted" mechanism as the ctx-cancel case, not a parallel one.
func TestReconciliation_RateLimitedEarlyExitPersistsActuallyAttemptedCount(t *testing.T) {
	t.Parallel()
	ids := []int{1, 2, 3, 4, 5}
	fetcher := newFakeReconciliationFetcher()
	for _, id := range ids {
		fetcher.pages[id] = planit.FetchPageResult{}
	}
	fetcher.pageErrs[2] = &planit.RateLimitError{}
	apps := newFakeApps()
	state := newFakeStateStore()
	opts := ReconciliationOptions{Interval: 7 * 24 * time.Hour, MaxStragglersPerAuthority: 10, AuthoritiesPerCycle: 5} // planned to sweep all 5 this cycle
	h := newReconciliationHandler(t, fetcher, apps, state, ids, opts)

	h.Run(context.Background())

	if len(state.saves) != 1 {
		t.Fatalf("expected exactly one state save, got %d", len(state.saves))
	}
	saved := state.saves[0]
	if saved.cursor == nil || saved.cursor.NextIndex != 2 {
		t.Errorf("cursor: got %+v, want NextIndex=2 (authorities 1 and 2 actually attempted, NOT the planned slice end of 5)", saved.cursor)
	}
	if !saved.lastPollTime.IsZero() {
		t.Errorf("lastPollTime: got %v, want zero (pass cut short by rate limit, not completed, so last-run must not advance)", saved.lastPollTime)
	}
}

// TestReconciliation_CompletedPassResetsCursorAndStampsLastPollTime proves a
// pass that reaches the end of the authority list resets the cursor to nil
// and stamps LastPollTime -- the only branch that advances it -- and that Due
// then goes false until Interval elapses.
func TestReconciliation_CompletedPassResetsCursorAndStampsLastPollTime(t *testing.T) {
	t.Parallel()
	ids := []int{1, 2, 3}
	fetcher := newFakeReconciliationFetcher()
	for _, id := range ids {
		fetcher.pages[id] = planit.FetchPageResult{}
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	opts := ReconciliationOptions{Interval: 7 * 24 * time.Hour, MaxStragglersPerAuthority: 10, AuthoritiesPerCycle: 10} // whole set fits in one cycle
	h := newReconciliationHandler(t, fetcher, apps, state, ids, opts)

	out := h.Run(context.Background())
	if out.authoritiesSwept != 3 {
		t.Fatalf("authoritiesSwept: got %d, want 3", out.authoritiesSwept)
	}
	if len(state.saves) != 1 {
		t.Fatalf("expected exactly one state save, got %d", len(state.saves))
	}
	saved := state.saves[0]
	if saved.cursor != nil {
		t.Errorf("cursor: got %+v, want nil (pass completed)", saved.cursor)
	}
	wantLastRun := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) // newReconciliationHandler's pinned clock
	if !saved.lastPollTime.Equal(wantLastRun) {
		t.Errorf("lastPollTime: got %v, want %v (pass completed, so last-run advances)", saved.lastPollTime, wantLastRun)
	}

	if h.Due(context.Background(), wantLastRun.Add(time.Hour)) {
		t.Error("Due should be false immediately after a completed pass, within Interval")
	}
	if !h.Due(context.Background(), wantLastRun.Add(8*24*time.Hour)) {
		t.Error("Due should be true once Interval has elapsed since the completed pass")
	}
}

// TestReconciliationDue_TrueMidPassRegardlessOfInterval pins the mid-pass Due
// override: a persisted cursor means a pass is in progress, so the cycle
// continues it unconditionally -- Interval only gates STARTING a new pass.
func TestReconciliationDue_TrueMidPassRegardlessOfInterval(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	opts := ReconciliationOptions{Interval: 7 * 24 * time.Hour, MaxStragglersPerAuthority: 10, AuthoritiesPerCycle: 50}
	fetcher := newFakeReconciliationFetcher()
	apps := newFakeApps()
	state := newFakeStateStore()
	// A mid-pass cursor with a very recent last-run (well within Interval).
	state.states[sentinelLaneC] = PollState{LastPollTime: now.Add(-time.Minute), Cursor: &PollCursor{NextIndex: 30}}
	h := newReconciliationHandler(t, fetcher, apps, state, nil, opts)

	if !h.Due(context.Background(), now) {
		t.Error("Due should be true whenever a pass is mid-flight (cursor present), regardless of Interval")
	}
}

// TestReconciliation_StaleCursorPastEndOfAuthorityListRestartsAtZero covers
// the defensive case: a persisted NextIndex >= len(ids) (e.g. the authority
// list shrank) must be treated as no cursor at all -- a fresh pass at index
// 0 -- rather than sweeping zero authorities forever.
func TestReconciliation_StaleCursorPastEndOfAuthorityListRestartsAtZero(t *testing.T) {
	t.Parallel()
	ids := []int{1, 2, 3}
	fetcher := newFakeReconciliationFetcher()
	for _, id := range ids {
		fetcher.pages[id] = planit.FetchPageResult{}
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[sentinelLaneC] = PollState{Cursor: &PollCursor{NextIndex: 99}} // stale: the authority list shrank
	opts := ReconciliationOptions{Interval: 7 * 24 * time.Hour, MaxStragglersPerAuthority: 10, AuthoritiesPerCycle: 10}
	h := newReconciliationHandler(t, fetcher, apps, state, ids, opts)

	out := h.Run(context.Background())
	if out.authoritiesSwept != 3 {
		t.Errorf("authoritiesSwept: got %d, want 3 (a stale past-end cursor must restart at 0, not skip everything)", out.authoritiesSwept)
	}
}

// TestReconciliationDue covers the interval gate: never-run is due; within
// the interval is not due; past the interval is due again.
func TestReconciliationDue(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	opts := ReconciliationOptions{Interval: 7 * 24 * time.Hour, MaxStragglersPerAuthority: 10}

	tests := []struct {
		name     string
		lastRun  time.Time
		hasState bool
		want     bool
	}{
		{name: "never run", want: true},
		{name: "within interval", lastRun: now.Add(-1 * time.Hour), hasState: true, want: false},
		{name: "past interval", lastRun: now.Add(-8 * 24 * time.Hour), hasState: true, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fetcher := newFakeReconciliationFetcher()
			apps := newFakeApps()
			state := newFakeStateStore()
			if tc.hasState {
				state.states[sentinelLaneC] = PollState{LastPollTime: tc.lastRun}
			}
			h := newReconciliationHandler(t, fetcher, apps, state, nil, opts)
			if got := h.Due(context.Background(), now); got != tc.want {
				t.Errorf("Due: got %v, want %v", got, tc.want)
			}
		})
	}
}
