//go:build integration

package pgbackup

import (
	"compress/gzip"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

func TestTableClassification_EveryTableIsIncludedOrExcluded(t *testing.T) {
	pool := pgtest.New(t)
	known := map[string]bool{}
	for _, tb := range Tables {
		known[tb.Name] = true
	}
	for _, n := range Excluded {
		if known[n] {
			t.Fatalf("table %q is in both Tables and Excluded", n)
		}
		known[n] = true
	}

	rows, err := pool.Query(context.Background(),
		`SELECT table_name FROM information_schema.tables
		 WHERE table_schema = 'public' AND table_type = 'BASE TABLE'`)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer rows.Close()
	present := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		present[name] = true
		if !known[name] {
			t.Errorf("table %q is in neither pgbackup.Tables nor pgbackup.Excluded: classify it", name)
		}
	}
	for _, tb := range Tables {
		if !present[tb.Name] {
			t.Errorf("included table %q does not exist", tb.Name)
		}
	}
}

func seed(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	stmts := []string{
		`INSERT INTO users (user_id, email, push_enabled, digest_day, zone_preferences, tier, last_active_at, last_active_at_epoch)
		 VALUES ('u1', 'a@example.com', true, 1, '{"z":{"a":[1,2]}}', 'pro', now(), 1),
		        ('u2', NULL, false, 2, '{}', 'free', now(), 2)`,
		`INSERT INTO watch_zones (id, user_id, name, location, radius_metres)
		 VALUES ('11111111-1111-1111-1111-111111111111', 'u1', 'Home, "quoted"', ST_SetSRID(ST_MakePoint(-0.1278, 51.5074), 4326)::geography, 500)`,
		`INSERT INTO saved_applications (user_id, application_uid, saved_at, snapshot)
		 VALUES ('u1', 'app1', now(), '{"k":"v\nline"}'), ('u1', 'app2', now(), NULL)`,
		`INSERT INTO notifications (id, user_id, application_uid, authority_id, event_type, created_at)
		 VALUES ('n1', 'u1', 'app1', 7, 'NewApplication', now())`,
		`INSERT INTO notification_state (user_id, last_read_at) VALUES ('u1', now())`,
		`INSERT INTO device_registrations (user_id, token, platform, registered_at) VALUES ('u1', 'tok', 'ios', now())`,
		`INSERT INTO offer_codes (code, tier, duration_days, created_at, max_redemptions, redemption_count)
		 VALUES ('CODE1', 'pro', 30, now(), 5, 1)`,
		`INSERT INTO offer_code_redemptions (code, user_id, redeemed_at) VALUES ('CODE1', 'u1', now()), ('CODE1', NULL, NULL)`,
	}
	for _, s := range stmts {
		if _, err := pool.Exec(context.Background(), s); err != nil {
			t.Fatalf("seed %q: %v", s, err)
		}
	}
}

type memSink struct{ blobs map[string][]byte }

func (m *memSink) Put(_ context.Context, name string, body io.Reader) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m.blobs[name] = data
	return nil
}

func TestExporter_RoundTripsAllTablesThroughCopyFrom(t *testing.T) {
	pool := pgtest.New(t)
	ctx := context.Background()
	names := make([]string, len(Tables))
	for i, tb := range Tables {
		names[i] = tb.Name
	}
	pgtest.Truncate(t, pool, names...)
	seed(t, pool)

	sink := &memSink{blobs: map[string][]byte{}}
	now := func() time.Time { return time.Date(2026, 10, 28, 5, 15, 0, 0, time.UTC) }
	res, err := New(NewPGSource(pool), sink, now, slog.New(slog.DiscardHandler)).Run(ctx)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Rows != 11 {
		t.Errorf("total rows = %d, want 11", res.Rows)
	}

	// Capture the original rows as text, wipe, restore from the dump, compare.
	before := snapshotText(t, pool)
	pgtest.Truncate(t, pool, names...)
	for _, tb := range Tables {
		zr, err := gzip.NewReader(strings.NewReader(string(sink.blobs["2026/10/28/"+tb.Name+".csv.gz"])))
		if err != nil {
			t.Fatalf("gzip %s: %v", tb.Name, err)
		}
		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = conn.Conn().PgConn().CopyFrom(ctx, zr, "COPY "+tb.Name+" FROM STDIN WITH (FORMAT csv, HEADER)")
		conn.Release()
		if err != nil {
			t.Fatalf("copy from %s: %v", tb.Name, err)
		}
	}
	after := snapshotText(t, pool)
	for _, tb := range Tables {
		if before[tb.Name] != after[tb.Name] {
			t.Errorf("table %s differs after restore\nbefore: %s\nafter:  %s", tb.Name, before[tb.Name], after[tb.Name])
		}
	}
	if _, ok := sink.blobs["2026/10/28/manifest.json"]; !ok {
		t.Error("manifest missing")
	}
}

func snapshotText(t *testing.T, pool *pgxpool.Pool) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, tb := range Tables {
		var s string
		err := pool.QueryRow(context.Background(),
			"SELECT COALESCE(string_agg(x::text, E'\\n' ORDER BY x::text), '') FROM (SELECT * FROM "+tb.Name+") x").Scan(&s)
		if err != nil {
			t.Fatalf("snapshot %s: %v", tb.Name, err)
		}
		out[tb.Name] = s
	}
	return out
}
