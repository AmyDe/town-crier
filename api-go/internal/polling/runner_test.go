package polling

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

type runWorld struct {
	now      time.Time
	windows  []WindowState
	deltaAt  map[planit.Work]time.Time
	calls    int
	events   []string
	reads    []PlannedWork
	script   func(w *runWorld, item PlannedWork) WindowReadResult
	pageCost time.Duration
}

func (w *runWorld) clock() time.Time { return w.now }

func (w *runWorld) Load(context.Context, time.Time, time.Time) ([]WindowState, error) {
	return slices.Clone(w.windows), nil
}

func (w *runWorld) LatestPageZeroAt(_ context.Context, work planit.Work) (*time.Time, error) {
	if at, ok := w.deltaAt[work]; ok {
		return &at, nil
	}
	return nil, nil
}

func (w *runWorld) CallsToday(context.Context, time.Time) (int, error) { return w.calls, nil }

func (w *runWorld) read(item PlannedWork) WindowReadResult {
	w.reads = append(w.reads, item)
	w.now = w.now.Add(w.pageCost)
	if w.script != nil {
		return w.script(w, item)
	}
	return w.complete(item)
}

func (w *runWorld) complete(item PlannedWork) WindowReadResult {
	if item.DifferentStart != nil {
		w.deltaAt[item.Work] = w.now
		return WindowReadResult{Pages: 2}
	}
	at := w.now
	for i := range w.windows {
		if w.windows[i].Axis == item.Axis && w.windows[i].Day.Equal(item.Day) {
			w.windows[i].LastCompleteAt = &at
			return WindowReadResult{Pages: 1, Complete: true}
		}
	}
	w.windows = append(w.windows, WindowState{Axis: item.Axis, Day: item.Day, LastCompleteAt: &at})
	return WindowReadResult{Pages: 1, Complete: true}
}

func (w *runWorld) ReadWindow(_ context.Context, item PlannedWork, _ WindowState) WindowReadResult {
	return w.read(item)
}

func (w *runWorld) ReadDelta(_ context.Context, item PlannedWork) WindowReadResult {
	return w.read(item)
}

func (w *runWorld) allWindowsComplete() {
	today := time.Date(w.now.In(budgetLocation).Year(), w.now.In(budgetLocation).Month(), w.now.In(budgetLocation).Day(), 0, 0, 0, 0, time.UTC)
	at := w.now.Add(-time.Hour)
	for age := 0; age <= coverageBandMaxAge; age++ {
		for _, axis := range []planit.Axis{planit.AxisStart, planit.AxisDecided} {
			w.windows = append(w.windows, completeAt(axis, today.AddDate(0, 0, -age), at))
		}
	}
}

type fakeRunLease struct {
	acquire  LeaseAcquireResult
	err      error
	acquired int
	released int
	ttl      time.Duration
}

func (f *fakeRunLease) TryAcquire(_ context.Context, ttl time.Duration) (LeaseAcquireResult, error) {
	f.acquired++
	f.ttl = ttl
	return f.acquire, f.err
}

func (f *fakeRunLease) Release(context.Context, LeaseHandle) LeaseReleaseOutcome {
	f.released++
	return LeaseReleased
}

type fakeRunFlusher struct {
	w      *runWorld
	resets int
	flush  int
}

func (f *fakeRunFlusher) Reset() { f.resets++; f.w.events = append(f.w.events, "reset") }
func (f *fakeRunFlusher) Flush(context.Context) error {
	f.flush++
	f.w.events = append(f.w.events, "flush")
	return nil
}

type fakeCounters struct{ c RunCounts }

func (f *fakeCounters) Counts() RunCounts { return f.c }

type fakeHealthSource struct {
	in    HealthInputs
	err   error
	calls int
}

func (f *fakeHealthSource) Inputs(context.Context, time.Time) (HealthInputs, error) {
	f.calls++
	return f.in, f.err
}

// fakeRunSwitch is on for the first onChecks checks (all checks when negative).
type fakeRunSwitch struct {
	onChecks int
	err      error
	checks   int
}

func (f *fakeRunSwitch) Enabled(context.Context) (bool, error) {
	f.checks++
	if f.err != nil {
		return false, f.err
	}
	return f.onChecks < 0 || f.checks <= f.onChecks, nil
}

