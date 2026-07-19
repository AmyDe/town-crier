// ADR 0044: the pure planner. NextWork picks the next lane for the executor
// loop in NationalPollHandler.Handle to run, given typed state (loaded fresh
// from the stores by Handle before every call — a handful of cheap point
// reads, not a full LRU scan; there are only 4 lanes) and the clock. It
// performs NO I/O and NO writes: every decision is a pure function of its
// two arguments, which is what makes it exhaustively unit-testable with
// nothing beyond an injected clock.
package polling

import (
	"fmt"
	"time"

	// Bundles the full IANA tzdata into the binary so "Europe/London" always
	// resolves, even on a minimal container base image with no system
	// tzdata (ADR 0044: nothing in this codebase called time.LoadLocation
	// before this package, so this gap was previously untested).
	_ "time/tzdata"
)

// LaneD is ADR 0042's historical backfill lane, tagged into the same
// single-letter vocabulary LaneA/LaneB/LaneC already use so the planner can
// select uniformly across all four lanes. Lane D's own telemetry
// (backfillMetricsLane in backfill.go) predates this and is left as its own
// constant — both hold the same "D" string, and backfill.go is unchanged
// (ADR 0044: "Lane D unchanged internally").
const LaneD LaneName = "D"

// CivilTime is a wall-clock hour:minute, used to bound Lane C's daytime
// eligibility window (ADR 0044 §3) independent of any date — the caller
// supplies the *time.Location the comparison happens in.
type CivilTime struct {
	Hour   int
	Minute int
}

// ParseCivilTime parses a 24-hour "HH:MM" config value (POLLING_DAY_START /
// POLLING_DAY_END, e.g. "07:00").
func ParseCivilTime(s string) (CivilTime, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return CivilTime{}, fmt.Errorf("parse civil time %q: %w", s, err)
	}
	return CivilTime{Hour: t.Hour(), Minute: t.Minute()}, nil
}

// minutesOfDay converts an hour:minute pair to minutes since local midnight,
// for a simple integer window comparison.
func (c CivilTime) minutesOfDay() int {
	return c.Hour*60 + c.Minute
}

// LaneState is one A/B/C lane's planner-relevant slice of its persisted
// poll_state: when it last ran (drives LRU ordering) and its currently
// active resume cursor, if any (non-nil means mid-drain, which makes the
// lane due regardless of the freshness interval — ADR 0044 §3).
type LaneState struct {
	LastPollTime time.Time
	Cursor       *PollCursor
}

// LaneDState is Lane D's (ADR 0042) planner-relevant state: its own
// LastRunTime for LRU ordering, and whether it has declared itself Complete
// (BackfillState.Complete) — a Complete lane never has work again.
type LaneDState struct {
	LastPollTime time.Time
	Complete     bool
}

// PlannerState is the pure planner's typed snapshot of every lane's
// planner-relevant state. LaneC and LaneD are pointers: nil means the lane
// is not wired at all (POLLING_BACKFILL_ENABLED off, or a test exercising a
// narrower lane set) — NextWork must never pick a lane that is not present,
// however eligible it would otherwise be. LaneA and LaneB are always
// present: NewNationalPollHandler requires both non-nil at construction.
type PlannerState struct {
	LaneA LaneState
	LaneB LaneState
	LaneC *LaneState
	LaneD *LaneDState
}

// WorkItem is the next lane the executor loop should run this iteration, as
// chosen by NextWork: a lane name plus that lane's currently-known resume
// cursor (nil when it has none). The one-page executor for A/B/C
// independently re-reads its own persisted state before building its PlanIt
// query, so a WorkItem.Cursor that goes stale between NextWork returning and
// the executor running can never cause a skip — it exists so a caller that
// wants the cursor already has it, without a second lookup.
type WorkItem struct {
	Lane   LaneName
	Cursor *PollCursor
}

// PlannerOptions configure NextWork's eligibility windows (ADR 0044 §3).
type PlannerOptions struct {
	// FreshnessInterval is how often A/B are due absent an active cursor
	// (POLLING_LANE_FRESHNESS_INTERVAL, default 15m).
	FreshnessInterval time.Duration
	// DayStart and DayEnd bound Lane C's daytime eligibility window in
	// Location (POLLING_DAY_START / POLLING_DAY_END, default 07:00/19:00).
	// Lane D is eligible exactly outside this window.
	DayStart CivilTime
	DayEnd   CivilTime
	// Location is the timezone the daytime window is evaluated in
	// (Europe/London in production — see the package's tzdata import).
	Location *time.Location
}

// Planner is the pure ADR 0044 planner. NextWork performs no I/O and no
// writes: Handle is responsible for loading PlannerState from the stores
// before every call.
type Planner struct {
	opts PlannerOptions
}

// NewPlanner wires a Planner over the given eligibility-window options.
func NewPlanner(opts PlannerOptions) *Planner {
	return &Planner{opts: opts}
}

