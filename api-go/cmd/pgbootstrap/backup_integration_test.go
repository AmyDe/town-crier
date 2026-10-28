//go:build integration

package main

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

func TestBackupRole_CanReadBackupTablesButNotApplicationsOrWrite(t *testing.T) {
	pool := pgtest.New(t)
	ctx := context.Background()
	const role = "tc_backup_grants_test"

	if _, err := pool.Exec(ctx, "DROP OWNED BY "+role); err != nil {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42704" {
			t.Fatalf("pre-clean role: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, "DROP ROLE IF EXISTS "+role); err != nil {
		t.Fatalf("drop role: %v", err)
	}
	if _, err := pool.Exec(ctx, "CREATE ROLE "+role+" NOLOGIN"); err != nil {
		t.Fatalf("create role: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP OWNED BY "+role)
		_, _ = pool.Exec(context.Background(), "DROP ROLE IF EXISTS "+role)
	})

	sql, err := buildBackupGrantsSQL(bootstrapParams{Role: role, DB: "towncrier_test", OID: testMIObjectID})
	if err != nil {
		t.Fatalf("render grants: %v", err)
	}
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("apply grants: %v", err)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(conn.Release)
	if _, err := conn.Exec(ctx, "SET ROLE "+role); err != nil {
		t.Fatalf("set role: %v", err)
	}
	t.Cleanup(func() { _, _ = conn.Exec(context.Background(), "RESET ROLE") })

	allowed := []string{"SELECT count(*) FROM users", "SELECT count(*) FROM offer_code_redemptions", "SELECT max(version_id) FROM goose_db_version"}
	for _, q := range allowed {
		if _, err := conn.Exec(ctx, q); err != nil {
			t.Errorf("%q should be allowed: %v", q, err)
		}
	}

	denied := []string{
		"SELECT count(*) FROM applications",
		"INSERT INTO notification_state (user_id, last_read_at) VALUES ('x', now())",
		"UPDATE users SET tier = 'pro'",
		"DELETE FROM users",
	}
	for _, q := range denied {
		_, err := conn.Exec(ctx, q)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("%q: error = %v, want permission denied (42501)", q, err)
		}
	}
}
