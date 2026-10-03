package polling

import (
	"slices"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

// HealthLevel is the value of the poll.health span attribute.
type HealthLevel string

// The health levels.
const (
	HealthOK       HealthLevel = "ok"
	HealthDegraded HealthLevel = "degraded"
	HealthCritical HealthLevel = "critical"
)

// HealthReason is one entry of the poll.health_reasons span attribute.
type HealthReason string

// The health reasons, in reporting order.
const (
	ReasonAlertBandUnverified HealthReason = "alert_band_unverified"
	ReasonWindowShort         HealthReason = "window_short"
	ReasonWindowViolation     HealthReason = "window_violation"
	ReasonWindowMissed        HealthReason = "window_missed"
	ReasonEventsLow           HealthReason = "events_low"
	ReasonOracleMiss          HealthReason = "oracle_miss"
	ReasonForbidden           HealthReason = "forbidden"
	ReasonSurge               HealthReason = "surge"
)

const eventsLowFraction = 0.2

// Health is the outcome of one run's health check.
type Health struct {
	Level   HealthLevel
	Reasons []HealthReason
}

// HealthInputs is everything ComputeHealth reads. Windows holds the poll_window
// rows for the alert band. The counters cover the last 24h, except
// ShortWindows (windows short at least twice in the current budget day),
// NewAppDaily14 (the 14 rolling 24h buckets of new_application events before
// the last 24h) and OracleMisses7d.
type HealthInputs struct {
	Windows         []WindowState
	ShortWindows    int
	Violations24h   int
	Missed24h       int
	Forbidden24h    int
	Surge24h        int
	NewAppEvents24h int
	NewAppDaily14   []int
	OracleEnabled   bool
	OracleMisses7d  int
}

// ComputeHealth applies the section 8 rules. It is pure.
func ComputeHealth(in HealthInputs, now time.Time) Health {
	var reasons []HealthReason
	add := func(cond bool, r HealthReason) {
		if cond {
			reasons = append(reasons, r)
		}
	}
	add(alertBandUnverified(in.Windows, now), ReasonAlertBandUnverified)
	add(in.ShortWindows > 0, ReasonWindowShort)
	add(in.Violations24h > 0, ReasonWindowViolation)
	add(in.Missed24h > 0, ReasonWindowMissed)
	add(eventsLow(in, now), ReasonEventsLow)
	add(in.OracleEnabled && in.OracleMisses7d > 0, ReasonOracleMiss)
	add(in.Forbidden24h > 0, ReasonForbidden)
	add(in.Surge24h > 0, ReasonSurge)

	level := HealthOK
	if len(reasons) > 0 {
		level = HealthDegraded
	}
	if in.Forbidden24h > 0 || in.Surge24h > 0 {
		level = HealthCritical
	}
	return Health{Level: level, Reasons: reasons}
}

// alertBandUnverified judges only between 06:00 and 18:00, when the night that
// just ended is the one to verify.
func alertBandUnverified(windows []WindowState, now time.Time) bool {
	local := now.In(budgetLocation)
	if local.Hour() < dayStartHour || local.Hour() >= dayEndHour {
		return false
	}
	nightStart := time.Date(local.Year(), local.Month(), local.Day()-1, dayEndHour, 0, 0, 0, budgetLocation)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	byRef := make(map[WindowRef]WindowState, len(windows))
	for _, w := range windows {
		byRef[WindowRef{Axis: w.Axis, Day: w.Day}] = w
	}
	for age := 0; age <= alertBandMaxAge; age++ {
		for _, axis := range [...]planit.Axis{planit.AxisStart, planit.AxisDecided} {
			w := byRef[WindowRef{Axis: axis, Day: today.AddDate(0, 0, -age)}]
			if w.LastCompleteAt == nil || w.LastCompleteAt.Before(nightStart) {
				return true
			}
		}
	}
	return false
}

func eventsLow(in HealthInputs, now time.Time) bool {
	if wd := now.In(budgetLocation).Weekday(); wd == time.Saturday || wd == time.Sunday {
		return false
	}
	if len(in.NewAppDaily14) == 0 {
		return false
	}
	sorted := slices.Clone(in.NewAppDaily14)
	slices.Sort(sorted)
	n := len(sorted)
	median := float64(sorted[n/2])
	if n%2 == 0 {
		median = float64(sorted[n/2-1]+sorted[n/2]) / 2
	}
	return median > 0 && float64(in.NewAppEvents24h) < median*eventsLowFraction
}
