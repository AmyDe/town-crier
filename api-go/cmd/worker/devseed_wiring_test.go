package main

import (
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// TestBuildDevSeeder_UnconfiguredReturnsNil proves buildDevSeeder returns a
// genuinely nil worker.DevSeedRunner — not a typed-nil *devseed.Seeder, which
// would defeat runDevSeed's nil guard — whenever either half of the dev-seed
// job's dedicated prod-read config (DEV_SEED_PROD_AZURE_CLIENT_ID /
// DEV_SEED_PROD_POSTGRES_USER) is absent. This mirrors the "unconfigured
// optional job" posture buildPollOrchestrator/buildPushSender already use.
func TestBuildDevSeeder_UnconfiguredReturnsNil(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cfg  platform.Config
	}{
		{"both unset", platform.Config{}},
		{"missing azure client id", platform.Config{DevSeedProdPostgresUser: "towncrier_dev_seed_reader"}},
		{"missing postgres user", platform.Config{DevSeedProdAzureClientID: "11111111-2222-3333-4444-555555555555"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runner := buildDevSeeder(tc.cfg, &stores{}, discardLogger())
			if runner != nil {
				t.Fatalf("buildDevSeeder: got non-nil runner, want nil (job unconfigured)")
			}
		})
	}
}

// TestBuildDevSeeder_ConfiguredReturnsNonNilSeeder proves buildDevSeeder builds
// a real Seeder once both halves of the prod-read config are present. It needs
// no live Azure infra: azidentity.NewManagedIdentityCredential and
// postgres.NewTokenCredentialPool both construct lazily — the credential holds
// no token until GetToken is called, and the pgx pool opens no connection until
// a query is issued — the same "SDK clients constructed in main(), but actual
// connections open lazily" posture every other builder in this file relies on.
func TestBuildDevSeeder_ConfiguredReturnsNonNilSeeder(t *testing.T) {
	t.Parallel()
	cfg := platform.Config{
		DevSeedProdAzureClientID: "11111111-2222-3333-4444-555555555555",
		DevSeedProdPostgresUser:  "towncrier_dev_seed_reader",
		DevSeedProdPostgresDB:    "town_crier_prod",
		DevSeedLimit:             5,
		PostgresHost:             "psql-town-crier-shared.postgres.database.azure.com",
		PostgresSSLMode:          "require",
	}
	st := &stores{
		app:  applications.NewPostgresStore(nil),
		zone: watchzones.NewPostgresStore(nil),
	}

	runner := buildDevSeeder(cfg, st, discardLogger())

	if runner == nil {
		t.Fatal("buildDevSeeder: got nil runner, want a configured Seeder")
	}
}
