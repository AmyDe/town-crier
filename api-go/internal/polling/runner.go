package polling

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

const (
	runLeaseTTL = 65 * time.Minute
	runSpanName = "PlanIt poll run"
	tracerName  = "github.com/AmyDe/town-crier/api-go/internal/polling"
)

// OracleOutcome is what one oracle pass reports: Stop is set when a PlanIt
// limit ended the pass early, and the counts are the dev trial's per-night
// figures.
type OracleOutcome struct {
	Pages       int
	Stop        StopReason
	WideRecords int
	InBand      int
	NewDiffs    int
	Classified  map[string]int
}

type runLease interface {
	TryAcquire(ctx context.Context, ttl time.Duration) (LeaseAcquireResult, error)
	Release(ctx context.Context, handle LeaseHandle) LeaseReleaseOutcome
}

type runState interface {
	Load(ctx context.Context, from, to time.Time) ([]WindowState, error)
	LatestPageZeroAt(ctx context.Context, work planit.Work) (*time.Time, error)
}

type callCounter interface {
	CallsToday(ctx context.Context, now time.Time) (int, error)
}

type workReader interface {
	ReadWindow(ctx context.Context, w PlannedWork, state WindowState) WindowReadResult
	ReadDelta(ctx context.Context, w PlannedWork) WindowReadResult
}

type runCounters interface {
	Counts() RunCounts
}

type pushResetter interface{ Reset() }

type runFlusher interface {
	Flush(ctx context.Context) error
}

type healthSource interface {
	Inputs(ctx context.Context, now time.Time) (HealthInputs, error)
}

type pollSwitch interface {
	Enabled(ctx context.Context) (bool, error)
}

type oracleRunner interface {
	Run(ctx context.Context, now time.Time) (OracleOutcome, error)
}

// RunnerConfig holds the run-level settings: the planner limits, the run
// budget (POLLING_RUN_BUDGET_MINUTES) and whether the dev oracle runs.
type RunnerConfig struct {
	Plan          WindowPlanConfig
	RunBudget     time.Duration
	OracleEnabled bool
}

// RunnerDeps are the collaborators of a Runner. Push and Oracle may be nil.
type RunnerDeps struct {
	Switch   pollSwitch
	Lease    runLease
	State    runState
	Calls    callCounter
	Reader   workReader
	Counters runCounters
	Push     pushResetter
	Flusher  runFlusher
	Health   healthSource
	Oracle   oracleRunner
	Now      func() time.Time
	Log      *slog.Logger
}

// RunResult summarises one Runner.Run. Acquired is false when a peer held the
// lease and no work was done.
type RunResult struct {
	Acquired   bool
	Pages      int
	CallsToday int
	Stop       StopReason
	RetryAfter *time.Duration
	Health     Health
	Counts     RunCounts
}

// Runner is one hourly poll run.
type Runner struct {
	cfg RunnerConfig
	d   RunnerDeps
}

// NewRunner wires a Runner.
func NewRunner(cfg RunnerConfig, deps RunnerDeps) *Runner {
	return &Runner{cfg: cfg, d: deps}
}

// Run takes the "polling" lease and, until the run budget is spent or the
// polling switch is off, plans and reads windows and deltas through the
// WindowReader. It then flushes pushes, computes health (not for a switched-off
// run) and releases the lease. A held lease returns without work.
// Limits such as backoff, the daily cap, 429, 403 and timeouts end the run
// normally; only lease and state failures return an error.
func (r *Runner) Run(ctx context.Context) (RunResult, error) {
	acq, err := r.d.Lease.TryAcquire(ctx, runLeaseTTL)
	if err != nil {
		return RunResult{}, fmt.Errorf("acquire polling lease: %w", err)
	}
	if acq.TransientErr != nil {
		return RunResult{}, fmt.Errorf("acquire polling lease: %w", acq.TransientErr)
	}
	if !acq.Acquired {
		r.d.Log.InfoContext(ctx, "poll.lease_held")
		return RunResult{}, nil
	}
	defer r.d.Lease.Release(context.WithoutCancel(ctx), acq.Handle)

	ctx, span := otel.Tracer(tracerName).Start(ctx, runSpanName)
	defer span.End()

	res, err := r.loop(ctx)
	res.Acquired = true
	res.Counts = r.d.Counters.Counts()
	if err != nil {
		span.RecordError(err)
	}

	flushCtx := context.WithoutCancel(ctx)
	if ferr := r.d.Flusher.Flush(flushCtx); ferr != nil {
		r.d.Log.WarnContext(ctx, "poll.flush_failed", slog.Any("error", ferr))
	}
	if res.Stop != StopDisabled {
		res.Health = r.health(flushCtx, res.Counts)
	}
	r.setSpanAttrs(span, res)
	return res, err
}

