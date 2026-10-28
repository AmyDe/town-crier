//go:build integration

package postgres

import (
	"context"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestNextLockTimeout_PicksLowestPendingAndItsLimit(t *testing.T) {
	ctx := context.Background()

	// goose_db_version lives in a private schema so the shared test database's
	// real migration history is untouched.
	const schema = "next_lock_timeout_probe"
	cfg, err := pgx.ParseConfig(DSNFromEnv())
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	cfg.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*cfg)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("postgres not reachable: %v", err)
	}
	if _, err := db.ExecContext(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE; CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })

	migrateMu.Lock()
	defer migrateMu.Unlock()
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set dialect: %v", err)
	}

	noTx := "-- +goose NO TRANSACTION\n-- +goose Up\nSELECT 1;\n"
	normal := "-- +goose Up\nSELECT 1;\n"
	fsys := fstest.MapFS{
		"m/00003_concurrent.sql": {Data: []byte(noTx)},
		"m/00001_concurrent.sql": {Data: []byte(noTx)},
		"m/00002_normal.sql":     {Data: []byte(normal)},
		"m/README.md":            {Data: []byte("not a migration")},
	}
	lt := LockTimeouts{Normal: time.Second, NoTx: 2 * time.Second}

	steps := []struct {
		applied int64
		want    time.Duration
		wantOK  bool
	}{
		{0, lt.NoTx, true},
		{1, lt.Normal, true},
		{2, lt.NoTx, true},
		{3, 0, false},
	}
	for _, s := range steps {
		if s.applied > 0 {
			if _, err := db.ExecContext(ctx, "INSERT INTO goose_db_version (version_id, is_applied) VALUES ($1, true)", s.applied); err != nil {
				t.Fatalf("mark %d applied: %v", s.applied, err)
			}
		}
		got, ok, err := nextLockTimeout(ctx, db, fsys, "m", lt)
		if err != nil {
			t.Fatalf("after version %d: %v", s.applied, err)
		}
		if ok != s.wantOK || got != s.want {
			t.Fatalf("after version %d: got (%v, %v), want (%v, %v)", s.applied, got, ok, s.want, s.wantOK)
		}
	}
}
