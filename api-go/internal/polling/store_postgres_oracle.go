package polling

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

var (
	_ deltaSeenStore    = (*PostgresDeltaSeenStore)(nil)
	_ memberStore       = (*PostgresOracleStore)(nil)
	_ oracleMemberStore = (*PostgresOracleStore)(nil)
	_ oracleDiffStore   = (*PostgresOracleStore)(nil)
)

// PostgresDeltaSeenStore reads and writes delta_seen.
type PostgresDeltaSeenStore struct {
	pool *pgxpool.Pool
}

// NewPostgresDeltaSeenStore wires the store over pool.
func NewPostgresDeltaSeenStore(pool *pgxpool.Pool) *PostgresDeltaSeenStore {
	return &PostgresDeltaSeenStore{pool: pool}
}

// Upsert records rows, keeping the latest seen_at per key.
func (s *PostgresDeltaSeenStore) Upsert(ctx context.Context, rows []DeltaSeen) error {
	latest := make(map[string]DeltaSeen, len(rows))
	for _, r := range rows {
		latest[fmt.Sprintf("%d|%s|%s|%d", r.Axis, r.Day.Format(time.DateOnly), r.UID, r.AreaID)] = r
	}
	var axes, uids []string
	var days, seen []time.Time
	var areas []int32
	for _, r := range latest {
		axes = append(axes, axisName(r.Axis))
		days = append(days, r.Day)
		uids = append(uids, r.UID)
		areas = append(areas, int32(r.AreaID))
		seen = append(seen, r.SeenAt)
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO delta_seen (axis, day, uid, area_id, seen_at)
SELECT * FROM unnest($1::text[], $2::date[], $3::text[], $4::int[], $5::timestamptz[])
ON CONFLICT (axis, day, uid, area_id) DO UPDATE SET seen_at = EXCLUDED.seen_at`,
		axes, days, uids, areas, seen)
	if err != nil {
		return fmt.Errorf("upsert delta_seen: %w", err)
	}
	return nil
}

// Take deletes every row of the window and returns those seen before the cutoff.
func (s *PostgresDeltaSeenStore) Take(ctx context.Context, ref WindowRef, before time.Time) ([]AppKey, error) {
	rows, err := s.pool.Query(ctx, `WITH gone AS (DELETE FROM delta_seen WHERE axis = $1 AND day = $2 RETURNING uid, area_id, seen_at)
SELECT uid, area_id FROM gone WHERE seen_at < $3`, axisName(ref.Axis), ref.Day, before)
	if err != nil {
		return nil, fmt.Errorf("take delta_seen: %w", err)
	}
	keys, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (AppKey, error) {
		var k AppKey
		err := r.Scan(&k.UID, &k.AreaID)
		return k, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan delta_seen: %w", err)
	}
	return keys, nil
}

// PostgresOracleStore reads and writes poll_window_member and poll_oracle_diff.
type PostgresOracleStore struct {
	pool *pgxpool.Pool
}

// NewPostgresOracleStore wires the store over pool.
func NewPostgresOracleStore(pool *pgxpool.Pool) *PostgresOracleStore {
	return &PostgresOracleStore{pool: pool}
}

// Replace swaps the window's members for keys. A read that returned nothing is
// stored as one row with an empty uid, so an empty window still has a read_at
// and the oracle does not skip it.
func (s *PostgresOracleStore) Replace(ctx context.Context, ref WindowRef, readAt time.Time, keys []AppKey) error {
	uids := make([]string, 0, len(keys)+1)
	areas := make([]int32, 0, len(keys)+1)
	for _, k := range keys {
		uids = append(uids, k.UID)
		areas = append(areas, int32(k.AreaID))
	}
	if len(keys) == 0 {
		uids, areas = append(uids, ""), append(areas, 0)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // no-op after Commit
	axis := axisName(ref.Axis)
	if _, err := tx.Exec(ctx, "DELETE FROM poll_window_member WHERE axis = $1 AND day = $2", axis, ref.Day); err != nil {
		return fmt.Errorf("delete poll_window_member: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO poll_window_member (axis, day, uid, area_id, read_at)
SELECT $1, $2, u, a, $3 FROM unnest($4::text[], $5::int[]) AS t(u, a)`, axis, ref.Day, readAt, uids, areas); err != nil {
		return fmt.Errorf("insert poll_window_member: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit poll_window_member: %w", err)
	}
	return nil
}

// Members returns the stored members of every window on axis with day >= from.
func (s *PostgresOracleStore) Members(ctx context.Context, axis planit.Axis, from time.Time) (map[time.Time]WindowMembers, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT day, uid, area_id, read_at FROM poll_window_member WHERE axis = $1 AND day >= $2", axisName(axis), from)
	if err != nil {
		return nil, fmt.Errorf("select poll_window_member: %w", err)
	}
	defer rows.Close()
	out := map[time.Time]WindowMembers{}
	for rows.Next() {
		var (
			day    time.Time
			uid    string
			area   int
			readAt time.Time
		)
		if err := rows.Scan(&day, &uid, &area, &readAt); err != nil {
			return nil, fmt.Errorf("scan poll_window_member: %w", err)
		}
		day = dateOnly(day)
		m, ok := out[day]
		if !ok {
			m = WindowMembers{ReadAt: readAt, Keys: map[AppKey]struct{}{}}
		}
		if uid != "" {
			m.Keys[AppKey{UID: uid, AreaID: area}] = struct{}{}
		}
		out[day] = m
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate poll_window_member: %w", err)
	}
	return out, nil
}

// Insert adds diffs, ignoring keys that already have a row.
func (s *PostgresOracleStore) Insert(ctx context.Context, rows []OracleDiff) error {
	batch := &pgx.Batch{}
	for _, d := range rows {
		batch.Queue(`INSERT INTO poll_oracle_diff (axis, day, uid, area_id, found_at) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (axis, day, uid, area_id) DO NOTHING`, axisName(d.Axis), d.Day, d.UID, d.AreaID, d.FoundAt)
	}
	res := s.pool.SendBatch(ctx, batch)
	defer res.Close()
	for range rows {
		if _, err := res.Exec(); err != nil {
			return fmt.Errorf("insert poll_oracle_diff: %w", err)
		}
	}
	return nil
}

// Unclassified returns the axis's diffs that have no reason yet.
func (s *PostgresOracleStore) Unclassified(ctx context.Context, axis planit.Axis) ([]OracleDiff, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT day, uid, area_id, found_at FROM poll_oracle_diff WHERE axis = $1 AND reason IS NULL ORDER BY found_at", axisName(axis))
	if err != nil {
		return nil, fmt.Errorf("select poll_oracle_diff: %w", err)
	}
	diffs, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (OracleDiff, error) {
		d := OracleDiff{Axis: axis}
		err := r.Scan(&d.Day, &d.UID, &d.AreaID, &d.FoundAt)
		d.Day = dateOnly(d.Day)
		return d, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan poll_oracle_diff: %w", err)
	}
	return diffs, nil
}

// Classify sets the diff's reason.
func (s *PostgresOracleStore) Classify(ctx context.Context, d OracleDiff, reason string) error {
	_, err := s.pool.Exec(ctx,
		"UPDATE poll_oracle_diff SET reason = $1 WHERE axis = $2 AND day = $3 AND uid = $4 AND area_id = $5",
		reason, axisName(d.Axis), d.Day, d.UID, d.AreaID)
	if err != nil {
		return fmt.Errorf("classify poll_oracle_diff: %w", err)
	}
	return nil
}
