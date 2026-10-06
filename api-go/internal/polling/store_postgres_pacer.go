package polling

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// planItCallLockKey is the pg_advisory_xact_lock key serialising pacing
// decisions across every poller process.
const planItCallLockKey int64 = 0x7063616c6c // "pcall"

const planItCallColumns = `id, at, work, window_day, page_index, status, total, EXTRACT(EPOCH FROM retry_after)::float8`

var _ planItCallLog = (*PostgresPlanItCallStore)(nil)

// PostgresPlanItCallStore is the Postgres-backed planit_call log.
type PostgresPlanItCallStore struct {
	pool *pgxpool.Pool
}

// NewPostgresPlanItCallStore wires the store over pool.
func NewPostgresPlanItCallStore(pool *pgxpool.Pool) *PostgresPlanItCallStore {
	return &PostgresPlanItCallStore{pool: pool}
}

// Begin opens a transaction holding the pacing advisory lock until it ends.
func (s *PostgresPlanItCallStore) Begin(ctx context.Context) (planItCallTx, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", planItCallLockKey); err != nil {
		tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // best-effort; the lock error is the one to report
		return nil, fmt.Errorf("acquire pacing lock: %w", err)
	}
	return &postgresPlanItCallTx{tx: tx}, nil
}

// Latest returns the most recent call, or nil when the log is empty.
func (s *PostgresPlanItCallStore) Latest(ctx context.Context) (*PlanItCall, error) {
	return latestPlanItCall(ctx, s.pool)
}

// CountBetween counts calls with from <= at < to.
func (s *PostgresPlanItCallStore) CountBetween(ctx context.Context, from, to time.Time) (int, error) {
	return countPlanItCalls(ctx, s.pool, from, to)
}

type planItCallQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func latestPlanItCall(ctx context.Context, q planItCallQueryer) (*PlanItCall, error) {
	var (
		c       PlanItCall
		retrySy *float64
	)
	err := q.QueryRow(ctx, "SELECT "+planItCallColumns+" FROM planit_call ORDER BY at DESC, id DESC LIMIT 1").
		Scan(&c.ID, &c.At, &c.Work, &c.WindowDay, &c.PageIndex, &c.Status, &c.Total, &retrySy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // an empty log is a valid state, not an error
	}
	if err != nil {
		return nil, fmt.Errorf("select latest planit_call: %w", err)
	}
	if retrySy != nil {
		d := time.Duration(*retrySy * float64(time.Second))
		c.RetryAfter = &d
	}
	return &c, nil
}

func countPlanItCalls(ctx context.Context, q planItCallQueryer, from, to time.Time) (int, error) {
	var n int
	if err := q.QueryRow(ctx, "SELECT count(*) FROM planit_call WHERE at >= $1 AND at < $2", from, to).Scan(&n); err != nil {
		return 0, fmt.Errorf("count planit_call: %w", err)
	}
	return n, nil
}

type postgresPlanItCallTx struct {
	tx pgx.Tx
}

func (t *postgresPlanItCallTx) Latest(ctx context.Context) (*PlanItCall, error) {
	return latestPlanItCall(ctx, t.tx)
}

func (t *postgresPlanItCallTx) CountBetween(ctx context.Context, from, to time.Time) (int, error) {
	return countPlanItCalls(ctx, t.tx, from, to)
}

func (t *postgresPlanItCallTx) NthLatestAt(ctx context.Context, n int) (*time.Time, error) {
	var at time.Time
	err := t.tx.QueryRow(ctx, "SELECT at FROM planit_call ORDER BY at DESC, id DESC OFFSET $1 LIMIT 1", n-1).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // fewer than n calls is a valid state, not an error
	}
	if err != nil {
		return nil, fmt.Errorf("select planit_call %d back: %w", n, err)
	}
	return &at, nil
}

func (t *postgresPlanItCallTx) Insert(ctx context.Context, c PlanItCall) (int64, error) {
	var id int64
	err := t.tx.QueryRow(ctx,
		"INSERT INTO planit_call (at, work, window_day, page_index) VALUES ($1, $2, $3, $4) RETURNING id",
		c.At, c.Work, c.WindowDay, c.PageIndex).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert planit_call: %w", err)
	}
	return id, nil
}

func (t *postgresPlanItCallTx) Finish(ctx context.Context, id int64, r CallResult) error {
	var retrySecs *float64
	if r.RetryAfter != nil {
		v := r.RetryAfter.Seconds()
		retrySecs = &v
	}
	_, err := t.tx.Exec(ctx,
		"UPDATE planit_call SET status = $2, total = $3, retry_after = make_interval(secs => $4) WHERE id = $1",
		id, r.Status, r.Total, retrySecs)
	if err != nil {
		return fmt.Errorf("update planit_call: %w", err)
	}
	return nil
}

func (t *postgresPlanItCallTx) Commit(ctx context.Context) error {
	if err := t.tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (t *postgresPlanItCallTx) Rollback(ctx context.Context) error {
	if err := t.tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return fmt.Errorf("rollback: %w", err)
	}
	return nil
}
