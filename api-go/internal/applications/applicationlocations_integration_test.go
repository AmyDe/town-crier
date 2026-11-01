//go:build integration

package applications

import (
	"context"
	"testing"

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
