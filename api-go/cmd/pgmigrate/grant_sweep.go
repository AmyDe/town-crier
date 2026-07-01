package main

import (
	"context"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"
)

// grantRolePattern matches a safe, unquoted SQL identifier, mirroring
// cmd/pgbootstrap's identifierPattern. The app-role name is validated against it
// before being interpolated into the grant sweep, so buildGrantSweepSQL is not a
// SQL-injection surface: the role is the only interpolated value and the two
// GRANT statements are the only dynamic SQL the migrator emits.
var grantRolePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// buildGrantSweepSQL returns the idempotent post-migration grant sweep for role,
// or an error if role is not a plain SQL identifier (in which case no SQL is
// emitted). It re-grants the DML-only app role everything a migration may have
// just created, ownership-agnostically via ON ALL TABLES / ALL SEQUENCES. It is
// pure and unit-tested; grantAppRole is the thin DB wiring around it.
func buildGrantSweepSQL(role string) (string, error) {
	if !grantRolePattern.MatchString(role) {
		return "", fmt.Errorf("invalid grant role %q: must be a plain SQL identifier", role)
	}
	// role is a validated plain SQL identifier, so interpolating it is safe.
	return fmt.Sprintf(
		"GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO %s;\n"+
			"GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO %s;\n",
		role, role,
	), nil
}

// grantAppRole runs the idempotent post-migration grant sweep so the DML-only
// app role keeps SELECT/INSERT/UPDATE/DELETE on every table (and USAGE/SELECT on
// every sequence) the migrations just created, regardless of which identity owns
// them. This closes the table-ownership hazard in ADR 0036: whoever runs a
// migration grants the app DML on the tables it created at creation time,
// without relying on ALTER DEFAULT PRIVILEGES and its owner-scoping.
//
// An empty role skips the sweep (logged, not an error). It reuses the Entra
// token already carried in dsn by opening its own short-lived connection; it
// does NOT fetch a second token.
func grantAppRole(ctx context.Context, dsn, role string) error {
	if role == "" {
		fmt.Println("pgmigrate: grant sweep skipped (no app role configured)")
		return nil
	}

	sweepSQL, err := buildGrantSweepSQL(role)
	if err != nil {
		return err
	}

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect for grant sweep: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	// The sweep carries no parameters, so pgx uses the simple protocol and runs
	// both GRANTs in one round trip. GRANT ... ON ALL TABLES emits a benign
	// NOTICE ("no privileges were granted for ...") for objects the migrator
	// cannot grant on (tables owned by another role, PostGIS system views); a
	// NOTICE is not an error, so Exec returns nil and those objects keep their
	// existing bootstrap grants. Only a genuine SQL error (e.g. the role does not
	// exist) returns non-nil here, which fails the deploy loudly.
	if _, err := conn.Exec(ctx, sweepSQL); err != nil {
		return fmt.Errorf("execute grant sweep for role %q: %w", role, err)
	}

	fmt.Printf("pgmigrate: grant sweep applied — DML granted to %q\n", role)
	return nil
}
