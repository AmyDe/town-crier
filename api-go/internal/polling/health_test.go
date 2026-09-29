package polling

import (
	"slices"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

func healthyBand(now time.Time) []WindowState {
	local := now.In(budgetLocation)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	done := time.Date(local.Year(), local.Month(), local.Day()-1, 19, 0, 0, 0, budgetLocation)
	var out []WindowState
	for age := 0; age <= alertBandMaxAge; age++ {
		for _, axis := range []planit.Axis{planit.AxisStart, planit.AxisDecided} {
			out = append(out, completeAt(axis, today.AddDate(0, 0, -age), done))
		}
	}
	return out
}

func steadyInputs(now time.Time) HealthInputs {
	return HealthInputs{
		Windows:         healthyBand(now),
		NewAppEvents24h: 1000,
		NewAppDaily14:   []int{900, 1000, 1100, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000},
	}
}

func TestComputeHealth_Matrix(t *testing.T) {
	t.Parallel()
	wed := londonAt(6, 10, 9, 0)
	sat := londonAt(6, 13, 9, 0)
	tests := []struct {
		name   string
		now    time.Time
		mutate func(in *HealthInputs)
		level  HealthLevel
		want   []HealthReason
	}{
		{"steady state is ok", wed, func(*HealthInputs) {}, HealthOK, nil},
		{"alert band window never read", wed, func(in *HealthInputs) { in.Windows = in.Windows[1:] }, HealthDegraded, []HealthReason{ReasonAlertBandUnverified}},
		{"alert band window completed before the night began", wed, func(in *HealthInputs) {
			old := londonAt(6, 9, 17, 59)
			in.Windows[0].LastCompleteAt = &old
		}, HealthDegraded, []HealthReason{ReasonAlertBandUnverified}},
		{"alert band not judged before 06:00", londonAt(6, 10, 5, 59), func(in *HealthInputs) { in.Windows = nil }, HealthOK, nil},
		{"alert band not judged once the next night begins", londonAt(6, 10, 18, 30), func(in *HealthInputs) { in.Windows = nil }, HealthOK, nil},
		{"window short twice in a budget day", wed, func(in *HealthInputs) { in.ShortWindows = 1 }, HealthDegraded, []HealthReason{ReasonWindowShort}},
		{"integrity violation", wed, func(in *HealthInputs) { in.Violations24h = 1 }, HealthDegraded, []HealthReason{ReasonWindowViolation}},
		{"delta saw what the window missed", wed, func(in *HealthInputs) { in.Missed24h = 2 }, HealthDegraded, []HealthReason{ReasonWindowMissed}},
		{"events low on a weekday", wed, func(in *HealthInputs) { in.NewAppEvents24h = 199 }, HealthDegraded, []HealthReason{ReasonEventsLow}},
		{"events at 20 percent of the median are fine", wed, func(in *HealthInputs) { in.NewAppEvents24h = 200 }, HealthOK, nil},
		{"events low ignored at the weekend", sat, func(in *HealthInputs) { in.NewAppEvents24h = 0 }, HealthOK, nil},
		{"no history means no events_low", wed, func(in *HealthInputs) { in.NewAppEvents24h = 0; in.NewAppDaily14 = nil }, HealthOK, nil},
		{"oracle miss in dev", wed, func(in *HealthInputs) { in.OracleEnabled = true; in.OracleMisses7d = 1 }, HealthDegraded, []HealthReason{ReasonOracleMiss}},
		{"oracle miss ignored when oracle is off", wed, func(in *HealthInputs) { in.OracleMisses7d = 1 }, HealthOK, nil},
		{"forbidden is critical", wed, func(in *HealthInputs) { in.Forbidden24h = 1 }, HealthCritical, []HealthReason{ReasonForbidden}},
		{"surge is critical", wed, func(in *HealthInputs) { in.Surge24h = 1 }, HealthCritical, []HealthReason{ReasonSurge}},
		{"critical outranks degraded and every reason is listed", wed, func(in *HealthInputs) {
			in.Surge24h = 1
			in.Violations24h = 3
		}, HealthCritical, []HealthReason{ReasonWindowViolation, ReasonSurge}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in := steadyInputs(tt.now)
			tt.mutate(&in)
			got := ComputeHealth(in, tt.now)
			if got.Level != tt.level || !slices.Equal(got.Reasons, tt.want) {
				t.Fatalf("got %+v, want level %s reasons %v", got, tt.level, tt.want)
			}
		})
	}
}
