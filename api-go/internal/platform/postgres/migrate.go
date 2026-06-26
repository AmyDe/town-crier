package postgres

import (
	"context"
	"embed"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrateMu serialises Migrate calls. goose's SetBaseFS/SetDialect mutate package
// globals, so concurrent callers (parallel tests each standing up the schema)
// would race; the mutex also serialises the idempotent goose.Up against the
// shared database.
var migrateMu sync.Mutex

// Migrate applies all pending embedded goose migrations to the database at dsn.
// It opens a short-lived *sql.DB via the pgx stdlib driver (goose needs the
// database/sql interface), runs goose.Up, and closes it. Migrations are
// idempotent, so calling Migrate repeatedly against an up-to-date database is a
// no-op.
func Migrate(ctx context.Context, dsn string) error {
	migrateMu.Lock()
	defer migrateMu.Unlock()

	connConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("parse dsn: %w", err)
	}

	db := stdlib.OpenDB(*connConfig)
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
