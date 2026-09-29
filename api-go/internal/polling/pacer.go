package polling

import (
	"context"
	"errors"
	"fmt"
	"time"

	// Bundles IANA tzdata so Europe/London resolves in a minimal container.
	_ "time/tzdata"

	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

// Sentinel errors returned by Pacer.Do when no request was made and no
// planit_call row was written.
var (
	ErrBackedOff       = errors.New("polling: planit backoff active")
	ErrBudgetExhausted = errors.New("polling: daily planit call budget exhausted")
)

const (
	backoffRateLimitDefault = 15 * time.Minute
	backoffRateLimitMax     = 3 * time.Hour
	backoffForbidden        = 24 * time.Hour
	backoffTransient        = 30 * time.Minute
	statusAborted           = 0
	settleTimeout           = 5 * time.Second
)

var budgetLocation = mustLoadLocation("Europe/London")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(fmt.Sprintf("load %s: %v", name, err))
	}
	return loc
}

// BudgetDay returns the half-open [start, end) budget day containing t. A
// budget day runs 18:00 to 18:00 Europe/London, so it is 23h or 25h across a
// DST change.
func BudgetDay(t time.Time) (start, end time.Time) {
	l := t.In(budgetLocation)
	y, m, day := l.Date()
	if l.Hour() < 18 {
		day--
	}
	start = time.Date(y, m, day, 18, 0, 0, 0, budgetLocation)
	end = time.Date(y, m, day+1, 18, 0, 0, 0, budgetLocation)
	return start, end
}

// PlanItCall is one row of the append-only planit_call log. Status is nil for
// a timeout or transport error and 0 when the caller aborted the request.
type PlanItCall struct {
	ID         int64
	At         time.Time
	Work       string
	WindowDay  *time.Time
	PageIndex  int
	Status     *int
	Total      *int
	RetryAfter *time.Duration
}

// CallResult is the outcome recorded on a planit_call row after the request.
type CallResult struct {
	Status     *int
	Total      *int
	RetryAfter *time.Duration
}

// planItCallTx is one serialised pacing decision. Begin takes the advisory lock;
// it is held until Commit or Rollback.
type planItCallTx interface {
	Latest(ctx context.Context) (*PlanItCall, error)
	CountBetween(ctx context.Context, from, to time.Time) (int, error)
	Insert(ctx context.Context, c PlanItCall) (int64, error)
	Finish(ctx context.Context, id int64, r CallResult) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type planItCallLog interface {
	Begin(ctx context.Context) (planItCallTx, error)
	Latest(ctx context.Context) (*PlanItCall, error)
	CountBetween(ctx context.Context, from, to time.Time) (int, error)
}

// PacerConfig holds the pacing limits: DailyCap requests per budget day and
// MinSpacing between consecutive requests.
type PacerConfig struct {
	DailyCap   int
	MinSpacing time.Duration
}

// BackoffState reports whether PlanIt is in backoff after the latest call.
type BackoffState struct {
	Active bool
	Until  time.Time
	Reason string
}

// Pacer serialises every PlanIt request through the planit_call log: it
// enforces backoff, the daily budget and minimum spacing, and never retries.
type Pacer struct {
	log   planItCallLog
	cfg   PacerConfig
	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) error
}

// NewPacer wires a Pacer. now and sleep are injected so tests control time.
func NewPacer(log planItCallLog, cfg PacerConfig, now func() time.Time, sleep func(ctx context.Context, d time.Duration) error) *Pacer {
	return &Pacer{log: log, cfg: cfg, now: now, sleep: sleep}
}

