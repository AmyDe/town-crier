package polling

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

var (
	_ pollEventRecorder = (*PostgresPollEventStore)(nil)
	_ oracleEventStore  = (*PostgresPollEventStore)(nil)
	_ healthSource      = (*PostgresHealthStore)(nil)
)

// PostgresPollEventStore reads and writes poll_event, the durable record of
// short reads, integrity violations, cross-check misses, surges and oracle
// runs that health reads back across hourly runs.
type PostgresPollEventStore struct {
	pool *pgxpool.Pool
}

// NewPostgresPollEventStore wires the store over pool.
func NewPostgresPollEventStore(pool *pgxpool.Pool) *PostgresPollEventStore {
	return &PostgresPollEventStore{pool: pool}
}

// Record appends one event. A surge has no axis.
func (s *PostgresPollEventStore) Record(ctx context.Context, e PollEvent) error {
	var axis *string
	if e.Kind != EventSurge {
		a := axisName(e.Axis)
		axis = &a
	}
	_, err := s.pool.Exec(ctx,
		"INSERT INTO poll_event (at, kind, axis, day, detail) VALUES ($1, $2, $3, $4, $5)",
		e.At, e.Kind, axis, e.Day, e.Detail)
	if err != nil {
		return fmt.Errorf("insert poll_event %s: %w", e.Kind, err)
	}
	return nil
}

// DoneSince reports whether an oracle_done event for axis exists at or after since.
func (s *PostgresPollEventStore) DoneSince(ctx context.Context, axis planit.Axis, since time.Time) (bool, error) {
	var done bool
	err := s.pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM poll_event WHERE kind = 'oracle_done' AND axis = $1 AND at >= $2)",
		axisName(axis), since).Scan(&done)
	if err != nil {
		return false, fmt.Errorf("select oracle_done: %w", err)
	}
	return done, nil
}

// PostgresHealthStore gathers HealthInputs from poll_event, planit_call,
// application_event and poll_oracle_diff.
type PostgresHealthStore struct {
	pool *pgxpool.Pool
}

// NewPostgresHealthStore wires the store over pool.
func NewPostgresHealthStore(pool *pgxpool.Pool) *PostgresHealthStore {
	return &PostgresHealthStore{pool: pool}
}

const (
	shortWindowsQuery = `SELECT count(*) FROM (
	SELECT 1 FROM poll_event WHERE kind = 'window_short' AND at >= $1 AND at < $2
	GROUP BY axis, day HAVING count(*) >= 2) s`
	eventCount24hQuery = "SELECT count(*) FROM poll_event WHERE kind = $1 AND at > $2"
	dailyBucketsQuery  = `SELECT count(e.id) FROM generate_series(1, 14) g(i)
LEFT JOIN application_event e ON e.kind = 'new_application'
  AND e.detected_at >  $1::timestamptz - make_interval(hours => 24 * (g.i + 1))
  AND e.detected_at <= $1::timestamptz - make_interval(hours => 24 * g.i)
GROUP BY g.i ORDER BY g.i`
)

// Inputs returns everything ComputeHealth needs except Windows and OracleEnabled.
func (s *PostgresHealthStore) Inputs(ctx context.Context, now time.Time) (HealthInputs, error) {
	var in HealthInputs
	dayAgo, weekAgo := now.Add(-24*time.Hour), now.Add(-7*24*time.Hour)
	budgetFrom, budgetTo := BudgetDay(now)

	counts := []struct {
		dst   *int
		sql   string
		args  []any
		label string
	}{
		{&in.ShortWindows, shortWindowsQuery, []any{budgetFrom, budgetTo}, "short windows"},
		{&in.Violations24h, eventCount24hQuery, []any{EventWindowViolation, dayAgo}, "violations"},
		{&in.Missed24h, eventCount24hQuery, []any{EventWindowMissed, dayAgo}, "missed"},
		{&in.Surge24h, eventCount24hQuery, []any{EventSurge, dayAgo}, "surges"},
		{&in.Forbidden24h, "SELECT count(*) FROM planit_call WHERE status = 403 AND at > $1", []any{dayAgo}, "forbidden calls"},
		{&in.NewAppEvents24h, "SELECT count(*) FROM application_event WHERE kind = 'new_application' AND detected_at > $1", []any{dayAgo}, "new_application events"},
		{&in.OracleMisses7d, "SELECT count(*) FROM poll_oracle_diff WHERE reason = 'miss' AND found_at > $1", []any{weekAgo}, "oracle misses"},
	}
	for _, c := range counts {
		if err := s.pool.QueryRow(ctx, c.sql, c.args...).Scan(c.dst); err != nil {
			return HealthInputs{}, fmt.Errorf("count %s: %w", c.label, err)
		}
	}

	rows, err := s.pool.Query(ctx, dailyBucketsQuery, now)
	if err != nil {
		return HealthInputs{}, fmt.Errorf("select new_application history: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var n int
		if err := rows.Scan(&n); err != nil {
			return HealthInputs{}, fmt.Errorf("scan new_application history: %w", err)
		}
		in.NewAppDaily14 = append(in.NewAppDaily14, n)
	}
	if err := rows.Err(); err != nil {
		return HealthInputs{}, fmt.Errorf("iterate new_application history: %w", err)
	}
	return in, nil
}
