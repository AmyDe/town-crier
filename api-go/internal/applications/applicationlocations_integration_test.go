//go:build integration

package applications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

type locRow struct {
	code  string
	name  string
	uid   string
	area  int
	state string
	start string
	lon   *float64
	lat   *float64
}

func lonLat(lon, lat float64) (*float64, *float64) { return &lon, &lat }

// addTestPrimaryKey gives application_locations the key the upsert conflicts on.
// The migration adds it only after the production load, so tests add it here.
func addTestPrimaryKey(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `ALTER TABLE application_locations ADD CONSTRAINT application_locations_test_pk PRIMARY KEY (authority_code, planit_name)`); err != nil {
		t.Fatalf("add test primary key: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `ALTER TABLE application_locations DROP CONSTRAINT IF EXISTS application_locations_test_pk`); err != nil {
			t.Errorf("drop test primary key: %v", err)
		}
	})
}

func thinPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications", "application_locations", "read_path_control")
	addTestPrimaryKey(t, pool)
	return pool
}

func setThinSync(t *testing.T, pool *pgxpool.Pool, on bool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `INSERT INTO read_path_control (id, thin_sync, updated_at) VALUES (true, $1, now())
		ON CONFLICT (id) DO UPDATE SET thin_sync = EXCLUDED.thin_sync`, on); err != nil {
		t.Fatalf("set thin_sync: %v", err)
	}
}

func insertLocRow(t *testing.T, pool *pgxpool.Pool, r locRow) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), insertLocSQL, locArgs(r)...); err != nil {
		t.Fatalf("insert application %s/%s: %v", r.code, r.name, err)
	}
}

const insertLocSQL = `INSERT INTO applications
	(authority_code, planit_name, uid, area_name, area_id, app_state, start_date, location, last_different)
	VALUES ($1, $2, $3, 'Area', $4, NULLIF($5, ''), NULLIF($6, '')::date,
		CASE WHEN $7::float8 IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($7, $8), 4326)::geography END, now())`

func locArgs(r locRow) []any {
	return []any{r.code, r.name, r.uid, r.area, r.state, r.start, r.lon, r.lat}
}

