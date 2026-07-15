//go:build integration

package polling

import (
	"context"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

// newPGBackfillStateStore returns a PostgresBackfillStateStore over a
// migrated test database, with the migration's seeded singleton row restored
// after Truncate (TRUNCATE removes it along with everything else). Integration
// tests MUST NOT call t.Parallel: the pgtest harness holds a session-level
// advisory lock so all integration tests in all packages serialise on the
// single docker-compose database (see pgtest.New doc).
func newPGBackfillStateStore(t *testing.T) *PostgresBackfillStateStore {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "backfill_state")
	if _, err := pool.Exec(context.Background(), "INSERT INTO backfill_state (id) VALUES (true)"); err != nil {
		t.Fatalf("reseed singleton backfill_state row: %v", err)
	}
	return NewPostgresBackfillStateStore(pool)
}

// TestPostgresBackfillStateStore_SaveThenGet_RoundTrips proves a full Save
// then Get round-trips every field against real Postgres, including the
// date-typed window_end column and the nullable last_run_time.
func TestPostgresBackfillStateStore_SaveThenGet_RoundTrips(t *testing.T) {
	ctx := context.Background()
	store := newPGBackfillStateStore(t)

	windowEnd := time.Date(2019, 4, 20, 0, 0, 0, 0, time.UTC)
	lastRun := time.Date(2026, 7, 15, 3, 0, 0, 0, time.UTC)
	want := BackfillState{
		WindowEnd:               windowEnd,
		CursorNextIndex:         600,
		WindowRecordsSeen:       120,
		ConsecutiveEmptyWindows: 4,
		Complete:                false,
		LastRunTime:             lastRun,
	}

	if err := store.Save(ctx, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.WindowEnd.Equal(want.WindowEnd) {
		t.Errorf("WindowEnd: got %v, want %v", got.WindowEnd, want.WindowEnd)
	}
	if got.CursorNextIndex != want.CursorNextIndex {
		t.Errorf("CursorNextIndex: got %d, want %d", got.CursorNextIndex, want.CursorNextIndex)
	}
	if got.WindowRecordsSeen != want.WindowRecordsSeen {
		t.Errorf("WindowRecordsSeen: got %d, want %d", got.WindowRecordsSeen, want.WindowRecordsSeen)
	}
	if got.ConsecutiveEmptyWindows != want.ConsecutiveEmptyWindows {
		t.Errorf("ConsecutiveEmptyWindows: got %d, want %d", got.ConsecutiveEmptyWindows, want.ConsecutiveEmptyWindows)
	}
	if got.Complete != want.Complete {
		t.Errorf("Complete: got %v, want %v", got.Complete, want.Complete)
	}
	if !got.LastRunTime.Equal(want.LastRunTime) {
		t.Errorf("LastRunTime: got %v, want %v", got.LastRunTime, want.LastRunTime)
	}
}

// TestPostgresBackfillStateStore_GetReturnsSeededRowBeforeAnySave proves the
// migration's own seed row is readable with no prior Save: the store never
// has a "not found" case.
func TestPostgresBackfillStateStore_GetReturnsSeededRowBeforeAnySave(t *testing.T) {
	ctx := context.Background()
	store := newPGBackfillStateStore(t)

	got, err := store.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.WindowEnd.IsZero() {
		t.Errorf("WindowEnd: got %v, want zero (never started)", got.WindowEnd)
	}
	if got.Complete {
		t.Error("Complete: got true, want false")
	}
	if got.CursorNextIndex != 0 {
		t.Errorf("CursorNextIndex: got %d, want 0", got.CursorNextIndex)
	}
}

// TestPostgresBackfillStateStore_CompleteRoundTrips proves the terminal
// Complete=true state persists correctly.
func TestPostgresBackfillStateStore_CompleteRoundTrips(t *testing.T) {
	ctx := context.Background()
	store := newPGBackfillStateStore(t)

	if err := store.Save(ctx, BackfillState{Complete: true, ConsecutiveEmptyWindows: 12}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.Complete {
		t.Error("Complete: got false, want true")
	}
}
