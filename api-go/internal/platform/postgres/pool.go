// Package postgres provides the connection pool and migration plumbing for the
// Cosmos -> Postgres + PostGIS migration (docs/memo/0010, epic #645).
//
// DSNFromEnv + NewPool + Migrate stand up the local, seedable database the
// integration-tagged tests run against (Phase 0). NewPoolFromEnv adds
// auth-mode-aware connection for dev/prod: POSTGRES_AUTH=azure-managed-identity
// selects passwordless Entra-token auth (the token is injected as the
// connection password via pgx BeforeConnect), while any other value keeps the
// password/trust path untouched. No store reads are wired onto Postgres yet —
// Cosmos remains the live datastore; this is the MI-auth connection layer
// (#653) the later wiring phases build on.
package postgres

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// POSTGRES_AUTH selects how NewPoolFromEnv authenticates. azureManagedIdentity
// switches to passwordless Entra-token auth; any other value (unset or
// "password") keeps the existing password/trust path so pgtest and
// docker-compose are untouched.
const (
	authModeAzureManagedIdentity = "azure-managed-identity"
)

// defaultSSLMode is applied when POSTGRES_SSLMODE is unset. require is the
// minimum for Azure Database for PostgreSQL Flexible Server.
const defaultSSLMode = "require"

// azurePostgresTokenScope is the fixed OSS RDBMS AAD resource for Azure Database
// for PostgreSQL. It is not environment-specific.
const azurePostgresTokenScope = "https://ossrdbms-aad.database.windows.net/.default"

// defaultDSN points at the docker-compose Postgres on host port 5433. It is used
// when TEST_DATABASE_URL is unset so a developer with the compose stack up can
// run the integration tests with no extra configuration.
// G101 is a false positive here: these are throwaway local docker-compose
// credentials for the test database, not a real secret.
//
//nolint:gosec // G101: local test credentials, not a production secret
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

// ConnParams are the discrete, passwordless connection inputs for Entra
// managed-identity auth, read from the POSTGRES_* env vars. The password is
// never one of them: it is a short-lived Entra token minted per connection.
type ConnParams struct {
	Host    string
	DB      string
	User    string
	SSLMode string
}

// tokenSource yields a bearer token to use as the Postgres connection password.
// It is a consumer-side interface: the real implementation wraps an Azure
// credential and tests inject a hand-written fake, so neither the build nor the
// unit tests touch a live token endpoint.
type tokenSource interface {
	Token(ctx context.Context) (string, error)
}

// NewPoolFromEnv constructs a pgx pool whose authentication mode is selected by
// POSTGRES_AUTH. When the value is anything other than "azure-managed-identity"
// (including unset and "password") it is byte-for-byte the existing
// password/trust path — NewPool over DSNFromEnv — so pgtest, make
// test-integration, and docker-compose are unaffected. When it is
// "azure-managed-identity" it builds a passwordless pool that injects a fresh
// Entra token (scope azurePostgresTokenScope) as the connection password on
// every new physical connection, honouring AZURE_CLIENT_ID exactly as the
// Cosmos and Service Bus clients do.
func NewPoolFromEnv(ctx context.Context) (*pgxpool.Pool, error) {
	if os.Getenv("POSTGRES_AUTH") != authModeAzureManagedIdentity {
		return NewPool(ctx, DSNFromEnv())
	}

	cred, err := newManagedIdentityCredential(os.Getenv("AZURE_CLIENT_ID"))
	if err != nil {
		return nil, err
	}
	return NewTokenCredentialPool(ctx, paramsFromEnv(), cred)
}

// NewTokenCredentialPool builds a passwordless pgx pool that authenticates every
// physical connection with a fresh Entra access token (scope
// azurePostgresTokenScope) drawn from cred and used as the Postgres password.
// pgx's BeforeConnect runs per new connection, so the pool refreshes the token
// as it grows or replaces connections with no manual refresh loop. The admin
// bootstrap command reuses this with a DefaultAzureCredential.
func NewTokenCredentialPool(ctx context.Context, p ConnParams, cred azcore.TokenCredential) (*pgxpool.Pool, error) {
	return newTokenPool(ctx, p, aadTokenSource{cred: cred})
}

func newTokenPool(ctx context.Context, p ConnParams, ts tokenSource) (*pgxpool.Pool, error) {
	cfg, err := buildMIPoolConfig(p, ts)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create managed-identity postgres pool: %w", err)
	}
	return pool, nil
}

// buildMIPoolConfig parses a passwordless DSN built from p and attaches a
// BeforeConnect hook that sets each connection's password to a freshly fetched
// token. It opens no connection, so it is fully unit-testable with a fake
// tokenSource.
func buildMIPoolConfig(p ConnParams, ts tokenSource) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(miDSN(p))
	if err != nil {
		return nil, fmt.Errorf("parse managed-identity dsn: %w", err)
	}
	cfg.BeforeConnect = func(ctx context.Context, cc *pgx.ConnConfig) error {
		token, terr := ts.Token(ctx)
		if terr != nil {
			return fmt.Errorf("acquire postgres access token: %w", terr)
		}
		cc.Password = token
		return nil
	}
	return cfg, nil
}

// miDSN builds a passwordless Postgres DSN from the discrete parameters. The
// user is encoded with no password (url.User, not url.UserPassword), so the
// connection string never carries a secret; the password arrives per connection
// via BeforeConnect.
func miDSN(p ConnParams) string {
	sslMode := p.SSLMode
	if sslMode == "" {
		sslMode = defaultSSLMode
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.User(p.User),
		Host:     p.Host,
		Path:     "/" + p.DB,
		RawQuery: url.Values{"sslmode": {sslMode}}.Encode(),
	}
	return u.String()
}

// paramsFromEnv reads the discrete passwordless connection inputs, defaulting
// sslmode to require.
func paramsFromEnv() ConnParams {
	return ConnParams{
		Host:    os.Getenv("POSTGRES_HOST"),
		DB:      os.Getenv("POSTGRES_DB"),
		User:    os.Getenv("POSTGRES_USER"),
		SSLMode: os.Getenv("POSTGRES_SSLMODE"),
	}
}

// aadTokenSource fetches Entra access tokens for Azure Database for PostgreSQL
// from an azcore.TokenCredential and returns the raw token for use as the
// connection password.
type aadTokenSource struct {
	cred azcore.TokenCredential
}

func (s aadTokenSource) Token(ctx context.Context) (string, error) {
	tok, err := s.cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{azurePostgresTokenScope},
	})
	if err != nil {
		return "", fmt.Errorf("get azure postgres token: %w", err)
	}
	return tok.Token, nil
}

// newManagedIdentityCredential builds a user-assigned managed-identity
// credential pinned to clientID when set, mirroring the Cosmos and Service Bus
// wiring (cosmos.go:213, servicebus/client.go:70). An empty clientID falls back
// to the ambient managed identity.
func newManagedIdentityCredential(clientID string) (azcore.TokenCredential, error) {
	credOpts := &azidentity.ManagedIdentityCredentialOptions{}
	if clientID != "" {
		credOpts.ID = azidentity.ClientID(clientID)
	}
	cred, err := azidentity.NewManagedIdentityCredential(credOpts)
	if err != nil {
		return nil, fmt.Errorf("build managed-identity credential: %w", err)
	}
	return cred, nil
}
