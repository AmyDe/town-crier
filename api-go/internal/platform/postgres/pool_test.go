package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// fakeTokenSource is a hand-written stand-in for the Entra token acquirer so the
// connection-layer tests never touch live Azure.
type fakeTokenSource struct {
	token string
	err   error
}

func (f fakeTokenSource) Token(ctx context.Context) (string, error) {
	return f.token, f.err
}

func testMIParams() ConnParams {
	return ConnParams{
		Host:    "psql-town-crier-shared.postgres.database.azure.com",
		DB:      "town_crier_dev",
		User:    "towncrier_api",
		SSLMode: "require",
	}
}

func TestMIDSN_OmitsPassword(t *testing.T) {
	t.Parallel()
	dsn := miDSN(testMIParams())
	if !strings.Contains(dsn, "towncrier_api@") {
		t.Fatalf("miDSN() = %q, want user@host form", dsn)
	}
	if strings.Contains(dsn, "towncrier_api:") {
		t.Fatalf("miDSN() = %q, must not embed a password after the user", dsn)
	}
	if !strings.Contains(dsn, "sslmode=require") {
		t.Fatalf("miDSN() = %q, want sslmode=require", dsn)
	}
}

func TestBuildMIPoolConfig_InjectsTokenAsPassword(t *testing.T) {
	// Not parallel: clears PGPASSWORD so an ambient env var can't bleed into the
	// no-password assertion below.
	t.Setenv("PGPASSWORD", "")

	const token = "fake-entra-token-xyz"
	cfg, err := buildMIPoolConfig(testMIParams(), fakeTokenSource{token: token})
	if err != nil {
		t.Fatalf("buildMIPoolConfig() error = %v", err)
	}
	if cfg.BeforeConnect == nil {
		t.Fatal("buildMIPoolConfig() left BeforeConnect nil, want a token hook")
	}
	if cfg.ConnConfig.Password != "" {
		t.Fatalf("parsed DSN carried a password %q, want none", cfg.ConnConfig.Password)
	}

	cc := &pgx.ConnConfig{}
	if err := cfg.BeforeConnect(context.Background(), cc); err != nil {
		t.Fatalf("BeforeConnect() error = %v", err)
	}
	if cc.Password != token {
		t.Fatalf("BeforeConnect set Password = %q, want %q", cc.Password, token)
	}
}

func TestBuildMIPoolConfig_BeforeConnectPropagatesTokenError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("token boom")
	cfg, err := buildMIPoolConfig(testMIParams(), fakeTokenSource{err: sentinel})
	if err != nil {
		t.Fatalf("buildMIPoolConfig() error = %v", err)
	}
	gotErr := cfg.BeforeConnect(context.Background(), &pgx.ConnConfig{})
	if !errors.Is(gotErr, sentinel) {
		t.Fatalf("BeforeConnect() error = %v, want it to wrap %v", gotErr, sentinel)
	}
}

func TestNewPoolFromEnv_DefaultsToPasswordPath(t *testing.T) {
	cases := []struct {
		name string
		auth string
	}{
		{"unset", ""},
		{"password", "password"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const dsn = "postgres://towncrier:towncrier@localhost:5433/towncrier_test?sslmode=disable"
			t.Setenv("TEST_DATABASE_URL", dsn)
			t.Setenv("POSTGRES_AUTH", tc.auth)

			pool, err := NewPoolFromEnv(context.Background())
			if err != nil {
				t.Fatalf("NewPoolFromEnv() error = %v", err)
			}
			defer pool.Close()
			if got := pool.Config().ConnString(); got != dsn {
				t.Fatalf("default path used DSN %q, want DSNFromEnv %q", got, dsn)
			}
		})
	}
}

func TestNewPoolFromEnv_ManagedIdentityBuildsPasswordlessPool(t *testing.T) {
	t.Setenv("POSTGRES_AUTH", "azure-managed-identity")
	t.Setenv("POSTGRES_HOST", "psql-town-crier-shared.postgres.database.azure.com")
	t.Setenv("POSTGRES_DB", "town_crier_dev")
	t.Setenv("POSTGRES_USER", "towncrier_api")
	t.Setenv("POSTGRES_SSLMODE", "require")
	t.Setenv("PGPASSWORD", "")

	pool, err := NewPoolFromEnv(context.Background())
	if err != nil {
		t.Fatalf("NewPoolFromEnv() error = %v", err)
	}
	defer pool.Close()
	conn := pool.Config().ConnString()
	if !strings.Contains(conn, "town_crier_dev") {
		t.Fatalf("MI pool DSN %q missing target database", conn)
	}
	if strings.Contains(conn, "towncrier_api:") {
		t.Fatalf("MI pool DSN %q must not embed a password", conn)
	}
}