func countRows(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

// thinDrift counts rows that differ between application_locations and a fresh
// copy of applications, using EXCEPT so it shares no logic with the reconcile.
func thinDrift(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	const cols = "authority_code, planit_name, uid, area_id, location::geometry, app_state, start_date"
	var n int
	err := pool.QueryRow(context.Background(), `SELECT
		(SELECT count(*) FROM (SELECT `+cols+` FROM applications WHERE location IS NOT NULL EXCEPT ALL SELECT `+cols+` FROM application_locations) a)
		+ (SELECT count(*) FROM (SELECT `+cols+` FROM application_locations EXCEPT ALL SELECT `+cols+` FROM applications WHERE location IS NOT NULL) b)`).Scan(&n)
	if err != nil {
		t.Fatalf("measure drift: %v", err)
	}
	return n
}

func TestApplicationLocationsTrigger_KeepsThinTableEqualToSource(t *testing.T) {
	pool := thinPool(t)
	ctx := context.Background()
	setThinSync(t, pool, true)
	lon, lat := lonLat(-0.1, 51.5)
	row := locRow{code: "100", name: "A/1", uid: "u1", area: 7, state: "Undecided", start: "2026-01-02", lon: lon, lat: lat}

	steps := []struct {
		name     string
		run      func() error
		wantRows int
	}{
		{"insert", func() error { _, err := pool.Exec(ctx, insertLocSQL, locArgs(row)...); return err }, 1},
		{"update copied column", func() error {
			_, err := pool.Exec(ctx, `UPDATE applications SET app_state = 'Approved', uid = 'u2', area_id = 9, start_date = '2026-02-03' WHERE planit_name = 'A/1'`)
			return err
		}, 1},
		{"update location", func() error {
			_, err := pool.Exec(ctx, `UPDATE applications SET location = ST_SetSRID(ST_MakePoint(-0.2, 51.6), 4326)::geography WHERE planit_name = 'A/1'`)
			return err
		}, 1},
		{"update primary key", func() error {
			_, err := pool.Exec(ctx, `UPDATE applications SET planit_name = 'A/2' WHERE planit_name = 'A/1'`)
			return err
		}, 1},
		{"location to NULL", func() error {
			_, err := pool.Exec(ctx, `UPDATE applications SET location = NULL WHERE planit_name = 'A/2'`)
			return err
		}, 0},
		{"location back", func() error {
			_, err := pool.Exec(ctx, `UPDATE applications SET location = ST_SetSRID(ST_MakePoint(-0.3, 51.7), 4326)::geography WHERE planit_name = 'A/2'`)
			return err
		}, 1},
		{"delete", func() error {
			_, err := pool.Exec(ctx, `DELETE FROM applications WHERE planit_name = 'A/2'`)
			return err
		}, 0},
	}
	for _, s := range steps {
		if err := s.run(); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		if got := countRows(t, pool, "application_locations"); got != s.wantRows {
			t.Fatalf("%s: thin rows = %d, want %d", s.name, got, s.wantRows)
		}
		if d := thinDrift(t, pool); d != 0 {
			t.Fatalf("%s: thin table drifts from source by %d rows", s.name, d)
		}
	}
}

func TestApplicationLocationsTrigger_LastScrapedOnlyUpdateWritesNothing(t *testing.T) {
	pool := thinPool(t)
	ctx := context.Background()
	setThinSync(t, pool, true)
	lon, lat := lonLat(-0.1, 51.5)
	insertLocRow(t, pool, locRow{code: "100", name: "A/1", uid: "u1", area: 7, state: "Undecided", lon: lon, lat: lat})
	xminBefore := thinXmin(t, pool)

	if _, err := pool.Exec(ctx, `UPDATE applications SET last_scraped = now(), description = 'changed', url = 'x'`); err != nil {
		t.Fatalf("update: %v", err)
	}

	if got := thinXmin(t, pool); got != xminBefore {
		t.Errorf("thin row xmin = %s, want unchanged %s", got, xminBefore)
	}
}

func thinXmin(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var x string
	if err := pool.QueryRow(context.Background(), `SELECT xmin::text FROM application_locations`).Scan(&x); err != nil {
		t.Fatalf("read thin xmin: %v", err)
	}
	return x
}

func TestApplicationLocationsTrigger_FailedThinWriteDoesNotAbortUpsert(t *testing.T) {
	pool := thinPool(t)
	ctx := context.Background()
	setThinSync(t, pool, true)
	if _, err := pool.Exec(ctx, `ALTER TABLE application_locations ADD CONSTRAINT application_locations_test_fail CHECK (false)`); err != nil {
		t.Fatalf("add failing constraint: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `ALTER TABLE application_locations DROP CONSTRAINT IF EXISTS application_locations_test_fail`)
	})
	lon, lat := lonLat(-0.1, 51.5)

	insertLocRow(t, pool, locRow{code: "100", name: "A/1", uid: "u1", area: 7, lon: lon, lat: lat})

	if got := countRows(t, pool, "applications"); got != 1 {
		t.Errorf("applications rows = %d, want 1: the upsert was aborted", got)
	}
	if got := countRows(t, pool, "application_locations"); got != 0 {
		t.Errorf("thin rows = %d, want 0", got)
	}
}

func TestApplicationLocationsTrigger_WritesNothingWhenSyncOffOrNoControlRow(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, pool *pgxpool.Pool)
	}{
		{"no control row", func(*testing.T, *pgxpool.Pool) {}},
		{"thin_sync false", func(t *testing.T, pool *pgxpool.Pool) { setThinSync(t, pool, false) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool := thinPool(t)
			tc.setup(t, pool)
			lon, lat := lonLat(-0.1, 51.5)

			insertLocRow(t, pool, locRow{code: "100", name: "A/1", uid: "u1", area: 7, lon: lon, lat: lat})
			if _, err := pool.Exec(context.Background(), `UPDATE applications SET app_state = 'Approved'`); err != nil {
				t.Fatalf("update: %v", err)
			}

			if got := countRows(t, pool, "application_locations"); got != 0 {
				t.Errorf("thin rows = %d, want 0", got)
			}
		})
	}
}

