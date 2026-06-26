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
func Migrate(ctx context.Context, dsn string) (err error) {
	migrateMu.Lock()
	defer migrateMu.Unlock()

	connConfig, parseErr := pgx.ParseConfig(dsn)
	if parseErr != nil {
		return fmt.Errorf("parse dsn: %w", parseErr)
	}

	db := stdlib.OpenDB(*connConfig)
	defer func() {
		if cerr := db.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close migration db: %w", cerr)
		}
	}()

	goose.SetBaseFS(migrationsFS)
	if dialectErr := goose.SetDialect("postgres"); dialectErr != nil {
		return fmt.Errorf("set goose dialect: %w", dialectErr)
	}
	if upErr := goose.UpContext(ctx, db, "migrations"); upErr != nil {
		return fmt.Errorf("goose up: %w", upErr)
	}
	return nil
}
