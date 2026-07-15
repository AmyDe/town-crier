package polling

import (
	"context"
	"fmt"
	"time"
)

// Compile-time check: the store satisfies the consumer-side interface.
var _ backfillStateAccess = (*PostgresBackfillStateStore)(nil)

// PostgresBackfillStateStore reads and writes Lane D's singleton
// backfill_state row (GH#967, ADR 0042; migration 0022_backfill_state.sql).
// The migration seeds the one row, so the store never has a "not found"
// case: Get is a plain point SELECT, Save a plain UPDATE — no upsert, no
// candidate-id list, no LRU query, because there is exactly one thing to
// track.
type PostgresBackfillStateStore struct {
	db querier
}

// NewPostgresBackfillStateStore returns a store over the given pgx pool (or
// any querier — a pgx.Tx also satisfies the interface).
func NewPostgresBackfillStateStore(db querier) *PostgresBackfillStateStore {
	return &PostgresBackfillStateStore{db: db}
}

const getBackfillStateQuery = `
SELECT window_end, cursor_next_index, window_records_seen, consecutive_empty_windows, complete, last_run_time
FROM backfill_state
LIMIT 1`

// Get point-reads the singleton backfill state row. window_end and
// last_run_time are nullable columns (NULL before the lane's first-ever
// run); a nil scan target maps to BackfillState's documented zero-time
// "never started"/"never run" sentinel.
func (s *PostgresBackfillStateStore) Get(ctx context.Context) (BackfillState, error) {
	var (
		windowEnd   *time.Time
		lastRunTime *time.Time
		state       BackfillState
	)
	err := s.db.QueryRow(ctx, getBackfillStateQuery).Scan(
		&windowEnd, &state.CursorNextIndex, &state.WindowRecordsSeen, &state.ConsecutiveEmptyWindows, &state.Complete, &lastRunTime,
	)
	if err != nil {
		return BackfillState{}, fmt.Errorf("read backfill state: %w", err)
	}
	if windowEnd != nil {
		state.WindowEnd = windowEnd.UTC()
	}
	if lastRunTime != nil {
		state.LastRunTime = lastRunTime.UTC()
	}
	return state, nil
}

const saveBackfillStateQuery = `
UPDATE backfill_state SET
    window_end                = $1,
    cursor_next_index         = $2,
    window_records_seen       = $3,
    consecutive_empty_windows = $4,
    complete                  = $5,
    last_run_time             = $6`

// Save updates the singleton backfill state row. A zero WindowEnd or
// LastRunTime writes SQL NULL, matching Get's "never started"/"never run"
// sentinel.
func (s *PostgresBackfillStateStore) Save(ctx context.Context, state BackfillState) error {
	var windowEnd, lastRunTime *time.Time
	if !state.WindowEnd.IsZero() {
		w := state.WindowEnd.UTC()
		windowEnd = &w
	}
	if !state.LastRunTime.IsZero() {
		l := state.LastRunTime.UTC()
		lastRunTime = &l
	}

	_, err := s.db.Exec(ctx, saveBackfillStateQuery,
		windowEnd, state.CursorNextIndex, state.WindowRecordsSeen, state.ConsecutiveEmptyWindows, state.Complete, lastRunTime,
	)
	if err != nil {
		return fmt.Errorf("save backfill state: %w", err)
	}
	return nil
}
