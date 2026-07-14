package polling

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// --- hand-written fakes for the handler's dependencies ---

// fakePlanIt serves pre-canned pages keyed by (authorityID, page) and can be
// primed with a rate-limit or transient error per authority — either on every
// fetch (errs) or on one specific fetch ordinal (nthErrs, via failNthFetch).
type fakePlanIt struct {
	pages   map[pageKey]planit.FetchPageResult
	errs    map[int]error      // authorityID -> error returned on every page fetch
	nthErrs map[int]nthFailure // authorityID -> error returned on one fetch only
	calls   map[int]int        // authorityID -> fetches served so far
	fetched []pageKey
	mu      sync.Mutex
}

// nthFailure primes a single failing fetch: err is returned on the nth fetch
// (1-based, counting every fetch for that authority including the freshness
// probe) and every other fetch is served normally. Models a PlanIt transport
// timeout part-way through a drain.
type nthFailure struct {
	n   int
	err error
}

// pageKey identifies one fake fetch: the authority, the requested 0-based
// record index, and the sort direction (descending distinguishes the
// freshness probe's newest-first fetch from an ascending drain fetch that
// happens to request the same index — e.g. a resume that overlaps back to 0).
type pageKey struct {
	authority  int
	index      int
	descending bool
}

func newFakePlanIt() *fakePlanIt {
	return &fakePlanIt{
		pages:   map[pageKey]planit.FetchPageResult{},
		errs:    map[int]error{},
		nthErrs: map[int]nthFailure{},
		calls:   map[int]int{},
	}
}

// failNthFetch primes the fake to return err on the nth fetch for authorityID
// (1-based, counting the freshness probe), serving every other fetch normally.
func (f *fakePlanIt) failNthFetch(authorityID, n int, err error) {
	f.nthErrs[authorityID] = nthFailure{n: n, err: err}
}

func (f *fakePlanIt) FetchApplicationsPage(_ context.Context, authorityID int, _ *time.Time, startIndex int, descending bool) (planit.FetchPageResult, error) {
	key := pageKey{authority: authorityID, index: startIndex, descending: descending}
	f.mu.Lock()
	f.fetched = append(f.fetched, key)
	f.calls[authorityID]++
	nth := f.calls[authorityID]
	f.mu.Unlock()
	if err := f.errs[authorityID]; err != nil {
		return planit.FetchPageResult{}, err
	}
	if fail, ok := f.nthErrs[authorityID]; ok && fail.n == nth {
		return planit.FetchPageResult{}, fail.err
	}
	res, ok := f.pages[key]
	if !ok {
		return planit.FetchPageResult{From: startIndex, Applications: nil, HasMorePages: false}, nil
	}
	return res, nil
}

func (f *fakePlanIt) fetchCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.fetched)
}

// fakeApps records upserts and answers existence point reads.
type fakeApps struct {
	existing  map[string]applications.PlanningApplication // uid -> existing record
	upserts   []applications.PlanningApplication
	upsertErr error
	getErr    error
}

func newFakeApps() *fakeApps {
	return &fakeApps{existing: map[string]applications.PlanningApplication{}}
}

func (f *fakeApps) GetByUID(_ context.Context, uid, _ string) (applications.PlanningApplication, bool, error) {
	if f.getErr != nil {
		return applications.PlanningApplication{}, false, f.getErr
	}
	a, ok := f.existing[uid]
	return a, ok, nil
}

func (f *fakeApps) Upsert(_ context.Context, a applications.PlanningApplication) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.upserts = append(f.upserts, a)
	return nil
}

// fakeStateStore records saves and answers gets / LRU ordering.
type fakeStateStore struct {
	states   map[int]PollState
	saves    []savedState
	lruOrder []int
	lruErr   error
}

type savedState struct {
	authorityID   int
	lastPollTime  time.Time
	highWaterMark time.Time
	cursor        *PollCursor
}

func newFakeStateStore() *fakeStateStore {
	return &fakeStateStore{states: map[int]PollState{}}
}

