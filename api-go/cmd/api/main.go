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
	// so the instruments record nothing locally). It is passed to newRouter for the
	// watch-zone lifecycle counters (tc-21np).
	registry := metrics.New(otel.Meter(metrics.MeterName))

	// Strictly non-fatal passwordless-connectivity probe (tc-vnna / #653). When the
	// API is configured for Azure managed-identity Postgres auth, prove from the
	// logs that we can connect with an Entra token and no stored password. It runs
	// in its own short-lived, self-terminating goroutine so it never blocks boot,
	// wires no route or store, and only logs (Info on success, Warn on any failure).
	if os.Getenv("POSTGRES_AUTH") == "azure-managed-identity" {
		go probePostgresManagedIdentity(logger)
	}

	validator, err := auth.NewAuth0Validator(cfg.Auth0Domain, cfg.Auth0Audience, logger)
	if err != nil {
		log.Fatal(err)
	}

	// Build the shared Postgres pool unconditionally — Postgres + PostGIS is the
	// only datastore. The pool is fatal if it cannot be built: the API cannot serve
	// any store-backed route without it.
	pool, perr := postgres.NewPoolFromEnv(context.Background())
	if perr != nil {
		log.Fatalf("postgres: build pool: %v", perr)
	}
	defer pool.Close()

	store := profiles.NewPostgresStore(pool)
	adminStore := profiles.NewPostgresAdminStore(pool)
	deviceStore := devicetokens.NewPostgresStore(pool)
	stateStore := notificationstate.NewPostgresStore(pool)
	notifStore := notifications.NewPostgresStore(pool)
	watchZoneStore := watchzones.NewPostgresStore(pool)
	appStore := applications.NewPostgresStore(pool)
	savedStore := savedapplications.NewPostgresStore(pool)
	offerStore := offercodes.NewPostgresStore(pool)
	appleNotifStore := subscriptions.NewPostgresNotificationStore(pool, time.Now)

	// cascade bundles the per-store deleters DELETE /v1/me runs for a complete GDPR
	// erasure (bead tc-qkf2). The notification store satisfies erasure.ChildDeleter
	// (DeleteAllByUserID) directly; the other stores satisfy erasure.ChildDeleter or
	// erasure.RedemptionAnonymiser on their exported Store interface. The watch-zone
	// deleter and the export reader bind to the same watch-zone store the routes use
	// (bead tc-s8g1).
	cascadeWatchZoneDeleter, exportWatchZoneReader := gdprWatchZoneWiring(watchZoneStore)
	cascade := profiles.CascadeDeleters{
		Notifications:       notifStore,
		WatchZones:          cascadeWatchZoneDeleter,
		SavedApplications:   savedStore,
		DeviceRegistrations: deviceStore,
		NotificationState:   erasure.NotificationStateChild(stateStore),
		OfferCodes:          offerStore,
	}

	// exportReaders bundles the per-collection readers GET /v1/me/data uses to
	// source the GDPR export's child collections (bead tc-lllv). The adapters
	// (export_adapters.go) map store records to profiles-local export row types,
	// keeping store -> row mapping out of profiles.
	exportReaders := profiles.ExportReaders{
		WatchZones:           exportWatchZoneReader,
		Notifications:        notificationExportReader{store: notifStore},
		SavedApplications:    savedApplicationExportReader{store: savedStore},
		DeviceRegistrations:  deviceRegistrationExportReader{store: deviceStore},
		OfferCodeRedemptions: offerCodeExportReader{store: offerStore},
	}

	// The JWS verifier embeds the Apple Root CA - G3 and needs no datastore, so it
	// is always available.
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

	srv := platform.NewServer(":"+cfg.Port, newRouter(validator, cfg.CorsAllowedOrigins, store, manager, cascade, exportReaders, deviceStore, stateStore, notifStore, watchZoneStore, appStore, savedStore, geocodeClient, designationClient, offerStore, adminStore, cfg.AdminAPIKey, cfg.SiteBuildKey, jwsVerifier, appleNotifStore, cfg.AppleBundleID, cfg.AppleEnvironments, registry, logger))

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
// make the "API connects passwordless" milestone (#653) provable from logs and is
// strictly non-fatal: every failure is a Warn and never affects the running API.
// The goroutine self-terminates within its own bounded timeout, so it owns its
// lifetime and leaks nothing.
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
