//go:build integration

package polling

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/planit"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

func newPollEventPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "poll_event", "planit_call", "application_event", "poll_oracle_diff", "delta_seen", "poll_window_member", "notifications")
	return pool
}

func mustRecord(t *testing.T, s *PostgresPollEventStore, e PollEvent) {
	t.Helper()
	if err := s.Record(context.Background(), e); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresPollEventStore_DoneSinceIsPerAxis(t *testing.T) {
	ctx := context.Background()
	s := NewPostgresPollEventStore(newPollEventPool(t))
	at := time.Date(2026, 6, 10, 23, 0, 0, 0, time.UTC)
	mustRecord(t, s, PollEvent{Kind: EventOracleDone, At: at, Axis: planit.AxisDecided})

	tests := []struct {
		axis  planit.Axis
		since time.Time
		want  bool
	}{
		{planit.AxisDecided, at.Add(-time.Hour), true},
		{planit.AxisDecided, at.Add(time.Second), false},
		{planit.AxisStart, at.Add(-time.Hour), false},
	}
	for _, tt := range tests {
		got, err := s.DoneSince(ctx, tt.axis, tt.since)
		if err != nil || got != tt.want {
			t.Fatalf("DoneSince(%v, %s) = %v, %v; want %v", tt.axis, tt.since, got, err, tt.want)
		}
	}
}

func TestPostgresPollEventStore_RejectsUnknownKind(t *testing.T) {
	s := NewPostgresPollEventStore(newPollEventPool(t))

	if err := s.Record(context.Background(), PollEvent{Kind: "bogus", At: time.Now()}); err == nil {
		t.Fatal("want a check violation")
	}
}

func TestPostgresHealthStore_Inputs(t *testing.T) {
	ctx := context.Background()
	pool := newPollEventPool(t)
	events := NewPostgresPollEventStore(pool)
	health := NewPostgresHealthStore(pool)
	now := londonAt(6, 10, 9, 0)
	day := utcDay(2026, 6, 9)
	other := utcDay(2026, 6, 8)

	// budget day is 2026-06-09 18:00 to 2026-06-10 18:00 London.
	mustRecord(t, events, PollEvent{Kind: EventWindowShort, At: londonAt(6, 9, 19, 0), Axis: planit.AxisStart, Day: &day})
	mustRecord(t, events, PollEvent{Kind: EventWindowShort, At: londonAt(6, 10, 2, 0), Axis: planit.AxisStart, Day: &day})
	mustRecord(t, events, PollEvent{Kind: EventWindowShort, At: londonAt(6, 10, 3, 0), Axis: planit.AxisDecided, Day: &day})
	mustRecord(t, events, PollEvent{Kind: EventWindowShort, At: londonAt(6, 9, 17, 0), Axis: planit.AxisDecided, Day: &day})
	mustRecord(t, events, PollEvent{Kind: EventWindowShort, At: londonAt(6, 8, 19, 0), Axis: planit.AxisStart, Day: &other})
	mustRecord(t, events, PollEvent{Kind: EventWindowViolation, At: now.Add(-2 * time.Hour), Axis: planit.AxisStart, Day: &day, Detail: 4})
	mustRecord(t, events, PollEvent{Kind: EventWindowViolation, At: now.Add(-25 * time.Hour), Axis: planit.AxisStart, Day: &day, Detail: 4})
	mustRecord(t, events, PollEvent{Kind: EventWindowMissed, At: now.Add(-time.Hour), Axis: planit.AxisStart, Day: &day, Detail: 2})
	mustRecord(t, events, PollEvent{Kind: EventSurge, At: now.Add(-3 * time.Hour)})

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO planit_call (at, work, page_index, status) VALUES ($1,'window_start',0,403), ($2,'window_start',0,403), ($3,'window_start',0,200)`,
		now.Add(-time.Hour), now.Add(-30*time.Hour), now.Add(-time.Hour))
	for _, ago := range []time.Duration{time.Hour, 5 * time.Hour, 23 * time.Hour} {
		exec(`INSERT INTO application_event (uid, authority_code, kind, detected_at) VALUES ('n', '1', 'new_application', $1)`, now.Add(-ago))
	}
	exec(`INSERT INTO application_event (uid, authority_code, kind, detected_at) VALUES ('d', '1', 'decision', $1)`, now.Add(-time.Hour))
	exec(`INSERT INTO application_event (uid, authority_code, kind, detected_at, status) VALUES
		('s1', '1', 'decision', $1, 'sent'), ('s2', '1', 'decision', $2, 'sent'), ('x', '1', 'decision', $1, 'stale')`,
		now.Add(-2*time.Hour), now.Add(-30*time.Hour))
	exec(`INSERT INTO notifications (id, user_id, application_uid, authority_id, event_type, created_at) VALUES
		('a', 'u1', 'app1', 1, 'new_application', $1), ('b', 'u2', 'app1', 1, 'new_application', $2)`,
		now.Add(-time.Hour), now.Add(-25*time.Hour))
	for _, ago := range []time.Duration{25 * time.Hour, 26 * time.Hour, 49 * time.Hour, 14*24*time.Hour + 5*time.Hour} {
		exec(`INSERT INTO application_event (uid, authority_code, kind, detected_at) VALUES ('h', '1', 'new_application', $1)`, now.Add(-ago))
	}
	exec(`INSERT INTO poll_oracle_diff (axis, day, uid, area_id, found_at, reason) VALUES
		('start', $1, 'm1', 1, $2, 'miss'), ('start', $1, 'm2', 1, $3, 'miss'), ('start', $1, 'm3', 1, $2, 'late')`,
		day, now.Add(-24*time.Hour), now.Add(-8*24*time.Hour))

	in, err := health.Inputs(ctx, now)

	if err != nil {
		t.Fatal(err)
	}
	if in.ShortWindows != 1 {
		t.Errorf("ShortWindows = %d, want 1 (only start/06-09 has two shorts in the budget day)", in.ShortWindows)
	}
	if in.Violations24h != 1 || in.Missed24h != 1 || in.Surge24h != 1 {
		t.Errorf("violations=%d missed=%d surge=%d, want 1 each", in.Violations24h, in.Missed24h, in.Surge24h)
	}
	if in.Forbidden24h != 1 {
		t.Errorf("Forbidden24h = %d, want 1", in.Forbidden24h)
	}
	if in.NewAppEvents24h != 3 {
		t.Errorf("NewAppEvents24h = %d, want 3", in.NewAppEvents24h)
	}
	if len(in.NewAppDaily14) != 14 || in.NewAppDaily14[0] != 2 || in.NewAppDaily14[1] != 1 || in.NewAppDaily14[13] != 1 {
		t.Errorf("NewAppDaily14 = %v", in.NewAppDaily14)
	}
	if in.OracleMisses7d != 1 {
		t.Errorf("OracleMisses7d = %d, want 1", in.OracleMisses7d)
	}
	if in.Decisions24h != 3 || in.StaleEvents24h != 1 || in.Notifications24h != 1 {
		t.Errorf("decisions=%d stale=%d notifications=%d, want 3, 1, 1", in.Decisions24h, in.StaleEvents24h, in.Notifications24h)
	}
	oldestPending := now.Add(-14*24*time.Hour - 5*time.Hour)
	if in.PendingEvents != 8 || in.OldestPendingAt == nil || !in.OldestPendingAt.Equal(oldestPending) {
		t.Errorf("pending=%d oldest=%v, want 8 at %v", in.PendingEvents, in.OldestPendingAt, oldestPending)
	}
}

func TestPostgresHealthStore_InputsWithNothingPending(t *testing.T) {
	health := NewPostgresHealthStore(newPollEventPool(t))

	in, err := health.Inputs(context.Background(), londonAt(6, 10, 9, 0))

	if err != nil {
		t.Fatal(err)
	}
	if in.PendingEvents != 0 || in.OldestPendingAt != nil {
		t.Errorf("pending=%d oldest=%v, want 0 and nil", in.PendingEvents, in.OldestPendingAt)
	}
}

func TestPostgresDeltaSeenStore_UpsertAndTake(t *testing.T) {
	ctx := context.Background()
	s := NewPostgresDeltaSeenStore(newPollEventPool(t))
	day := utcDay(2026, 6, 9)
	early := time.Date(2026, 6, 10, 10, 0, 0, 0, time.UTC)
	late := early.Add(10 * time.Hour)
	rows := []DeltaSeen{
		{Axis: planit.AxisStart, Day: day, UID: "a", AreaID: 1, SeenAt: early},
		{Axis: planit.AxisStart, Day: day, UID: "a", AreaID: 1, SeenAt: early},
		{Axis: planit.AxisStart, Day: day, UID: "b", AreaID: 1, SeenAt: late},
		{Axis: planit.AxisDecided, Day: day, UID: "c", AreaID: 1, SeenAt: early},
	}
	if err := s.Upsert(ctx, rows); err != nil {
		t.Fatal(err)
	}
	if err := s.Upsert(ctx, []DeltaSeen{{Axis: planit.AxisStart, Day: day, UID: "a", AreaID: 1, SeenAt: early.Add(time.Hour)}}); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}

	got, err := s.Take(ctx, WindowRef{Axis: planit.AxisStart, Day: day}, early.Add(5*time.Hour))

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != (AppKey{UID: "a", AreaID: 1}) {
		t.Fatalf("Take returned %+v, want only a (b was seen after the cutoff)", got)
	}
	again, _ := s.Take(ctx, WindowRef{Axis: planit.AxisStart, Day: day}, late.Add(time.Hour))
	if len(again) != 0 {
		t.Fatalf("the window's rows must be deleted by Take, got %+v", again)
	}
	other, _ := s.Take(ctx, WindowRef{Axis: planit.AxisDecided, Day: day}, late)
	if len(other) != 1 {
		t.Fatalf("other axis rows must survive: %+v", other)
	}
}

func TestPostgresOracleStore_MembersReplaceAndEmptyWindow(t *testing.T) {
	ctx := context.Background()
	s := NewPostgresOracleStore(newPollEventPool(t))
	day := utcDay(2026, 6, 9)
	empty := utcDay(2026, 6, 8)
	first := time.Date(2026, 6, 10, 1, 0, 0, 0, time.UTC)
	second := first.Add(24 * time.Hour)

	if err := s.Replace(ctx, WindowRef{Axis: planit.AxisStart, Day: day}, first, []AppKey{{"a", 1}, {"b", 1}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Replace(ctx, WindowRef{Axis: planit.AxisStart, Day: day}, second, []AppKey{{"b", 1}, {"c", 2}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Replace(ctx, WindowRef{Axis: planit.AxisStart, Day: empty}, second, nil); err != nil {
		t.Fatal(err)
	}

	got, err := s.Members(ctx, planit.AxisStart, utcDay(2026, 5, 1))

	if err != nil {
		t.Fatal(err)
	}
	m := got[day]
	if !m.ReadAt.Equal(second) || len(m.Keys) != 2 {
		t.Fatalf("members = %+v", m)
	}
	if _, ok := m.Keys[AppKey{"c", 2}]; !ok {
		t.Fatalf("keys = %+v", m.Keys)
	}
	e, ok := got[empty]
	if !ok || !e.ReadAt.Equal(second) || len(e.Keys) != 0 {
		t.Fatalf("an empty window read must still be visible: %+v ok=%v", e, ok)
	}
	if other, _ := s.Members(ctx, planit.AxisDecided, utcDay(2026, 5, 1)); len(other) != 0 {
		t.Fatalf("decided members = %+v", other)
	}
}

func TestPostgresOracleStore_DiffsInsertClassifyAndList(t *testing.T) {
	ctx := context.Background()
	s := NewPostgresOracleStore(newPollEventPool(t))
	day := utcDay(2026, 6, 9)
	found := time.Date(2026, 6, 10, 2, 0, 0, 0, time.UTC)
	d := OracleDiff{Axis: planit.AxisStart, Day: day, UID: "x", AreaID: 1, FoundAt: found}

	if err := s.Insert(ctx, []OracleDiff{d}); err != nil {
		t.Fatal(err)
	}
	if err := s.Insert(ctx, []OracleDiff{{Axis: planit.AxisStart, Day: day, UID: "x", AreaID: 1, FoundAt: found.Add(24 * time.Hour)}}); err != nil {
		t.Fatalf("second insert of the same key must be a no-op: %v", err)
	}
	pending, err := s.Unclassified(ctx, planit.AxisStart)
	if err != nil || len(pending) != 1 || !pending[0].FoundAt.Equal(found) || pending[0].Reason != "" || !pending[0].Day.Equal(day) {
		t.Fatalf("pending = %+v err = %v", pending, err)
	}

	if err := s.Classify(ctx, d, ReasonOracleMissed); err != nil {
		t.Fatal(err)
	}

	if pending, _ := s.Unclassified(ctx, planit.AxisStart); len(pending) != 0 {
		t.Fatalf("still pending: %+v", pending)
	}
}