func (f *fakeStateStore) Get(_ context.Context, authorityID int) (PollState, bool, error) {
	s, ok := f.states[authorityID]
	return s, ok, nil
}

func (f *fakeStateStore) Save(_ context.Context, authorityID int, lastPollTime, highWaterMark time.Time, cursor *PollCursor) error {
	f.saves = append(f.saves, savedState{authorityID, lastPollTime, highWaterMark, cursor})
	f.states[authorityID] = PollState{LastPollTime: lastPollTime, HighWaterMark: highWaterMark, Cursor: cursor}
	return nil
}

func (f *fakeStateStore) GetLeastRecentlyPolled(_ context.Context, candidates []int) (LeastRecentlyPolledResult, error) {
	if f.lruErr != nil {
		return LeastRecentlyPolledResult{}, f.lruErr
	}
	order := f.lruOrder
	if order == nil {
		order = candidates
	}
	return LeastRecentlyPolledResult{AuthorityIDs: order, NeverPolledCount: 0}, nil
}

// fakeAuthorities returns a fixed active set.
type fakeAuthorities struct {
	ids []int
	err error
}

func (f fakeAuthorities) ActiveAuthorityIDs(context.Context) ([]int, error) { return f.ids, f.err }

// fakeCycle reports a fixed cycle type.
type fakeCycle struct{ cycle CycleType }

func (f fakeCycle) Current() CycleType { return f.cycle }

func testApp(name string, areaID int, lastDifferent time.Time) applications.PlanningApplication {
	state := "Undecided"
	return applications.PlanningApplication{
		Name:          name,
		UID:           name + "/FUL",
		AreaName:      "Area",
		AreaID:        areaID,
		Address:       "1 St",
		Description:   "d",
		AppState:      &state,
		LastDifferent: lastDifferent,
	}
}

func newHandler(t *testing.T, pi *fakePlanIt, apps *fakeApps, state *fakeStateStore, auth fakeAuthorities, cycle CycleType, opts HandlerOptions) *PollPlanItHandler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(discard{}, nil))
	clock := func() time.Time { return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC) }
	return NewPollPlanItHandler(pi, apps, state, auth, fakeCycle{cycle}, opts, clock, logger)
}

// newHandlerWithFanOut builds a handler wired with the per-app decision
// dispatcher and zone enqueuer, used by the fan-out tests.
func newHandlerWithFanOut(t *testing.T, pi *fakePlanIt, apps *fakeApps, state *fakeStateStore, auth fakeAuthorities, opts HandlerOptions, disp DecisionDispatcher, enq NotificationEnqueuer) *PollPlanItHandler {
	t.Helper()
	h := newHandler(t, pi, apps, state, auth, CycleSeed, opts)
	h.WithFanOut(disp, enq)
	return h
}

// planitPage wraps a single application into a one-fetch, no-more-pages result
// starting at index 0.
func planitPage(app applications.PlanningApplication) planit.FetchPageResult {
	return planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{app},
		HasMorePages: false,
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

func defaultHandlerOpts() HandlerOptions {
	max := 3
	return HandlerOptions{MaxPagesPerAuthorityPerCycle: &max, HandlerBudget: 4 * time.Minute}
}

// --- tests ---

func TestHandler_IngestsAndUpsertsNewApplications(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	pi.pages[pageKey{authority: 99, index: 0}] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{testApp("24/0001", 99, ld)},
		HasMorePages: false,
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())

	res, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if res.ApplicationCount != 1 || res.AuthoritiesPolled != 1 {
		t.Errorf("counts: apps=%d authorities=%d", res.ApplicationCount, res.AuthoritiesPolled)
	}
	if len(apps.upserts) != 1 {
		t.Errorf("upserts: got %d, want 1", len(apps.upserts))
	}
	if res.TerminationReason != TerminationNatural {
		t.Errorf("termination: got %v, want Natural", res.TerminationReason)
	}
	// Natural end advances HWM to the max LastDifferent observed and clears cursor.
	if len(state.saves) != 1 || !state.saves[0].highWaterMark.Equal(ld) || state.saves[0].cursor != nil {
		t.Errorf("state save: %+v", state.saves)
	}
}

