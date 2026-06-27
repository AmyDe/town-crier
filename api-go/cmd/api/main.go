// Command api serves the Town Crier HTTP API.
package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/designations"
	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/erasure"
	"github.com/AmyDe/town-crier/api-go/internal/geocoding"
	"github.com/AmyDe/town-crier/api-go/internal/metrics"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/notificationstate"
	"github.com/AmyDe/town-crier/api-go/internal/offercodes"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/savedapplications"
	"github.com/AmyDe/town-crier/api-go/internal/subscriptions"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
	"go.opentelemetry.io/otel"
)

func main() {
	cfg, err := platform.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	// NewOTelLogger fans every record out to stdout JSON (ContainerAppConsoleLogs)
	// AND the otelslog bridge. The bridge targets the GLOBAL LoggerProvider, which
	// is the no-op provider until SetupTelemetry (called next) installs an SDK one;
	// the global provider's delegation upgrades this logger in place, so building
	// it before SetupTelemetry is correct (tc-8x8g / ADR 0027).
	logger := platform.NewOTelLogger(os.Stdout, cfg.LogLevel)

	// SetupTelemetry self-disables when OTEL_EXPORTER_OTLP_ENDPOINT is unset, so
	// local/dev boots leave the no-op Tracer and Logger providers in place. When
	// the ACA OTel agent injects the endpoint, traces and logs export to it over
	// OTLP/gRPC.
	shutdownTelemetry, err := platform.SetupTelemetry(context.Background(), logger)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			logger.Error("telemetry shutdown", "error", err)
		}
	}()

	// The business-metric registry is built from the global MeterProvider that
	// SetupTelemetry just installed (a no-op provider when telemetry is disabled,
	// so the instruments record nothing locally). Every Cosmos container is wired
	// to it below so towncrier.cosmos.request_charge_ru flows, and it is passed to
	// newRouter for the watch-zone lifecycle counters (tc-21np).
	registry := metrics.New(otel.Meter(metrics.MeterName))

	// Strictly non-fatal passwordless-connectivity probe (tc-vnna / #653). When
	// the API is configured for Azure managed-identity Postgres auth, prove from
	// the logs that we can connect with an Entra token and no stored password —
	// the migration milestone (epic #645). It runs in its own short-lived,
	// self-terminating goroutine so it never blocks boot, wires no route or store,
	// and only logs (Info on success, Warn on any failure). Cosmos stays live.
	if os.Getenv("POSTGRES_AUTH") == "azure-managed-identity" {
		go probePostgresManagedIdentity(logger)
	}

	// APPS_ZONES_BACKEND selects Postgres for the Applications + WatchZones stores.
	// STORE_BACKEND selects Postgres for ALL stores simultaneously (full cutover,
	// issue #669 Slice 7a). Only the exact string "postgres" (trimmed) activates
	// either flag; unset / "cosmos" / anything else keeps Cosmos.
	// STORE_BACKEND=postgres implies apps+zones too, so a single pool is built
	// whenever EITHER flag selects Postgres. The pool is fatal if it cannot be
	// built — explicit Postgres selection is intentional and a misconfigured pool
	// must never silently fall back to Cosmos.
	appsZonesBackend := resolveBackend(cfg.AppsZonesBackend)
	fullBackend := resolveStoreBackend(cfg.StoreBackend)
	var (
		pgAppStore        *applications.PostgresStore
		pgWatchZoneStore  *watchzones.PostgresStore
		pgProfileStore    *profiles.PostgresStore
		pgAdminStore      *profiles.PostgresAdminStore
		pgDeviceStore     *devicetokens.PostgresStore
		pgStateStore      *notificationstate.PostgresStore
		pgNotifStore      *notifications.PostgresStore
		pgSavedStore      *savedapplications.PostgresStore
		pgOfferStore      *offercodes.PostgresStore
		pgAppleNotifStore *subscriptions.PostgresNotificationStore
	)
	if appsZonesBackend == backendPostgres || fullBackend == backendPostgres {
		pool, perr := postgres.NewPoolFromEnv(context.Background())
		if perr != nil {
			log.Fatalf("postgres backend: build pool: %v", perr)
		}
		defer pool.Close()
		pgAppStore = applications.NewPostgresStore(pool)
		pgWatchZoneStore = watchzones.NewPostgresStore(pool)
		if fullBackend == backendPostgres {
			pgProfileStore = profiles.NewPostgresStore(pool)
			pgAdminStore = profiles.NewPostgresAdminStore(pool)
			pgDeviceStore = devicetokens.NewPostgresStore(pool)
			pgStateStore = notificationstate.NewPostgresStore(pool)
			pgNotifStore = notifications.NewPostgresStore(pool)
			pgSavedStore = savedapplications.NewPostgresStore(pool)
			pgOfferStore = offercodes.NewPostgresStore(pool)
			pgAppleNotifStore = subscriptions.NewPostgresNotificationStore(pool, time.Now)
			logger.Info("all stores backend selected", "backend", "postgres")
		} else {
			logger.Info("apps/zones backend selected", "backend", "postgres")
		}
	}

	validator, err := auth.NewAuth0Validator(cfg.Auth0Domain, cfg.Auth0Audience, logger)
	if err != nil {
		log.Fatal(err)
	}

	cosmos, err := platform.NewCosmosContainer(cfg, logger)
	if err != nil {
		log.Fatal(err)
	}
	cosmos.WithMetrics(registry)
	var cosmosProfileStore *profiles.CosmosStore
	if cosmos != nil {
		cosmosProfileStore = profiles.NewCosmosStore(cosmos)
	}
	// store is the flag-selected profile store: Postgres when STORE_BACKEND=postgres,
	// Cosmos otherwise. chooseProfileStore returns a genuine nil interface when the
	// chosen backend has no store configured, so newRouter's nil-means-unwired guards
	// never trip on a typed nil.
	store := chooseProfileStore(fullBackend, pgProfileStore, cosmosProfileStore)

	devices, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosDeviceRegistrationsContainer, logger)
	if err != nil {
		log.Fatal(err)
	}
	devices.WithMetrics(registry)
	var cosmosDeviceStore *devicetokens.CosmosStore
	if devices != nil {
		cosmosDeviceStore = devicetokens.NewCosmosStore(devices)
	}
	deviceStore := chooseDeviceStore(fullBackend, pgDeviceStore, cosmosDeviceStore)

	stateContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosNotificationStateContainer, logger)
	if err != nil {
		log.Fatal(err)
	}
	stateContainer.WithMetrics(registry)
	notificationsContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosNotificationsContainer, logger)
	if err != nil {
		log.Fatal(err)
	}
	notificationsContainer.WithMetrics(registry)
	var cosmosStateStore *notificationstate.CosmosStore
	if stateContainer != nil && notificationsContainer != nil {
		cosmosStateStore = notificationstate.NewCosmosStore(stateContainer, notificationsContainer)
	}
	stateStore := chooseStateStore(fullBackend, pgStateStore, cosmosStateStore)
	// cosmosNotifStore is the Cosmos latest-unread path. notifStore is the
	// flag-selected notifUnreadReader for NearbyRoutes: in Postgres mode
	// pgNotifStore satisfies the interface; in Cosmos mode cosmosNotifStore
	// does (it only implements GetLatestUnreadByApplications).
	var cosmosNotifStore *notifications.CosmosStore
	if notificationsContainer != nil {
		cosmosNotifStore = notifications.NewCosmosStore(notificationsContainer)
	}
	notifStore := chooseNotifStore(fullBackend, pgNotifStore, cosmosNotifStore)

	watchZones, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosWatchZonesContainer, logger)
	if err != nil {
		log.Fatal(err)
	}
	watchZones.WithMetrics(registry)
	// cosmosWatchZoneStore is always the concrete Cosmos store: it backs the GDPR
	// erasure cascade, the data export, and the admin location/bbox backfill — all
	// Cosmos-era paths that stay on Cosmos regardless of the route backend.
	// watchZoneStore is the flag-selected consumer-side interface the route wiring
	// uses (Postgres on dev, Cosmos elsewhere); chooseZoneStore returns a genuine
	// nil interface when neither backing is configured.
	var cosmosWatchZoneStore *watchzones.CosmosStore
	if watchZones != nil {
		cosmosWatchZoneStore = watchzones.NewCosmosStore(watchZones)
	}
	// The watch-zone and applications stores follow STORE_BACKEND when set
	// (which implies apps+zones on Postgres), otherwise APPS_ZONES_BACKEND.
	// Both choosers receive the same effective backend.
	zoneAndAppsBackend := appsZonesBackend
	if fullBackend == backendPostgres {
		zoneAndAppsBackend = backendPostgres
	}
	watchZoneStore := chooseZoneStore(zoneAndAppsBackend, pgWatchZoneStore, cosmosWatchZoneStore)

	appsContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosApplicationsContainer, logger)
	if err != nil {
		log.Fatal(err)
	}
	appsContainer.WithMetrics(registry)
	var cosmosAppStore *applications.CosmosStore
	if appsContainer != nil {
		cosmosAppStore = applications.NewCosmosStore(appsContainer)
	}
	// appStore is the flag-selected consumer-side interface the application routes
	// use (Postgres on dev, Cosmos elsewhere); a genuine nil interface leaves the
	// routes unwired on a store-less boot.
	appStore := chooseAppStore(zoneAndAppsBackend, pgAppStore, cosmosAppStore)

	savedContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosSavedApplicationsContainer, logger)
	if err != nil {
		log.Fatal(err)
	}
	savedContainer.WithMetrics(registry)
	var cosmosSavedStore *savedapplications.CosmosStore
	if savedContainer != nil {
		cosmosSavedStore = savedapplications.NewCosmosStore(savedContainer)
	}
	savedStore := chooseSavedStore(fullBackend, pgSavedStore, cosmosSavedStore)

	offerCodesContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosOfferCodesContainer, logger)
	if err != nil {
		log.Fatal(err)
	}
	offerCodesContainer.WithMetrics(registry)
	var cosmosOfferStore *offercodes.CosmosStore
	if offerCodesContainer != nil {
		cosmosOfferStore = offercodes.NewCosmosStore(offerCodesContainer)
	}
	offerStore := chooseOfferStore(fullBackend, pgOfferStore, cosmosOfferStore)

	// cascadeWatchZoneDeleter and exportWatchZoneReader bind both GDPR paths to
	// the flag-selected watch-zone store (watchZoneStore) — Postgres on dev/prod
	// cutover, Cosmos elsewhere — so account erasure and the data export cover a
	// Postgres-resident user's zones (bead tc-s8g1). The admin backfill migrator
	// still uses cosmosWatchZoneStore directly.
	cascadeWatchZoneDeleter, exportWatchZoneReader := gdprWatchZoneWiring(watchZoneStore)

	// cascade bundles the per-store deleters DELETE /v1/me runs for a complete
	// GDPR erasure (bead tc-qkf2). All six members are now flag-selected via the
	// STORE_BACKEND choosers above, so the cascade targets the same backend the
	// routes use. In Postgres mode pgNotifStore satisfies erasure.ChildDeleter
	// (DeleteAllByUserID); in Cosmos mode the dedicated DeleteStore wraps the
	// container. The other four stores satisfy erasure.ChildDeleter or
	// erasure.RedemptionAnonymiser directly on their exported Store interface.
	var cascadeNotifDeleter erasure.ChildDeleter
	if fullBackend == backendPostgres && pgNotifStore != nil {
		cascadeNotifDeleter = pgNotifStore
	} else if notificationsContainer != nil {
		cascadeNotifDeleter = notifications.NewDeleteStore(notificationsContainer)
	}

	var cascade profiles.CascadeDeleters
	if cascadeNotifDeleter != nil && watchZoneStore != nil && savedStore != nil && deviceStore != nil && stateStore != nil && offerStore != nil {
		cascade = profiles.CascadeDeleters{
			Notifications:       cascadeNotifDeleter,
			WatchZones:          cascadeWatchZoneDeleter,
			SavedApplications:   savedStore,
			DeviceRegistrations: deviceStore,
			NotificationState:   erasure.NotificationStateChild(stateStore),
			OfferCodes:          offerStore,
		}
	}

	// exportReaders bundles the per-collection readers GET /v1/me/data uses to
	// source the GDPR export's child collections (bead tc-lllv). All five members
	// are now flag-selected, so the export reads from the same backend the routes
	// use. In Postgres mode pgNotifStore satisfies allByUserReader (AllByUser); in
	// Cosmos mode the dedicated DigestStore provides the full-document AllByUser
	// read. The adapters (export_adapters.go) map store records to profiles-local
	// export row types, keeping store → row mapping out of profiles.
	var exportNotifReader allByUserReader
	if fullBackend == backendPostgres && pgNotifStore != nil {
		exportNotifReader = pgNotifStore
	} else if notificationsContainer != nil {
		exportNotifReader = notifications.NewDigestStore(notificationsContainer)
	}

	var exportReaders profiles.ExportReaders
	if exportNotifReader != nil && watchZoneStore != nil && savedStore != nil && deviceStore != nil && offerStore != nil {
		exportReaders = profiles.ExportReaders{
			WatchZones:           exportWatchZoneReader,
			Notifications:        notificationExportReader{store: exportNotifReader},
			SavedApplications:    savedApplicationExportReader{store: savedStore},
			DeviceRegistrations:  deviceRegistrationExportReader{store: deviceStore},
			OfferCodeRedemptions: offerCodeExportReader{store: offerStore},
		}
	}

	// The admin grant/list operations query the Users container cross-partition
	// (by email, and the full list), so the Cosmos admin store reuses the same
	// container as the profile store. adminStore is the flag-selected interface.
	var cosmosAdminStore *profiles.AdminStore
	if cosmos != nil {
		cosmosAdminStore = profiles.NewAdminStore(cosmos)
	}
	adminStore := chooseAdminStore(fullBackend, pgAdminStore, cosmosAdminStore)

	appleNotificationsContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosAppleNotificationsContainer, logger)
	if err != nil {
		log.Fatal(err)
	}
	appleNotificationsContainer.WithMetrics(registry)
	var cosmosAppleNotifStore *subscriptions.CosmosNotificationStore
	if appleNotificationsContainer != nil {
		cosmosAppleNotifStore = subscriptions.NewCosmosNotificationStore(appleNotificationsContainer, time.Now)
	}
	appleNotifStore := chooseAppleNotifStore(fullBackend, pgAppleNotifStore, cosmosAppleNotifStore)

	// The JWS verifier embeds the Apple Root CA - G3 and needs no Cosmos, so it is
	// always available; the subscription routes only wire when the backing stores
	// are present.
	appleRoots, err := subscriptions.LoadAppleRootCertificates()
	if err != nil {
		log.Fatal(err)
	}
	jwsVerifier, err := subscriptions.NewJWSVerifier(appleRoots, time.Now)
	if err != nil {
		log.Fatal(err)
	}

	// Real M2M client only when fully configured; otherwise the no-op fallback.
	var manager profiles.Auth0Manager = profiles.NoOpAuth0Client{}
	if cfg.Auth0M2MConfigured() {
		// Wrap the transport so Auth0 token/PATCH/DELETE calls emit OTel client
		// spans (Type=HTTP in AppDependencies) named "Auth0 token"; the host lands
		// in server.address.
		auth0HTTP := platform.WrapHTTPClient(
			&http.Client{Timeout: 30 * time.Second},
			func(string, *http.Request) string { return "Auth0 token" },
		)
		manager = profiles.NewAuth0Client(
			auth0HTTP,
			"https://"+cfg.Auth0Domain,
			cfg.Auth0M2MClientID,
			cfg.Auth0M2MClientSecret,
		)
	}

	// Geocode and designation clients call live UK services (the config defaults);
	// each gets its own timeout-bounded HTTP client. Construction can only fail
	// with an unparseable base URL, which is a startup misconfiguration.
	geocodeClient, err := geocoding.NewClient(cfg.PostcodesIoBaseURL, &http.Client{Timeout: 30 * time.Second})
	if err != nil {
		log.Fatalf("geocoding client: %v", err)
	}
	designationClient, err := designations.NewClient(cfg.GovUkBaseURL, &http.Client{Timeout: 30 * time.Second})
	if err != nil {
		log.Fatalf("designations client: %v", err)
	}

	srv := platform.NewServer(":"+cfg.Port, newRouter(validator, cfg.CorsAllowedOrigins, store, manager, cascade, exportReaders, deviceStore, stateStore, notifStore, watchZoneStore, cosmosWatchZoneStore, appStore, savedStore, geocodeClient, designationClient, offerStore, adminStore, cfg.AdminAPIKey, cfg.SiteBuildKey, jwsVerifier, appleNotifStore, cfg.AppleBundleID, cfg.AppleEnvironments, registry, logger))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("api listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("serve", "error", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "error", err)
	}
	logger.Info("api stopped")
}

// probePostgresManagedIdentity opens the managed-identity Postgres pool, runs a
// single SELECT current_user, and logs who we connected as. It exists solely to
// make the "dev API connects passwordless" milestone (#653) provable from logs
// and is strictly non-fatal: every failure is a Warn and never affects the
// running API. The goroutine self-terminates within its own bounded timeout, so
// it owns its lifetime and leaks nothing.
func probePostgresManagedIdentity(logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := postgres.NewPoolFromEnv(ctx)
	if err != nil {
		logger.Warn("postgres managed-identity probe: build pool", "error", err)
		return
	}
	defer pool.Close()

	var currentUser string
	if err := pool.QueryRow(ctx, "SELECT current_user").Scan(&currentUser); err != nil {
		logger.Warn("postgres managed-identity probe: query", "error", err)
		return
	}
	logger.Info("postgres managed-identity probe: connected passwordless", "currentUser", currentUser)
}
