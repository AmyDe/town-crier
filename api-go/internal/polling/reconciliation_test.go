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

// fakeReconciliationFetcher serves pre-canned light per-authority pages and
// per-uid hydration records.
type fakeReconciliationFetcher struct {
	pages        map[int]planit.FetchPageResult // authorityID -> light page
	pageErrs     map[int]error                  // authorityID -> fetch error
	hydrated     map[string]applications.PlanningApplication
	hydrateErrs  map[string]error
	hydrateCalls []string
}

func newFakeReconciliationFetcher() *fakeReconciliationFetcher {
	return &fakeReconciliationFetcher{
		pages:       map[int]planit.FetchPageResult{},
		pageErrs:    map[int]error{},
		hydrated:    map[string]applications.PlanningApplication{},
		hydrateErrs: map[string]error{},
	}
}

func (f *fakeReconciliationFetcher) FetchReconciliationPage(_ context.Context, authorityID, _ int) (planit.FetchPageResult, error) {
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

func defaultReconciliationOpts() ReconciliationOptions {
	return ReconciliationOptions{Interval: 7 * 24 * time.Hour, MaxStragglersPerAuthority: 10}
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

	h := newReconciliationHandler(t, fetcher, apps, state, []int{99}, ReconciliationOptions{Interval: 7 * 24 * time.Hour, MaxStragglersPerAuthority: 2})
	out := h.Run(context.Background())

	if out.stragglers != 2 {
		t.Errorf("stragglers: got %d, want 2 (capped)", out.stragglers)
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