func TestHandler_SkipsUpsertWhenBusinessFieldsUnchanged(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	app := testApp("24/0001", 99, ld)
	pi.pages[pageKey{authority: 99, index: 0}] = planit.FetchPageResult{From: 0, Applications: []applications.PlanningApplication{app}}
	apps := newFakeApps()
	// An identical record already exists (only LastDifferent would differ).
	existing := app
	existing.LastDifferent = ld.Add(-time.Hour)
	apps.existing[app.UID] = existing
	state := newFakeStateStore()
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())

	res, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(apps.upserts) != 0 {
		t.Errorf("unchanged business fields must skip upsert, got %d upserts", len(apps.upserts))
	}
	// The application was still seen, so it counts toward the cycle's app count.
	if res.ApplicationCount != 1 {
		t.Errorf("application count: got %d, want 1 (seen but not upserted)", res.ApplicationCount)
	}
}

func TestHandler_CapsPagesAndSavesResumableCursor(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	// Every page is full (HasMorePages true) so only the MaxPages cap stops it.
	full := make([]applications.PlanningApplication, 100)
	for i := range full {
		full[i] = testApp("app", 99, ld)
	}
	// Fetches at index 0, 100, 200, 300, 400 (each returns a full 100-record page).
	for i := range 5 {
		idx := i * 100
		pi.pages[pageKey{authority: 99, index: idx}] = planit.FetchPageResult{From: idx, Applications: full, HasMorePages: true, Total: platform.Ptr(500)}
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())

	res, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	// MaxPages = 3, so exactly 3 fetches.
	if pi.fetchCount() != 3 {
		t.Errorf("fetches: got %d, want 3 (MaxPages cap)", pi.fetchCount())
	}
	if res.TerminationReason != TerminationNatural {
		t.Errorf("cap-hit on a single authority is a natural cycle end: got %v", res.TerminationReason)
	}
	// Cap hit saves a resumable cursor at the next unfetched index: 3 fetches of
	// 100 records starting at 0 land at from(200)+records(100) = 300.
	if len(state.saves) != 1 {
		t.Fatalf("state saves: got %d, want 1", len(state.saves))
	}
	cur := state.saves[0].cursor
	if cur == nil || cur.NextIndex != 300 {
		t.Errorf("cursor: %+v, want NextIndex=300", cur)
	}
}

func TestHandler_ResumesFromSavedCursor(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	hwm := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	pi.pages[pageKey{authority: 99, index: 300}] = planit.FetchPageResult{From: 300, Applications: []applications.PlanningApplication{testApp("a", 99, hwm)}, HasMorePages: false}
	apps := newFakeApps()
	state := newFakeStateStore()
	// Existing state: HWM at 2026-06-13, cursor pointing at record index 400
	// against that date.
	state.states[99] = PollState{
		LastPollTime:  hwm,
		HighWaterMark: hwm,
		Cursor:        &PollCursor{DifferentStart: hwm, NextIndex: 400, KnownTotal: platform.Ptr(500)},
	}
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())

	if _, err := h.Handle(context.Background()); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	// An active cursor fires the freshness probe first (index 0, descending),
	// then the ascending drain resumes overlapping by 100 records: start at
	// max(0, 400-100) = 300.
	if len(pi.fetched) != 2 {
		t.Fatalf("fetched: got %+v, want 2 (probe + drain)", pi.fetched)
	}
	if !pi.fetched[0].descending || pi.fetched[0].index != 0 {
		t.Errorf("probe fetch: got %+v, want index=0 descending=true", pi.fetched[0])
	}
	if pi.fetched[1].descending || pi.fetched[1].index != 300 {
		t.Errorf("drain fetch: got %+v, want index=300 descending=false (resume with -100 overlap)", pi.fetched[1])
	}
}

