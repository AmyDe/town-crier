package main

import (
	"strings"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// storeBackend selects which datastore backs the Applications and WatchZones
// stores the worker reads and writes — the poll Upsert + notify
// FindZonesContaining, the digest watch-zone read, and the dormant erasure
// cascade. The other stores are always Cosmos; only these two move to Postgres +
// PostGIS in the hybrid migration phase (memo 0010, epic #645, issue #664 Phase
// B). This duplicates cmd/api/backend.go on purpose: the two package-main binaries
// each own their wiring (as buildPushSender / buildEmailSender already do), so the
// worker mirrors the API rather than sharing a package.
type storeBackend int

const (
	// backendCosmos is the default for every value of APPS_ZONES_BACKEND other
	// than the exact string "postgres" — including unset and "cosmos" — so prod
	// (flag unset) is never silently flipped off Cosmos.
	backendCosmos storeBackend = iota
	// backendPostgres serves Applications + WatchZones from Postgres + PostGIS.
	// It is set on the dev container only.
	backendPostgres
)

// appsZonesBackendPostgres is the only APPS_ZONES_BACKEND value that selects
// Postgres. The flag is dedicated and explicit — never inferred from
// POSTGRES_AUTH — so a future prod POSTGRES_AUTH can never flip prod's stores.
const appsZonesBackendPostgres = "postgres"

// resolveBackend maps the APPS_ZONES_BACKEND flag to a storeBackend. Only the
// exact value "postgres" (whitespace-trimmed) selects Postgres; every other
// value, including "" and "cosmos", keeps Cosmos.
func resolveBackend(flag string) storeBackend {
	if strings.TrimSpace(flag) == appsZonesBackendPostgres {
		return backendPostgres
	}
	return backendCosmos
}

// chooseAppStore returns the Applications store for the selected backend as a
// genuine consumer-side interface. When the chosen backend has no backing store
// configured it returns a true nil interface — never a typed-nil pointer boxed in
// a non-nil interface — so the worker's nil-guard discipline (a missing store
// leaves the mode unwired) holds.
func chooseAppStore(backend storeBackend, pg *applications.PostgresStore, cosmos *applications.CosmosStore) applications.Store {
	if backend == backendPostgres {
		if pg == nil {
			return nil
		}
		return pg
	}
	if cosmos == nil {
		return nil
	}
	return cosmos
}

// chooseZoneStore mirrors chooseAppStore for the WatchZones store.
func chooseZoneStore(backend storeBackend, pg *watchzones.PostgresStore, cosmos *watchzones.CosmosStore) watchzones.Store {
	if backend == backendPostgres {
		if pg == nil {
			return nil
		}
		return pg
	}
	if cosmos == nil {
		return nil
	}
	return cosmos
}
