package polling

import (
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

func plannerTestConfig() WindowPlanConfig {
	return WindowPlanConfig{
		DeltaSlots:    []CivilTime{{9, 0}, {12, 0}, {15, 0}, {17, 0}},
		DeltaMaxPages: 20,
		DailyCap:      300,
		DayAllowance:  60,
	}
}

func utcDay(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func londonAt(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, londonTZ)
}

func completeAt(axis planit.Axis, day, at time.Time) WindowState {
	return WindowState{Axis: axis, Day: day, LastCompleteAt: &at}
}

// allAlertComplete marks every alert-band window (ages 0..14 from today) complete at at.
func allAlertComplete(today time.Time, at time.Time) []WindowState {
	var out []WindowState
	for age := 0; age <= 14; age++ {
		day := today.AddDate(0, 0, -age)
		out = append(out, completeAt(planit.AxisStart, day, at), completeAt(planit.AxisDecided, day, at))
	}
	return out
}

func TestNextWindowWork_DayRunsOnlyDueDeltas(t *testing.T) {
	t.Parallel()
	cfg := plannerTestConfig()
	today := utcDay(2026, 6, 10)
	slot9 := londonAt(2026, 6, 10, 9, 0)

	tests := []struct {
		name    string
		now     time.Time
		started map[planit.Work]time.Time
		want    planit.Work
	}{
		{"before first slot nothing", londonAt(2026, 6, 10, 8, 59), nil, ""},
		{"at first slot start delta", slot9, nil, planit.WorkDeltaStart},
		{"start delta done since slot runs decided", londonAt(2026, 6, 10, 9, 30),
			map[planit.Work]time.Time{planit.WorkDeltaStart: londonAt(2026, 6, 10, 9, 5)}, planit.WorkDeltaDecided},
		{"both done nothing", londonAt(2026, 6, 10, 9, 30),
			map[planit.Work]time.Time{
				planit.WorkDeltaStart:   londonAt(2026, 6, 10, 9, 5),
				planit.WorkDeltaDecided: londonAt(2026, 6, 10, 9, 20),
			}, ""},
		{"call before the slot does not count", londonAt(2026, 6, 10, 12, 1),
			map[planit.Work]time.Time{
				planit.WorkDeltaStart:   londonAt(2026, 6, 10, 9, 5),
				planit.WorkDeltaDecided: londonAt(2026, 6, 10, 9, 20),
			}, planit.WorkDeltaStart},
		{"latest slot governs late in day", londonAt(2026, 6, 10, 17, 59),
			map[planit.Work]time.Time{planit.WorkDeltaStart: londonAt(2026, 6, 10, 15, 10)}, planit.WorkDeltaStart},
		{"06:00 is day", londonAt(2026, 6, 10, 6, 0), nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			state := WindowPlanState{DeltaStartedAt: tc.started}
			got := NextWindowWork(cfg, state, tc.now, 0)
			if tc.want == "" {
				if got != nil {
					t.Fatalf("want nil, got %+v", got)
				}
				return
			}
			if got == nil || got.Work != tc.want {
				t.Fatalf("want %s, got %+v", tc.want, got)
			}
			wantDiff := today.AddDate(0, 0, -1)
			if got.DifferentStart == nil || !got.DifferentStart.Equal(wantDiff) {
				t.Fatalf("DifferentStart = %v, want %v", got.DifferentStart, wantDiff)
			}
			if !got.From.Equal(today.AddDate(0, 0, -14)) {
				t.Fatalf("From = %v, want today-14", got.From)
			}
			if got.MaxPages != cfg.DeltaMaxPages {
				t.Fatalf("MaxPages = %d", got.MaxPages)
			}
		})
	}
}

func TestNextWindowWork_DayIgnoresDueWindows(t *testing.T) {
	t.Parallel()
	got := NextWindowWork(plannerTestConfig(), WindowPlanState{}, londonAt(2026, 6, 10, 8, 0), 0)
	if got != nil {
		t.Fatalf("want nil in the day, got %+v", got)
	}
}