// TestHandler_CapHitSavesNextIndexAsFromPlusRecords pins the acceptance
// criterion directly: a single-fetch cap hit (MaxPages=1, HasMorePages=true)
// must persist NextIndex == from+records, independent of the multi-fetch
// scenario TestHandler_CapsPagesAndSavesResumableCursor already covers.
func TestHandler_CapHitSavesNextIndexAsFromPlusRecords(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	pi.pages[pageKey{authority: 99, index: 0}] = planit.FetchPageResult{
		From:         0,
		Applications: []applications.PlanningApplication{testApp("a", 99, ld), testApp("b", 99, ld)},
		HasMorePages: true,
		Total:        platform.Ptr(500),
	}
	one := 1
	opts := HandlerOptions{MaxPagesPerAuthorityPerCycle: &one, HandlerBudget: 4 * time.Minute}
	apps := newFakeApps()
	state := newFakeStateStore()
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, opts)

	if _, err := h.Handle(context.Background()); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(state.saves) != 1 {
		t.Fatalf("state saves: got %d, want 1", len(state.saves))
	}
	cur := state.saves[0].cursor
	if cur == nil || cur.NextIndex != 2 {
		t.Errorf("cursor: %+v, want NextIndex=2 (from=0 + records=2)", cur)
	}
}

// TestHandler_LegacyCursorNextPageResumesViaIndexFallback pins the second
// resume acceptance criterion: a legacy cursor with only NextIndex populated
// via the store's (cursor_next_page-1)*100 fallback (store_postgres_test.go /
// store_postgres_integration_test.go cover the store-level conversion; this
// test proves the HANDLER applies the same resumeOverlapRecords maths to a
// cursor regardless of whether it originated from the legacy column) resumes
// at index (19-1)*100 - 100 = 1700.
func TestHandler_LegacyCursorNextPageResumesViaIndexFallback(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	hwm := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	pi.pages[pageKey{authority: 99, index: 1700}] = planit.FetchPageResult{From: 1700, Applications: nil, HasMorePages: false}
	apps := newFakeApps()
	state := newFakeStateStore()
	// The store layer is responsible for converting a legacy cursor_next_page
	// into NextIndex = (19-1)*100 = 1800 on read; the fake here stands in for
	// that already-converted value, so this test isolates the handler's resume
	// maths from the store's conversion.
	state.states[99] = PollState{
		LastPollTime:  hwm,
		HighWaterMark: hwm,
		Cursor:        &PollCursor{DifferentStart: hwm, NextIndex: 1800},
	}
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())

	if _, err := h.Handle(context.Background()); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	// The active cursor also fires the probe (index 0, descending) first.
	if len(pi.fetched) != 2 {
		t.Fatalf("fetched: got %+v, want 2 (probe + drain)", pi.fetched)
	}
	if pi.fetched[1].descending || pi.fetched[1].index != 1700 {
		t.Errorf("drain fetch: got %+v, want index 1700 ((19-1)*100 - 100)", pi.fetched[1])
	}
}

func TestHandler_RateLimitStopsCycleAndCarriesRetryAfter(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	ra := 90 * time.Second
	pi.errs[99] = &planit.RateLimitError{RetryAfter: &ra}
	apps := newFakeApps()
	state := newFakeStateStore()
	// Two authorities; the first 429s and the cycle must stop before the second.
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99, 200}}, CycleSeed, defaultHandlerOpts())

	res, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if res.TerminationReason != TerminationRateLimited || !res.RateLimited {
		t.Errorf("termination: got %v rateLimited=%v, want RateLimited", res.TerminationReason, res.RateLimited)
	}
	if res.RetryAfter == nil || *res.RetryAfter != ra {
		t.Errorf("retry-after: got %v, want %v", res.RetryAfter, ra)
	}
	// The second authority (200) must NOT be fetched after the 429.
	for _, k := range pi.fetched {
		if k.authority == 200 {
			t.Errorf("authority 200 should not be polled after a 429: %+v", pi.fetched)
		}
	}
}

