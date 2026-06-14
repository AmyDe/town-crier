// Command api serves the Town Crier HTTP API — the Go port of the .NET API
// (GH#418). It must stay contract-identical to the .NET implementation until
// cutover.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/designations"
	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/geocoding"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/notificationstate"
	"github.com/AmyDe/town-crier/api-go/internal/offercodes"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/savedapplications"
	"github.com/AmyDe/town-crier/api-go/internal/subscriptions"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
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

	validator, err := auth.NewAuth0Validator(cfg.Auth0Domain, cfg.Auth0Audience, logger)
	if err != nil {
		log.Fatal(err)
	}

	cosmos, err := platform.NewCosmosContainer(cfg, logger)
	if err != nil {
		log.Fatal(err)
	}
	var store *profiles.CosmosStore
	if cosmos != nil {
		store = profiles.NewCosmosStore(cosmos)
	}

	devices, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosDeviceRegistrationsContainer, logger)
	if err != nil {
		log.Fatal(err)
	}
	var deviceStore *devicetokens.CosmosStore
	if devices != nil {
		deviceStore = devicetokens.NewCosmosStore(devices)
	}

	stateContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosNotificationStateContainer, logger)
	if err != nil {
		log.Fatal(err)
	}
	notificationsContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosNotificationsContainer, logger)
	if err != nil {
		log.Fatal(err)
	}
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
	var watchZoneStore *watchzones.CosmosStore
	if watchZones != nil {
		watchZoneStore = watchzones.NewCosmosStore(watchZones)
	}

	appsContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosApplicationsContainer, logger)
	if err != nil {
		log.Fatal(err)
	}
	var appStore *applications.CosmosStore
	if appsContainer != nil {
		appStore = applications.NewCosmosStore(appsContainer)
	}

	savedContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosSavedApplicationsContainer, logger)
	if err != nil {
		log.Fatal(err)
	}
	var savedStore *savedapplications.CosmosStore
	if savedContainer != nil {
		savedStore = savedapplications.NewCosmosStore(savedContainer)
	}

	offerCodesContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosOfferCodesContainer, logger)
	if err != nil {
		log.Fatal(err)
	}
	var offerStore *offercodes.CosmosStore
	if offerCodesContainer != nil {
		offerStore = offercodes.NewCosmosStore(offerCodesContainer)
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

	// Real M2M client only when fully configured; otherwise the no-op fallback,
	// matching .NET's conditional IAuth0ManagementClient registration.
	var manager profiles.Auth0Manager = profiles.NoOpAuth0Client{}
	if cfg.Auth0M2MConfigured() {
		manager = profiles.NewAuth0Client(
			&http.Client{Timeout: 30 * time.Second},
			"https://"+cfg.Auth0Domain,
			cfg.Auth0M2MClientID,
			cfg.Auth0M2MClientSecret,
		)
	}

	// Geocode and designation clients call live UK services (the config defaults);
	// each gets its own timeout-bounded HTTP client.
	geocodeClient := geocoding.NewClient(cfg.PostcodesIoBaseURL, &http.Client{Timeout: 30 * time.Second})
	designationClient := designations.NewClient(cfg.GovUkBaseURL, &http.Client{Timeout: 30 * time.Second})

	srv := platform.NewServer(":"+cfg.Port, newRouter(validator, cfg.CorsAllowedOrigins, store, manager, cfg.ProDomains, deviceStore, stateStore, notifStore, watchZoneStore, appStore, savedStore, geocodeClient, designationClient, offerStore, adminStore, cfg.AdminAPIKey, jwsVerifier, appleNotifStore, cfg.AppleBundleID, logger))

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
