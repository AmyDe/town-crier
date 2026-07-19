package polling

import (
	"testing"
	"time"
)

// london is the Europe/London location every planner test compares local
// eligibility windows against (ADR 0044 §3/§6's DST acceptance criterion).
// Loaded directly here (not via the package's cached location) so a test
// failure to load can never be masked by a shared package-level fallback.
func london(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Fatalf("load Europe/London: %v", err)
	}
	return loc
}

func testPlannerOptions(t *testing.T) PlannerOptions {
	t.Helper()
	dayStart, err := ParseCivilTime("07:00")
	if err != nil {
		t.Fatalf("ParseCivilTime(07:00): %v", err)
	}
	dayEnd, err := ParseCivilTime("19:00")
	if err != nil {
		t.Fatalf("ParseCivilTime(19:00): %v", err)
	}
	return PlannerOptions{
		FreshnessInterval: 15 * time.Minute,
		DayStart:          dayStart,
		DayEnd:            dayEnd,
		Location:          london(t),
	}
}

// TestParseCivilTime covers the "HH:MM" config shape (POLLING_DAY_START /
// POLLING_DAY_END) and its failure mode.
func TestParseCivilTime(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		in         string
		wantHour   int
		wantMinute int
		wantErr    bool
	}{
		{name: "day start default", in: "07:00", wantHour: 7, wantMinute: 0},
		{name: "day end default", in: "19:00", wantHour: 19, wantMinute: 0},
		{name: "midnight", in: "00:00", wantHour: 0, wantMinute: 0},
		{name: "with minutes", in: "07:30", wantHour: 7, wantMinute: 30},
		{name: "malformed", in: "not-a-time", wantErr: true},
		{name: "empty", in: "", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseCivilTime(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseCivilTime(%q): err=%v, wantErr=%v", tc.in, err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if got.Hour != tc.wantHour || got.Minute != tc.wantMinute {
				t.Errorf("ParseCivilTime(%q): got %02d:%02d, want %02d:%02d", tc.in, got.Hour, got.Minute, tc.wantHour, tc.wantMinute)
			}
		})
	}
}

// TestPlanner_Eligible pins ADR 0044 §3's eligibility table, independent of
// has-work: A/B are eligible 24/7; C only inside the Europe/London daytime
// window; D only outside it. Includes the GH#978 acceptance-criterion clock
// points (03:00 and 15:00) plus GMT/BST boundary and DST-transition cases
// that a naive UTC-only comparison would get wrong.
func TestPlanner_Eligible(t *testing.T) {
	t.Parallel()
	p := NewPlanner(testPlannerOptions(t))

	tests := []struct {
		name     string
		utc      time.Time
		lane     LaneName
		eligible bool
	}{
		// GH#978 acceptance criterion: A/B at 03:00 and 15:00 local (winter, GMT).
		{"A at 03:00 GMT", time.Date(2026, 1, 15, 3, 0, 0, 0, time.UTC), LaneA, true},
		{"A at 15:00 GMT", time.Date(2026, 1, 15, 15, 0, 0, 0, time.UTC), LaneA, true},
		{"B at 03:00 GMT", time.Date(2026, 1, 15, 3, 0, 0, 0, time.UTC), LaneB, true},
		{"B at 15:00 GMT", time.Date(2026, 1, 15, 15, 0, 0, 0, time.UTC), LaneB, true},
		{"C at 15:00 GMT: eligible", time.Date(2026, 1, 15, 15, 0, 0, 0, time.UTC), LaneC, true},
		{"C at 03:00 GMT: not eligible", time.Date(2026, 1, 15, 3, 0, 0, 0, time.UTC), LaneC, false},
		{"D at 03:00 GMT: eligible", time.Date(2026, 1, 15, 3, 0, 0, 0, time.UTC), LaneD, true},
		{"D at 15:00 GMT: not eligible", time.Date(2026, 1, 15, 15, 0, 0, 0, time.UTC), LaneD, false},

		// Window boundaries (GMT, no DST offset in play).
		{"C just before window opens (06:59 GMT)", time.Date(2026, 1, 15, 6, 59, 0, 0, time.UTC), LaneC, false},
		{"C exactly at window open (07:00 GMT)", time.Date(2026, 1, 15, 7, 0, 0, 0, time.UTC), LaneC, true},
		{"C just before window closes (18:59 GMT)", time.Date(2026, 1, 15, 18, 59, 0, 0, time.UTC), LaneC, true},
		{"C exactly at window close (19:00 GMT)", time.Date(2026, 1, 15, 19, 0, 0, 0, time.UTC), LaneC, false},

		// BST (summer, UTC+1): the same UTC clock reading (06:30) that is
		// "not yet daytime" in GMT is "already daytime" in BST — proves the
		// comparison genuinely runs in Europe/London local time, not UTC.
		{"C at 06:30 UTC in BST (07:30 local: eligible)", time.Date(2026, 7, 15, 6, 30, 0, 0, time.UTC), LaneC, true},
		{"C at 06:30 UTC in GMT (06:30 local: not eligible)", time.Date(2026, 1, 15, 6, 30, 0, 0, time.UTC), LaneC, false},
		{"C at 18:59 UTC in BST (19:59 local: past close)", time.Date(2026, 7, 15, 18, 59, 0, 0, time.UTC), LaneC, false},

		// DST transition days themselves (UK 2026: spring-forward 2026-03-29,
		// fall-back 2026-10-25).
		{"C on spring-forward day, after the 01:00 UTC transition (07:30 BST)", time.Date(2026, 3, 29, 6, 30, 0, 0, time.UTC), LaneC, true},
		{"C on fall-back day, after the 01:00 UTC transition (06:30 GMT)", time.Date(2026, 10, 25, 6, 30, 0, 0, time.UTC), LaneC, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := p.Eligible(tc.lane, tc.utc)
			if got != tc.eligible {
				t.Errorf("Eligible(%s, %v): got %v, want %v", tc.lane, tc.utc, got, tc.eligible)
			}
		})
	}
}