func TestHandler_AuthorityErrorIsCountedAndCycleContinues(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	pi.errs[99] = errors.New("planit 500 exhausted")
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	pi.pages[pageKey{authority: 200, index: 0}] = planit.FetchPageResult{From: 0, Applications: []applications.PlanningApplication{testApp("ok", 200, ld)}}
	apps := newFakeApps()
	state := newFakeStateStore()
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99, 200}}, CycleSeed, defaultHandlerOpts())

	res, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if res.AuthorityErrors != 1 {
		t.Errorf("authority errors: got %d, want 1", res.AuthorityErrors)
	}
	// The cycle continues past the erroring authority and ingests the second.
	if res.ApplicationCount != 1 {
		t.Errorf("application count: got %d, want 1 (second authority succeeded)", res.ApplicationCount)
	}
	if res.TerminationReason != TerminationNatural {
		t.Errorf("termination: got %v, want Natural", res.TerminationReason)
	}
}

func TestHandler_NoActiveAuthoritiesIsCleanEmptyCycle(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	h := newHandler(t, pi, newFakeApps(), newFakeStateStore(), fakeAuthorities{ids: []int{}}, CycleWatched, defaultHandlerOpts())

	res, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if res.ApplicationCount != 0 || res.AuthorityErrors != 0 || res.TerminationReason != TerminationNatural {
		t.Errorf("empty cycle: %+v", res)
	}
	if pi.fetchCount() != 0 {
		t.Errorf("no authorities means no PlanIt calls, got %d", pi.fetchCount())
	}
}

// TestHandler_RecordsOldestHWMAgeOnResult pins tc-3jx8d: the staleness of the
// oldest candidate authority's high-water mark must land on the returned
// result (not just the OTel metrics registry, which never reaches App
// Insights) so runPollSB can stamp it on the "Polling Cycle (SB)" span.
func TestHandler_RecordsOldestHWMAgeOnResult(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	apps := newFakeApps()
	state := newFakeStateStore()
	// newHandler pins the clock to 2026-06-14T12:00:00Z; four days earlier.
	lastPoll := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	state.states[99] = PollState{LastPollTime: lastPoll}
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())

	res, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if res.OldestHWMAgeSeconds == nil {
		t.Fatal("expected OldestHWMAgeSeconds to be set")
	}
	wantAge := 4 * 24 * time.Hour
	if *res.OldestHWMAgeSeconds != wantAge.Seconds() {
		t.Errorf("OldestHWMAgeSeconds: got %v, want %v", *res.OldestHWMAgeSeconds, wantAge.Seconds())
	}
	if res.OldestHWMNeverPolled {
		t.Error("OldestHWMNeverPolled: got true, want false (authority has a PollState)")
	}
}

// TestHandler_RecordsOldestHWMNeverPolledOnResult covers the never-polled
// candidate: no PollState means the age is measured from the Unix epoch and
// the result must flag never_polled so dashboards can distinguish it from a
// genuinely stale high-water mark.
func TestHandler_RecordsOldestHWMNeverPolledOnResult(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	apps := newFakeApps()
	state := newFakeStateStore() // no PollState for authority 99
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())

	res, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if res.OldestHWMAgeSeconds == nil {
		t.Fatal("expected OldestHWMAgeSeconds to be set even when never polled")
	}
	if !res.OldestHWMNeverPolled {
		t.Error("OldestHWMNeverPolled: got false, want true (no PollState)")
	}
}

// TestHandler_OmitsOldestHWMWhenNoActiveAuthorities covers the empty
// candidate set: nothing to report, so the field stays nil rather than
// reporting a misleading zero.
func TestHandler_OmitsOldestHWMWhenNoActiveAuthorities(t *testing.T) {
	t.Parallel()
	h := newHandler(t, newFakePlanIt(), newFakeApps(), newFakeStateStore(), fakeAuthorities{ids: []int{}}, CycleWatched, defaultHandlerOpts())

	res, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if res.OldestHWMAgeSeconds != nil {
		t.Errorf("OldestHWMAgeSeconds: got %v, want nil for an empty candidate set", *res.OldestHWMAgeSeconds)
	}
}

