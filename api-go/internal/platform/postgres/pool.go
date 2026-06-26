// Package postgres provides the connection pool and migration plumbing for the
// local Postgres + PostGIS test environment. It is Phase 0 of the Cosmos ->
// Postgres migration (docs/memo/0010): it stands up a real, seedable database
// that the integration-tagged tests run against. No production code path uses
// it yet — Cosmos remains the live datastore.
package postgres

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// defaultDSN points at the docker-compose Postgres on host port 5433. It is used
// when TEST_DATABASE_URL is unset so a developer with the compose stack up can
// run the integration tests with no extra configuration.
const defaultDSN = "postgres://towncrier:towncrier@localhost:5433/towncrier_test?sslmode=disable"

// DSNFromEnv returns the test database DSN from TEST_DATABASE_URL, falling back
// to the docker-compose default when the variable is unset or empty.
func DSNFromEnv() string {
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}
	return defaultDSN
}

// NewPool constructs a pgx connection pool for the given DSN. The pool opens
// connections lazily, so a malformed DSN fails here at parse time while an
// unreachable-but-valid DSN succeeds and only errors when a query is issued
// (callers ping to detect that — see the pgtest helper).
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	return pool, nil
}