func (r *Runner) loop(ctx context.Context) (RunResult, error) {
	var res RunResult
	if r.d.Push != nil {
		r.d.Push.Reset()
	}

	start := r.d.Now()
	budgetCtx, cancel := context.WithTimeout(ctx, r.cfg.RunBudget)
	defer cancel()

	skipped := map[WindowRef]struct{}{}
	oracleTried := false
	for {
		now := r.d.Now()
		if now.Sub(start) >= r.cfg.RunBudget {
			res.Stop = StopRunBudget
			return res, r.finish(ctx, &res, now)
		}
		on, err := r.d.Switch.Enabled(ctx)
		if err != nil {
			res.Stop = StopError
			return res, fmt.Errorf("read polling switch: %w", err)
		}
		if !on {
			res.Stop = StopDisabled
			return res, r.finish(ctx, &res, now)
		}
		state, err := r.loadState(ctx, now, skipped)
		if err != nil {
			res.Stop = StopError
			return res, err
		}
		calls, err := r.d.Calls.CallsToday(ctx, now)
		if err != nil {
			res.Stop = StopError
			return res, fmt.Errorf("count calls today: %w", err)
		}
		res.CallsToday = calls

		if r.cfg.OracleEnabled && r.d.Oracle != nil && !oracleTried && alertBandComplete(state, now) {
			oracleTried = true
			out, oerr := r.d.Oracle.Run(budgetCtx, now)
			res.Pages += out.Pages
			if oerr != nil {
				r.d.Log.WarnContext(ctx, "poll.oracle_failed", slog.Any("error", oerr))
			}
			if stop := r.budgetAware(out.Stop, ctx, budgetCtx); stop != "" {
				res.Stop = stop
				return res, r.finish(ctx, &res, r.d.Now())
			}
			continue
		}

		item := NextWindowWork(r.cfg.Plan, state, now, calls)
		if item == nil {
			res.Stop = StopNoWork
			return res, r.finish(ctx, &res, now)
		}

		var out WindowReadResult
		if item.DifferentStart != nil {
			out = r.d.Reader.ReadDelta(budgetCtx, *item)
		} else {
			out = r.d.Reader.ReadWindow(budgetCtx, *item, windowStateFor(state, item))
		}
		res.Pages += out.Pages
		if stop := r.budgetAware(out.Stop, ctx, budgetCtx); stop != "" {
			if out.Err != nil && stop != StopRunBudget {
				r.d.Log.WarnContext(ctx, "poll.read_stopped", slog.String("work", string(item.Work)), slog.String("stop", string(stop)), slog.Any("error", out.Err))
			}
			res.Stop = stop
			var rl *planit.RateLimitError
			if errors.As(out.Err, &rl) {
				res.RetryAfter = rl.RetryAfter
			}
			return res, r.finish(ctx, &res, r.d.Now())
		}
		if item.DifferentStart == nil && !out.Complete {
			skipped[WindowRef{Axis: item.Axis, Day: item.Day}] = struct{}{}
		}
	}
}

// finish refreshes calls_today for the span once the loop has ended.
func (r *Runner) finish(ctx context.Context, res *RunResult, now time.Time) error {
	calls, err := r.d.Calls.CallsToday(context.WithoutCancel(ctx), now)
	if err != nil {
		return fmt.Errorf("count calls today: %w", err)
	}
	res.CallsToday = calls
	return nil
}

// budgetAware maps a read's stop reason, treating a timeout caused by the run
// budget's own deadline as run_budget.
func (r *Runner) budgetAware(stop StopReason, parent, budget context.Context) StopReason {
	if stop == StopTimeout && budget.Err() != nil && parent.Err() == nil {
		return StopRunBudget
	}
	return stop
}

func (r *Runner) loadState(ctx context.Context, now time.Time, skipped map[WindowRef]struct{}) (WindowPlanState, error) {
	local := now.In(budgetLocation)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	windows, err := r.d.State.Load(ctx, today.AddDate(0, 0, -coverageBandMaxAge), today)
	if err != nil {
		return WindowPlanState{}, fmt.Errorf("load poll_window: %w", err)
	}
	for i := range windows {
		if _, ok := skipped[WindowRef{Axis: windows[i].Axis, Day: windows[i].Day}]; ok {
			at := now
			windows[i].LastCompleteAt = &at
		}
	}
	for ref := range skipped {
		if !hasWindow(windows, ref) {
			at := now
			windows = append(windows, WindowState{Axis: ref.Axis, Day: ref.Day, LastCompleteAt: &at})
		}
	}
	deltas := map[planit.Work]time.Time{}
	for _, work := range [...]planit.Work{planit.WorkDeltaStart, planit.WorkDeltaDecided} {
		at, err := r.d.State.LatestPageZeroAt(ctx, work)
		if err != nil {
			return WindowPlanState{}, fmt.Errorf("latest %s call: %w", work, err)
		}
		if at != nil {
			deltas[work] = *at
		}
	}
	return WindowPlanState{Windows: windows, DeltaStartedAt: deltas}, nil
}

