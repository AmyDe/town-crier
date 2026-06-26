package main

import (
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// TestResolveBackend pins the APPS_ZONES_BACKEND contract for the worker, exactly
// as cmd/api does: only the exact value "postgres" (whitespace-trimmed) selects
// Postgres; every other value, including unset and "cosmos", keeps Cosmos so prod
// (flag unset) is never silently flipped, and a non-canonical casing is treated
// as "any other value".
func TestResolveBackend(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		flag string
		want storeBackend
	}{
		{"exact postgres selects postgres", "postgres", backendPostgres},
		{"surrounding whitespace is trimmed", "  postgres\t", backendPostgres},
		{"unset defaults to cosmos", "", backendCosmos},
		{"explicit cosmos stays cosmos", "cosmos", backendCosmos},
		{"uppercase is not the canonical value", "Postgres", backendCosmos},
		{"junk defaults to cosmos", "nonsense", backendCosmos},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := resolveBackend(tc.flag); got != tc.want {
				t.Fatalf("resolveBackend(%q) = %v, want %v", tc.flag, got, tc.want)
			}
		})
	}
}

// TestChooseAppStore covers both the backend selection and the typed-nil trap:
// when the chosen backend has no backing store the chooser must return a GENUINE
// nil interface (not a typed-nil pointer boxed in a non-nil interface), so the
// worker's nil-guard discipline (a missing store leaves the mode unwired) holds.
// The selected interface is what the poll handler consumes for the Applications
// Upsert write.
func TestChooseAppStore(t *testing.T) {
	t.Parallel()

	cosmos := applications.NewCosmosStore(newFakeItems())
	pg := applications.NewPostgresStore(nil) // querier is never touched in these tests

	t.Run("postgres backend returns the postgres store", func(t *testing.T) {
		t.Parallel()
		got := chooseAppStore(backendPostgres, pg, cosmos)
		if _, ok := got.(*applications.PostgresStore); !ok {
			t.Fatalf("got %T, want *applications.PostgresStore", got)
		}
	})

	t.Run("cosmos backend returns the cosmos store", func(t *testing.T) {
		t.Parallel()
		got := chooseAppStore(backendCosmos, pg, cosmos)
		if _, ok := got.(*applications.CosmosStore); !ok {
			t.Fatalf("got %T, want *applications.CosmosStore", got)
		}
	})

	t.Run("postgres backend with no pool yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseAppStore(backendPostgres, nil, cosmos); got != nil {
			t.Fatalf("got %T (non-nil interface), want a genuine nil so the mode stays unwired", got)
		}
	})

	t.Run("cosmos backend with no container yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseAppStore(backendCosmos, pg, nil); got != nil {
			t.Fatalf("got %T (non-nil interface), want a genuine nil so the mode stays unwired", got)
		}
	})
}

// TestChooseZoneStore mirrors TestChooseAppStore for the watch-zone store — the
// store the worker threads into the notify fan-out (FindZonesContaining), the
// poll authority provider, the digest reader, and the dormant erasure cascade.
func TestChooseZoneStore(t *testing.T) {
	t.Parallel()

	cosmos := watchzones.NewCosmosStore(newFakeItems())
	pg := watchzones.NewPostgresStore(nil)

	t.Run("postgres backend returns the postgres store", func(t *testing.T) {
		t.Parallel()
		got := chooseZoneStore(backendPostgres, pg, cosmos)
		if _, ok := got.(*watchzones.PostgresStore); !ok {
			t.Fatalf("got %T, want *watchzones.PostgresStore", got)
		}
	})

	t.Run("cosmos backend returns the cosmos store", func(t *testing.T) {
		t.Parallel()
		got := chooseZoneStore(backendCosmos, pg, cosmos)
		if _, ok := got.(*watchzones.CosmosStore); !ok {
			t.Fatalf("got %T, want *watchzones.CosmosStore", got)
		}
	})

	t.Run("postgres backend with no pool yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseZoneStore(backendPostgres, nil, cosmos); got != nil {
			t.Fatalf("got %T (non-nil interface), want a genuine nil so the mode stays unwired", got)
		}
	})

	t.Run("cosmos backend with no container yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseZoneStore(backendCosmos, pg, nil); got != nil {
			t.Fatalf("got %T (non-nil interface), want a genuine nil so the mode stays unwired", got)
		}
	})
}