// Eligible reports whether lane may run AT ALL right now, independent of
// whether it currently has work (ADR 0044 §3's "Eligible" column): A and B
// are eligible 24/7; C only inside the daytime window; D only outside it.
func (p *Planner) Eligible(lane LaneName, now time.Time) bool {
	switch lane {
	case LaneA, LaneB:
		return true
	case LaneC:
		return p.daytime(now)
	case LaneD:
		return !p.daytime(now)
	default:
		return false
	}
}

// daytime reports whether now falls within [DayStart, DayEnd) in
// p.opts.Location — the wall-clock comparison DST-correctness depends on:
// converting to local time via time.Time.In before comparing, rather than
// operating on now's UTC clock fields directly.
func (p *Planner) daytime(now time.Time) bool {
	local := now.In(p.opts.Location)
	return withinWindow(local, p.opts.DayStart, p.opts.DayEnd)
}

// withinWindow reports whether local's time-of-day falls within
// [start, end). Handles both a window that does not wrap past midnight (the
// expected shape — ADR 0044's default 07:00-19:00 never wraps) and one that
// does, for robustness against a future reconfiguration.
func withinWindow(local time.Time, start, end CivilTime) bool {
	minutesOfDay := local.Hour()*60 + local.Minute()
	startMin := start.minutesOfDay()
	endMin := end.minutesOfDay()
	if startMin <= endMin {
		return minutesOfDay >= startMin && minutesOfDay < endMin
	}
	return minutesOfDay >= startMin || minutesOfDay < endMin
}

// hasWorkAB reports whether an A/B lane has work right now: a lane with an
// active mid-drain cursor always has work regardless of the freshness
// interval; otherwise it is due when it has never run or the freshness
// interval has elapsed since its last run.
func hasWorkAB(s LaneState, now time.Time, freshness time.Duration) bool {
	if s.Cursor != nil {
		return true
	}
	return s.LastPollTime.IsZero() || now.Sub(s.LastPollTime) >= freshness
}

// laneCIdleAnchorInterval bounds how often Lane C anchors a BRAND NEW epoch
// once it has no backlog to walk — ADR 0044 §5/§6's "daily epoch cadence"
// (no dedicated env var is named for it, unlike FreshnessInterval or the
// day window, so this is a hardcoded constant, mirroring resumeOverlapRecords
// in handler.go). Without this gate, a fully caught-up Lane C would anchor a
// fresh near-zero-width epoch on every single planner iteration it wins
// (last_different-ascending epochs are pinned to "now", so a caught-up lane
// always has SOME sliver of a new epoch available) and busy-loop issuing
// near-empty requests for the rest of every daytime cycle — a request-volume
// hammering risk distinct from (and not covered by) the rows-served metric
// ADR 0041/0044 are built around. A lane with an ACTIVE cursor (a genuine
// backlog mid-drain) is exempt: it must keep grinding without waiting out
// this interval, or a real backlog would take up to a day per epoch to
// clear.
const laneCIdleAnchorInterval = 24 * time.Hour

// hasWorkC reports whether Lane C currently has unwalked epoch pages: an
// active cursor (mid-epoch) always has work, so a genuine backlog drains
// without delay; otherwise a new epoch is anchored only once
// laneCIdleAnchorInterval has elapsed since Lane C's last run, so a fully
// caught-up lane settles to a quiet daily check instead of busy-anchoring a
// fresh near-empty epoch every time it is picked.
func hasWorkC(s LaneState, now time.Time) bool {
	if s.Cursor != nil {
		return true
	}
	return s.LastPollTime.IsZero() || now.Sub(s.LastPollTime) >= laneCIdleAnchorInterval
}

// NextWork picks the next lane to run: among every eligible lane that
// currently has work, the one with the oldest LastPollTime (LRU / round-
// robin — ADR 0044 §3). Returns nil when nothing eligible has work
// (everything is caught up — the "Natural" cycle-end case).
func (p *Planner) NextWork(state PlannerState, now time.Time) *WorkItem {
	type candidate struct {
		lane         LaneName
		lastPollTime time.Time
		cursor       *PollCursor
	}
	var candidates []candidate

	if p.Eligible(LaneA, now) && hasWorkAB(state.LaneA, now, p.opts.FreshnessInterval) {
		candidates = append(candidates, candidate{LaneA, state.LaneA.LastPollTime, state.LaneA.Cursor})
	}
	if p.Eligible(LaneB, now) && hasWorkAB(state.LaneB, now, p.opts.FreshnessInterval) {
		candidates = append(candidates, candidate{LaneB, state.LaneB.LastPollTime, state.LaneB.Cursor})
	}
	if state.LaneC != nil && p.Eligible(LaneC, now) && hasWorkC(*state.LaneC, now) {
		candidates = append(candidates, candidate{LaneC, state.LaneC.LastPollTime, state.LaneC.Cursor})
	}
	if state.LaneD != nil && p.Eligible(LaneD, now) && !state.LaneD.Complete {
		candidates = append(candidates, candidate{LaneD, state.LaneD.LastPollTime, nil})
	}

	if len(candidates) == 0 {
		return nil
	}

	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.lastPollTime.Before(best.lastPollTime) {
			best = c
		}
	}
	return &WorkItem{Lane: best.lane, Cursor: best.cursor}
}
