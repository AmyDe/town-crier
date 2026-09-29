package polling

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

var _ windowStateWriter = (*PostgresWindowStore)(nil)

// PostgresWindowStore reads and writes the poll_window table.
type PostgresWindowStore struct {
	pool *pgxpool.Pool
}

// NewPostgresWindowStore wires the store over pool.
func NewPostgresWindowStore(pool *pgxpool.Pool) *PostgresWindowStore {
	return &PostgresWindowStore{pool: pool}
}

func axisName(a planit.Axis) string {
	if a == planit.AxisDecided {
		return "decided"
	}
	return "start"
}

// Load returns the poll_window rows with from <= day <= to.
func (s *PostgresWindowStore) Load(ctx context.Context, from, to time.Time) ([]WindowState, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT axis, day, last_complete_at, last_full_read_at, last_total FROM poll_window WHERE day >= $1 AND day <= $2 ORDER BY day DESC, axis",
		from, to)
	if err != nil {
		return nil, fmt.Errorf("select poll_window: %w", err)
	}
	defer rows.Close()
	var out []WindowState
	for rows.Next() {
		var (
			axis string
			w    WindowState
		)
		if err := rows.Scan(&axis, &w.Day, &w.LastCompleteAt, &w.LastFullReadAt, &w.LastTotal); err != nil {
			return nil, fmt.Errorf("scan poll_window: %w", err)
		}
		w.Axis = planit.AxisStart
		if axis == "decided" {
			w.Axis = planit.AxisDecided
		}
		w.Day = time.Date(w.Day.Year(), w.Day.Month(), w.Day.Day(), 0, 0, 0, 0, time.UTC)
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate poll_window: %w", err)
	}
	return out, nil
}

// MarkProbeComplete records a count-probe completion: only last_complete_at
// moves, so the forced full read keeps its own clock.
func (s *PostgresWindowStore) MarkProbeComplete(ctx context.Context, ref WindowRef, at time.Time) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO poll_window (axis, day, last_complete_at) VALUES ($1, $2, $3)
		 ON CONFLICT (axis, day) DO UPDATE SET last_complete_at = EXCLUDED.last_complete_at`,
		axisName(ref.Axis), ref.Day, at)
	if err != nil {
		return fmt.Errorf("mark poll_window probe: %w", err)
	}
	return nil
}

// MarkFullRead records a complete full read of the window.
func (s *PostgresWindowStore) MarkFullRead(ctx context.Context, ref WindowRef, at time.Time, total int) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO poll_window (axis, day, last_complete_at, last_full_read_at, last_total) VALUES ($1, $2, $3, $3, $4)
		 ON CONFLICT (axis, day) DO UPDATE SET last_complete_at = EXCLUDED.last_complete_at,
		   last_full_read_at = EXCLUDED.last_full_read_at, last_total = EXCLUDED.last_total`,
		axisName(ref.Axis), ref.Day, at, total)
	if err != nil {
		return fmt.Errorf("mark poll_window full read: %w", err)
	}
	return nil
}

// LatestPageZeroAt returns the time of the latest planit_call row for work
// with page_index 0, or nil when there is none. It is the delta due marker.
func (s *PostgresWindowStore) LatestPageZeroAt(ctx context.Context, work planit.Work) (*time.Time, error) {
	var at *time.Time
	err := s.pool.QueryRow(ctx,
		"SELECT max(at) FROM planit_call WHERE work = $1 AND page_index = 0", string(work)).Scan(&at)
	if err != nil {
		return nil, fmt.Errorf("select latest page-zero call: %w", err)
	}
	return at, nil
}
