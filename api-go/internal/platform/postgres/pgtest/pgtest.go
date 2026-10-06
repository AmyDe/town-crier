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

// serializeLockKey is the fixed key for the cross-process advisory lock New
// acquires. Its only purpose is to make every test that touches the shared
// database mutually exclusive; the value is arbitrary.
const serializeLockKey int64 = 0x70_67_74_65_73_74 // "pgtest"

// New returns a migrated connection pool for the test database. It reads the DSN
// from TEST_DATABASE_URL (or the docker-compose default), pings to confirm
// reachability, and applies all pending migrations. If the database is
// unreachable the test is skipped with instructions to bring the stack up. The
// pool is closed automatically via t.Cleanup.
//
// New holds a session-level advisory lock from before migration until the test
// ends, so tests using this harness run one at a time even across packages:
// `go test ./...` runs package binaries in parallel against one database, and
// without the lock they race in goose up on an empty database and TRUNCATE each
// other's fixtures. A test that calls New must NOT call t.Parallel: the lock is
// released in t.Cleanup, which for a parallel test runs only after the whole
// parallel group finishes, so two parallel New callers would deadlock.
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

	// Registered before the lock cleanup so it runs LAST (cleanups are LIFO): the
	// lock is released and its connection returned before the pool is closed.
	t.Cleanup(pool.Close)
	acquireSerializeLock(t, pool)

	if err := postgres.Migrate(ctx, dsn); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	return pool
}

// acquireSerializeLock takes the global advisory lock on a dedicated pooled
// connection and registers a cleanup that releases it. See New for why.
//
// It polls pg_try_advisory_lock rather than blocking in pg_advisory_lock: a
// blocked waiter keeps a statement open, and CREATE INDEX CONCURRENTLY in the
// lock holder's migration waits for every open statement to finish, so a
// blocking wait hangs the suite in a cycle Postgres cannot detect.
func acquireSerializeLock(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	conn, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire advisory-lock connection: %v", err)
	}
	for {
		var locked bool
		if err := conn.QueryRow(context.Background(), "SELECT pg_try_advisory_lock($1)", serializeLockKey).Scan(&locked); err != nil {
			conn.Release()
			t.Fatalf("acquire advisory lock: %v", err)
		}
		if locked {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Cleanup(func() {
		if _, err := conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", serializeLockKey); err != nil {
			t.Logf("release advisory lock: %v", err)
		}
		conn.Release()
	})
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
