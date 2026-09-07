//go:build integration

package polling

import (
	"context"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

// newPGRecentSweepStateStore returns a PostgresRecentSweepStateStore over a
// migrated test database, with the migration's seeded singleton row restored
// after Truncate. Integration tests MUST NOT call t.Parallel: the pgtest
// harness serialises all integration tests on the single docker-compose
// database via a session-level advisory lock (see pgtest.New doc).
func newPGRecentSweepStateStore(t *testing.T) *PostgresRecentSweepStateStore {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "recent_sweep_state")
	if _, err := pool.Exec(context.Background(), "INSERT INTO recent_sweep_state (id) VALUES (1)"); err != nil {
		t.Fatalf("reseed singleton recent_sweep_state row: %v", err)
	}
	return NewPostgresRecentSweepStateStore(pool)
}

// TestPostgresRecentSweepStateStore_SaveThenGet_RoundTrips proves a full Save
// then Get round-trips every field against real Postgres, including the
// date-typed lap_anchor / window_end columns and the nullable timestamptz
// columns.
func TestPostgresRecentSweepStateStore_SaveThenGet_RoundTrips(t *testing.T) {
	ctx := context.Background()
	store := newPGRecentSweepStateStore(t)

	lapAnchor := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
	lastLap := time.Date(2026, 9, 1, 3, 0, 0, 0, time.UTC)
	lastRun := time.Date(2026, 9, 7, 4, 30, 0, 0, time.UTC)
	want := RecentSweepState{
		LapAnchor:          lapAnchor,
		WindowEnd:          windowEnd,
		CursorNextIndex:    1200,
		LapsCompleted:      3,
		LastLapCompletedAt: lastLap,
		LastRunTime:        lastRun,
	}

	if err := store.Save(ctx, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.LapAnchor.Equal(want.LapAnchor) {
		t.Errorf("LapAnchor: got %v, want %v", got.LapAnchor, want.LapAnchor)
	}
	if !got.WindowEnd.Equal(want.WindowEnd) {
		t.Errorf("WindowEnd: got %v, want %v", got.WindowEnd, want.WindowEnd)
	}
	if got.CursorNextIndex != want.CursorNextIndex {
		t.Errorf("CursorNextIndex: got %d, want %d", got.CursorNextIndex, want.CursorNextIndex)
	}
	if got.LapsCompleted != want.LapsCompleted {
		t.Errorf("LapsCompleted: got %d, want %d", got.LapsCompleted, want.LapsCompleted)
	}
	if !got.LastLapCompletedAt.Equal(want.LastLapCompletedAt) {
		t.Errorf("LastLapCompletedAt: got %v, want %v", got.LastLapCompletedAt, want.LastLapCompletedAt)
	}
	if !got.LastRunTime.Equal(want.LastRunTime) {
		t.Errorf("LastRunTime: got %v, want %v", got.LastRunTime, want.LastRunTime)
	}
}

// TestPostgresRecentSweepStateStore_GetReturnsSeededRowBeforeAnySave proves the
// migration's own seed row is readable with no prior Save: the store never has
// a "not found" case, and the never-started sentinels are the zero values.
func TestPostgresRecentSweepStateStore_GetReturnsSeededRowBeforeAnySave(t *testing.T) {
	ctx := context.Background()
	store := newPGRecentSweepStateStore(t)

	got, err := store.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.LapAnchor.IsZero() {
		t.Errorf("LapAnchor: got %v, want zero (never started)", got.LapAnchor)
	}
	if !got.WindowEnd.IsZero() {
		t.Errorf("WindowEnd: got %v, want zero", got.WindowEnd)
	}
	if got.CursorNextIndex != 0 || got.LapsCompleted != 0 {
		t.Errorf("counters: got index=%d laps=%d, want 0/0", got.CursorNextIndex, got.LapsCompleted)
	}
}

// TestPostgresRecentSweepStateStore_SaveTargetsSingletonRow proves Save is an
// UPDATE scoped to the one keyed row (WHERE id = 1): repeated Saves never
// insert a second row and never touch anything but id = 1 — the deliberate fix
// for backfill_state's keyless, WHERE-less UPDATE. The id smallint CHECK (id =
// 1) makes a second row impossible to insert, so the observable proof is that
// the singleton invariant holds across successive Saves and id stays 1.
func TestPostgresRecentSweepStateStore_SaveTargetsSingletonRow(t *testing.T) {
	ctx := context.Background()
	store := newPGRecentSweepStateStore(t)

	first := RecentSweepState{LapAnchor: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), CursorNextIndex: 300}
	second := RecentSweepState{LapAnchor: time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC), CursorNextIndex: 900, LapsCompleted: 1}
	if err := store.Save(ctx, first); err != nil {
		t.Fatalf("Save first: %v", err)
	}
	if err := store.Save(ctx, second); err != nil {
		t.Fatalf("Save second: %v", err)
	}

	pool := pgtest.New(t)
	var (
		count int
		id    int
	)
	if err := pool.QueryRow(ctx, "SELECT count(*), max(id) FROM recent_sweep_state").Scan(&count, &id); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Errorf("row count: got %d, want 1 (Save must never insert)", count)
	}
	if id != 1 {
		t.Errorf("row id: got %d, want 1 (Save must target the id = 1 singleton)", id)
	}

	got, err := store.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.CursorNextIndex != 900 || got.LapsCompleted != 1 {
		t.Errorf("last write not reflected: got index=%d laps=%d, want 900/1", got.CursorNextIndex, got.LapsCompleted)
	}
}