// fullPage builds a 100-record page starting at index from, every record
// carrying lastDifferent, with more pages behind it — the shape of a mid-drain
// PlanIt page.
func fullPage(from int, lastDifferent time.Time, total int) planit.FetchPageResult {
	apps := make([]applications.PlanningApplication, 100)
	for i := range apps {
		apps[i] = testApp("app", 99, lastDifferent)
	}
	return planit.FetchPageResult{From: from, Applications: apps, HasMorePages: true, Total: platform.Ptr(total)}
}

// TestHandler_MidDrainFetchErrorFreezesHWMAndSavesCursor pins the GH#958 fix: a
// PlanIt fetch error part-way through the drain must be treated as an early stop
// (like a cap hit or a 429), not as a natural end of window. The HWM stays frozen
// at the existing value and a resumable cursor is persisted at the next unfetched
// record index, so the next visit resumes the drain instead of restarting the same
// window from index 0 (which also silently disabled the freshness probe).
func TestHandler_MidDrainFetchErrorFreezesHWMAndSavesCursor(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	hwm := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	// Drain records sit inside the HWM's own churn day, exactly as on a large
	// backlog: advancing the HWM to them would not move the date on at all.
	ld := hwm.Add(9 * time.Hour)
	pi.pages[pageKey{authority: 99, index: 0}] = fullPage(0, ld, 500)
	pi.pages[pageKey{authority: 99, index: 100}] = fullPage(100, ld, 500)
	// The third drain fetch times out after the first two ingested their records.
	pi.failNthFetch(99, 3, errors.New("planit timeout after retries"))

	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[99] = PollState{LastPollTime: hwm, HighWaterMark: hwm}
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())

	res, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if res.AuthorityErrors != 1 {
		t.Errorf("AuthorityErrors: got %d, want 1", res.AuthorityErrors)
	}
	if res.ApplicationCount != 200 {
		t.Errorf("ApplicationCount: got %d, want 200 (two ingested fetches)", res.ApplicationCount)
	}
	if len(state.saves) != 1 {
		t.Fatalf("state saves: got %d, want 1", len(state.saves))
	}
	save := state.saves[0]
	if !save.highWaterMark.Equal(hwm) {
		t.Errorf("HWM: got %v, want %v (frozen — a fetch error is not a completed window)", save.highWaterMark, hwm)
	}
	if !save.lastPollTime.Equal(time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("LastPollTime: got %v, want the pinned now (the scheduler must still rotate off)", save.lastPollTime)
	}
	cur := save.cursor
	if cur == nil {
		t.Fatal("cursor: got nil, want a resumable cursor at the next unfetched index")
	}
	if cur.NextIndex != 200 {
		t.Errorf("cursor NextIndex: got %d, want 200 (from=100 + records=100 of the last SUCCESSFUL fetch)", cur.NextIndex)
	}
	if cur.KnownTotal == nil || *cur.KnownTotal != 500 {
		t.Errorf("cursor KnownTotal: got %v, want 500 (preserved)", cur.KnownTotal)
	}
	if !cur.DifferentStart.Equal(hwm) {
		t.Errorf("cursor DifferentStart: got %v, want %v", cur.DifferentStart, hwm)
	}
}

