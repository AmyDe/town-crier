package polling

import (
	"fmt"
	"time"

	// Bundles IANA tzdata so Europe/London resolves in a minimal container.
	_ "time/tzdata"

	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

const (
	alertBandMaxAge    = 14
	coverageBandMaxAge = 90
	coverageMaxStale   = 7 * 24 * time.Hour
	deltaMaskDays      = 14
	dayStartHour       = 6
	dayEndHour         = 18
)

// CivilTime is a wall-clock hour and minute in Europe/London.
type CivilTime struct {
	Hour   int
	Minute int
}

// ParseCivilTime parses a 24-hour "HH:MM" config value.
func ParseCivilTime(s string) (CivilTime, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return CivilTime{}, fmt.Errorf("parse civil time %q: %w", s, err)
	}
	return CivilTime{Hour: t.Hour(), Minute: t.Minute()}, nil
}

// WindowPlanConfig holds the planner limits (POLLING_DELTA_SLOTS,
// POLLING_DELTA_MAX_PAGES, POLLING_DAILY_CALL_CAP, POLLING_DAY_ALLOWANCE).
type WindowPlanConfig struct {
	DeltaSlots    []CivilTime
	DeltaMaxPages int
	DailyCap      int
	DayAllowance  int
}

// WindowRef identifies one day window. Day is a date at UTC midnight.
type WindowRef struct {
	Axis planit.Axis
	Day  time.Time
}

// WindowState is one poll_window row.
type WindowState struct {
	Axis           planit.Axis
	Day            time.Time
	LastCompleteAt *time.Time
	LastFullReadAt *time.Time
	LastTotal      *int
}

// WindowPlanState is everything NextWindowWork reads. Windows holds the
// poll_window rows for the 0 to 90 day range (a missing row is a window never
// read). DeltaStartedAt maps each delta work to its latest planit_call with
// page_index 0.
type WindowPlanState struct {
	Windows        []WindowState
	DeltaStartedAt map[planit.Work]time.Time
}

// PlannedWork is one unit of PlanIt work chosen by NextWindowWork. For a
// window read From and To are both Day. For a delta From is the axis mask date
// and DifferentStart is set.
type PlannedWork struct {
	Work           planit.Work
	Axis           planit.Axis
	Day            time.Time
	From, To       time.Time
	DifferentStart *time.Time
	MaxPages       int
}

// NextWindowWork chooses the next work item, or nil when the run should end.
// It is pure: all clocks are Europe/London and nothing is read but its
// arguments. In the day (06:00 to 18:00) only a due delta pass runs; at night
// due alert-band windows come first (newest day first, start before decided),
// then coverage-band windows while callsToday stays below cap minus allowance.
func NextWindowWork(cfg WindowPlanConfig, state WindowPlanState, now time.Time, callsToday int) *PlannedWork {
	local := now.In(budgetLocation)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)

	if local.Hour() >= dayStartHour && local.Hour() < dayEndHour {
		return nextDelta(cfg, state, local, today)
	}

	nightStart, _ := BudgetDay(now)
	byRef := make(map[WindowRef]WindowState, len(state.Windows))
	for _, w := range state.Windows {
		byRef[WindowRef{Axis: w.Axis, Day: w.Day}] = w
	}
	axes := [...]planit.Axis{planit.AxisStart, planit.AxisDecided}

	for age := 0; age <= alertBandMaxAge; age++ {
		day := today.AddDate(0, 0, -age)
		for _, axis := range axes {
			w := byRef[WindowRef{Axis: axis, Day: day}]
			if w.LastCompleteAt == nil || w.LastCompleteAt.Before(nightStart) {
				return windowWork(axis, day)
			}
		}
	}

	if callsToday >= cfg.DailyCap-cfg.DayAllowance {
		return nil
	}
	for age := alertBandMaxAge + 1; age <= coverageBandMaxAge; age++ {
		day := today.AddDate(0, 0, -age)
		for _, axis := range axes {
			w := byRef[WindowRef{Axis: axis, Day: day}]
			if w.LastCompleteAt == nil || now.Sub(*w.LastCompleteAt) > coverageMaxStale {
				return windowWork(axis, day)
			}
		}
	}
	return nil
}

func nextDelta(cfg WindowPlanConfig, state WindowPlanState, local, today time.Time) *PlannedWork {
	var slot time.Time
	found := false
	for _, s := range cfg.DeltaSlots {
		t := time.Date(local.Year(), local.Month(), local.Day(), s.Hour, s.Minute, 0, 0, budgetLocation)
		if t.After(local) {
			continue
		}
		if !found || t.After(slot) {
			slot, found = t, true
		}
	}
	if !found {
		return nil
	}
	diff := today.AddDate(0, 0, -1)
	from := today.AddDate(0, 0, -deltaMaskDays)
	for _, c := range [...]struct {
		work planit.Work
		axis planit.Axis
	}{
		{planit.WorkDeltaStart, planit.AxisStart},
		{planit.WorkDeltaDecided, planit.AxisDecided},
	} {
		if started, ok := state.DeltaStartedAt[c.work]; ok && !started.Before(slot) {
			continue
		}
		return &PlannedWork{Work: c.work, Axis: c.axis, From: from, DifferentStart: &diff, MaxPages: cfg.DeltaMaxPages}
	}
	return nil
}

func windowWork(axis planit.Axis, day time.Time) *PlannedWork {
	work := planit.WorkWindowStart
	if axis == planit.AxisDecided {
		work = planit.WorkWindowDecided
	}
	return &PlannedWork{Work: work, Axis: axis, Day: day, From: day, To: day}
}
