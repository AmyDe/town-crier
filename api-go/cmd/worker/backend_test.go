package main

import (
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/notificationstate"
	"github.com/AmyDe/town-crier/api-go/internal/offercodes"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/savedapplications"
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

// TestResolveStoreBackend pins the STORE_BACKEND contract: only the exact value
// "postgres" (whitespace-trimmed) routes ALL stores to Postgres. It is
// structurally identical to resolveBackend so the two flags use consistent
// semantics.
func TestResolveStoreBackend(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		flag string
		want storeBackend
	}{
		{"exact postgres selects postgres", "postgres", backendPostgres},
		{"surrounding whitespace is trimmed", " postgres ", backendPostgres},
		{"unset defaults to cosmos", "", backendCosmos},
		{"explicit cosmos stays cosmos", "cosmos", backendCosmos},
		{"uppercase is not the canonical value", "Postgres", backendCosmos},
		{"junk defaults to cosmos", "nonsense", backendCosmos},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := resolveStoreBackend(tc.flag); got != tc.want {
				t.Fatalf("resolveStoreBackend(%q) = %v, want %v", tc.flag, got, tc.want)
			}
		})
	}
}

// TestResolveAppsZonesBackend verifies the OR-combination rule: apps+zones use
// Postgres when EITHER APPS_ZONES_BACKEND=postgres OR STORE_BACKEND=postgres,
// so a full-cutover STORE_BACKEND flag also moves apps+zones without requiring
// both flags.
func TestResolveAppsZonesBackend(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		appsZonesFlag string
		storeFlag     string
		want          storeBackend
	}{
		{"both unset -> cosmos", "", "", backendCosmos},
		{"APPS_ZONES_BACKEND=postgres -> postgres", "postgres", "", backendPostgres},
		{"STORE_BACKEND=postgres -> postgres", "", "postgres", backendPostgres},
		{"both postgres -> postgres", "postgres", "postgres", backendPostgres},
		{"APPS_ZONES_BACKEND=cosmos STORE_BACKEND=postgres -> postgres", "cosmos", "postgres", backendPostgres},
		{"APPS_ZONES_BACKEND=postgres STORE_BACKEND=cosmos -> postgres", "postgres", "cosmos", backendPostgres},
		{"junk values -> cosmos", "nonsense", "banana", backendCosmos},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := resolveAppsZonesBackend(tc.appsZonesFlag, tc.storeFlag); got != tc.want {
				t.Fatalf("resolveAppsZonesBackend(%q, %q) = %v, want %v",
					tc.appsZonesFlag, tc.storeFlag, got, tc.want)
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

// TestChoosePGStoreChooserNilGuard covers the typed-nil trap for all new
// per-store choosers that take (pg, cosmos) without a backend enum: when the
// pg store is nil the chooser must return the cosmos store (or a genuine nil
// interface if cosmos is also nil), and when pg is non-nil it must be returned
// regardless.
//
// Each sub-test uses the narrowest check that proves the typed-nil property:
// asserting the returned interface is nil (not just zero-valued) or that the
// concrete type matches what was passed in.
func TestChoosePGStoreChooserNilGuard(t *testing.T) {
	t.Parallel()

	t.Run("chooseProfileStore selects pg when non-nil", func(t *testing.T) {
		t.Parallel()
		pg := profiles.NewPostgresStore(nil)
		got := chooseProfileStore(pg, nil)
		if got == nil {
			t.Fatal("got nil, want the postgres store")
		}
		if _, ok := got.(*profiles.PostgresStore); !ok {
			t.Fatalf("got %T, want *profiles.PostgresStore", got)
		}
	})
	t.Run("chooseProfileStore falls back to cosmos when pg nil", func(t *testing.T) {
		t.Parallel()
		cosmos := profiles.NewCosmosStore(nil)
		got := chooseProfileStore(nil, cosmos)
		if _, ok := got.(*profiles.CosmosStore); !ok {
			t.Fatalf("got %T, want *profiles.CosmosStore", got)
		}
	})
	t.Run("chooseProfileStore returns genuine nil when both nil", func(t *testing.T) {
		t.Parallel()
		if got := chooseProfileStore(nil, nil); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})

	t.Run("chooseAdminProfileStore selects pg when non-nil", func(t *testing.T) {
		t.Parallel()
		pg := profiles.NewPostgresAdminStore(nil)
		got := chooseAdminProfileStore(pg, nil)
		if got == nil {
			t.Fatal("got nil, want the postgres admin store")
		}
		if _, ok := got.(*profiles.PostgresAdminStore); !ok {
			t.Fatalf("got %T, want *profiles.PostgresAdminStore", got)
		}
	})
	t.Run("chooseAdminProfileStore falls back to cosmos when pg nil", func(t *testing.T) {
		t.Parallel()
		cosmos := profiles.NewAdminStore(nil)
		got := chooseAdminProfileStore(nil, cosmos)
		if _, ok := got.(*profiles.AdminStore); !ok {
			t.Fatalf("got %T, want *profiles.AdminStore", got)
		}
	})
	t.Run("chooseAdminProfileStore returns genuine nil when both nil", func(t *testing.T) {
		t.Parallel()
		if got := chooseAdminProfileStore(nil, nil); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})

	t.Run("chooseDeviceStore selects pg when non-nil", func(t *testing.T) {
		t.Parallel()
		pg := devicetokens.NewPostgresStore(nil)
		got := chooseDeviceStore(pg, nil)
		if _, ok := got.(*devicetokens.PostgresStore); !ok {
			t.Fatalf("got %T, want *devicetokens.PostgresStore", got)
		}
	})
	t.Run("chooseDeviceStore falls back to cosmos when pg nil", func(t *testing.T) {
		t.Parallel()
		cosmos := devicetokens.NewCosmosStore(nil)
		got := chooseDeviceStore(nil, cosmos)
		if _, ok := got.(*devicetokens.CosmosStore); !ok {
			t.Fatalf("got %T, want *devicetokens.CosmosStore", got)
		}
	})
	t.Run("chooseDeviceStore returns genuine nil when both nil", func(t *testing.T) {
		t.Parallel()
		if got := chooseDeviceStore(nil, nil); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})

	t.Run("chooseNotifStateStore selects pg when non-nil", func(t *testing.T) {
		t.Parallel()
		pg := notificationstate.NewPostgresStore(nil)
		got := chooseNotifStateStore(pg, nil)
		if _, ok := got.(*notificationstate.PostgresStore); !ok {
			t.Fatalf("got %T, want *notificationstate.PostgresStore", got)
		}
	})
	t.Run("chooseNotifStateStore falls back to cosmos when pg nil", func(t *testing.T) {
		t.Parallel()
		cosmos := notificationstate.NewCosmosStore(nil, nil)
		got := chooseNotifStateStore(nil, cosmos)
		if _, ok := got.(*notificationstate.CosmosStore); !ok {
			t.Fatalf("got %T, want *notificationstate.CosmosStore", got)
		}
	})
	t.Run("chooseNotifStateStore returns genuine nil when both nil", func(t *testing.T) {
		t.Parallel()
		if got := chooseNotifStateStore(nil, nil); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})

	t.Run("chooseSavedStore selects pg when non-nil", func(t *testing.T) {
		t.Parallel()
		pg := savedapplications.NewPostgresStore(nil)
		got := chooseSavedStore(pg, nil)
		if _, ok := got.(*savedapplications.PostgresStore); !ok {
			t.Fatalf("got %T, want *savedapplications.PostgresStore", got)
		}
	})
	t.Run("chooseSavedStore falls back to cosmos when pg nil", func(t *testing.T) {
		t.Parallel()
		cosmos := savedapplications.NewCosmosStore(nil)
		got := chooseSavedStore(nil, cosmos)
		if _, ok := got.(*savedapplications.CosmosStore); !ok {
			t.Fatalf("got %T, want *savedapplications.CosmosStore", got)
		}
	})
	t.Run("chooseSavedStore returns genuine nil when both nil", func(t *testing.T) {
		t.Parallel()
		if got := chooseSavedStore(nil, nil); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})

	t.Run("chooseOfferStore selects pg when non-nil", func(t *testing.T) {
		t.Parallel()
		pg := offercodes.NewPostgresStore(nil)
		got := chooseOfferStore(pg, nil)
		if _, ok := got.(*offercodes.PostgresStore); !ok {
			t.Fatalf("got %T, want *offercodes.PostgresStore", got)
		}
	})
	t.Run("chooseOfferStore falls back to cosmos when pg nil", func(t *testing.T) {
		t.Parallel()
		cosmos := offercodes.NewCosmosStore(nil)
		got := chooseOfferStore(nil, cosmos)
		if _, ok := got.(*offercodes.CosmosStore); !ok {
			t.Fatalf("got %T, want *offercodes.CosmosStore", got)
		}
	})
	t.Run("chooseOfferStore returns genuine nil when both nil", func(t *testing.T) {
		t.Parallel()
		if got := chooseOfferStore(nil, nil); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})

	t.Run("chooseNotifDigestStore selects pg when non-nil", func(t *testing.T) {
		t.Parallel()
		pg := notifications.NewPostgresStore(nil)
		got := chooseNotifDigestStore(pg, nil)
		if _, ok := got.(*notifications.PostgresStore); !ok {
			t.Fatalf("got %T, want *notifications.PostgresStore", got)
		}
	})
	t.Run("chooseNotifDigestStore falls back to cosmos when pg nil", func(t *testing.T) {
		t.Parallel()
		cosmos := notifications.NewDigestStore(nil)
		got := chooseNotifDigestStore(nil, cosmos)
		if _, ok := got.(*notifications.DigestStore); !ok {
			t.Fatalf("got %T, want *notifications.DigestStore", got)
		}
	})
	t.Run("chooseNotifDigestStore returns genuine nil when both nil", func(t *testing.T) {
		t.Parallel()
		if got := chooseNotifDigestStore(nil, nil); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})
}
