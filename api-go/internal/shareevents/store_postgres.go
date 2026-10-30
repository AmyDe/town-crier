package shareevents

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Store is the write port for the daily share totals.
type Store interface {
	// Increment adds one to the total for (day, destination).
	Increment(ctx context.Context, day time.Time, destination string) error
}

var _ Store = (*PostgresStore)(nil)

// PostgresStore persists the totals in the share_event_totals table.
type PostgresStore struct {
	db execer
}

// NewPostgresStore returns a store over db. *pgxpool.Pool satisfies execer.
func NewPostgresStore(db execer) *PostgresStore {
	return &PostgresStore{db: db}
}

// Increment implements Store.
func (s *PostgresStore) Increment(ctx context.Context, day time.Time, destination string) error {
	const q = `
		INSERT INTO share_event_totals (day, destination, total)
		VALUES ($1, $2, 1)
		ON CONFLICT (day, destination)
		DO UPDATE SET total = share_event_totals.total + 1`
	if _, err := s.db.Exec(ctx, q, day, destination); err != nil {
		return fmt.Errorf("increment share total: %w", err)
	}
	return nil
}
