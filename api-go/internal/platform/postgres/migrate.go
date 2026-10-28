package postgres

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

const (
	defaultLockTimeout     = 5 * time.Second
	defaultNoTxLockTimeout = 120 * time.Second
)

// LockTimeouts bounds how long a migration waits for a lock. NoTx applies to
// files marked "-- +goose NO TRANSACTION", whose concurrent index builds also
// wait on older transactions.
type LockTimeouts struct {
	Normal time.Duration
	NoTx   time.Duration
}

// IsLockNotAvailable reports whether err carries SQLSTATE 55P03 (lock_not_available).
func IsLockNotAvailable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "55P03"
}

func hasNoTransaction(sql string) bool {
	return strings.Contains(sql, "-- +goose NO TRANSACTION")
}

// Migrate applies all pending embedded goose migrations to the database at dsn.
// Each migration runs under a session lock_timeout (5s, or 120s for NO
// TRANSACTION files) and fails with SQLSTATE 55P03 if it cannot get its lock.
// Migrations are idempotent, so calling Migrate against an up-to-date database
// is a no-op.
func Migrate(ctx context.Context, dsn string) error {
	return MigrateFS(ctx, dsn, migrationsFS, "migrations", LockTimeouts{Normal: defaultLockTimeout, NoTx: defaultNoTxLockTimeout})
}

// MigrateFS is Migrate over an arbitrary migration filesystem and lock limits.
// The database/sql pool is capped at one connection so each SET lock_timeout
// applies to the migration that follows it.
func MigrateFS(ctx context.Context, dsn string, fsys fs.FS, dir string, lt LockTimeouts) (err error) {
	migrateMu.Lock()
	defer migrateMu.Unlock()

	connConfig, parseErr := pgx.ParseConfig(dsn)
	if parseErr != nil {
		return fmt.Errorf("parse dsn: %w", parseErr)
	}

	db := stdlib.OpenDB(*connConfig)
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	defer func() {
		if cerr := db.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close migration db: %w", cerr)
		}
	}()

	goose.SetBaseFS(fsys)
	if dialectErr := goose.SetDialect("postgres"); dialectErr != nil {
		return fmt.Errorf("set goose dialect: %w", dialectErr)
	}

	for {
		timeout, ok, nextErr := nextLockTimeout(ctx, db, fsys, dir, lt)
		if nextErr != nil {
			return nextErr
		}
		if !ok {
			return nil
		}
		if _, setErr := db.ExecContext(ctx, fmt.Sprintf("SET lock_timeout = %d", timeout.Milliseconds())); setErr != nil {
			return fmt.Errorf("set lock_timeout: %w", setErr)
		}
		if upErr := goose.UpByOneContext(ctx, db, dir); upErr != nil {
			if errors.Is(upErr, goose.ErrNoNextVersion) {
				return nil
			}
			return fmt.Errorf("goose up: %w", upErr)
		}
	}
}

var migrationFileVersion = regexp.MustCompile(`^(\d+)_.*\.sql$`)

// nextLockTimeout returns the lock limit for the first pending migration, or
// ok=false when none is pending.
func nextLockTimeout(ctx context.Context, db *sql.DB, fsys fs.FS, dir string, lt LockTimeouts) (timeout time.Duration, ok bool, err error) {
	current, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		return 0, false, fmt.Errorf("goose db version: %w", err)
	}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return 0, false, fmt.Errorf("read migrations: %w", err)
	}
	var nextVersion int64
	var nextName string
	for _, e := range entries {
		m := migrationFileVersion.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		v, _ := strconv.ParseInt(m[1], 10, 64)
		if v > current && (nextName == "" || v < nextVersion) {
			nextVersion, nextName = v, e.Name()
		}
	}
	if nextName == "" {
		return 0, false, nil
	}
	body, err := fs.ReadFile(fsys, dir+"/"+nextName)
	if err != nil {
		return 0, false, fmt.Errorf("read migration %s: %w", nextName, err)
	}
	if hasNoTransaction(string(body)) {
		return lt.NoTx, true, nil
	}
	return lt.Normal, true, nil
}

// InvalidIndexes lists indexes in the database at dsn that a failed CONCURRENTLY
// build left INVALID.
func InvalidIndexes(ctx context.Context, dsn string) ([]string, error) {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	rows, err := conn.Query(ctx, `SELECT n.nspname || '.' || c.relname
FROM pg_index i
JOIN pg_class c ON c.oid = i.indexrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE NOT i.indisvalid
ORDER BY 1`)
	if err != nil {
		return nil, fmt.Errorf("query invalid indexes: %w", err)
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("scan invalid indexes: %w", err)
	}
	return names, nil
}
