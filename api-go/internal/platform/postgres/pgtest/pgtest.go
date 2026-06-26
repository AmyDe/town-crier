//go:build integration

// Package pgtest is the real-database test harness for the integration-tagged
// suite. It connects to the local Postgres + PostGIS (docker-compose), applies
// the embedded migrations, and hands tests a ready-to-seed pool. When no
// database is reachable it skips rather than fails, so a developer without the
// compose stack up sees a clean skip.
package pgtest

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres"
)

// New returns a migrated connection pool for the test database. It reads the DSN
// from TEST_DATABASE_URL (or the docker-compose default), pings to confirm
// reachability, and applies all pending migrations. If the database is
// unreachable the test is skipped with instructions to bring the stack up. The
// pool is closed automatically via t.Cleanup.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := postgres.DSNFromEnv()
	ctx := context.Background()

	pool, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		t.Skipf("postgres not reachable at %s: %v; run `docker compose -f api-go/docker-compose.yml up -d`", dsn, err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		t.Skipf("postgres not reachable at %s: %v; run `docker compose -f api-go/docker-compose.yml up -d`", dsn, err)
	}

	if err := postgres.Migrate(ctx, dsn); err != nil {
		pool.Close()
		t.Fatalf("migrate test database: %v", err)
	}

	t.Cleanup(pool.Close)
	return pool
}

// Truncate empties the named tables (RESTART IDENTITY CASCADE) so each test
// starts from a clean, deterministic state. Table names are sanitised as SQL
// identifiers; they originate from test code, never user input.
func Truncate(t *testing.T, pool *pgxpool.Pool, tables ...string) {
	t.Helper()
	if len(tables) == 0 {
		return
	}

	quoted := make([]string, len(tables))
	for i, table := range tables {
		quoted[i] = pgx.Identifier{table}.Sanitize()
	}
	stmt := "TRUNCATE " + strings.Join(quoted, ", ") + " RESTART IDENTITY CASCADE"

	if _, err := pool.Exec(context.Background(), stmt); err != nil {
		t.Fatalf("truncate %v: %v", tables, err)
	}
}
