package main

import (
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// TestGDPRWatchZoneWiring_BindsBothPaths proves the GDPR erasure cascade
// (DELETE /v1/me) and the data export (GET /v1/me/data) are both bound to the
// SAME watch-zone store the routes use, so an account deletion or export covers a
// user's watch zones rather than silently missing them (bead tc-s8g1). A genuine
// nil store yields genuine nil wiring so the cascade/export guards leave the GDPR
// paths unbuilt rather than holding a nil deleter the cascade would dereference.
func TestGDPRWatchZoneWiring_BindsBothPaths(t *testing.T) {
	t.Parallel()

	pg := watchzones.NewPostgresStore(nil) // querier never touched by these assertions

	t.Run("a present store binds both GDPR paths to it", func(t *testing.T) {
		t.Parallel()
		deleter, reader := gdprWatchZoneWiring(pg)

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

	t.Run("a nil store yields genuine nil wiring so the cascade/export guards leave the GDPR paths unbuilt", func(t *testing.T) {
		t.Parallel()
		var nilStore watchzones.Store
		deleter, reader := gdprWatchZoneWiring(nilStore)

		if deleter != nil {
			t.Errorf("cascade deleter = %T, want a genuine nil so the cascade is not built with a nil deleter", deleter)
		}
		if reader != nil {
			t.Errorf("export reader = %T, want a genuine nil so the export is not built with a nil reader", reader)
		}
	})
}