func newTestLocations(t *testing.T, pool *pgxpool.Pool) *ApplicationLocations {
	t.Helper()
	l := NewApplicationLocations(pool, NewReadPathSwitch(NewPostgresReadPathStore(pool)), slog.New(slog.DiscardHandler), time.Now)
	l.sleep = func(context.Context, time.Duration) error { return nil }
	return l
}

func seedSource(t *testing.T, pool *pgxpool.Pool, rows ...locRow) {
	t.Helper()
	for _, r := range rows {
		insertLocRow(t, pool, r)
	}
}

func pt(lon, lat float64) (*float64, *float64) { return &lon, &lat }

func fixtureRows() []locRow {
	l1, a1 := pt(-0.10, 51.50)
	l2, a2 := pt(-2.20, 53.40)
	l3, a3 := pt(-0.12, 51.51)
	l4, a4 := pt(-1.90, 52.50)
	return []locRow{
		{code: "100", name: "A/1", uid: "u1", area: 1, state: "Undecided", start: "2026-01-01", lon: l1, lat: a1},
		{code: "100", name: "A/2", uid: "u2", area: 1, state: "Approved", lon: l3, lat: a3},
		{code: "200", name: "B/1", uid: "u3", area: 2, lon: l2, lat: a2},
		{code: "300", name: "C/1", uid: "u4", area: 3, state: "Rejected", start: "2026-03-03", lon: l4, lat: a4},
		{code: "300", name: "C/2", uid: "u5", area: 3},
	}
}

func TestApplicationLocationsLoad_SortsByGeohashAndSkipsNullLocations(t *testing.T) {
	pool := thinPool(t)
	seedSource(t, pool, fixtureRows()...)
	l := newTestLocations(t, pool)

	n, err := l.Load(context.Background())

	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if n != 4 {
		t.Errorf("Load() = %d rows, want 4", n)
	}
	if d := thinDrift(t, pool); d != 0 {
		t.Errorf("thin drifts from source by %d rows", d)
	}
	rows, err := pool.Query(context.Background(), `SELECT ST_GeoHash(location::geometry, 10) FROM application_locations ORDER BY ctid`)
	if err != nil {
		t.Fatalf("read in physical order: %v", err)
	}
	defer rows.Close()
	var prev string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if h < prev {
			t.Errorf("geohash %q follows %q: physical order is not sorted", h, prev)
		}
		prev = h
	}
}

func TestApplicationLocationsLoad_ErrorsOnNonEmptyTable(t *testing.T) {
	pool := thinPool(t)
	seedSource(t, pool, fixtureRows()...)
	l := newTestLocations(t, pool)
	if _, err := l.Load(context.Background()); err != nil {
		t.Fatalf("first Load() error = %v", err)
	}

	_, err := l.Load(context.Background())

	if !errors.Is(err, ErrThinTableNotEmpty) {
		t.Errorf("second Load() error = %v, want ErrThinTableNotEmpty", err)
	}
	if n := countRows(t, pool, "application_locations"); n != 4 {
		t.Errorf("thin rows = %d, want 4 (no duplicates)", n)
	}
}

