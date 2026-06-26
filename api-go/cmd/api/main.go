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

	// APPS_ZONES_BACKEND gates the Applications + WatchZones route stores onto
	// Postgres (dev only; prod and every other store stay Cosmos — issue #657
	// Slice 2). When it selects Postgres we build ONE pool — MI-token auth from
	// the dev container's POSTGRES_*/AZURE_CLIENT_ID via NewPoolFromEnv — and back
	// both Postgres stores with it. Dev explicitly wants Postgres, so a pool that
	// can't be built is fatal rather than a silent Cosmos fallback. The pool closes
	// on shutdown. When the flag is unset (prod, local) these stay nil and the
	// existing Cosmos construction is byte-for-byte unchanged.
	appsZonesBackend := resolveBackend(cfg.AppsZonesBackend)
	var (
		pgAppStore       *applications.PostgresStore
		pgWatchZoneStore *watchzones.PostgresStore
	)
	if appsZonesBackend == backendPostgres {
		pool, perr := postgres.NewPoolFromEnv(context.Background())
		if perr != nil {
			log.Fatalf("apps/zones backend=postgres: build postgres pool: %v", perr)
		}
		defer pool.Close()
		pgAppStore = applications.NewPostgresStore(pool)
		pgWatchZoneStore = watchzones.NewPostgresStore(pool)
		logger.Info("apps/zones backend selected", "backend", "postgres")
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
	var store *profiles.CosmosStore
	if cosmos != nil {
		store = profiles.NewCosmosStore(cosmos)
	}

	devices, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosDeviceRegistrationsContainer, logger)
	if err != nil {
		log.Fatal(err)
	}
	devices.WithMetrics(registry)
	var deviceStore *devicetokens.CosmosStore
	if devices != nil {
		deviceStore = devicetokens.NewCosmosStore(devices)
	}

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
	var stateStore *notificationstate.CosmosStore
	if stateContainer != nil && notificationsContainer != nil {
		stateStore = notificationstate.NewCosmosStore(stateContainer, notificationsContainer)
	}
	// The Notifications container also backs the per-application latest-unread
	// lookup used by GET /v1/me/watch-zones/{zoneId}/applications.
	var notifStore *notifications.CosmosStore
	if notificationsContainer != nil {
		notifStore = notifications.NewCosmosStore(notificationsContainer)
	}

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
	watchZoneStore := chooseZoneStore(appsZonesBackend, pgWatchZoneStore, cosmosWatchZoneStore)

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
	appStore := chooseAppStore(appsZonesBackend, pgAppStore, cosmosAppStore)

	savedContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosSavedApplicationsContainer, logger)
	if err != nil {
		log.Fatal(err)
	}
	savedContainer.WithMetrics(registry)
	var savedStore *savedapplications.CosmosStore
	if savedContainer != nil {
		savedStore = savedapplications.NewCosmosStore(savedContainer)
	}

	offerCodesContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosOfferCodesContainer, logger)
	if err != nil {
		log.Fatal(err)
	}
	offerCodesContainer.WithMetrics(registry)
	var offerStore *offercodes.CosmosStore
	if offerCodesContainer != nil {
		offerStore = offercodes.NewCosmosStore(offerCodesContainer)
	}

	// cascade bundles the per-container deleters DELETE /v1/me runs for a complete
	// GDPR erasure (bead tc-qkf2). It is populated only when Cosmos is configured —
	// the same condition under which profiles.Routes is wired — so the handler's
	// deleters are never nil when the route is reachable. The four DeleteAllByUserID
	// stores satisfy erasure.ChildDeleter directly; notification-state is bridged by
	// erasure.NotificationStateChild (its store method is DeleteByUserID), shared with
	// the dormant-cleanup worker (bead tc-gf0g). The offer-code anonymiser scrubs the
	// redeemer back-reference (redeemedByUserId + redeemedAt) without deleting the
	// admin-issued code (bead tc-5jyh).
	var cascade profiles.CascadeDeleters
	if notificationsContainer != nil && watchZones != nil && savedContainer != nil && devices != nil && stateContainer != nil && offerStore != nil {
		cascade = profiles.CascadeDeleters{
			Notifications:       notifications.NewDeleteStore(notificationsContainer),
			WatchZones:          cosmosWatchZoneStore,
			SavedApplications:   savedStore,
			DeviceRegistrations: deviceStore,
			NotificationState:   erasure.NotificationStateChild(stateStore),
			OfferCodes:          offerStore,
		}
	}

	// exportReaders bundles the per-collection readers GET /v1/me/data uses to
	// source the GDPR export's child collections (bead tc-lllv). Like cascade it is
	// populated only when Cosmos is configured — the same condition under which
	// profiles.Routes is wired — so the export's readers are never nil when the
	// route is reachable; on a Cosmos-less local boot the readers stay nil and the
	// export renders every collection as [] (never null). The adapters
	// (export_adapters.go) map each store's records to the profiles-local export
	// row types, keeping the store -> row mapping out of profiles (which must not
	// import the feature packages — offercodes already imports profiles). The
	// notifications reader uses the digest store's full-document AllByUser read so
	// every exported field is carried, not the latest-unread projection.
	var exportReaders profiles.ExportReaders
	if notificationsContainer != nil && watchZones != nil && savedContainer != nil && devices != nil && offerStore != nil {
		exportReaders = profiles.ExportReaders{
			WatchZones:           watchZoneExportReader{store: cosmosWatchZoneStore},
			Notifications:        notificationExportReader{store: notifications.NewDigestStore(notificationsContainer)},
			SavedApplications:    savedApplicationExportReader{store: savedStore},
			DeviceRegistrations:  deviceRegistrationExportReader{store: deviceStore},
			OfferCodeRedemptions: offerCodeExportReader{store: offerStore},
		}
	}

	// The admin grant/list operations query the Users container cross-partition
	// (by email, and the full list), so the admin store reuses the same container
	// as the profile store.
	var adminStore *profiles.AdminStore
	if cosmos != nil {
		adminStore = profiles.NewAdminStore(cosmos)
	}

	appleNotificationsContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosAppleNotificationsContainer, logger)
	if err != nil {
		log.Fatal(err)
	}
	appleNotificationsContainer.WithMetrics(registry)
	var appleNotifStore *subscriptions.CosmosNotificationStore
	if appleNotificationsContainer != nil {
		appleNotifStore = subscriptions.NewCosmosNotificationStore(appleNotificationsContainer, time.Now)
	}

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
