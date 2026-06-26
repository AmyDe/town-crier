package postgres

import (
	"context"
	"testing"
)

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
