package appevents

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore reads and updates application_event through pgx.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore wires a PostgresStore over pool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

const pendingQuery = `SELECT id, uid, authority_code, kind, event_date, detected_at
FROM application_event WHERE status = 'pending' ORDER BY detected_at, id`

// Pending returns pending events, oldest detected_at first.
func (s *PostgresStore) Pending(ctx context.Context) ([]Event, error) {
	rows, err := s.pool.Query(ctx, pendingQuery)
	if err != nil {
		return nil, fmt.Errorf("query pending events: %w", err)
	}
	events, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Event, error) {
		var e Event
		err := r.Scan(&e.ID, &e.UID, &e.AuthorityCode, &e.Kind, &e.EventDate, &e.DetectedAt)
		return e, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan pending events: %w", err)
	}
	return events, nil
}

// MarkSent records a successful fan-out.
func (s *PostgresStore) MarkSent(ctx context.Context, id int64, at time.Time) error {
	return s.setStatus(ctx, StatusSent, []int64{id}, at)
}

// MarkStale retires events that must not alert.
func (s *PostgresStore) MarkStale(ctx context.Context, ids []int64, at time.Time) error {
	return s.setStatus(ctx, StatusStale, ids, at)
}

func (s *PostgresStore) setStatus(ctx context.Context, status string, ids []int64, at time.Time) error {
	_, err := s.pool.Exec(ctx,
		"UPDATE application_event SET status = $1, processed_at = $2 WHERE id = ANY($3) AND status = 'pending'",
		status, at, ids)
	if err != nil {
		return fmt.Errorf("mark %d events %s: %w", len(ids), status, err)
	}
	return nil
}

// CountActiveSince counts events detected at or after since that are not stale.
func (s *PostgresStore) CountActiveSince(ctx context.Context, since time.Time) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		"SELECT count(*) FROM application_event WHERE status <> 'stale' AND detected_at >= $1", since).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count active events: %w", err)
	}
	return n, nil
}