// SleepContext sleeps for d or until ctx is done.
func SleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("sleep: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

// Do makes exactly one PlanIt request under the pacing rules. It returns
// ErrBackedOff or ErrBudgetExhausted, having sent nothing and written no row,
// when a limit applies. Otherwise it records a row before the request, waits
// out the minimum spacing, runs fn, records the outcome and returns fn's
// result and error unchanged. windowDay may be zero for work with no window.
//
// The advisory lock is held until the outcome is recorded so concurrent callers
// see a settled latest row. A crash mid-request rolls the row back.
func (p *Pacer) Do(ctx context.Context, work planit.Work, windowDay time.Time, pageIndex int, fn func(ctx context.Context) (planit.FetchPageResult, error)) (planit.FetchPageResult, error) {
	tx, err := p.log.Begin(ctx)
	if err != nil {
		return planit.FetchPageResult{}, fmt.Errorf("begin pacing tx: %w", err)
	}
	settled := false
	defer func() {
		if !settled {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()

	now := p.now()
	latest, err := tx.Latest(ctx)
	if err != nil {
		return planit.FetchPageResult{}, fmt.Errorf("read latest planit call: %w", err)
	}
	if st := backoffFor(latest, now); st.Active {
		return planit.FetchPageResult{}, fmt.Errorf("%w until %s (%s)", ErrBackedOff, st.Until.Format(time.RFC3339), st.Reason)
	}

	// The row's At is the scheduled send time, so concurrent callers queue
	// behind each other's slots rather than measuring spacing from insertion.
	at := now
	if latest != nil {
		if earliest := latest.At.Add(p.cfg.MinSpacing); earliest.After(at) {
			at = earliest
		}
	}
	from, to := BudgetDay(at)
	used, err := tx.CountBetween(ctx, from, to)
	if err != nil {
		return planit.FetchPageResult{}, fmt.Errorf("count planit calls: %w", err)
	}
	if used >= p.cfg.DailyCap {
		return planit.FetchPageResult{}, fmt.Errorf("%w (%d of %d)", ErrBudgetExhausted, used, p.cfg.DailyCap)
	}

	row := PlanItCall{At: at, Work: string(work), PageIndex: pageIndex}
	if !windowDay.IsZero() {
		wd := windowDay
		row.WindowDay = &wd
	}
	id, err := tx.Insert(ctx, row)
	if err != nil {
		return planit.FetchPageResult{}, fmt.Errorf("insert planit call: %w", err)
	}

	var res planit.FetchPageResult
	workErr := p.sleep(ctx, at.Sub(now))
	if workErr == nil {
		res, workErr = fn(ctx)
	}

	settleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), settleTimeout)
	defer cancel()
	if err := tx.Finish(settleCtx, id, resultFor(res, workErr)); err != nil {
		return res, errors.Join(workErr, fmt.Errorf("record planit call result: %w", err))
	}
	settled = true
	if err := tx.Commit(settleCtx); err != nil {
		return res, errors.Join(workErr, fmt.Errorf("commit planit call: %w", err))
	}
	return res, workErr
}

// CallsToday counts the calls in the budget day containing now.
func (p *Pacer) CallsToday(ctx context.Context, now time.Time) (int, error) {
	from, to := BudgetDay(now)
	n, err := p.log.CountBetween(ctx, from, to)
	if err != nil {
		return 0, fmt.Errorf("count planit calls: %w", err)
	}
	return n, nil
}

// Backoff reports the backoff state implied by the latest call at now.
func (p *Pacer) Backoff(ctx context.Context, now time.Time) (BackoffState, error) {
	latest, err := p.log.Latest(ctx)
	if err != nil {
		return BackoffState{}, fmt.Errorf("read latest planit call: %w", err)
	}
	return backoffFor(latest, now), nil
}

func backoffFor(latest *PlanItCall, now time.Time) BackoffState {
	if latest == nil {
		return BackoffState{}
	}
	var wait time.Duration
	var reason string
	switch s := latest.Status; {
	case s == nil:
		wait, reason = backoffTransient, "timeout"
	case *s == 429:
		wait, reason = backoffRateLimitDefault, "rate_limited"
		if latest.RetryAfter != nil {
			wait = min(*latest.RetryAfter, backoffRateLimitMax)
		}
	case *s == 403:
		wait, reason = backoffForbidden, "forbidden"
	case *s >= 500:
		wait, reason = backoffTransient, "server_error"
	default:
		return BackoffState{}
	}
	until := latest.At.Add(wait)
	return BackoffState{Active: now.Before(until), Until: until, Reason: reason}
}

func resultFor(res planit.FetchPageResult, err error) CallResult {
	var rl *planit.RateLimitError
	var forbidden *planit.ForbiddenError
	var httpErr *planit.HTTPError
	switch {
	case err == nil:
		return CallResult{Status: intPtr(200), Total: res.Total}
	case errors.As(err, &rl):
		return CallResult{Status: intPtr(429), RetryAfter: rl.RetryAfter}
	case errors.As(err, &forbidden):
		return CallResult{Status: intPtr(403)}
	case errors.As(err, &httpErr):
		return CallResult{Status: intPtr(httpErr.StatusCode)}
	case errors.Is(err, planit.ErrTimeout):
		return CallResult{}
	case errors.Is(err, context.Canceled):
		return CallResult{Status: intPtr(statusAborted)}
	default:
		return CallResult{}
	}
}

func intPtr(v int) *int { return &v }