func hasWindow(ws []WindowState, ref WindowRef) bool {
	for _, w := range ws {
		if w.Axis == ref.Axis && w.Day.Equal(ref.Day) {
			return true
		}
	}
	return false
}

func windowStateFor(state WindowPlanState, item *PlannedWork) WindowState {
	for _, w := range state.Windows {
		if w.Axis == item.Axis && w.Day.Equal(item.Day) {
			return w
		}
	}
	return WindowState{Axis: item.Axis, Day: item.Day}
}

// alertBandComplete reports whether it is night and every alert-band window
// has completed since the night began.
func alertBandComplete(state WindowPlanState, now time.Time) bool {
	local := now.In(budgetLocation)
	if local.Hour() >= dayStartHour && local.Hour() < dayEndHour {
		return false
	}
	nightStart, _ := BudgetDay(now)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	byRef := make(map[WindowRef]WindowState, len(state.Windows))
	for _, w := range state.Windows {
		byRef[WindowRef{Axis: w.Axis, Day: w.Day}] = w
	}
	for age := 0; age <= alertBandMaxAge; age++ {
		for _, axis := range [...]planit.Axis{planit.AxisStart, planit.AxisDecided} {
			w := byRef[WindowRef{Axis: axis, Day: today.AddDate(0, 0, -age)}]
			if w.LastCompleteAt == nil || w.LastCompleteAt.Before(nightStart) {
				return false
			}
		}
	}
	return true
}

func (r *Runner) health(ctx context.Context, counts RunCounts) Health {
	now := r.d.Now()
	in, err := r.d.Health.Inputs(ctx, now)
	if err != nil {
		r.d.Log.WarnContext(ctx, "poll.health_failed", slog.Any("error", err))
		return Health{}
	}
	local := now.In(budgetLocation)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	in.Windows, err = r.d.State.Load(ctx, today.AddDate(0, 0, -alertBandMaxAge), today)
	if err != nil {
		r.d.Log.WarnContext(ctx, "poll.health_failed", slog.Any("error", err))
		return Health{}
	}
	in.OracleEnabled = r.cfg.OracleEnabled
	in.SurgeThisRun = counts.Surge
	return ComputeHealth(in, now)
}

func (r *Runner) setSpanAttrs(span trace.Span, res RunResult) {
	span.SetAttributes(
		attribute.Int("poll.pages", res.Pages),
		attribute.Int("poll.calls_today", res.CallsToday),
		attribute.String("poll.stop_reason", string(res.Stop)),
		attribute.Bool("poll.enabled", res.Stop != StopDisabled),
		attribute.Int("poll.window_violations", res.Counts.Violations),
		attribute.Int("poll.window_missed", res.Counts.Missed),
	)
	if res.RetryAfter != nil {
		span.SetAttributes(attribute.Int64("poll.retry_after_seconds", int64(res.RetryAfter.Seconds())))
	}
	if res.Health.Level == "" {
		return
	}
	reasons := make([]string, len(res.Health.Reasons))
	for i, reason := range res.Health.Reasons {
		reasons[i] = string(reason)
	}
	// App Insights drops slice-valued span attributes, so the reasons are joined.
	facts := res.Health.Facts
	span.SetAttributes(
		attribute.String("poll.health", string(res.Health.Level)),
		attribute.String("poll.health_reasons", strings.Join(reasons, ",")),
		attribute.Int("poll.alert_band_unverified", facts.AlertBandUnverified),
		attribute.Int("poll.events.new_application_24h", facts.NewApplications24h),
		attribute.Int("poll.events.decision_24h", facts.Decisions24h),
		attribute.Int("poll.events.stale_24h", facts.StaleEvents24h),
		attribute.Int("poll.events.pending", facts.PendingEvents),
		attribute.Int64("poll.events.oldest_pending_minutes", int64(facts.OldestPending.Minutes())),
		attribute.Int("poll.notifications_24h", facts.Notifications24h),
		attribute.Int("poll.events.surge_24h", facts.Surges24h),
	)
}
