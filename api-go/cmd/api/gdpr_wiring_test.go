package main

import (
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// TestGDPRWatchZoneWiring_FollowsBackendFlag proves the GDPR erasure cascade
// (DELETE /v1/me) and the data export (GET /v1/me/data) are both bound to the
// SAME flag-selected watch-zone store the routes use — so when
// APPS_ZONES_BACKEND=postgres an account deletion or export covers a
// Postgres-resident user's watch zones, instead of silently missing them on the
// always-Cosmos store (bead tc-s8g1). With the flag unset both stay on Cosmos.
func TestGDPRWatchZoneWiring_FollowsBackendFlag(t *testing.T) {
	t.Parallel()

	cosmos := watchzones.NewCosmosStore(newFakeItems())
	pg := watchzones.NewPostgresStore(nil) // querier never touched by these assertions

	t.Run("postgres backend binds both GDPR paths to the Postgres store", func(t *testing.T) {
		t.Parallel()
		deleter, reader := gdprWatchZoneWiring(chooseZoneStore(backendPostgres, pg, cosmos))

		if _, ok := deleter.(*watchzones.PostgresStore); !ok {
			t.Errorf("cascade deleter = %T, want *watchzones.PostgresStore", deleter)
		}
		adapter, ok := reader.(watchZoneExportReader)
		if !ok {
			t.Fatalf("export reader = %T, want watchZoneExportReader", reader)
		}
		if _, ok := adapter.store.(*watchzones.PostgresStore); !ok {
			t.Errorf("export reader store = %T, want *watchzones.PostgresStore", adapter.store)
		}
	})

	t.Run("cosmos backend binds both GDPR paths to the Cosmos store", func(t *testing.T) {
		t.Parallel()
		deleter, reader := gdprWatchZoneWiring(chooseZoneStore(backendCosmos, pg, cosmos))

		if _, ok := deleter.(*watchzones.CosmosStore); !ok {
			t.Errorf("cascade deleter = %T, want *watchzones.CosmosStore", deleter)
		}
		adapter, ok := reader.(watchZoneExportReader)
		if !ok {
			t.Fatalf("export reader = %T, want watchZoneExportReader", reader)
		}
		if _, ok := adapter.store.(*watchzones.CosmosStore); !ok {
			t.Errorf("export reader store = %T, want *watchzones.CosmosStore", adapter.store)
		}
	})

	t.Run("nil store yields genuine nil wiring so the cascade/export guards leave the GDPR paths unbuilt", func(t *testing.T) {
		t.Parallel()
		deleter, reader := gdprWatchZoneWiring(chooseZoneStore(backendPostgres, nil, cosmos))

		if deleter != nil {
			t.Errorf("cascade deleter = %T, want a genuine nil so the cascade is not built with a nil deleter", deleter)
		}
		if reader != nil {
			t.Errorf("export reader = %T, want a genuine nil so the export is not built with a nil reader", reader)
		}
	})
}