// TestHandler_DrainErrorAfterProbeSavesCursorAtResumeIndex covers the probe-then-
// error path: the probe ingested records (so the authority is past finishAuthority's
// nothing-to-write early return) and the first drain fetch then failed. No drain
// fetch succeeded, so the cursor must be persisted at the resume index it started
// from — never cleared, which would destroy the drain progress of every prior visit.
func TestHandler_DrainErrorAfterProbeSavesCursorAtResumeIndex(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	hwm := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	probeLD := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	pi.pages[pageKey{authority: 99, index: 0, descending: true}] = planit.FetchPageResult{
		From: 0, Applications: []applications.PlanningApplication{testApp("newest", 99, probeLD)}, HasMorePages: false,
	}
	// Fetch 1 is the probe; fetch 2 is the first drain fetch (resume index 300).
	pi.failNthFetch(99, 2, errors.New("planit timeout after retries"))

	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[99] = PollState{
		LastPollTime: hwm, HighWaterMark: hwm,
		Cursor: &PollCursor{DifferentStart: hwm, NextIndex: 400, KnownTotal: platform.Ptr(500)},
	}
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())

	res, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if res.AuthorityErrors != 1 {
		t.Errorf("AuthorityErrors: got %d, want 1", res.AuthorityErrors)
	}
	if len(state.saves) != 1 {
		t.Fatalf("state saves: got %d, want 1", len(state.saves))
	}
	save := state.saves[0]
	if !save.highWaterMark.Equal(hwm) {
		t.Errorf("HWM: got %v, want %v (frozen)", save.highWaterMark, hwm)
	}
	if save.cursor == nil {
		t.Fatal("cursor: got nil (cleared), want it preserved at the resume index")
	}
	if save.cursor.NextIndex != 300 {
		t.Errorf("cursor NextIndex: got %d, want 300 (the resume index; the failing fetch contributed nothing)", save.cursor.NextIndex)
	}
}

// TestHandler_ErroredVisitSpanReportsHWMNotAdvanced pins the telemetry half of
// GH#958: the visit that stopped on a fetch error must report
// polling.hwm_advanced=false, so a starving authority is visible on the poll
// dashboard instead of masquerading as a cleanly drained window.
// Deliberately not t.Parallel(): recordSpans mutates the global TracerProvider.
func TestHandler_ErroredVisitSpanReportsHWMNotAdvanced(t *testing.T) {
	pi := newFakePlanIt()
	hwm := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	ld := hwm.Add(9 * time.Hour)
	pi.pages[pageKey{authority: 99, index: 0}] = fullPage(0, ld, 500)
	pi.pages[pageKey{authority: 99, index: 100}] = fullPage(100, ld, 500)
	pi.failNthFetch(99, 3, errors.New("planit timeout after retries"))

	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[99] = PollState{LastPollTime: hwm, HighWaterMark: hwm}
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())

	spans := recordSpans(t, func() {
		if _, err := h.Handle(context.Background()); err != nil {
			t.Fatalf("Handle: %v", err)
		}
	})
	span, ok := spanNamed(spans, "PlanIt authority poll")
	if !ok {
		t.Fatalf("expected a %q span among %d recorded spans", "PlanIt authority poll", len(spans))
	}
	if v, ok := attrValue(span, "polling.hwm_advanced"); !ok || v.AsBool() {
		t.Errorf("polling.hwm_advanced: got %v (ok=%v), want false (a fetch error freezes the HWM)", v, ok)
	}
	if v, ok := attrValue(span, "polling.cap_hit"); !ok || v.AsBool() {
		t.Errorf("polling.cap_hit: got %v (ok=%v), want false (the drain stopped on an error, not the cap)", v, ok)
	}
	if v, ok := attrValue(span, "polling.next_index"); !ok || v.AsInt64() != 200 {
		t.Errorf("polling.next_index: got %v (ok=%v), want 200", v, ok)
	}
}

func TestHandler_CancelledContextTerminatesTimeBounded(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	apps := newFakeApps()
	state := newFakeStateStore()
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99, 200}}, CycleSeed, defaultHandlerOpts())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before the loop starts

	res, err := h.Handle(ctx)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if res.TerminationReason != TerminationTimeBounded {
		t.Errorf("termination: got %v, want TimeBounded for a cancelled context", res.TerminationReason)
	}
	if pi.fetchCount() != 0 {
		t.Errorf("cancelled context must not fetch any pages, got %d", pi.fetchCount())
	}
}
