package polling

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/appevents"
	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

// The poll_event kinds. Health reads them back across runs.
const (
	EventWindowShort     = "window_short"
	EventWindowViolation = "window_violation"
	EventWindowMissed    = "window_missed"
	EventSurge           = "surge"
	EventOracleDone      = "oracle_done"
)

// PollEvent is one durable poll_event row. Day is set for window-scoped kinds.
type PollEvent struct {
	Kind   string
	At     time.Time
	Axis   planit.Axis
	Day    *time.Time
	Detail int
}

// DeltaSeen is one delta_seen row: a record a delta pass ingested whose axis
// date lies in the alert band.
type DeltaSeen struct {
	Axis   planit.Axis
	Day    time.Time
	UID    string
	AreaID int
	SeenAt time.Time
}

type deltaSeenStore interface {
	Upsert(ctx context.Context, rows []DeltaSeen) error
	// Take deletes the window's rows and returns those with seen_at before the cutoff.
	Take(ctx context.Context, ref WindowRef, before time.Time) ([]AppKey, error)
}

type memberStore interface {
	Replace(ctx context.Context, ref WindowRef, readAt time.Time, keys []AppKey) error
}

type pollEventRecorder interface {
	Record(ctx context.Context, e PollEvent) error
}

type pageDispatcher interface {
	Dispatch(ctx context.Context) (appevents.Result, error)
}

// RunCounts are the per-run integrity and cross-check totals.
type RunCounts struct {
	Violations int
	Missed     int
	Surge      bool
}

// RunObserver implements ReadHooks for the runner: the integrity check, the
// delta cross-check, dev-only window membership, event dispatch after each
// page and durable health events.
type RunObserver struct {
	delta   deltaSeenStore
	members memberStore
	events  pollEventRecorder
	disp    pageDispatcher
	now     func() time.Time
	log     *slog.Logger

	mu     sync.Mutex
	counts RunCounts
}

var _ ReadHooks = (*RunObserver)(nil)

// NewRunObserver wires the observer. members is nil in prod, where
// poll_window_member is never written.
func NewRunObserver(delta deltaSeenStore, members memberStore, events pollEventRecorder, disp pageDispatcher, now func() time.Time, log *slog.Logger) *RunObserver {
	return &RunObserver{delta: delta, members: members, events: events, disp: disp, now: now, log: log}
}

// Counts returns the totals since the observer was created; each process
// makes one run.
func (o *RunObserver) Counts() RunCounts {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.counts
}

// PageFetched checks that every record's axis date is inside the window (or,
// for a delta, on or after the mask). Violating records are still ingested.
func (o *RunObserver) PageFetched(ctx context.Context, q planit.WindowQuery, p planit.FetchPageResult) {
	bad, first := 0, ""
	for _, app := range p.Applications {
		d := axisDate(app, q.Axis)
		if d != nil && withinQuery(dateOnly(*d), q) {
			continue
		}
		if bad == 0 {
			first = app.UID
		}
		bad++
	}
	if bad == 0 {
		return
	}
	o.mu.Lock()
	o.counts.Violations += bad
	o.mu.Unlock()
	o.log.ErrorContext(ctx, "poll.window_violation",
		slog.String("work", string(q.Work)), slog.String("axis", axisName(q.Axis)),
		slog.String("from", q.From.Format(time.DateOnly)), slog.String("to", q.To.Format(time.DateOnly)),
		slog.Int("index", q.Index), slog.Int("violations", bad), slog.String("first_uid", first))
	o.record(ctx, PollEvent{Kind: EventWindowViolation, Axis: q.Axis, Day: dayFor(q), Detail: bad})
}