// TestPlanner_NextWork_ABDueInterval covers Lane A/B's due-vs-not-due
// freshness gate independent of eligibility (which is always true for A/B).
func TestPlanner_NextWork_ABDueInterval(t *testing.T) {
	t.Parallel()
	p := NewPlanner(testPlannerOptions(t))
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC) // 12:00 GMT, outside C's... no, inside; use night instead
	now = time.Date(2026, 1, 15, 3, 0, 0, 0, time.UTC)   // 03:00 GMT: C ineligible, D eligible-but-complete below

	tests := []struct {
		name         string
		lastPollTime time.Time
		hasCursor    bool
		wantWork     bool
	}{
		{"never polled", time.Time{}, false, true},
		{"polled just now: not due", now, false, false},
		{"polled 14m ago: not yet due", now.Add(-14 * time.Minute), false, false},
		{"polled exactly 15m ago: due", now.Add(-15 * time.Minute), false, true},
		{"polled 16m ago: due", now.Add(-16 * time.Minute), false, true},
		{"polled just now but mid-drain: due regardless", now, true, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var cursor *PollCursor
			if tc.hasCursor {
				cursor = &PollCursor{NextIndex: 300}
			}
			state := PlannerState{
				LaneA: LaneState{LastPollTime: tc.lastPollTime, Cursor: cursor},
				// Lane B parked far in the future relative to now so it is
				// never due and never wins the LRU tie-break in this test.
				LaneB: LaneState{LastPollTime: now.Add(time.Hour)},
				LaneD: &LaneDState{Complete: true}, // exclude D from candidacy
			}
			item := p.NextWork(state, now)
			gotWork := item != nil && item.Lane == LaneA
			if gotWork != tc.wantWork {
				t.Errorf("NextWork: got item=%+v, wantWork=%v", item, tc.wantWork)
			}
		})
	}
}

