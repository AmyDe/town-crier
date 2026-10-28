//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"testing/fstest"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

func TestMigrateFS_BlockedLockFailsWithinTimeout(t *testing.T) {
	pool := pgtest.New(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, "CREATE TABLE IF NOT EXISTS lock_probe (id int)"); err != nil {
		t.Fatalf("create probe: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP TABLE IF EXISTS lock_probe") })

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	if _, err := tx.Exec(ctx, "LOCK TABLE lock_probe IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatalf("lock: %v", err)
	}

	fsys := fstest.MapFS{
		"migrations/99991_probe.sql": {Data: []byte("-- +goose Up\nALTER TABLE lock_probe ADD COLUMN x int;\n-- +goose Down\nALTER TABLE lock_probe DROP COLUMN x;\n")},
	}
	start := time.Now()
	err = postgres.MigrateFS(ctx, postgres.DSNFromEnv(), fsys, "migrations", postgres.LockTimeouts{Normal: 300 * time.Millisecond, NoTx: time.Second})
	if err == nil {
		t.Fatal("want lock error, got nil")
	}
	if !postgres.IsLockNotAvailable(err) {
		t.Fatalf("want 55P03, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("took %s, want fast failure", elapsed)
	}
}

func TestInvalidIndexes_ReportsInvalidIndex(t *testing.T) {
	pool := pgtest.New(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, "CREATE TABLE invalid_idx_probe (id int)"); err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP TABLE IF EXISTS invalid_idx_probe") })
	if _, err := pool.Exec(ctx, "CREATE INDEX invalid_idx_probe_i ON invalid_idx_probe (id)"); err != nil {
		t.Fatalf("index: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE pg_index SET indisvalid = false WHERE indexrelid = 'invalid_idx_probe_i'::regclass"); err != nil {
		t.Skipf("cannot mark index invalid without superuser: %v", err)
	}

	got, err := postgres.InvalidIndexes(ctx, postgres.DSNFromEnv())
	if err != nil {
		t.Fatalf("InvalidIndexes: %v", err)
	}
	found := false
	for _, n := range got {
		if n == "public.invalid_idx_probe_i" {
			found = true
		}
	}
	if !found {
		t.Fatalf("got %v, want public.invalid_idx_probe_i", got)
	}
}
