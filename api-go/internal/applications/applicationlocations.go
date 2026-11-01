package applications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrThinTableNotEmpty is returned by Load when application_locations already
// holds rows. The table has no primary key at load time, so a second load would
// duplicate every row.
var ErrThinTableNotEmpty = errors.New("application_locations is not empty")

const (
	reconcileBatchSize = 200
	deadlockRetries    = 3
	deadlockBackoff    = 100 * time.Millisecond
	bucketCount        = 7
	sqlStateDeadlock   = "40P01"
)

var allBuckets = []int{0, 1, 2, 3, 4, 5, 6}

// ReconcileResult counts what one Reconcile run changed. Skipped is true when
// thin_sync was off and nothing ran. Failed counts authorities abandoned after
// repeated deadlocks.
type ReconcileResult struct {
	Skipped     bool
	Authorities int
	Upserted    int
	Deleted     int
	Failed      int
}

type readPathState interface {
	State(ctx context.Context) (ReadPathState, error)
}

// ApplicationLocations builds and repairs the thin application_locations table
// from applications.
type ApplicationLocations struct {
	pool   *pgxpool.Pool
	sw     readPathState
	logger *slog.Logger
	now    func() time.Time
	sleep  func(ctx context.Context, d time.Duration) error

	// beforeBatch runs inside every repair batch transaction. Tests use it to
	// force a deadlock.
	beforeBatch func(ctx context.Context, tx pgx.Tx) error
}

// NewApplicationLocations wires the load and reconcile jobs. sw supplies the
// thin_sync switch that gates Reconcile.
func NewApplicationLocations(pool *pgxpool.Pool, sw readPathState, logger *slog.Logger, now func() time.Time) *ApplicationLocations {
	return &ApplicationLocations{pool: pool, sw: sw, logger: logger, now: now, sleep: thinSleep}
}

func thinSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Load copies every application that has a location into the empty
// application_locations table, ordered by geohash, in one statement. It returns
// ErrThinTableNotEmpty if the table holds rows.
func (l *ApplicationLocations) Load(ctx context.Context) (int64, error) {
	tx, err := l.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin load transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	if _, err := tx.Exec(ctx, "LOCK TABLE application_locations IN SHARE ROW EXCLUSIVE MODE"); err != nil {
		return 0, fmt.Errorf("lock application_locations: %w", err)
	}
	var occupied bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM application_locations)").Scan(&occupied); err != nil {
		return 0, fmt.Errorf("check application_locations is empty: %w", err)
	}
	if occupied {
		return 0, ErrThinTableNotEmpty
	}
	tag, err := tx.Exec(ctx, `INSERT INTO application_locations
			(authority_code, planit_name, uid, area_id, location, app_state, start_date)
		SELECT authority_code, planit_name, uid, area_id, location, app_state, start_date
		FROM applications
		WHERE location IS NOT NULL
		ORDER BY ST_GeoHash(location::geometry, 10)`)
	if err != nil {
		return 0, fmt.Errorf("load application_locations: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit load: %w", err)
	}
	return tag.RowsAffected(), nil
}

// Reconcile repairs drift between applications and application_locations. With
// all false it covers the authorities whose bucket is today's weekday (UTC);
// with all true it covers every authority. It does nothing while thin_sync is
// off.
func (l *ApplicationLocations) Reconcile(ctx context.Context, all bool) (ReconcileResult, error) {
	st, err := l.sw.State(ctx)
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("read thin_sync: %w", err)
	}
	if !st.ThinSync {
		l.logger.InfoContext(ctx, "application-locations reconcile skipped: thin_sync is off")
		return ReconcileResult{Skipped: true}, nil
	}

	buckets := allBuckets
	if !all {
		buckets = []int{bucketForWeekday(l.now().UTC().Weekday())}
	}
	listed, err := l.authorities(ctx)
	if err != nil {
		return ReconcileResult{}, err
	}
	codes := selectBuckets(listed, buckets)

	var res ReconcileResult
	for _, code := range codes {
		if err := ctx.Err(); err != nil {
			return res, fmt.Errorf("reconcile interrupted: %w", err)
		}
		up, del, err := l.reconcileAuthority(ctx, code)
		res.Upserted += up
		res.Deleted += del
		res.Authorities++
		if err != nil {
			if ctx.Err() != nil {
				return res, fmt.Errorf("reconcile authority %s: %w", code, err)
			}
			res.Failed++
			l.logger.ErrorContext(ctx, "application-locations reconcile abandoned an authority", "authority", code, "error", err)
		}
	}
	l.logger.InfoContext(ctx, "application-locations reconcile completed",
		"authorities", res.Authorities, "upserted", res.Upserted, "deleted", res.Deleted, "failed", res.Failed, "all", all)
	return res, nil
}