func TestApplicationLocationsReconcile_RepairsMissingExtraAndStaleThenSecondRunRepairsNothing(t *testing.T) {
	pool := thinPool(t)
	ctx := context.Background()
	seedSource(t, pool, fixtureRows()...)
	l := newTestLocations(t, pool)
	if _, err := l.Load(ctx); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	setThinSync(t, pool, true)
	drift := []string{
		`DELETE FROM application_locations WHERE planit_name = 'A/1'`,
		`INSERT INTO application_locations (authority_code, planit_name, location) VALUES ('200', 'GHOST', ST_SetSRID(ST_MakePoint(0, 0), 4326)::geography)`,
		`INSERT INTO application_locations (authority_code, planit_name, location) SELECT authority_code, planit_name || 'X', location FROM applications WHERE planit_name = 'C/1'`,
		`UPDATE application_locations SET uid = 'stale' WHERE planit_name = 'A/2'`,
		`UPDATE application_locations SET area_id = 99 WHERE planit_name = 'B/1'`,
		`UPDATE application_locations SET app_state = 'stale' WHERE planit_name = 'C/1'`,
		`UPDATE application_locations SET start_date = '2000-01-01' WHERE planit_name = 'C/1'`,
		`UPDATE application_locations SET location = ST_SetSRID(ST_MakePoint(-1.9, 52.5000001), 4326)::geography WHERE planit_name = 'C/1'`,
	}
	for _, q := range drift {
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	first, err := l.Reconcile(ctx, true)

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if first.Upserted != 4 || first.Deleted != 2 {
		t.Errorf("first run = %+v, want 4 upserted and 2 deleted", first)
	}
	if d := thinDrift(t, pool); d != 0 {
		t.Errorf("thin drifts from source by %d rows after repair", d)
	}
	second, err := l.Reconcile(ctx, true)
	if err != nil {
		t.Fatalf("second Reconcile() error = %v", err)
	}
	if second.Upserted != 0 || second.Deleted != 0 {
		t.Errorf("second run = %+v, want zero repairs", second)
	}
}

func TestApplicationLocationsReconcile_NullLocationSourceRowsAreExtrasNotMissing(t *testing.T) {
	pool := thinPool(t)
	ctx := context.Background()
	seedSource(t, pool, locRow{code: "100", name: "N/1", uid: "u", area: 1})
	setThinSync(t, pool, true)
	l := newTestLocations(t, pool)

	res, err := l.Reconcile(ctx, true)
	if err != nil || res.Upserted != 0 || res.Deleted != 0 {
		t.Fatalf("absent from thin: res = %+v, err = %v, want zero repairs", res, err)
	}

	if _, err := pool.Exec(ctx, `INSERT INTO application_locations (authority_code, planit_name, location) VALUES ('100', 'N/1', ST_SetSRID(ST_MakePoint(0, 0), 4326)::geography)`); err != nil {
		t.Fatalf("seed extra: %v", err)
	}
	res, err = l.Reconcile(ctx, true)
	if err != nil || res.Deleted != 1 || res.Upserted != 0 {
		t.Fatalf("present in thin: res = %+v, err = %v, want 1 deleted", res, err)
	}
	res, err = l.Reconcile(ctx, true)
	if err != nil || res.Upserted != 0 || res.Deleted != 0 {
		t.Errorf("second pass: res = %+v, err = %v, want zero repairs", res, err)
	}
}

func TestApplicationLocationsReconcile_DoesNothingWhileSyncOff(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, pool *pgxpool.Pool)
	}{
		{"no control row", func(*testing.T, *pgxpool.Pool) {}},
		{"thin_sync false", func(t *testing.T, pool *pgxpool.Pool) { setThinSync(t, pool, false) }},
	} {
		for _, all := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s all=%v", tc.name, all), func(t *testing.T) {
				pool := thinPool(t)
				seedSource(t, pool, fixtureRows()...)
				tc.setup(t, pool)
				l := newTestLocations(t, pool)

				res, err := l.Reconcile(context.Background(), all)

				if err != nil {
					t.Fatalf("Reconcile() error = %v", err)
				}
				if !res.Skipped || res.Upserted != 0 || res.Deleted != 0 {
					t.Errorf("res = %+v, want skipped with no repairs", res)
				}
				if n := countRows(t, pool, "application_locations"); n != 0 {
					t.Errorf("thin rows = %d, want 0", n)
				}
			})
		}
	}
}