type fakeOracleRun struct {
	w     *runWorld
	calls int
	out   OracleOutcome
	err   error
}

func (f *fakeOracleRun) Run(context.Context, time.Time) (OracleOutcome, error) {
	f.calls++
	f.w.events = append(f.w.events, "oracle")
	return f.out, f.err
}

type runnerRig struct {
	world    *runWorld
	lease    *fakeRunLease
	flusher  *fakeRunFlusher
	counters *fakeCounters
	health   *fakeHealthSource
	sw       *fakeRunSwitch
	runner   *Runner
}

func newRunnerRig(t *testing.T, now time.Time, oracle *fakeOracleRun) *runnerRig {
	t.Helper()
	w := &runWorld{now: now, deltaAt: map[planit.Work]time.Time{}}
	r := &runnerRig{
		world:    w,
		lease:    &fakeRunLease{acquire: LeaseAcquireResult{Acquired: true}},
		flusher:  &fakeRunFlusher{w: w},
		counters: &fakeCounters{},
		health:   &fakeHealthSource{},
		sw:       &fakeRunSwitch{onChecks: -1},
	}
	cfg := RunnerConfig{
		Plan:      plannerTestConfig(),
		RunBudget: 55 * time.Minute,
	}
	deps := RunnerDeps{
		Lease: r.lease, State: w, Calls: w, Reader: w, Counters: r.counters,
		Push: r.flusher, Flusher: r.flusher, Health: r.health, Switch: r.sw,
		Now: w.clock, Log: slog.New(slog.DiscardHandler),
	}
	if oracle != nil {
		oracle.w = w
		deps.Oracle = oracle
	}
	r.runner = NewRunner(cfg, deps)
	return r
}

func TestRunner_LeaseHeldExitsWithoutWork(t *testing.T) {
	t.Parallel()
	r := newRunnerRig(t, londonAt(6, 10, 20, 0), nil)
	r.lease.acquire = LeaseAcquireResult{Held: true}

	res, err := r.runner.Run(context.Background())

	if err != nil || res.Acquired {
		t.Fatalf("res = %+v err = %v", res, err)
	}
	if len(r.world.reads) != 0 || r.flusher.flush != 0 || r.flusher.resets != 0 || r.lease.released != 0 {
		t.Fatalf("held lease must do nothing: reads=%d flush=%d resets=%d released=%d", len(r.world.reads), r.flusher.flush, r.flusher.resets, r.lease.released)
	}
}

