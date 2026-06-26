// Command pgbootstrap maps the Town Crier API's Azure managed identity to a
// least-privilege Postgres role on the shared Flexible Server, then grants it
// DML only. It is a one-time, idempotent data-plane operation run by the Entra
// admin (a locally `az login`-ed human), not part of any deploy.
//
// It connects as the admin using the admin's own Entra token as the connection
// password (the same pgx BeforeConnect trick the API uses), via
// DefaultAzureCredential. The bootstrap SQL lives in the checked-in, reviewable
// bootstrap.sql and is rendered with validated identifiers. The command runs no
// destructive statement.
//
// Usage:
//
//	pgbootstrap -host <fqdn> -admin-user <aad-admin-upn> -db town_crier_dev \
//	    -mi-oid <managed-identity-object-id> [-role towncrier_api]
//	pgbootstrap -host <fqdn> -admin-user <aad-admin-upn> -db town_crier_dev -verify
//
// -verify connects (as the admin) and prints SELECT current_user so the operator
// can confirm Entra auth to the server works.
package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres"
)

// bootstrapSQLTemplate is the reviewable, checked-in least-privilege bootstrap.
//
//go:embed bootstrap.sql
var bootstrapSQLTemplate string

// identifierPattern matches a safe, unquoted SQL identifier (role / database
// name). OID values must be a UUID. Both are validated before being templated
// into SQL, so bootstrap.sql is not a SQL-injection surface.
var (
	identifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	uuidPattern       = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// bootstrapParams are the validated identifiers rendered into the bootstrap SQL.
type bootstrapParams struct {
	Role string
	DB   string
	OID  string
}

func (p bootstrapParams) validate() error {
	if !identifierPattern.MatchString(p.Role) {
		return fmt.Errorf("invalid role name %q: must be a plain SQL identifier", p.Role)
	}
	if !identifierPattern.MatchString(p.DB) {
		return fmt.Errorf("invalid database name %q: must be a plain SQL identifier", p.DB)
	}
	if !uuidPattern.MatchString(p.OID) {
		return fmt.Errorf("invalid managed-identity object id %q: must be a UUID", p.OID)
	}
	return nil
}

// buildBootstrapSQL validates the identifiers and renders the embedded template.
// Returning the SQL (rather than executing it) keeps the assembly fully
// unit-testable.
func buildBootstrapSQL(p bootstrapParams) (string, error) {
	if err := p.validate(); err != nil {
		return "", err
	}
	tmpl, err := template.New("bootstrap").Option("missingkey=error").Parse(bootstrapSQLTemplate)
	if err != nil {
		return "", fmt.Errorf("parse bootstrap template: %w", err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, p); err != nil {
		return "", fmt.Errorf("render bootstrap template: %w", err)
	}
	return buf.String(), nil
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("pgbootstrap", flag.ContinueOnError)
	var (
		host      = fs.String("host", os.Getenv("POSTGRES_HOST"), "Postgres server FQDN")
		adminUser = fs.String("admin-user", os.Getenv("POSTGRES_ADMIN_USER"), "Entra admin principal name (UPN) to connect as")
		db        = fs.String("db", envOr("POSTGRES_DB", "town_crier_dev"), "target database")
		miOID     = fs.String("mi-oid", os.Getenv("POSTGRES_MI_OBJECT_ID"), "managed-identity object id to map the role to")
		role      = fs.String("role", envOr("POSTGRES_ROLE", "towncrier_api"), "Postgres role name to create/grant")
		sslMode   = fs.String("sslmode", envOr("POSTGRES_SSLMODE", "require"), "sslmode")
		timeout   = fs.Duration("timeout", 30*time.Second, "overall timeout")
		verify    = fs.Bool("verify", false, "connect and print SELECT current_user, then exit (no bootstrap)")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *host == "" || *adminUser == "" {
		fmt.Fprintln(os.Stderr, "pgbootstrap: -host and -admin-user are required")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgbootstrap: build credential: %v\n", err)
		return 1
	}

	pool, err := postgres.NewTokenCredentialPool(ctx, postgres.ConnParams{
		Host:    *host,
		DB:      *db,
		User:    *adminUser,
		SSLMode: *sslMode,
	}, cred)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgbootstrap: build pool: %v\n", err)
		return 1
	}
	defer pool.Close()

	if *verify {
		return verifyConnection(ctx, pool)
	}

	sql, err := buildBootstrapSQL(bootstrapParams{Role: *role, DB: *db, OID: *miOID})
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgbootstrap: %v\n", err)
		return 2
	}

	// The rendered SQL carries no parameters, so pgx runs it via the simple
	// protocol, executing all statements (including the guarded DO block) in one
	// round trip. Identifiers were validated before rendering.
	if _, err := pool.Exec(ctx, sql); err != nil {
		fmt.Fprintf(os.Stderr, "pgbootstrap: run bootstrap: %v\n", err)
		return 1
	}

	fmt.Printf("pgbootstrap: bootstrapped role %q on database %q\n", *role, *db)
	return verifyConnection(ctx, pool)
}

func verifyConnection(ctx context.Context, pool *pgxpool.Pool) int {
	var currentUser string
	if err := pool.QueryRow(ctx, "SELECT current_user").Scan(&currentUser); err != nil {
		fmt.Fprintf(os.Stderr, "pgbootstrap: verify: %v\n", err)
		return 1
	}
	fmt.Printf("pgbootstrap: connected as %q\n", currentUser)
	return 0
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
