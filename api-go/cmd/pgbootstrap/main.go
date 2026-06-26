// Command pgbootstrap maps the Town Crier API's Azure managed identity to a
// least-privilege Postgres role on the shared Flexible Server, then grants it
// DML only. It is a one-time, idempotent data-plane operation run by the Entra
// admin (a locally `az login`-ed human), not part of any deploy.
//
// The bootstrap runs in two ordered phases, each on its own connection:
//
//  1. Phase 1 — admin DB (default: postgres): create the Entra-mapped role and
//     grant it CONNECT on the app database. The pgaadauth_create_principal_with_oid
//     function is provided per-database by the pgaadauth extension, which Azure
//     enables only in the postgres admin database. Postgres roles are
//     cluster-global, so the role is immediately usable by the app database.
//
//  2. Phase 2 — app DB (default: town_crier_dev): grant DML on the app
//     database's schema, tables, and sequences. ALTER DEFAULT PRIVILEGES ensures
//     future tables created by goose migrations are also covered.
//
// Both phases are idempotent: the principal create is guarded by a pg_roles
// existence check, and every GRANT is idempotent by design.
//
// Usage:
//
//	pgbootstrap -host <fqdn> -admin-user <aad-admin-upn> \
//	    [-admin-db postgres] [-db town_crier_dev] \
//	    -mi-oid <managed-identity-object-id> [-role towncrier_api]
//	pgbootstrap -host <fqdn> -admin-user <aad-admin-upn> -db town_crier_dev -verify
//
// -verify connects to the app DB (as the admin) and prints SELECT current_user so
// the operator can confirm Entra auth to the server works.
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

// principalSQLTemplate is the Phase 1 SQL (admin DB): create the Entra-mapped
// role and grant it CONNECT on the app database.
//
//go:embed principal.sql
var principalSQLTemplate string

// grantsSQLTemplate is the Phase 2 SQL (app DB): DML and sequence grants for
// the role created in Phase 1.
//
//go:embed grants.sql
var grantsSQLTemplate string

// identifierPattern matches a safe, unquoted SQL identifier (role / database
// name). OID values must be a UUID. Both are validated before being templated
// into SQL, so neither principal.sql nor grants.sql is a SQL-injection surface.
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

// buildPrincipalSQL validates identifiers and renders the Phase 1 template.
// Phase 1 must run against the admin database (default: postgres) where the
// pgaadauth extension is available. Returning the SQL keeps rendering fully
// unit-testable without a live database.
func buildPrincipalSQL(p bootstrapParams) (string, error) {
	return renderTemplate("principal", principalSQLTemplate, p)
}

// buildGrantsSQL validates identifiers and renders the Phase 2 template.
// Phase 2 must run against the app database (default: town_crier_dev).
// Returning the SQL keeps rendering fully unit-testable without a live database.
func buildGrantsSQL(p bootstrapParams) (string, error) {
	return renderTemplate("grants", grantsSQLTemplate, p)
}

// renderTemplate validates p and renders the named SQL template. It is the
// shared implementation for buildPrincipalSQL and buildGrantsSQL so that
// validation and rendering are never accidentally skipped in one phase.
func renderTemplate(name, tmplText string, p bootstrapParams) (string, error) {
	if err := p.validate(); err != nil {
		return "", err
	}
	tmpl, err := template.New(name).Option("missingkey=error").Parse(tmplText)
	if err != nil {
		return "", fmt.Errorf("parse %s template: %w", name, err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, p); err != nil {
		return "", fmt.Errorf("render %s template: %w", name, err)
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
		adminDB   = fs.String("admin-db", envOr("POSTGRES_ADMIN_DB", "postgres"), "admin database where pgaadauth functions live (Phase 1)")
		db        = fs.String("db", envOr("POSTGRES_DB", "town_crier_dev"), "app database to grant on (Phase 2)")
		miOID     = fs.String("mi-oid", os.Getenv("POSTGRES_MI_OBJECT_ID"), "managed-identity object id to map the role to")
		role      = fs.String("role", envOr("POSTGRES_ROLE", "towncrier_api"), "Postgres role name to create/grant")
		sslMode   = fs.String("sslmode", envOr("POSTGRES_SSLMODE", "require"), "sslmode")
		timeout   = fs.Duration("timeout", 30*time.Second, "overall timeout")
		verify    = fs.Bool("verify", false, "connect to the app DB and print SELECT current_user, then exit (no bootstrap)")
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

	// Phase 1 pool — admin database (postgres): pgaadauth extension lives here.
	adminPool, err := postgres.NewTokenCredentialPool(ctx, postgres.ConnParams{
		Host:    *host,
		DB:      *adminDB,
		User:    *adminUser,
		SSLMode: *sslMode,
	}, cred)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgbootstrap: build admin pool: %v\n", err)
		return 1
	}
	defer adminPool.Close()

	// Phase 2 pool — app database: DML grants are scoped per-database.
	appPool, err := postgres.NewTokenCredentialPool(ctx, postgres.ConnParams{
		Host:    *host,
		DB:      *db,
		User:    *adminUser,
		SSLMode: *sslMode,
	}, cred)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgbootstrap: build app pool: %v\n", err)
		return 1
	}
	defer appPool.Close()

	if *verify {
		return verifyConnection(ctx, appPool)
	}

	params := bootstrapParams{Role: *role, DB: *db, OID: *miOID}

	// Phase 1: create the Entra-mapped role in the admin DB where pgaadauth lives.
	principalSQL, err := buildPrincipalSQL(params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgbootstrap: %v\n", err)
		return 2
	}
	// The rendered SQL carries no parameters so pgx uses the simple protocol,
	// executing all statements (including the guarded DO block) in one round trip.
	if _, err := adminPool.Exec(ctx, principalSQL); err != nil {
		fmt.Fprintf(os.Stderr, "pgbootstrap: phase 1 (principal create): %v\n", err)
		return 1
	}
	fmt.Printf("pgbootstrap: phase 1 complete — role %q mapped to OID %s\n", *role, *miOID)

	// Phase 2: grant DML on the app database. Roles are cluster-global, so the
	// role created in Phase 1 is visible here immediately.
	grantsSQL, err := buildGrantsSQL(params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgbootstrap: %v\n", err)
		return 2
	}
	if _, err := appPool.Exec(ctx, grantsSQL); err != nil {
		fmt.Fprintf(os.Stderr, "pgbootstrap: phase 2 (grants): %v\n", err)
		return 1
	}
	fmt.Printf("pgbootstrap: phase 2 complete — DML grants applied on %q\n", *db)

	return verifyConnection(ctx, appPool)
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