func TestApplicationLocationsReconcile_DailyRunCoversOnlyTodaysBucket(t *testing.T) {
	pool := thinPool(t)
	ctx := context.Background()
	seedSource(t, pool, fixtureRows()...)
	setThinSync(t, pool, true)
	buckets := map[string]int{}
	for _, code := range []string{"100", "200", "300"} {
		var b int
		if err := pool.QueryRow(ctx, `SELECT abs(hashtext($1)::bigint) % 7`, code).Scan(&b); err != nil {
			t.Fatalf("bucket: %v", err)
		}
		buckets[code] = b
	}
	var day time.Time
	for d := 0; d < 7; d++ {
		cand := time.Date(2026, 11, 1+d, 10, 30, 0, 0, time.UTC)
		if cand.Weekday() == time.Weekday(buckets["100"]) {
			day = cand
		}
	}
	l := newTestLocations(t, pool)
	l.now = func() time.Time { return day }

	if _, err := l.Reconcile(ctx, false); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	rows, err := pool.Query(ctx, `SELECT DISTINCT authority_code FROM application_locations`)
	if err != nil {
		t.Fatalf("read thin authorities: %v", err)
	}
	got := map[string]bool{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[c] = true
	}
	rows.Close()
	for code, b := range buckets {
		want := b == buckets["100"]
		if got[code] != want {
			t.Errorf("authority %s (bucket %d) reconciled = %v, want %v", code, b, got[code], want)
		}
	}
}

