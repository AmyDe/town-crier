// Package appevents drains the application_event outbox: it applies quiet
// hours, the age limit and the surge check, then fans each surviving event out
// to watch-zone notifications.
package appevents

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

// Event kinds and statuses match the application_event CHECK constraints.
const (
	KindNewApplication = "new_application"
	KindDecision       = "decision"

	StatusPending = "pending"
	StatusSent    = "sent"
	StatusStale   = "stale"
)

// Event is one pending outbox row.
type Event struct {
	ID            int64
	UID           string
	AuthorityCode string
	Kind          string
	EventDate     *time.Time
	DetectedAt    time.Time
}

// Config holds the dispatcher's tunables. QuietStart and QuietEnd are offsets
// from midnight Europe/London; the quiet window is [QuietStart, QuietEnd) and
// wraps midnight when QuietStart is later than QuietEnd.
type Config struct {
	QuietStart     time.Duration
	QuietEnd       time.Duration
	SurgeThreshold int
	MaxAgeDays     int
}

// DefaultConfig returns quiet hours 22:00-07:00, a surge threshold of 10,000
// events per 24h and a 14-day age limit.
func DefaultConfig() Config {
	return Config{QuietStart: 22 * time.Hour, QuietEnd: 7 * time.Hour, SurgeThreshold: 10000, MaxAgeDays: 14}
}

// ParseTimeOfDay parses "HH:MM" into an offset from midnight.
func ParseTimeOfDay(s string) (time.Duration, error) {
	h, m, ok := strings.Cut(s, ":")
	hh, errH := strconv.Atoi(h)
	mm, errM := strconv.Atoi(m)
	if !ok || errH != nil || errM != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, fmt.Errorf("invalid time of day %q, want HH:MM", s)
	}
	return time.Duration(hh)*time.Hour + time.Duration(mm)*time.Minute, nil
}

// Result reports what one Dispatch call did. Surge is true when the surge check
// tripped and every pending event was marked stale; the caller surfaces it as
// the appevents.surge health signal.
type Result struct {
	Quiet bool
	Surge bool
	Sent  int
	Stale int
}

type eventStore interface {
	// Pending returns pending events, oldest detected_at first.
	Pending(ctx context.Context) ([]Event, error)
	MarkStale(ctx context.Context, ids []int64, at time.Time) error
	MarkSent(ctx context.Context, id int64, at time.Time) error
	// CountActiveSince counts non-stale events detected at or after since.
	CountActiveSince(ctx context.Context, since time.Time) (int, error)
}

type applicationLoader interface {
	GetByUID(ctx context.Context, uid, authorityCode string) (applications.PlanningApplication, bool, error)
}

type decisionDispatcher interface {
	Dispatch(ctx context.Context, app applications.PlanningApplication) error
}

type enqueuer interface {
	EnqueueForApplication(ctx context.Context, app applications.PlanningApplication, detectedAt time.Time) error
}

type pushFlusher interface {
	Flush(ctx context.Context) error
}

// Dispatcher drains application_event. It holds no state between calls.
type Dispatcher struct {
	store    eventStore
	apps     applicationLoader
	decision decisionDispatcher
	enqueuer enqueuer
	push     pushFlusher
	cfg      Config
	now      func() time.Time
	logger   *slog.Logger
	london   *time.Location
}

// NewDispatcher wires a Dispatcher.
func NewDispatcher(store eventStore, apps applicationLoader, decision decisionDispatcher, enq enqueuer, push pushFlusher, cfg Config, now func() time.Time, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{store: store, apps: apps, decision: decision, enqueuer: enq, push: push, cfg: cfg, now: now, logger: logger, london: londonLocation()}
}