func TestNextWindowWork_NightOrdersAlertWindowsNewestFirstStartBeforeDecided(t *testing.T) {
	t.Parallel()
	cfg := plannerTestConfig()
	now := londonAt(2026, 6, 10, 20, 0)
	today := utcDay(2026, 6, 10)

	var state WindowPlanState
	var got []WindowRef
	for i := 0; i < 5; i++ {
		w := NextWindowWork(cfg, state, now, 0)
		if w == nil {
			t.Fatal("unexpected nil")
		}
		got = append(got, WindowRef{Axis: w.Axis, Day: w.Day})
		state.Windows = append(state.Windows, completeAt(w.Axis, w.Day, now))
	}
	want := []WindowRef{
		{planit.AxisStart, today},
		{planit.AxisDecided, today},
		{planit.AxisStart, today.AddDate(0, 0, -1)},
		{planit.AxisDecided, today.AddDate(0, 0, -1)},
		{planit.AxisStart, today.AddDate(0, 0, -2)},
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("step %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestNextWindowWork_NightWindowFields(t *testing.T) {
	t.Parallel()
	w := NextWindowWork(plannerTestConfig(), WindowPlanState{}, londonAt(2026, 6, 10, 20, 0), 0)
	if w == nil || w.Work != planit.WorkWindowStart || w.DifferentStart != nil ||
		!w.From.Equal(utcDay(2026, 6, 10)) || !w.To.Equal(utcDay(2026, 6, 10)) {
		t.Fatalf("got %+v", w)
	}
	state := WindowPlanState{Windows: []WindowState{completeAt(planit.AxisStart, utcDay(2026, 6, 10), londonAt(2026, 6, 10, 19, 0))}}
	w = NextWindowWork(plannerTestConfig(), state, londonAt(2026, 6, 10, 20, 0), 0)
	if w == nil || w.Work != planit.WorkWindowDecided {
		t.Fatalf("got %+v", w)
	}
}

func TestNextWindowWork_AlertDueRelativeToStartOfCurrentNight(t *testing.T) {
	t.Parallel()
	cfg := plannerTestConfig()
	tests := []struct {
		name       string
		now        time.Time
		completeAt time.Time
		wantDue    bool
	}{
		{"complete during this night", londonAt(2026, 6, 10, 23, 0), londonAt(2026, 6, 10, 18, 30), false},
		{"complete before this night began", londonAt(2026, 6, 10, 23, 0), londonAt(2026, 6, 10, 17, 59), true},
		{"complete exactly at 18:00", londonAt(2026, 6, 10, 23, 0), londonAt(2026, 6, 10, 18, 0), false},
		{"after midnight, last evening's read counts", londonAt(2026, 6, 11, 2, 0), londonAt(2026, 6, 10, 19, 0), false},
		{"after midnight, read from the previous night is due", londonAt(2026, 6, 11, 2, 0), londonAt(2026, 6, 9, 23, 0), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			todayLondon := tc.now.In(londonTZ)
			today := utcDay(todayLondon.Year(), todayLondon.Month(), todayLondon.Day())
			state := WindowPlanState{Windows: allAlertComplete(today, tc.completeAt)}
			state.Windows = append(state.Windows, allCoverageComplete(today, tc.now)...)
			got := NextWindowWork(cfg, state, tc.now, 0)
			if (got != nil) != tc.wantDue {
				t.Fatalf("due = %v, want %v (%+v)", got != nil, tc.wantDue, got)
			}
		})
	}
}

func allCoverageComplete(today, at time.Time) []WindowState {
	var out []WindowState
	for age := 15; age <= 90; age++ {
		day := today.AddDate(0, 0, -age)
		out = append(out, completeAt(planit.AxisStart, day, at), completeAt(planit.AxisDecided, day, at))
	}
	return out
}

func TestNextWindowWork_BandEdges(t *testing.T) {
	t.Parallel()
	cfg := plannerTestConfig()
	now := londonAt(2026, 6, 10, 20, 0)
	today := utcDay(2026, 6, 10)

	// Only the window at the given age is missing; everything else is complete.
	only := func(age int) *PlannedWork {
		var state WindowPlanState
		for a := 0; a <= 91; a++ {
			for _, ax := range []planit.Axis{planit.AxisStart, planit.AxisDecided} {
				if a == age && ax == planit.AxisStart {
					continue
				}
				state.Windows = append(state.Windows, completeAt(ax, today.AddDate(0, 0, -a), now))
			}
		}
		return NextWindowWork(cfg, state, now, 0)
	}
	for _, age := range []int{0, 14, 15, 90} {
		w := only(age)
		if w == nil || !w.Day.Equal(today.AddDate(0, 0, -age)) {
			t.Fatalf("age %d: got %+v", age, w)
		}
	}
	if w := only(91); w != nil {
		t.Fatalf("age 91 must never be read, got %+v", w)
	}
}

func TestNextWindowWork_CoverageDueAfterSevenDays(t *testing.T) {
	t.Parallel()
	cfg := plannerTestConfig()
	now := londonAt(2026, 6, 10, 20, 0)
	today := utcDay(2026, 6, 10)
	tests := []struct {
		name string
		last *time.Time
		due  bool
	}{
		{"never read", nil, true},
		{"read 8 days ago", tp(now.AddDate(0, 0, -8)), true},
		{"read 6 days ago", tp(now.AddDate(0, 0, -6)), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			state := WindowPlanState{Windows: allAlertComplete(today, now)}
			for age := 15; age <= 90; age++ {
				for _, ax := range []planit.Axis{planit.AxisStart, planit.AxisDecided} {
					ws := WindowState{Axis: ax, Day: today.AddDate(0, 0, -age), LastCompleteAt: tp(now)}
					if age == 20 && ax == planit.AxisDecided {
						ws.LastCompleteAt = tc.last
					}
					state.Windows = append(state.Windows, ws)
				}
			}
			got := NextWindowWork(cfg, state, now, 0)
			if (got != nil) != tc.due {
				t.Fatalf("due = %v, want %v", got != nil, tc.due)
			}
			if got != nil && (got.Axis != planit.AxisDecided || !got.Day.Equal(today.AddDate(0, 0, -20))) {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func tp(t time.Time) *time.Time { return &t }

func TestNextWindowWork_AlertBeforeCoverageAndNewestCoverageFirst(t *testing.T) {
	t.Parallel()
	cfg := plannerTestConfig()
	now := londonAt(2026, 6, 10, 20, 0)
	today := utcDay(2026, 6, 10)

	w := NextWindowWork(cfg, WindowPlanState{}, now, 0)
	if w == nil || !w.Day.Equal(today) {
		t.Fatalf("alert must come first, got %+v", w)
	}

	state := WindowPlanState{Windows: allAlertComplete(today, now)}
	w = NextWindowWork(cfg, state, now, 0)
	if w == nil || w.Work != planit.WorkWindowStart || !w.Day.Equal(today.AddDate(0, 0, -15)) {
		t.Fatalf("want newest coverage window, got %+v", w)
	}
}

func TestNextWindowWork_CoverageStopsAtCapMinusAllowance(t *testing.T) {
	t.Parallel()
	cfg := plannerTestConfig()
	now := londonAt(2026, 6, 10, 20, 0)
	today := utcDay(2026, 6, 10)
	state := WindowPlanState{Windows: allAlertComplete(today, now)}

	if w := NextWindowWork(cfg, state, now, 239); w == nil {
		t.Fatal("239 < 240 should still read coverage")
	}
	if w := NextWindowWork(cfg, state, now, 240); w != nil {
		t.Fatalf("240 is the cut-off, got %+v", w)
	}

	unverified := WindowPlanState{}
	if w := NextWindowWork(cfg, unverified, now, 290); w == nil {
		t.Fatal("alert windows ignore the allowance")
	}
}

func TestNextWindowWork_DSTBoundaries(t *testing.T) {
	t.Parallel()
	cfg := plannerTestConfig()
	tests := []struct {
		name    string
		now     time.Time
		wantDay bool
	}{
		{"spring forward 05:59 BST is night", londonAt(2026, 3, 29, 5, 59), false},
		{"spring forward 06:00 BST is day", londonAt(2026, 3, 29, 6, 0), true},
		{"spring forward 17:59 is day", londonAt(2026, 3, 29, 17, 59), true},
		{"spring forward 18:00 is night", londonAt(2026, 3, 29, 18, 0), false},
		{"fall back 05:59 GMT is night", londonAt(2026, 10, 25, 5, 59), false},
		{"fall back 06:00 GMT is day", londonAt(2026, 10, 25, 6, 0), true},
		{"fall back 18:00 GMT is night", londonAt(2026, 10, 25, 18, 0), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := NextWindowWork(cfg, WindowPlanState{}, tc.now, 0)
			if tc.wantDay {
				// A day with no delta due yet or a delta; never a window.
				if w != nil && (w.Work == planit.WorkWindowStart || w.Work == planit.WorkWindowDecided) {
					t.Fatalf("window in the day: %+v", w)
				}
				return
			}
			if w == nil || w.Work != planit.WorkWindowStart {
				t.Fatalf("want night window, got %+v", w)
			}
		})
	}
}

func TestNextWindowWork_DSTNightStartAndAge(t *testing.T) {
	t.Parallel()
	cfg := plannerTestConfig()
	// 00:30 BST on 29 Mar is not yet BST for the previous evening: night began 18:00 GMT on the 28th.
	now := londonAt(2026, 3, 29, 0, 30)
	today := utcDay(2026, 3, 29)
	state := WindowPlanState{Windows: allAlertComplete(today, londonAt(2026, 3, 28, 18, 30))}
	state.Windows = append(state.Windows, allCoverageComplete(today, now)...)
	if w := NextWindowWork(cfg, state, now, 0); w != nil {
		t.Fatalf("read since 18:00 must not be due, got %+v", w)
	}

	// 00:30 BST on 11 Jun is 23:30 UTC on the 10th: the London date is the 11th.
	w := NextWindowWork(cfg, WindowPlanState{}, londonAt(2026, 6, 11, 0, 30), 0)
	if w == nil || !w.Day.Equal(utcDay(2026, 6, 11)) {
		t.Fatalf("age is measured in London days, got %+v", w)
	}
}
