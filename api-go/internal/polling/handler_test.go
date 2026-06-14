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
)

// --- hand-written fakes for the handler's dependencies ---

// fakePlanIt serves pre-canned pages keyed by (authorityID, page) and can be
// primed with a rate-limit or transient error per authority.
type fakePlanIt struct {
	pages   map[pageKey]planit.FetchPageResult
	errs    map[int]error // authorityID -> error returned on every page fetch
	fetched []pageKey
	mu      sync.Mutex
}

type pageKey struct {
	authority int
	page      int
}

func newFakePlanIt() *fakePlanIt {
	return &fakePlanIt{pages: map[pageKey]planit.FetchPageResult{}, errs: map[int]error{}}
}

func (f *fakePlanIt) FetchApplicationsPage(_ context.Context, authorityID int, _ *time.Time, page int) (planit.FetchPageResult, error) {
	f.mu.Lock()
	f.fetched = append(f.fetched, pageKey{authorityID, page})
	f.mu.Unlock()
	if err := f.errs[authorityID]; err != nil {
		return planit.FetchPageResult{}, err
	}
	res, ok := f.pages[pageKey{authorityID, page}]
	if !ok {
		return planit.FetchPageResult{Page: page, Applications: nil, HasMorePages: false}, nil
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
}

func newFakeApps() *fakeApps {
	return &fakeApps{existing: map[string]applications.PlanningApplication{}}
}

func (f *fakeApps) GetByUID(_ context.Context, uid, _ string) (applications.PlanningApplication, bool, error) {
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

// planitPage wraps a single application into a one-page, no-more-pages result.
func planitPage(app applications.PlanningApplication) planit.FetchPageResult {
	return planit.FetchPageResult{
		Page:         1,
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
	pi.pages[pageKey{99, 1}] = planit.FetchPageResult{
		Page:         1,
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
	pi.pages[pageKey{99, 1}] = planit.FetchPageResult{Page: 1, Applications: []applications.PlanningApplication{app}}
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
	for p := 1; p <= 5; p++ {
		pi.pages[pageKey{99, p}] = planit.FetchPageResult{Page: p, Applications: full, HasMorePages: true, Total: ptr(500)}
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())

	res, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	// MaxPages = 3, so exactly 3 pages fetched.
	if pi.fetchCount() != 3 {
		t.Errorf("pages fetched: got %d, want 3 (MaxPages cap)", pi.fetchCount())
	}
	if res.TerminationReason != TerminationNatural {
		t.Errorf("cap-hit on a single authority is a natural cycle end: got %v", res.TerminationReason)
	}
	// Cap hit saves a resumable cursor at next unfetched page (4) and freezes HWM.
	if len(state.saves) != 1 {
		t.Fatalf("state saves: got %d, want 1", len(state.saves))
	}
	cur := state.saves[0].cursor
	if cur == nil || cur.NextPage != 4 {
		t.Errorf("cursor: %+v, want NextPage=4", cur)
	}
}

func TestHandler_ResumesFromSavedCursor(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	hwm := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	for p := 1; p <= 5; p++ {
		pi.pages[pageKey{99, p}] = planit.FetchPageResult{Page: p, Applications: []applications.PlanningApplication{testApp("a", 99, hwm)}, HasMorePages: false}
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	// Existing state: HWM at 2026-06-13, cursor pointing at page 4 against that date.
	state.states[99] = PollState{
		LastPollTime:  hwm,
		HighWaterMark: hwm,
		Cursor:        &PollCursor{DifferentStart: hwm, NextPage: 4, KnownTotal: ptr(500)},
	}
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())

	if _, err := h.Handle(context.Background()); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	// Resume overlaps by one page: start at max(1, NextPage-1) = 3.
	if len(pi.fetched) == 0 || pi.fetched[0].page != 3 {
		t.Errorf("first fetched page: got %+v, want page 3 (resume with -1 overlap)", pi.fetched)
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
	pi.pages[pageKey{200, 1}] = planit.FetchPageResult{Page: 1, Applications: []applications.PlanningApplication{testApp("ok", 200, ld)}}
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