// TestPlanner_NextWork_LRUPicksOldestAmongEligibleWithWork pins the round-
// robin core: among eligible-with-work lanes, the one with the oldest
// LastPollTime wins, regardless of lane ordering.
func TestPlanner_NextWork_LRUPicksOldestAmongEligibleWithWork(t *testing.T) {
	t.Parallel()
	p := NewPlanner(testPlannerOptions(t))
	now := time.Date(2026, 1, 15, 3, 0, 0, 0, time.UTC) // night: C ineligible, D eligible

	tests := []struct {
		name  string
		state PlannerState
		want  LaneName
	}{
		{
			name: "A older than B: A wins",
			state: PlannerState{
				LaneA: LaneState{LastPollTime: now.Add(-20 * time.Minute)},
				LaneB: LaneState{LastPollTime: now.Add(-16 * time.Minute)},
				LaneD: &LaneDState{Complete: true},
			},
			want: LaneA,
		},
		{
			name: "B older than A: B wins",
			state: PlannerState{
				LaneA: LaneState{LastPollTime: now.Add(-16 * time.Minute)},
				LaneB: LaneState{LastPollTime: now.Add(-20 * time.Minute)},
				LaneD: &LaneDState{Complete: true},
			},
			want: LaneB,
		},
		{
			name: "D (eligible, incomplete, never polled) beats A/B not yet due",
			state: PlannerState{
				LaneA: LaneState{LastPollTime: now.Add(-5 * time.Minute)},
				LaneB: LaneState{LastPollTime: now.Add(-5 * time.Minute)},
				LaneD: &LaneDState{LastPollTime: time.Time{}, Complete: false},
			},
			want: LaneD,
		},
		{
			name: "never-polled A beats a D that has already run this cycle",
			state: PlannerState{
				LaneA: LaneState{LastPollTime: time.Time{}},
				LaneB: LaneState{LastPollTime: now.Add(time.Hour)},
				LaneD: &LaneDState{LastPollTime: now.Add(-time.Minute), Complete: false},
			},
			want: LaneA,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			item := p.NextWork(tc.state, now)
			if item == nil {
				t.Fatalf("NextWork: got nil, want lane %s", tc.want)
			}
			if item.Lane != tc.want {
				t.Errorf("NextWork: got lane %s, want %s", item.Lane, tc.want)
			}
		})
	}
}

// TestPlanner_NextWork_LaneCHasWork covers Lane C's has-work rule (ADR 0044
// §5/§6): an active mid-epoch cursor always has work regardless of how
// recently it last ran (a genuine backlog must grind without delay), but an
// idle lane (no cursor) only anchors a fresh epoch once
// laneCIdleAnchorInterval has elapsed since its last run — never on every
// single pick, which would busy-loop anchoring near-empty epochs forever
// once caught up.
func TestPlanner_NextWork_LaneCHasWork(t *testing.T) {
	t.Parallel()
	p := NewPlanner(testPlannerOptions(t))
	daytime := time.Date(2026, 1, 15, 15, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		lastPollTime time.Time
		hasCursor    bool
		wantWork     bool
	}{
		{"never polled: has work", time.Time{}, false, true},
		{"idle, ran seconds ago: no work yet", daytime.Add(-time.Minute), false, false},
		{"idle, ran just under a day ago: no work yet", daytime.Add(-laneCIdleAnchorInterval + time.Minute), false, false},
		{"idle, ran exactly a day ago: has work", daytime.Add(-laneCIdleAnchorInterval), false, true},
		{"idle, ran over a day ago: has work", daytime.Add(-laneCIdleAnchorInterval - time.Hour), false, true},
		{"mid-epoch, ran seconds ago: has work regardless (genuine backlog)", daytime.Add(-time.Minute), true, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var cursor *PollCursor
			if tc.hasCursor {
				cursor = &PollCursor{NextIndex: 300}
			}
			state := PlannerState{
				// A/B parked far in the future: never due, so C is the only
				// possible candidate.
				LaneA: LaneState{LastPollTime: daytime.Add(time.Hour)},
				LaneB: LaneState{LastPollTime: daytime.Add(time.Hour)},
				LaneC: &LaneState{LastPollTime: tc.lastPollTime, Cursor: cursor},
				LaneD: &LaneDState{Complete: true},
			}
			item := p.NextWork(state, daytime)
			gotWork := item != nil && item.Lane == LaneC
			if gotWork != tc.wantWork {
				t.Errorf("NextWork: got item=%+v, wantWork=%v", item, tc.wantWork)
			}
		})
	}
}