func TestRunner_SwitchedOffMakesNoPlanItCalls(t *testing.T) {
	t.Parallel()
	oracle := &fakeOracleRun{}
	r := newRunnerRig(t, londonAt(6, 10, 20, 0), oracle)
	r.runner.cfg.OracleEnabled = true
	r.sw.onChecks = 0

	res, err := r.runner.Run(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	if res.Stop != StopDisabled || len(r.world.reads) != 0 || oracle.calls != 0 {
		t.Fatalf("stop = %s reads = %d oracle = %d", res.Stop, len(r.world.reads), oracle.calls)
	}
	if r.flusher.flush != 1 {
		t.Fatalf("flush = %d, want pending events still delivered", r.flusher.flush)
	}
	if r.health.calls != 0 || res.Health.Level != "" {
		t.Fatalf("health computed for a disabled run: %+v", res.Health)
	}
}

func TestRunner_SwitchIsCheckedBeforeEveryItem(t *testing.T) {
	t.Parallel()
	r := newRunnerRig(t, londonAt(6, 10, 20, 0), nil)
	r.sw.onChecks = 2

	res, err := r.runner.Run(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	if res.Stop != StopDisabled || len(r.world.reads) != 2 {
		t.Fatalf("stop = %s reads = %d, want disabled after 2 reads", res.Stop, len(r.world.reads))
	}
}

func TestRunner_SwitchReadErrorFailsClosed(t *testing.T) {
	t.Parallel()
	r := newRunnerRig(t, londonAt(6, 10, 20, 0), nil)
	r.sw.err = errors.New("db down")

	res, err := r.runner.Run(context.Background())

	if err == nil || res.Stop != StopError || len(r.world.reads) != 0 {
		t.Fatalf("stop = %s reads = %d err = %v", res.Stop, len(r.world.reads), err)
	}
}

func TestRunner_TransientLeaseErrorIsReturned(t *testing.T) {
	t.Parallel()
	r := newRunnerRig(t, londonAt(6, 10, 20, 0), nil)
	r.lease.acquire = LeaseAcquireResult{TransientErr: errors.New("db down")}

	if _, err := r.runner.Run(context.Background()); err == nil {
		t.Fatal("want error")
	}
}

func TestRunner_NoWorkStopsFlushesAndReleases(t *testing.T) {
	t.Parallel()
	r := newRunnerRig(t, londonAt(6, 10, 20, 0), nil)
	r.world.allWindowsComplete()

	res, err := r.runner.Run(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	if !res.Acquired || res.Stop != StopNoWork || res.Pages != 0 {
		t.Fatalf("res = %+v", res)
	}
	if r.lease.ttl != 65*time.Minute || r.lease.released != 1 {
		t.Fatalf("ttl = %s released = %d", r.lease.ttl, r.lease.released)
	}
	if !slices.Equal(r.world.events, []string{"reset", "flush"}) {
		t.Fatalf("events = %v", r.world.events)
	}
}

func TestRunner_ReadsAlertWindowsNewestFirstUntilBackedOff(t *testing.T) {
	t.Parallel()
	r := newRunnerRig(t, londonAt(6, 10, 20, 0), nil)
	r.world.script = func(w *runWorld, item PlannedWork) WindowReadResult {
		if len(w.reads) == 4 {
			return WindowReadResult{Stop: StopBackoff, Err: ErrBackedOff}
		}
		return w.complete(item)
	}

	res, err := r.runner.Run(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	if res.Stop != StopBackoff {
		t.Fatalf("stop = %s", res.Stop)
	}
	want := []struct {
		axis planit.Axis
		day  time.Time
	}{
		{planit.AxisStart, utcDay(2026, 6, 10)},
		{planit.AxisDecided, utcDay(2026, 6, 10)},
		{planit.AxisStart, utcDay(2026, 6, 9)},
		{planit.AxisDecided, utcDay(2026, 6, 9)},
	}
	if len(r.world.reads) != len(want) {
		t.Fatalf("reads = %d", len(r.world.reads))
	}
	for i, wnt := range want {
		got := r.world.reads[i]
		if got.Axis != wnt.axis || !got.Day.Equal(wnt.day) {
			t.Fatalf("read %d = %+v", i, got)
		}
	}
	if res.Pages != 3 || r.flusher.flush != 1 || r.lease.released != 1 {
		t.Fatalf("pages = %d flush = %d released = %d", res.Pages, r.flusher.flush, r.lease.released)
	}
}

func TestRunner_StopReasonsFromReads(t *testing.T) {
	t.Parallel()
	for _, stop := range []StopReason{StopDailyCap, StopBackoff, StopRateLimited, StopForbidden, StopTimeout, StopError} {
		t.Run(string(stop), func(t *testing.T) {
			t.Parallel()
			r := newRunnerRig(t, londonAt(6, 10, 20, 0), nil)
			r.world.script = func(*runWorld, PlannedWork) WindowReadResult {
				return WindowReadResult{Pages: 2, Stop: stop, Err: errors.New("x")}
			}

			res, err := r.runner.Run(context.Background())

			if err != nil {
				t.Fatal(err)
			}
			if res.Stop != stop || res.Pages != 2 || len(r.world.reads) != 1 {
				t.Fatalf("res = %+v reads = %d", res, len(r.world.reads))
			}
			if r.flusher.flush != 1 || r.lease.released != 1 {
				t.Fatalf("flush = %d released = %d", r.flusher.flush, r.lease.released)
			}
		})
	}
}

func TestRunner_RunBudgetSpentStopsBetweenItems(t *testing.T) {
	t.Parallel()
	r := newRunnerRig(t, londonAt(6, 10, 20, 0), nil)
	r.world.pageCost = 20 * time.Minute

	res, err := r.runner.Run(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	if res.Stop != StopRunBudget || len(r.world.reads) != 3 {
		t.Fatalf("stop = %s reads = %d", res.Stop, len(r.world.reads))
	}
}

func TestRunner_DeadlineInsideAReadIsRunBudgetNotTimeout(t *testing.T) {
	t.Parallel()
	r := newRunnerRig(t, londonAt(6, 10, 20, 0), nil)
	r.runner.cfg.RunBudget = 20 * time.Millisecond
	r.world.script = func(*runWorld, PlannedWork) WindowReadResult {
		time.Sleep(60 * time.Millisecond)
		return WindowReadResult{Stop: StopTimeout, Err: context.DeadlineExceeded}
	}

	res, err := r.runner.Run(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	if res.Stop != StopRunBudget {
		t.Fatalf("stop = %s", res.Stop)
	}
}

func TestRunner_ShortWindowIsNotReadAgainInTheSameRun(t *testing.T) {
	t.Parallel()
	r := newRunnerRig(t, londonAt(6, 10, 20, 0), nil)
	r.world.script = func(w *runWorld, item PlannedWork) WindowReadResult {
		if len(w.reads) == 1 {
			return WindowReadResult{Pages: 3, Short: true}
		}
		return w.complete(item)
	}

	res, _ := r.runner.Run(context.Background())

	first := r.world.reads[0]
	for _, later := range r.world.reads[1:] {
		if later.Axis == first.Axis && later.Day.Equal(first.Day) {
			t.Fatalf("short window %+v was read again", first)
		}
	}
	if res.Stop != StopNoWork {
		t.Fatalf("stop = %s", res.Stop)
	}
}

func TestRunner_DayRunsDeltaOnly(t *testing.T) {
	t.Parallel()
	r := newRunnerRig(t, londonAt(6, 10, 9, 30), nil)

	res, err := r.runner.Run(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	if res.Stop != StopNoWork || len(r.world.reads) != 2 {
		t.Fatalf("res = %+v reads = %d", res, len(r.world.reads))
	}
	if r.world.reads[0].Work != planit.WorkDeltaStart || r.world.reads[1].Work != planit.WorkDeltaDecided {
		t.Fatalf("reads = %+v", r.world.reads)
	}
	if res.Pages != 4 {
		t.Fatalf("pages = %d", res.Pages)
	}
}

func TestRunner_OracleRunsOnceAfterAlertBandBeforeCoverage(t *testing.T) {
	t.Parallel()
	oracle := &fakeOracleRun{}
	r := newRunnerRig(t, londonAt(6, 10, 20, 0), oracle)
	r.runner.cfg.OracleEnabled = true

	if _, err := r.runner.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	if oracle.calls != 1 {
		t.Fatalf("oracle calls = %d", oracle.calls)
	}
	alertReads := 2 * (alertBandMaxAge + 1)
	idx := slices.Index(r.world.events, "oracle")
	if idx != 1 {
		t.Fatalf("events = %v", r.world.events)
	}
	for i, item := range r.world.reads {
		age := int(utcDay(2026, 6, 10).Sub(item.Day).Hours() / 24)
		if i < alertReads && age > alertBandMaxAge || i >= alertReads && age <= alertBandMaxAge {
			t.Fatalf("read %d age %d: coverage read before the alert band finished", i, age)
		}
	}
}

func TestRunner_OracleSkippedInTheDayAndWhenAlertBandIncomplete(t *testing.T) {
	t.Parallel()
	oracle := &fakeOracleRun{}
	day := newRunnerRig(t, londonAt(6, 10, 9, 30), oracle)
	day.runner.cfg.OracleEnabled = true
	if _, err := day.runner.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	night := newRunnerRig(t, londonAt(6, 10, 20, 0), &fakeOracleRun{})
	night.runner.cfg.OracleEnabled = true
	night.world.script = func(*runWorld, PlannedWork) WindowReadResult { return WindowReadResult{Stop: StopBackoff} }
	if _, err := night.runner.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	if oracle.calls != 0 || slices.Contains(night.world.events, "oracle") {
		t.Fatal("oracle must not run in the day or before the alert band is complete")
	}
}

func TestRunner_OracleStopEndsTheRun(t *testing.T) {
	t.Parallel()
	oracle := &fakeOracleRun{out: OracleOutcome{Pages: 2, Stop: StopRateLimited}}
	r := newRunnerRig(t, londonAt(6, 10, 20, 0), oracle)
	r.runner.cfg.OracleEnabled = true

	res, _ := r.runner.Run(context.Background())

	if res.Stop != StopRateLimited || res.Pages != 2*(alertBandMaxAge+1)+2 {
		t.Fatalf("res = %+v", res)
	}
	if got := len(r.world.reads); got != 2*(alertBandMaxAge+1) {
		t.Fatalf("coverage read after the oracle stopped the run: %d", got)
	}
}

func TestRunner_ReportsHealthAndCounts(t *testing.T) {
	t.Parallel()
	r := newRunnerRig(t, londonAt(6, 10, 20, 0), nil)
	r.world.allWindowsComplete()
	r.world.calls = 42
	r.counters.c = RunCounts{Violations: 3, Missed: 1}
	r.health.in = HealthInputs{Forbidden24h: 1}

	res, err := r.runner.Run(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	if res.CallsToday != 42 || res.Health.Level != HealthCritical || res.Counts.Violations != 3 || res.Counts.Missed != 1 {
		t.Fatalf("res = %+v", res)
	}
}

func TestRunner_HealthSourceFailureDoesNotFailTheRun(t *testing.T) {
	t.Parallel()
	r := newRunnerRig(t, londonAt(6, 10, 20, 0), nil)
	r.world.allWindowsComplete()
	r.health.err = errors.New("db")

	res, err := r.runner.Run(context.Background())

	if err != nil || res.Health.Level != "" {
		t.Fatalf("res = %+v err = %v", res, err)
	}
	if r.lease.released != 1 {
		t.Fatal("lease not released")
	}
}

func TestRunner_SpanCarriesRunAttributes(t *testing.T) {
	r := newRunnerRig(t, londonAt(6, 10, 20, 0), nil)
	r.world.allWindowsComplete()
	r.world.calls = 7
	r.counters.c = RunCounts{Violations: 2, Missed: 1}
	pendingSince := londonAt(6, 10, 18, 30)
	r.health.in = HealthInputs{
		Surge24h: 1, Violations24h: 1, NewAppEvents24h: 1400, Decisions24h: 1200,
		StaleEvents24h: 5, PendingEvents: 2, OldestPendingAt: &pendingSince, Notifications24h: 30,
	}

	spans := recordSpans(t, func() {
		if _, err := r.runner.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	})

	span, ok := spanNamed(spans, "PlanIt poll run")
	if !ok {
		t.Fatal("no PlanIt poll run span")
	}
	wantInt := map[string]int64{
		"poll.pages": 0, "poll.calls_today": 7, "poll.window_violations": 2, "poll.window_missed": 1,
		"poll.alert_band_unverified": 0, "poll.events.new_application_24h": 1400, "poll.events.decision_24h": 1200,
		"poll.events.stale_24h": 5, "poll.events.pending": 2, "poll.events.oldest_pending_minutes": 90,
		"poll.notifications_24h": 30,
	}
	for k, want := range wantInt {
		if v, ok := attrValue(span, k); !ok || v.AsInt64() != want {
			t.Fatalf("%s = %v (present %v), want %d", k, v, ok, want)
		}
	}
	if v, _ := attrValue(span, "poll.stop_reason"); v.AsString() != "no_work" {
		t.Fatalf("stop_reason = %q", v.AsString())
	}
	if v, _ := attrValue(span, "poll.health"); v.AsString() != "critical" {
		t.Fatalf("health = %q", v.AsString())
	}
	if v, _ := attrValue(span, "poll.health_reasons"); v.AsString() != "window_violation,surge" {
		t.Fatalf("reasons = %q", v.AsString())
	}
	if _, ok := attrValue(span, "poll.retry_after_seconds"); ok {
		t.Fatal("retry_after_seconds set on a run without a 429")
	}
}

func TestRunner_SpanCarriesRetryAfterOnRateLimit(t *testing.T) {
	r := newRunnerRig(t, londonAt(6, 10, 20, 0), nil)
	retry := 47 * time.Minute
	r.world.script = func(*runWorld, PlannedWork) WindowReadResult {
		return WindowReadResult{Pages: 1, Stop: StopRateLimited, Err: fmt.Errorf("fetch: %w", &planit.RateLimitError{RetryAfter: &retry})}
	}

	spans := recordSpans(t, func() {
		if _, err := r.runner.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	})

	span, ok := spanNamed(spans, "PlanIt poll run")
	if !ok {
		t.Fatal("no PlanIt poll run span")
	}
	if v, ok := attrValue(span, "poll.retry_after_seconds"); !ok || v.AsInt64() != 2820 {
		t.Fatalf("retry_after_seconds = %v (present %v), want 2820", v, ok)
	}
}