// Dispatch processes pending events once. Events stay pending during quiet
// hours and when a fan-out fails, so the next call retries them.
func (d *Dispatcher) Dispatch(ctx context.Context) (Result, error) {
	now := d.now()
	local := now.In(d.london)
	if d.inQuietHours(local) {
		return Result{Quiet: true}, nil
	}

	pending, err := d.store.Pending(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("load pending events: %w", err)
	}

	var res Result
	var staleIDs []int64
	var live []Event
	today := civilDay(local)
	for _, e := range pending {
		if today.Sub(d.effectiveDate(e)) > time.Duration(d.cfg.MaxAgeDays)*24*time.Hour {
			staleIDs = append(staleIDs, e.ID)
			continue
		}
		live = append(live, e)
	}
	if err := d.markStale(ctx, staleIDs, now); err != nil {
		return res, err
	}
	res.Stale = len(staleIDs)

	count, err := d.store.CountActiveSince(ctx, now.Add(-24*time.Hour))
	if err != nil {
		return res, fmt.Errorf("count recent events: %w", err)
	}
	if count > d.cfg.SurgeThreshold {
		ids := make([]int64, len(live))
		for i, e := range live {
			ids[i] = e.ID
		}
		if err := d.markStale(ctx, ids, now); err != nil {
			return res, err
		}
		res.Stale += len(ids)
		res.Surge = true
		d.logger.ErrorContext(ctx, "appevents surge: pending events marked stale", "appeventsSurge", true, "count", count, "threshold", d.cfg.SurgeThreshold)
		return res, nil
	}

	for _, e := range live {
		app, found, err := d.apps.GetByUID(ctx, e.UID, e.AuthorityCode)
		if err != nil {
			return res, fmt.Errorf("load application %q: %w", e.UID, err)
		}
		if !found {
			d.logger.ErrorContext(ctx, "appevents: application missing, marking event stale", "uid", e.UID, "authorityCode", e.AuthorityCode, "eventId", e.ID)
			if err := d.markStale(ctx, []int64{e.ID}, now); err != nil {
				return res, err
			}
			res.Stale++
			continue
		}
		if err := d.fanOut(ctx, e, app); err != nil {
			return res, fmt.Errorf("fan out event %d: %w", e.ID, err)
		}
		if err := d.store.MarkSent(ctx, e.ID, d.now()); err != nil {
			return res, fmt.Errorf("mark event %d sent: %w", e.ID, err)
		}
		res.Sent++
	}
	return res, nil
}

// Flush sends the coalesced pushes queued by Dispatch: at most one per
// (user, zone) per run.
func (d *Dispatcher) Flush(ctx context.Context) error {
	return d.push.Flush(ctx)
}

func (d *Dispatcher) fanOut(ctx context.Context, e Event, app applications.PlanningApplication) error {
	if e.Kind == KindDecision {
		if err := d.decision.Dispatch(ctx, app); err != nil {
			return err
		}
	}
	return d.enqueuer.EnqueueForApplication(ctx, app, e.DetectedAt)
}

func (d *Dispatcher) markStale(ctx context.Context, ids []int64, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	if err := d.store.MarkStale(ctx, ids, at); err != nil {
		return fmt.Errorf("mark events stale: %w", err)
	}
	return nil
}

func (d *Dispatcher) effectiveDate(e Event) time.Time {
	if e.EventDate != nil {
		return civilDay(*e.EventDate)
	}
	return civilDay(e.DetectedAt.In(d.london))
}

func (d *Dispatcher) inQuietHours(local time.Time) bool {
	offset := time.Duration(local.Hour())*time.Hour + time.Duration(local.Minute())*time.Minute + time.Duration(local.Second())*time.Second
	if d.cfg.QuietStart <= d.cfg.QuietEnd {
		return offset >= d.cfg.QuietStart && offset < d.cfg.QuietEnd
	}
	return offset >= d.cfg.QuietStart || offset < d.cfg.QuietEnd
}

// civilDay drops the time of day, keeping the calendar date in t's own zone, so
// day differences are exact multiples of 24h.
func civilDay(t time.Time) time.Time {
	y, m, day := t.Date()
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}
