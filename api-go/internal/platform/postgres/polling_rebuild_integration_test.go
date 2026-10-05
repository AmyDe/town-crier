//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

var pollingRebuildTables = []string{
	"planit_call", "poll_window", "delta_seen", "application_event", "poll_window_member", "poll_oracle_diff",
}

func TestPollingRebuildMigration_CreatesTables(t *testing.T) {
	pool := pgtest.New(t)
	for _, table := range pollingRebuildTables {
		var exists bool
		err := pool.QueryRow(context.Background(), `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("lookup %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %s missing", table)
		}
	}
}

func TestPollingRebuildMigration_LegacyStateTablesRemain(t *testing.T) {
	pool := pgtest.New(t)
	for _, table := range []string{"poll_state", "backfill_state", "recent_sweep_state"} {
		var exists bool
		if err := pool.QueryRow(context.Background(), `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
			t.Fatalf("lookup %s: %v", table, err)
		}
		if !exists {
			t.Errorf("legacy table %s must not be dropped", table)
		}
	}
}

func TestPollingRebuildMigration_Constraints(t *testing.T) {
	pool := pgtest.New(t)
	ctx := context.Background()
	pgtest.Truncate(t, pool, pollingRebuildTables...)

	mustExec := func(t *testing.T, sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("exec %q: %v", sql, err)
		}
	}

	tests := []struct {
		name     string
		sql      string
		wantCode string
		setup    func(t *testing.T)
	}{
		{name: "poll_window rejects unknown axis", sql: `INSERT INTO poll_window (axis, day) VALUES ('bogus', '2026-09-20')`, wantCode: "23514"},
		{name: "poll_window primary key", sql: `INSERT INTO poll_window (axis, day) VALUES ('start', '2026-09-20')`, wantCode: "23505",
			setup: func(t *testing.T) { mustExec(t, `INSERT INTO poll_window (axis, day) VALUES ('start', '2026-09-20')`) }},
		{name: "application_event rejects unknown kind", sql: `INSERT INTO application_event (uid, authority_code, kind) VALUES ('u', '1', 'churn')`, wantCode: "23514"},
		{name: "application_event rejects unknown status", sql: `INSERT INTO application_event (uid, authority_code, kind, status) VALUES ('u', '1', 'decision', 'held')`, wantCode: "23514"},
		{name: "delta_seen primary key", sql: `INSERT INTO delta_seen (axis, day, uid, area_id, seen_at) VALUES ('start', '2026-09-20', 'u', 1, now())`, wantCode: "23505",
			setup: func(t *testing.T) {
				mustExec(t, `INSERT INTO delta_seen (axis, day, uid, area_id, seen_at) VALUES ('start', '2026-09-20', 'u', 1, now())`)
			}},
		{name: "poll_window_member primary key", sql: `INSERT INTO poll_window_member (axis, day, uid, area_id, read_at) VALUES ('start', '2026-09-20', 'u', 1, now())`, wantCode: "23505",
			setup: func(t *testing.T) {
				mustExec(t, `INSERT INTO poll_window_member (axis, day, uid, area_id, read_at) VALUES ('start', '2026-09-20', 'u', 1, now())`)
			}},
		{name: "poll_oracle_diff primary key", sql: `INSERT INTO poll_oracle_diff (axis, day, uid, area_id, found_at) VALUES ('start', '2026-09-20', 'u', 1, now())`, wantCode: "23505",
			setup: func(t *testing.T) {
				mustExec(t, `INSERT INTO poll_oracle_diff (axis, day, uid, area_id, found_at) VALUES ('start', '2026-09-20', 'u', 1, now())`)
			}},
		{name: "planit_call requires at", sql: `INSERT INTO planit_call (work, page_index) VALUES ('window_start', 0)`, wantCode: "23502"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pgtest.Truncate(t, pool, pollingRebuildTables...)
			if tc.setup != nil {
				tc.setup(t)
			}
			assertPgCode(t, pool, tc.sql, tc.wantCode)
		})
	}
}

func TestPollingRebuildMigration_EventDefaultsAndPendingIndex(t *testing.T) {
	pool := pgtest.New(t)
	ctx := context.Background()
	pgtest.Truncate(t, pool, pollingRebuildTables...)

	var status string
	err := pool.QueryRow(ctx,
		`INSERT INTO application_event (uid, authority_code, kind) VALUES ('u', '1', 'new_application') RETURNING status`).Scan(&status)
	if err != nil {
		t.Fatalf("insert event: %v", err)
	}
	if status != "pending" {
		t.Errorf("default status: got %q, want pending", status)
	}

	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_indexes WHERE indexname IN ('application_event_pending','planit_call_at')`).Scan(&n); err != nil {
		t.Fatalf("index lookup: %v", err)
	}
	if n != 2 {
		t.Errorf("indexes: got %d, want 2", n)
	}
}

func assertPgCode(t *testing.T, pool *pgxpool.Pool, sql, wantCode string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), sql)
	pgErr, ok := err.(*pgconn.PgError) //nolint:errorlint // pgx returns *PgError directly from Exec
	if !ok {
		t.Fatalf("want *pgconn.PgError %s, got %v", wantCode, err)
	}
	if pgErr.Code != wantCode {
		t.Fatalf("SQLSTATE: got %s (%s), want %s", pgErr.Code, pgErr.Message, wantCode)
	}
}