func TestNewPoolFromEnv_SSLModeDefaultsToRequire(t *testing.T) {
	t.Setenv("POSTGRES_AUTH", "azure-managed-identity")
	t.Setenv("POSTGRES_HOST", "example.postgres.database.azure.com")
	t.Setenv("POSTGRES_DB", "town_crier_dev")
	t.Setenv("POSTGRES_USER", "towncrier_api")
	t.Setenv("POSTGRES_SSLMODE", "")
	t.Setenv("PGPASSWORD", "")

	pool, err := NewPoolFromEnv(context.Background())
	if err != nil {
		t.Fatalf("NewPoolFromEnv() error = %v", err)
	}
	defer pool.Close()
	if got := pool.Config().ConnConfig.TLSConfig; got == nil {
		t.Fatal("MI pool with default sslmode left TLS unconfigured, want sslmode=require")
	}
}

func TestDSNFromEnv_DefaultsWhenUnset(t *testing.T) {
	t.Setenv("TEST_DATABASE_URL", "")
	got := DSNFromEnv()
	want := "postgres://towncrier:towncrier@localhost:5433/towncrier_test?sslmode=disable"
	if got != want {
		t.Fatalf("DSNFromEnv() = %q, want default %q", got, want)
	}
}

func TestDSNFromEnv_HonoursOverride(t *testing.T) {
	want := "postgres://u:p@example.test:6000/db?sslmode=require"
	t.Setenv("TEST_DATABASE_URL", want)
	if got := DSNFromEnv(); got != want {
		t.Fatalf("DSNFromEnv() = %q, want override %q", got, want)
	}
}

func TestNewPool_RejectsMalformedDSN(t *testing.T) {
	t.Parallel()
	// An unescaped space in the password makes URL parsing fail, so the error
	// surfaces at construction without any network access.
	_, err := NewPool(context.Background(), "postgres://user:pass word@host:5432/db")
	if err == nil {
		t.Fatal("NewPool() with a malformed DSN returned nil error, want a parse error")
	}
}

func TestWithStatementGuards_SetsSessionLimitsAndServerCancel(t *testing.T) {
	t.Parallel()
	cfg, err := pgxpool.ParseConfig("postgres://u@localhost:5432/db?sslmode=disable")
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}

	WithStatementGuards(10*time.Minute, 10*time.Second)(cfg)

	params := cfg.ConnConfig.RuntimeParams
	if got := params["statement_timeout"]; got != "600000" {
		t.Errorf("statement_timeout = %q, want 600000", got)
	}
	if got := params["client_connection_check_interval"]; got != "10000" {
		t.Errorf("client_connection_check_interval = %q, want 10000", got)
	}
	h, ok := cfg.ConnConfig.BuildContextWatcherHandler(&pgconn.PgConn{}).(*pgconn.CancelRequestContextWatcherHandler)
	if !ok {
		t.Fatal("context watcher handler is not a CancelRequestContextWatcherHandler")
	}
	if h.CancelRequestDelay != 0 || h.DeadlineDelay <= 0 {
		t.Errorf("handler delays = (%v, %v), want (0, > 0)", h.CancelRequestDelay, h.DeadlineDelay)
	}
}

func TestNewPoolFromEnv_AppliesOptionsOnBothAuthPaths(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
	}{
		{"password", map[string]string{
			"POSTGRES_AUTH":     "",
			"TEST_DATABASE_URL": "postgres://towncrier:towncrier@localhost:5433/towncrier_test?sslmode=disable",
		}},
		{"managed identity", map[string]string{
			"POSTGRES_AUTH": "azure-managed-identity",
			"POSTGRES_HOST": "example.postgres.database.azure.com",
			"POSTGRES_DB":   "town_crier_dev",
			"POSTGRES_USER": "towncrier_api",
			"PGPASSWORD":    "",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			pool, err := NewPoolFromEnv(context.Background(), WithStatementGuards(time.Minute, time.Second))
			if err != nil {
				t.Fatalf("NewPoolFromEnv() error = %v", err)
			}
			defer pool.Close()
			if got := pool.Config().ConnConfig.RuntimeParams["statement_timeout"]; got != "60000" {
				t.Fatalf("statement_timeout = %q, want 60000", got)
			}
		})
	}
}
