//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

func TestWithStatementGuards_AppliesToConnections(t *testing.T) {
	observer := pgtest.New(t)
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, postgres.DSNFromEnv(), postgres.WithStatementGuards(10*time.Minute, 10*time.Second))
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)

	for setting, want := range map[string]string{"statement_timeout": "10min", "client_connection_check_interval": "10s"} {
		var got string
		if err := pool.QueryRow(ctx, "SHOW "+setting).Scan(&got); err != nil {
			t.Fatalf("SHOW %s: %v", setting, err)
		}
		if got != want {
			t.Errorf("%s = %q, want %q", setting, got, want)
		}
	}

	queryCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	_, err = pool.Exec(queryCtx, "SELECT pg_sleep(30) /* guard-cancel-probe */")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "57014" {
		t.Fatalf("Exec error = %v, want the server to cancel the statement (SQLSTATE 57014)", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		var running int
		if err := observer.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity
			WHERE state = 'active' AND query LIKE '%guard-cancel-probe%' AND pid <> pg_backend_pid()`).Scan(&running); err != nil {
			t.Fatalf("read pg_stat_activity: %v", err)
		}
		if running == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("cancelled query still running on the server after 3s")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
