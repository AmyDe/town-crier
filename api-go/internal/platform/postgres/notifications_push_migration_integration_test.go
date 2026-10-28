//go:build integration

package postgres_test

import (
	"context"
	"io/fs"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

const pushStatusMigrationVersion = 44

func TestNotificationsPushMigrations_UpDownUp(t *testing.T) {
	pool := pgtest.New(t)
	ctx := context.Background()

	assertPushSchema(ctx, t, pool, true)

	cfg, err := pgx.ParseConfig(postgres.DSNFromEnv())
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	db := stdlib.OpenDB(*cfg)
	t.Cleanup(func() { _ = db.Close() })

	migrations, err := fs.Sub(os.DirFS("."), "migrations")
	if err != nil {
		t.Fatalf("sub migrations fs: %v", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations)
	if err != nil {
		t.Fatalf("new goose provider: %v", err)
	}

	t.Cleanup(func() {
		if _, err := provider.Up(ctx); err != nil {
			t.Errorf("restore schema: %v", err)
		}
	})

	if _, err := provider.DownTo(ctx, pushStatusMigrationVersion-1); err != nil {
		t.Fatalf("down: %v", err)
	}
	assertPushSchema(ctx, t, pool, false)

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	assertPushSchema(ctx, t, pool, true)
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func assertPushSchema(ctx context.Context, t *testing.T, q queryRower, wantPresent bool) {
	t.Helper()

	var columns int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns
		WHERE table_name = 'notifications' AND column_name IN ('push_status', 'push_attempted_at')`).Scan(&columns); err != nil {
		t.Fatalf("count columns: %v", err)
	}
	wantColumns := 0
	if wantPresent {
		wantColumns = 2
	}
	if columns != wantColumns {
		t.Errorf("push columns = %d, want %d", columns, wantColumns)
	}

	var valid *bool
	err := q.QueryRow(ctx, `SELECT i.indisvalid FROM pg_index i
		JOIN pg_class c ON c.oid = i.indexrelid
		WHERE c.relname = 'notifications_push_pending'`).Scan(&valid)
	switch {
	case wantPresent && err != nil:
		t.Fatalf("read notifications_push_pending: %v", err)
	case wantPresent && (valid == nil || !*valid):
		t.Errorf("notifications_push_pending is not valid")
	case !wantPresent && err == nil:
		t.Errorf("notifications_push_pending still exists after down")
	}
}