// TestPlanner_NextWork_NilLaneCSkipsIt covers the test/wiring convenience of
// a nil Lane C (not wired): NextWork must never pick a lane that is not
// present in state, even when it would otherwise be eligible.
func TestPlanner_NextWork_NilLaneCSkipsIt(t *testing.T) {
	t.Parallel()
	p := NewPlanner(testPlannerOptions(t))
	daytime := time.Date(2026, 1, 15, 15, 0, 0, 0, time.UTC)

	state := PlannerState{
		LaneA: LaneState{LastPollTime: daytime.Add(time.Hour)}, // not due
		LaneB: LaneState{LastPollTime: daytime.Add(time.Hour)}, // not due
		LaneC: nil,                                             // not wired
		LaneD: nil,                                             // not wired (out-of-hours anyway at 15:00)
	}
	item := p.NextWork(state, daytime)
	if item != nil {
		t.Fatalf("NextWork: got %+v, want nil (nothing eligible has work)", item)
	}
}

// TestPlanner_NextWork_NilWhenIdle covers the pure "nothing to do" case:
// every lane either ineligible or with no work.
func TestPlanner_NextWork_NilWhenIdle(t *testing.T) {
	t.Parallel()
	p := NewPlanner(testPlannerOptions(t))
	night := time.Date(2026, 1, 15, 3, 0, 0, 0, time.UTC) // C ineligible; D eligible but complete

	state := PlannerState{
		LaneA: LaneState{LastPollTime: night.Add(-1 * time.Minute)}, // not due
		LaneB: LaneState{LastPollTime: night.Add(-1 * time.Minute)}, // not due
		LaneC: &LaneState{LastPollTime: night},                      // ineligible at night regardless
		LaneD: &LaneDState{LastPollTime: night, Complete: true},     // done forever
	}
	item := p.NextWork(state, night)
	if item != nil {
		t.Fatalf("NextWork: got %+v, want nil", item)
	}
}

// TestPlanner_NextWork_LaneDCompleteNeverHasWork covers Lane D: once
// Complete, it never has work again, even when eligible (out-of-hours) and
// otherwise the only candidate.
func TestPlanner_NextWork_LaneDCompleteNeverHasWork(t *testing.T) {
	t.Parallel()
	p := NewPlanner(testPlannerOptions(t))
	night := time.Date(2026, 1, 15, 3, 0, 0, 0, time.UTC)

	state := PlannerState{
		LaneA: LaneState{LastPollTime: night.Add(time.Hour)},
		LaneB: LaneState{LastPollTime: night.Add(time.Hour)},
		LaneD: &LaneDState{LastPollTime: time.Time{}, Complete: true},
	}
	item := p.NextWork(state, night)
	if item != nil {
		t.Fatalf("NextWork: got %+v, want nil (Lane D is Complete)", item)
	}
}

// TestPlanner_NextWork_ReturnsCursorFromState proves the returned WorkItem
// carries the picked lane's currently-known cursor, so a caller that wants
// it has it without a second lookup.
func TestPlanner_NextWork_ReturnsCursorFromState(t *testing.T) {
	t.Parallel()
	p := NewPlanner(testPlannerOptions(t))
	now := time.Date(2026, 1, 15, 3, 0, 0, 0, time.UTC)
	cursor := &PollCursor{NextIndex: 900, DifferentStart: now}

	state := PlannerState{
		LaneA: LaneState{LastPollTime: now, Cursor: cursor}, // mid-drain: due regardless of freshness
		LaneB: LaneState{LastPollTime: now.Add(time.Hour)},
		LaneD: &LaneDState{Complete: true},
	}
	item := p.NextWork(state, now)
	if item == nil || item.Lane != LaneA {
		t.Fatalf("NextWork: got %+v, want lane A", item)
	}
	if item.Cursor != cursor {
		t.Errorf("WorkItem.Cursor: got %+v, want the same cursor from state", item.Cursor)
	}
}
