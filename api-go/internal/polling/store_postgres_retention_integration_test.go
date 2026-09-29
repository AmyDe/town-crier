//go:build integration

package polling

import (
	"context"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

func TestPostgresRetentionStore_PurgesOnlyStaleRows(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "planit_call", "poll_event", "application_event", "poll_oracle_diff", "delta_seen", "poll_window_member", "poll_window")
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	old90, new90 := now.AddDate(0, 0, -91), now.AddDate(0, 0, -89)
	old14, new14 := now.AddDate(0, 0, -15), now.AddDate(0, 0, -13)
	old120, new120 := now.AddDate(0, 0, -121), now.AddDate(0, 0, -119)

	stmts := []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO planit_call (at, work, page_index) VALUES ($1, 'window_start', 0), ($2, 'window_start', 0)", []any{old90, new90}},
		{"INSERT INTO poll_event (at, kind) VALUES ($1, 'surge'), ($2, 'surge')", []any{old90, new90}},
		{"INSERT INTO application_event (uid, authority_code, kind, detected_at) VALUES ('a', '1', 'decision', $1), ('b', '1', 'decision', $2)", []any{old90, new90}},
		{"INSERT INTO poll_oracle_diff (axis, day, uid, area_id, found_at) VALUES ('start', '2026-01-01', 'a', 1, $1), ('start', '2026-01-01', 'b', 1, $2)", []any{old90, new90}},
		{"INSERT INTO delta_seen (axis, day, uid, area_id, seen_at) VALUES ('start', '2026-09-01', 'a', 1, $1), ('start', '2026-09-01', 'b', 1, $2)", []any{old14, new14}},
		{"INSERT INTO poll_window_member (axis, day, uid, area_id, read_at) VALUES ('start', $1, 'a', 1, $3), ('start', $2, 'b', 1, $3)", []any{old120, new120, now}},
		{"INSERT INTO poll_window (axis, day) VALUES ('start', $1), ('start', $2)", []any{old120, new120}},
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s.sql, s.args...); err != nil {
			t.Fatalf("seed %q: %v", s.sql, err)
		}
	}

	got, err := NewPostgresRetentionStore(pool).Purge(ctx, now)
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}

	for _, table := range []string{"planit_call", "poll_event", "application_event", "poll_oracle_diff", "delta_seen", "poll_window_member", "poll_window"} {
		if got[table] != 1 {
			t.Errorf("%s purged = %d, want 1", table, got[table])
		}
		var remaining int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&remaining); err != nil {
			t.Fatal(err)
		}
		if remaining != 1 {
			t.Errorf("%s remaining = %d, want 1", table, remaining)
		}
	}
}
