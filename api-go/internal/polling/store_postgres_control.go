package polling

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ pollControlStore = (*PostgresPollControlStore)(nil)

// PostgresPollControlStore is the Postgres-backed single-row poll_control table.
type PostgresPollControlStore struct {
	pool *pgxpool.Pool
}

// NewPostgresPollControlStore wires the store over pool.
func NewPostgresPollControlStore(pool *pgxpool.Pool) *PostgresPollControlStore {
	return &PostgresPollControlStore{pool: pool}
}

// Get returns the stored switch, or false when no row exists.
func (s *PostgresPollControlStore) Get(ctx context.Context) (PollControl, bool, error) {
	var c PollControl
	err := s.pool.QueryRow(ctx, "SELECT enabled, reason, updated_at FROM poll_control WHERE id").
		Scan(&c.Enabled, &c.Reason, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PollControl{}, false, nil
	}
	if err != nil {
		return PollControl{}, false, fmt.Errorf("select poll_control: %w", err)
	}
	return c, true, nil
}

// Set writes the single poll_control row, replacing any previous value.
func (s *PostgresPollControlStore) Set(ctx context.Context, c PollControl) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO poll_control (id, enabled, reason, updated_at) VALUES (true, $1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET enabled = EXCLUDED.enabled, reason = EXCLUDED.reason, updated_at = EXCLUDED.updated_at`,
		c.Enabled, c.Reason, c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert poll_control: %w", err)
	}
	return nil
}
