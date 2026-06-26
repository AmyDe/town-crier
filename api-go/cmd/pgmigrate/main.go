// Command pgmigrate applies the embedded goose schema migrations to a Town
// Crier app database on the shared Azure Database for PostgreSQL Flexible
// Server. It is a one-time, idempotent data-plane operation run by the Entra
// admin (a locally `az login`-ed human), not part of any deploy.
//
// The app role (towncrier_api) is DML-only by design and cannot run DDL, so the
// migrations must be applied by a privileged principal — the Entra admin = repo
// owner account, the same principal that ran cmd/pgbootstrap. Running them as
// the owner makes the owner the table owner, which is what the #653 bootstrap's
// ALTER DEFAULT PRIVILEGES relies on to auto-grant towncrier_api DML on the
// newly created tables.
//
// It authenticates passwordlessly: a short-lived Entra access token (scope
// https://ossrdbms-aad.database.windows.net/.default) is fetched from a
// DefaultAzureCredential and used as the connection password. goose opens a
// single stdlib connection and runs well within the token's validity, so no
// per-connection refresh is needed (unlike the long-lived API pool).
//
// Usage:
//
//	pgmigrate -host <fqdn> -admin-user <aad-admin-upn> [-db town_crier_dev] \
//	    [-sslmode require] [-timeout 60s]
package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres"
)

// defaultSSLMode is applied when -sslmode resolves to empty. require is the
// minimum for Azure Database for PostgreSQL Flexible Server.
const defaultSSLMode = "require"

// azurePostgresTokenScope is the fixed OSS RDBMS AAD resource for Azure Database
// for PostgreSQL. It is a public OAuth scope identifier, not a secret.
//
//nolint:gosec // G101: public AAD resource scope URL, not a credential
const azurePostgresTokenScope = "https://ossrdbms-aad.database.windows.net/.default"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("pgmigrate", flag.ContinueOnError)
	var (
		host      = fs.String("host", os.Getenv("POSTGRES_HOST"), "Postgres server FQDN")
		adminUser = fs.String("admin-user", os.Getenv("POSTGRES_ADMIN_USER"), "Entra admin principal name (UPN) to connect as")
		db        = fs.String("db", envOr("POSTGRES_DB", "town_crier_dev"), "app database to migrate")
		sslMode   = fs.String("sslmode", envOr("POSTGRES_SSLMODE", defaultSSLMode), "sslmode")
		timeout   = fs.Duration("timeout", 60*time.Second, "overall timeout")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *host == "" || *adminUser == "" {
		fmt.Fprintln(os.Stderr, "pgmigrate: -host and -admin-user are required")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgmigrate: build credential: %v\n", err)
		return 1
	}

	// Fetch the Entra token up front and pass it as the connection password.
	// goose runs a single short-lived connection, so one token covers the run;
	// there is no need for the API pool's per-connection BeforeConnect refresh.
	tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{azurePostgresTokenScope}})
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgmigrate: acquire postgres access token: %v\n", err)
		return 1
	}

	dsn := migrateDSN(*host, *db, *adminUser, *sslMode, tok.Token)
	if err := postgres.Migrate(ctx, dsn); err != nil {
		fmt.Fprintf(os.Stderr, "pgmigrate: migrate %q: %v\n", *db, err)
		return 1
	}

	fmt.Printf("pgmigrate: migrations applied to %q on %s\n", *db, *host)
	return 0
}

// migrateDSN builds a token-as-password Postgres DSN from discrete inputs. The
// admin UPN (which contains '#' and '@') and the token are percent-encoded via
// url.UserPassword, so neither breaks the userinfo/host split. The token is a
// short-lived secret carried only in-memory for the single migration run.
func migrateDSN(host, db, user, sslMode, token string) string {
	if sslMode == "" {
		sslMode = defaultSSLMode
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, token),
		Host:     host,
		Path:     "/" + db,
		RawQuery: url.Values{"sslmode": {sslMode}}.Encode(),
	}
	return u.String()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