type authorityBucket struct {
	code   string
	bucket int
}

func bucketForWeekday(d time.Weekday) int { return int(d) % bucketCount }

func selectBuckets(all []authorityBucket, buckets []int) []string {
	var out []string
	for _, a := range all {
		if slices.Contains(buckets, a.bucket) {
			out = append(out, a.code)
		}
	}
	return out
}

const thinSkipScan = `SELECT code FROM (
		WITH RECURSIVE s AS (
			(SELECT authority_code AS code FROM %[1]s ORDER BY authority_code LIMIT 1)
			UNION ALL
			SELECT (SELECT authority_code FROM %[1]s WHERE authority_code > s.code ORDER BY authority_code LIMIT 1)
			FROM s WHERE s.code IS NOT NULL
		) SELECT code FROM s WHERE code IS NOT NULL) x`

// authorities lists every authority code in either table with a recursive skip
// scan on the primary keys, because SELECT DISTINCT would read every row.
func (l *ApplicationLocations) authorities(ctx context.Context) ([]authorityBucket, error) {
	q := fmt.Sprintf(`SELECT code, (abs(hashtext(code)::bigint) %% %d)::int FROM (%s UNION %s) u ORDER BY code`,
		bucketCount, fmt.Sprintf(thinSkipScan, "applications"), fmt.Sprintf(thinSkipScan, "application_locations"))
	rows, err := l.pool.Query(ctx, q, pgx.QueryExecModeExec)
	if err != nil {
		return nil, fmt.Errorf("list authorities: %w", err)
	}
	defer rows.Close()
	var out []authorityBucket
	for rows.Next() {
		var a authorityBucket
		if err := rows.Scan(&a.code, &a.bucket); err != nil {
			return nil, fmt.Errorf("scan authority: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list authorities: %w", err)
	}
	return out, nil
}

const thinDiffQuery = `SELECT COALESCE(a.planit_name, t.planit_name), a.planit_name IS NOT NULL
	FROM (SELECT planit_name, uid, area_id, location, app_state, start_date
	      FROM applications WHERE authority_code = $1 AND location IS NOT NULL) a
	FULL JOIN (SELECT planit_name, uid, area_id, location, app_state, start_date
	           FROM application_locations WHERE authority_code = $1) t
	  ON a.planit_name = t.planit_name
	WHERE a.planit_name IS NULL OR t.planit_name IS NULL
	   OR a.uid IS DISTINCT FROM t.uid
	   OR a.area_id IS DISTINCT FROM t.area_id
	   OR a.location IS DISTINCT FROM t.location
	   OR a.app_state IS DISTINCT FROM t.app_state
	   OR a.start_date IS DISTINCT FROM t.start_date
	ORDER BY 1`

const thinUpsertQuery = `INSERT INTO application_locations
		(authority_code, planit_name, uid, area_id, location, app_state, start_date)
	SELECT a.authority_code, a.planit_name, a.uid, a.area_id, a.location, a.app_state, a.start_date
	FROM applications a
	JOIN unnest($2::text[]) AS k(planit_name) ON a.planit_name = k.planit_name
	WHERE a.authority_code = $1 AND a.location IS NOT NULL
	ORDER BY a.authority_code, a.planit_name
	FOR SHARE OF a
	ON CONFLICT (authority_code, planit_name) DO UPDATE SET
		uid = EXCLUDED.uid, area_id = EXCLUDED.area_id, location = EXCLUDED.location,
		app_state = EXCLUDED.app_state, start_date = EXCLUDED.start_date
	WHERE application_locations.uid IS DISTINCT FROM EXCLUDED.uid
	   OR application_locations.area_id IS DISTINCT FROM EXCLUDED.area_id
	   OR application_locations.location IS DISTINCT FROM EXCLUDED.location
	   OR application_locations.app_state IS DISTINCT FROM EXCLUDED.app_state
	   OR application_locations.start_date IS DISTINCT FROM EXCLUDED.start_date`

const thinDeleteQuery = `DELETE FROM application_locations t
	USING unnest($2::text[]) AS k(planit_name)
	WHERE t.authority_code = $1 AND t.planit_name = k.planit_name
	  AND NOT EXISTS (SELECT 1 FROM applications a
	                  WHERE a.authority_code = t.authority_code AND a.planit_name = t.planit_name
	                    AND a.location IS NOT NULL)`

// reconcileAuthority diffs one authority without locks, then repairs the
// differing keys in transactions of at most reconcileBatchSize keys. Extras are
// deleted in separate transactions so each delete takes a fresh snapshot.
func (l *ApplicationLocations) reconcileAuthority(ctx context.Context, code string) (upserted, deleted int, err error) {
	upsertKeys, deleteKeys, err := l.diff(ctx, code)
	if err != nil {
		return 0, 0, err
	}
	for batch := range slices.Chunk(upsertKeys, reconcileBatchSize) {
		n, err := l.runBatch(ctx, thinUpsertQuery, code, batch)
		upserted += n
		if err != nil {
			return upserted, deleted, fmt.Errorf("upsert batch: %w", err)
		}
	}
	for batch := range slices.Chunk(deleteKeys, reconcileBatchSize) {
		n, err := l.runBatch(ctx, thinDeleteQuery, code, batch)
		deleted += n
		if err != nil {
			return upserted, deleted, fmt.Errorf("delete batch: %w", err)
		}
	}
	return upserted, deleted, nil
}

func (l *ApplicationLocations) diff(ctx context.Context, code string) (upsertKeys, deleteKeys []string, err error) {
	rows, err := l.pool.Query(ctx, thinDiffQuery, pgx.QueryExecModeExec, code)
	if err != nil {
		return nil, nil, fmt.Errorf("diff authority %s: %w", code, err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var inSource bool
		if err := rows.Scan(&name, &inSource); err != nil {
			return nil, nil, fmt.Errorf("scan diff: %w", err)
		}
		if inSource {
			upsertKeys = append(upsertKeys, name)
		} else {
			deleteKeys = append(deleteKeys, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("diff authority %s: %w", code, err)
	}
	return upsertKeys, deleteKeys, nil
}

// runBatch runs query for one batch in its own transaction, retrying deadlocks,
// and returns the rows it changed.
func (l *ApplicationLocations) runBatch(ctx context.Context, query, code string, names []string) (int, error) {
	var changed int
	err := retryDeadlock(ctx, l.sleep, func() error {
		tx, err := l.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin batch: %w", err)
		}
		defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
		if l.beforeBatch != nil {
			if err := l.beforeBatch(ctx, tx); err != nil {
				return err
			}
		}
		tag, err := tx.Exec(ctx, query, pgx.QueryExecModeExec, code, names)
		if err != nil {
			return fmt.Errorf("repair batch: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit batch: %w", err)
		}
		changed = int(tag.RowsAffected())
		return nil
	})
	if err != nil {
		return 0, err
	}
	return changed, nil
}

func isDeadlock(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == sqlStateDeadlock
}

// retryDeadlock runs fn, retrying up to deadlockRetries times when it fails
// with a deadlock (40P01).
func retryDeadlock(ctx context.Context, sleep func(context.Context, time.Duration) error, fn func() error) error {
	var err error
	for attempt := 0; attempt <= deadlockRetries; attempt++ {
		if attempt > 0 {
			if serr := sleep(ctx, time.Duration(attempt)*deadlockBackoff); serr != nil {
				return serr
			}
		}
		if err = fn(); err == nil || !isDeadlock(err) {
			return err
		}
	}
	return err
}