func waitForBlockedBackend(t *testing.T, pool *pgxpool.Pool, like string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND query LIKE $1`, like).Scan(&n); err != nil {
			t.Fatalf("poll pg_stat_activity: %v", err)
		}
		if n > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no backend blocked on a lock for %q", like)
}

func TestApplicationLocationsReconcile_ConcurrentWriteWinsOverStaleSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name     string
		write    string
		wantRows int
		wantSt   string
	}{
		{"update is not overwritten", `UPDATE applications SET app_state = 'Fresh' WHERE planit_name = 'A/1'`, 1, "Fresh"},
		{"deleted row does not return", `DELETE FROM applications WHERE planit_name = 'A/1'`, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := thinPool(t)
			ctx := context.Background()
			lon, lat := pt(-0.1, 51.5)
			seedSource(t, pool, locRow{code: "100", name: "A/1", uid: "u1", area: 1, state: "Old", lon: lon, lat: lat})
			l := newTestLocations(t, pool)
			if _, err := l.Load(ctx); err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			setThinSync(t, pool, true)
			if _, err := pool.Exec(ctx, `UPDATE application_locations SET app_state = 'Stale'`); err != nil {
				t.Fatalf("seed drift: %v", err)
			}

			held, err := pool.Begin(ctx)
			if err != nil {
				t.Fatalf("begin: %v", err)
			}
			t.Cleanup(func() { _ = held.Rollback(context.Background()) })
			if _, err := held.Exec(ctx, tc.write); err != nil {
				t.Fatalf("held write: %v", err)
			}

			done := make(chan error, 1)
			go func() {
				_, err := l.Reconcile(ctx, true)
				done <- err
			}()
			waitForBlockedBackend(t, pool, "INSERT INTO application_locations%")
			if err := held.Commit(ctx); err != nil {
				t.Fatalf("commit held write: %v", err)
			}
			if err := <-done; err != nil {
				t.Fatalf("Reconcile() error = %v", err)
			}

			if n := countRows(t, pool, "application_locations"); n != tc.wantRows {
				t.Fatalf("thin rows = %d, want %d", n, tc.wantRows)
			}
			if tc.wantRows == 1 {
				var st string
				if err := pool.QueryRow(ctx, `SELECT app_state FROM application_locations`).Scan(&st); err != nil || st != tc.wantSt {
					t.Errorf("app_state = %q (err %v), want %q", st, err, tc.wantSt)
				}
			}
			if d := thinDrift(t, pool); d != 0 {
				t.Errorf("thin drifts from source by %d rows", d)
			}
		})
	}
}

func TestApplicationLocationsReconcile_RowDeletedDuringLoadDoesNotRemain(t *testing.T) {
	pool := thinPool(t)
	ctx := context.Background()
	seedSource(t, pool, fixtureRows()...)
	l := newTestLocations(t, pool)

	held, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = held.Rollback(context.Background()) })
	if _, err := held.Exec(ctx, `DELETE FROM applications WHERE planit_name = 'A/1'`); err != nil {
		t.Fatalf("held delete: %v", err)
	}
	if _, err := l.Load(ctx); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := held.Commit(ctx); err != nil {
		t.Fatalf("commit delete: %v", err)
	}
	if n := countRows(t, pool, "application_locations"); n != 4 {
		t.Fatalf("thin rows after load = %d, want 4 including the row deleted meanwhile", n)
	}
	setThinSync(t, pool, true)

	if _, err := l.Reconcile(ctx, true); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if n := countRows(t, pool, "application_locations"); n != 3 {
		t.Errorf("thin rows = %d, want 3", n)
	}
	if d := thinDrift(t, pool); d != 0 {
		t.Errorf("thin drifts from source by %d rows", d)
	}
}

func TestApplicationLocationsReconcile_DeadlockRetriesTheBatchAndSucceeds(t *testing.T) {
	pool := thinPool(t)
	ctx := context.Background()
	seedSource(t, pool, fixtureRows()...)
	setThinSync(t, pool, true)
	l := newTestLocations(t, pool)
	attempts := 0
	l.beforeBatch = func(context.Context, pgx.Tx) error {
		attempts++
		if attempts == 1 {
			return &pgconn.PgError{Code: "40P01"}
		}
		return nil
	}

	res, err := l.Reconcile(ctx, true)

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if res.Upserted != 4 || res.Failed != 0 {
		t.Errorf("res = %+v, want 4 upserted and no failures", res)
	}
	if attempts < 2 {
		t.Errorf("batch attempts = %d, want the failed batch retried", attempts)
	}
	if d := thinDrift(t, pool); d != 0 {
		t.Errorf("thin drifts from source by %d rows", d)
	}
}

func TestApplicationLocationsReconcile_PersistentDeadlockMovesOnToNextAuthority(t *testing.T) {
	pool := thinPool(t)
	ctx := context.Background()
	seedSource(t, pool, fixtureRows()...)
	setThinSync(t, pool, true)
	l := newTestLocations(t, pool)
	l.beforeBatch = func(context.Context, pgx.Tx) error { return &pgconn.PgError{Code: "40P01"} }

	res, err := l.Reconcile(ctx, true)

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if res.Failed != 3 || res.Upserted != 0 {
		t.Errorf("res = %+v, want 3 failed authorities and no repairs", res)
	}
}

func TestApplicationLocationsReconcile_RepairsLargeAuthorityInBatches(t *testing.T) {
	pool := thinPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO applications (authority_code, planit_name, uid, area_name, area_id, location, last_different)
		SELECT '900', 'N/' || g, 'u' || g, 'Area', 1, ST_SetSRID(ST_MakePoint(-1 + g * 0.0001, 52), 4326)::geography, now() FROM generate_series(1, 450) g`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	setThinSync(t, pool, true)
	l := newTestLocations(t, pool)
	batches := 0
	l.beforeBatch = func(context.Context, pgx.Tx) error { batches++; return nil }

	res, err := l.Reconcile(ctx, true)

	if err != nil || res.Upserted != 450 {
		t.Fatalf("res = %+v, err = %v, want 450 upserted", res, err)
	}
	if batches != 3 {
		t.Errorf("batches = %d, want 3 (200 + 200 + 50)", batches)
	}
}
