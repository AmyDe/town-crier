package polling

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

// StopReason says why a run stopped. Values match the poll.stop_reason span
// attribute.
type StopReason string

// The stop reasons a window or delta read can produce (StopNoWork and
// StopRunBudget are set by the runner).
const (
	StopNoWork      StopReason = "no_work"
	StopRunBudget   StopReason = "run_budget"
	StopDailyCap    StopReason = "daily_cap"
	StopBackoff     StopReason = "backoff"
	StopRateLimited StopReason = "rate_limited"
	StopForbidden   StopReason = "forbidden"
	StopTimeout     StopReason = "timeout"
	StopError       StopReason = "error"
)

// stopReasonFor classifies an error from a paced PlanIt fetch.
func stopReasonFor(err error) StopReason {
	var rl *planit.RateLimitError
	var forbidden *planit.ForbiddenError
	switch {
	case errors.Is(err, ErrBackedOff):
		return StopBackoff
	case errors.Is(err, ErrBudgetExhausted):
		return StopDailyCap
	case errors.As(err, &rl):
		return StopRateLimited
	case errors.As(err, &forbidden):
		return StopForbidden
	case errors.Is(err, planit.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		return StopTimeout
	default:
		return StopError
	}
}

type windowPageFetcher interface {
	FetchPage(ctx context.Context, q planit.WindowQuery) (planit.FetchPageResult, error)
}

type applicationIngester interface {
	Ingest(ctx context.Context, app applications.PlanningApplication) error
}

type windowStateWriter interface {
	MarkProbeComplete(ctx context.Context, ref WindowRef, at time.Time) error
	MarkFullRead(ctx context.Context, ref WindowRef, at time.Time, total int) error
}

type windowPageClient interface {
	FetchWindowPage(ctx context.Context, q planit.WindowQuery) (planit.FetchPageResult, error)
}

// PacedWindowFetcher sends every window page through the Pacer.
type PacedWindowFetcher struct {
	pacer  *Pacer
	client windowPageClient
}

// NewPacedWindowFetcher wires the fetcher over pacer and client.
func NewPacedWindowFetcher(pacer *Pacer, client windowPageClient) *PacedWindowFetcher {
	return &PacedWindowFetcher{pacer: pacer, client: client}
}

// FetchPage makes exactly one paced PlanIt request for q.
func (f *PacedWindowFetcher) FetchPage(ctx context.Context, q planit.WindowQuery) (planit.FetchPageResult, error) {
	var day time.Time
	if q.DifferentStart == nil {
		day = q.From
	}
	res, err := f.pacer.Do(ctx, q.Work, day, q.Index, func(ctx context.Context) (planit.FetchPageResult, error) {
		return f.client.FetchWindowPage(ctx, q)
	})
	if err != nil {
		return res, fmt.Errorf("fetch %s page %d: %w", q.Work, q.Index, err)
	}
	return res, nil
}

// WindowReadConfig holds POLLING_FULL_READ_MAX_AGE_DAYS as a duration.
type WindowReadConfig struct {
	FullReadMaxAge time.Duration
}

// WindowReadResult is the outcome of one window or delta read. Stop is empty
// when the read ran to its end. Complete is true only for a window whose
// last_complete_at was advanced. Probe is true when that took one request.
// Short is true when the completeness check failed. Capped is true when a delta
// stopped at its page cap.
type WindowReadResult struct {
	Pages    int
	Complete bool
	Probe    bool
	Short    bool
	Capped   bool
	Stop     StopReason
	Err      error
}

// WindowReader reads day windows and delta passes. Windows are atomic: it
// keeps no checkpoint, so a read that stops part-way leaves poll_window
// untouched and the next run starts again at index 0.
type WindowReader struct {
	fetch  windowPageFetcher
	ingest applicationIngester
	store  windowStateWriter
	cfg    WindowReadConfig
	now    func() time.Time
	log    *slog.Logger

	mu     sync.Mutex
	shorts map[shortKey]int
}

type shortKey struct {
	ref       WindowRef
	budgetDay time.Time
}

// NewWindowReader wires a WindowReader.
func NewWindowReader(fetch windowPageFetcher, ingest applicationIngester, store windowStateWriter, cfg WindowReadConfig, now func() time.Time, log *slog.Logger) *WindowReader {
	return &WindowReader{fetch: fetch, ingest: ingest, store: store, cfg: cfg, now: now, log: log, shorts: map[shortKey]int{}}
}

type appKey struct {
	uid  string
	area int
}

// ReadWindow reads one day window: a count probe when page 0's total matches
// the stored total and the last full read is recent, otherwise every page.
func (r *WindowReader) ReadWindow(ctx context.Context, w PlannedWork, state WindowState) WindowReadResult {
	ref := WindowRef{Axis: w.Axis, Day: w.Day}
	q := planit.WindowQuery{Work: w.Work, Axis: w.Axis, From: w.From, To: w.To}

	first, err := r.fetch.FetchPage(ctx, q)
	if err != nil {
		return stopped(0, err)
	}
	pages := 1
	if err := r.ingestPage(ctx, first); err != nil {
		return stopped(pages, err)
	}
	seen := map[appKey]struct{}{}
	addSeen(seen, first)
	total := len(first.Applications)
	if first.Total != nil {
		total = *first.Total
	}

	now := r.now()
	if state.LastTotal != nil && *state.LastTotal == total && state.LastFullReadAt != nil &&
		now.Sub(*state.LastFullReadAt) < r.cfg.FullReadMaxAge {
		if err := r.store.MarkProbeComplete(ctx, ref, now); err != nil {
			return stopped(pages, err)
		}
		return WindowReadResult{Pages: pages, Complete: true, Probe: true}
	}

	cur := first
	for cur.HasMorePages && len(cur.Applications) > 0 {
		q.Index = cur.From + len(cur.Applications)
		cur, err = r.fetch.FetchPage(ctx, q)
		if err != nil {
			return stopped(pages, err)
		}
		pages++
		if err := r.ingestPage(ctx, cur); err != nil {
			return stopped(pages, err)
		}
		addSeen(seen, cur)
	}

	if len(seen) < total {
		r.recordShort(ref, now)
		r.log.WarnContext(ctx, "poll.window_short",
			slog.Int("axis", int(w.Axis)), slog.String("day", w.Day.Format(time.DateOnly)),
			slog.Int("distinct", len(seen)), slog.Int("total", total), slog.Int("pages", pages))
		return WindowReadResult{Pages: pages, Short: true}
	}
	if err := r.store.MarkFullRead(ctx, ref, now, total); err != nil {
		return stopped(pages, err)
	}
	return WindowReadResult{Pages: pages, Complete: true}
}

// ReadDelta reads a delta pass from page 0 to the end, up to w.MaxPages.
func (r *WindowReader) ReadDelta(ctx context.Context, w PlannedWork) WindowReadResult {
	q := planit.WindowQuery{Work: w.Work, Axis: w.Axis, From: w.From, To: w.To, DifferentStart: w.DifferentStart}
	pages := 0
	for {
		res, err := r.fetch.FetchPage(ctx, q)
		if err != nil {
			return stopped(pages, err)
		}
		pages++
		if err := r.ingestPage(ctx, res); err != nil {
			return stopped(pages, err)
		}
		if !res.HasMorePages || len(res.Applications) == 0 {
			return WindowReadResult{Pages: pages}
		}
		if pages >= w.MaxPages {
			return WindowReadResult{Pages: pages, Capped: true}
		}
		q.Index = res.From + len(res.Applications)
	}
}

// ShortWindows returns the windows that were short at least twice in the
// budget day containing now. The count is held in memory, so it covers the
// life of this reader.
func (r *WindowReader) ShortWindows(now time.Time) []WindowRef {
	day, _ := BudgetDay(now)
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []WindowRef
	for k, n := range r.shorts {
		if n >= 2 && k.budgetDay.Equal(day) {
			out = append(out, k.ref)
		}
	}
	return out
}

func (r *WindowReader) recordShort(ref WindowRef, now time.Time) {
	day, _ := BudgetDay(now)
	r.mu.Lock()
	r.shorts[shortKey{ref: ref, budgetDay: day}]++
	r.mu.Unlock()
}

func (r *WindowReader) ingestPage(ctx context.Context, p planit.FetchPageResult) error {
	for _, app := range p.Applications {
		if err := r.ingest.Ingest(ctx, app); err != nil {
			return fmt.Errorf("ingest %s: %w", app.UID, err)
		}
	}
	return nil
}

func addSeen(seen map[appKey]struct{}, p planit.FetchPageResult) {
	for _, a := range p.Applications {
		seen[appKey{a.UID, a.AreaID}] = struct{}{}
	}
}

func stopped(pages int, err error) WindowReadResult {
	return WindowReadResult{Pages: pages, Stop: stopReasonFor(err), Err: err}
}
