package polling

import (
	"context"
	"fmt"
	"time"
)

// Compile-time check: the store satisfies the consumer-side interface.
var _ recentSweepStateAccess = (*PostgresRecentSweepStateStore)(nil)

// PostgresRecentSweepStateStore reads and writes Lane E's singleton
// recent_sweep_state row (GH#1134, ADR 0047; migration
// 0029_recent_sweep_state.sql). The migration seeds the one row, so the store
// never has a "not found" case: Get is a plain point SELECT, Save a plain
// UPDATE — scoped WHERE id = 1 (the deliberate fix for backfill_state's
// keyless, WHERE-less UPDATE).
type PostgresRecentSweepStateStore struct {
	db querier
}

// NewPostgresRecentSweepStateStore returns a store over the given pgx pool (or
// any querier — a pgx.Tx also satisfies the interface).
func NewPostgresRecentSweepStateStore(db querier) *PostgresRecentSweepStateStore {
	return &PostgresRecentSweepStateStore{db: db}
}

const getRecentSweepStateQuery = `
SELECT lap_anchor, window_end, cursor_next_index, laps_completed, last_lap_completed_at, last_run_time
FROM recent_sweep_state
WHERE id = 1`

// Get point-reads the singleton recent-sweep state row. lap_anchor,
// window_end, last_lap_completed_at and last_run_time are nullable (NULL
// before the lane's first-ever run / first lap completion); a nil scan target
// maps to RecentSweepState's zero-time "never started" sentinel.
func (s *PostgresRecentSweepStateStore) Get(ctx context.Context) (RecentSweepState, error) {
	var (
		lapAnchor          *time.Time
		windowEnd          *time.Time
		lastLapCompletedAt *time.Time
		lastRunTime        *time.Time
		state              RecentSweepState
	)
	err := s.db.QueryRow(ctx, getRecentSweepStateQuery).Scan(
		&lapAnchor, &windowEnd, &state.CursorNextIndex, &state.LapsCompleted, &lastLapCompletedAt, &lastRunTime,
	)
	if err != nil {
		return RecentSweepState{}, fmt.Errorf("read recent sweep state: %w", err)
	}
	if lapAnchor != nil {
		state.LapAnchor = lapAnchor.UTC()
	}
	if windowEnd != nil {
		state.WindowEnd = windowEnd.UTC()
	}
	if lastLapCompletedAt != nil {
		state.LastLapCompletedAt = lastLapCompletedAt.UTC()
	}
	if lastRunTime != nil {
		state.LastRunTime = lastRunTime.UTC()
	}
	return state, nil
}

const saveRecentSweepStateQuery = `
UPDATE recent_sweep_state SET
    lap_anchor            = $1,
    window_end            = $2,
    cursor_next_index     = $3,
    laps_completed        = $4,
    last_lap_completed_at = $5,
    last_run_time         = $6
WHERE id = 1`

// Save updates the singleton recent-sweep state row (WHERE id = 1). A zero
// LapAnchor, WindowEnd, LastLapCompletedAt or LastRunTime writes SQL NULL,
// matching Get's "never started" sentinel.
func (s *PostgresRecentSweepStateStore) Save(ctx context.Context, state RecentSweepState) error {
	var lapAnchor, windowEnd, lastLapCompletedAt, lastRunTime *time.Time
	if !state.LapAnchor.IsZero() {
		v := state.LapAnchor.UTC()
		lapAnchor = &v
	}
	if !state.WindowEnd.IsZero() {
		v := state.WindowEnd.UTC()
		windowEnd = &v
	}
	if !state.LastLapCompletedAt.IsZero() {
		v := state.LastLapCompletedAt.UTC()
		lastLapCompletedAt = &v
	}
	if !state.LastRunTime.IsZero() {
		v := state.LastRunTime.UTC()
		lastRunTime = &v
	}

	_, err := s.db.Exec(ctx, saveRecentSweepStateQuery,
		lapAnchor, windowEnd, state.CursorNextIndex, state.LapsCompleted, lastLapCompletedAt, lastRunTime,
	)
	if err != nil {
		return fmt.Errorf("save recent sweep state: %w", err)
	}
	return nil
}