// PageIngested records delta_seen rows for a delta page and then dispatches
// pending events. A dispatch failure is logged: the events stay pending.
func (o *RunObserver) PageIngested(ctx context.Context, q planit.WindowQuery, p planit.FetchPageResult) error {
	if q.DifferentStart != nil {
		if err := o.writeDeltaSeen(ctx, q, p); err != nil {
			return err
		}
	}
	res, err := o.disp.Dispatch(ctx)
	if err != nil {
		o.log.WarnContext(ctx, "poll.dispatch_failed", slog.Any("error", err))
		return nil
	}
	if res.Surge {
		o.mu.Lock()
		o.counts.Surge = true
		o.mu.Unlock()
		o.record(ctx, PollEvent{Kind: EventSurge})
	}
	return nil
}

// WindowShort records a failed completeness check.
func (o *RunObserver) WindowShort(ctx context.Context, ref WindowRef, distinct, total int) {
	day := ref.Day
	o.record(ctx, PollEvent{Kind: EventWindowShort, Axis: ref.Axis, Day: &day, Detail: total - distinct})
}

// FullReadComplete runs the delta cross-check for the window and, in dev,
// replaces its stored membership.
func (o *RunObserver) FullReadComplete(ctx context.Context, ref WindowRef, startedAt, completedAt time.Time, read map[AppKey]struct{}) error {
	seen, err := o.delta.Take(ctx, ref, startedAt)
	if err != nil {
		return fmt.Errorf("take delta_seen: %w", err)
	}
	missed := 0
	for _, k := range seen {
		if _, ok := read[k]; !ok {
			missed++
		}
	}
	if missed > 0 {
		o.mu.Lock()
		o.counts.Missed += missed
		o.mu.Unlock()
		o.log.ErrorContext(ctx, "poll.window_missed",
			slog.String("axis", axisName(ref.Axis)), slog.String("day", ref.Day.Format(time.DateOnly)), slog.Int("missed", missed))
		day := ref.Day
		o.record(ctx, PollEvent{Kind: EventWindowMissed, Axis: ref.Axis, Day: &day, Detail: missed})
	}
	if o.members != nil {
		keys := make([]AppKey, 0, len(read))
		for k := range read {
			keys = append(keys, k)
		}
		if err := o.members.Replace(ctx, ref, completedAt, keys); err != nil {
			return fmt.Errorf("replace poll_window_member: %w", err)
		}
	}
	return nil
}

func (o *RunObserver) writeDeltaSeen(ctx context.Context, q planit.WindowQuery, p planit.FetchPageResult) error {
	now := o.now()
	local := now.In(budgetLocation)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	oldest := today.AddDate(0, 0, -alertBandMaxAge)
	var rows []DeltaSeen
	for _, app := range p.Applications {
		d := axisDate(app, q.Axis)
		if d == nil {
			continue
		}
		day := dateOnly(*d)
		if day.Before(oldest) || day.After(today) {
			continue
		}
		rows = append(rows, DeltaSeen{Axis: q.Axis, Day: day, UID: app.UID, AreaID: app.AreaID, SeenAt: now})
	}
	if len(rows) == 0 {
		return nil
	}
	if err := o.delta.Upsert(ctx, rows); err != nil {
		return fmt.Errorf("upsert delta_seen: %w", err)
	}
	return nil
}

func (o *RunObserver) record(ctx context.Context, e PollEvent) {
	e.At = o.now()
	if err := o.events.Record(ctx, e); err != nil {
		o.log.WarnContext(ctx, "poll.event_record_failed", slog.String("kind", e.Kind), slog.Any("error", err))
	}
}

func axisDate(app applications.PlanningApplication, axis planit.Axis) *time.Time {
	if axis == planit.AxisDecided {
		return app.DecidedDate
	}
	return app.StartDate
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func withinQuery(d time.Time, q planit.WindowQuery) bool {
	if d.Before(dateOnly(q.From)) {
		return false
	}
	return q.DifferentStart != nil || !d.After(dateOnly(q.To))
}

func dayFor(q planit.WindowQuery) *time.Time {
	if q.DifferentStart != nil {
		return nil
	}
	d := dateOnly(q.From)
	return &d
}
