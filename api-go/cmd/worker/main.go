// Command worker runs the Town Crier background-worker modes as short-lived
// Container Apps Jobs — the Go port of the .NET town-crier.worker (GH#418
// Phase 2, epic tc-wad3). One process per job: WORKER_MODE selects the mode,
// the process runs it once, flushes telemetry, and exits with a status code.
//
// This skeleton implements only poll-bootstrap (the Service-Bus-only tracer
// bullet); the other four modes are loud stubs that exit 1 until their own beads
// land. The Go image is not deployed to any job until the final cutover, so a
// stub can never strand a real job.
package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/acsemail"
	"github.com/AmyDe/town-crier/api-go/internal/apns"
	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/digest"
	"github.com/AmyDe/town-crier/api-go/internal/dormant"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/notificationstate"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/savedapplications"
	"github.com/AmyDe/town-crier/api-go/internal/servicebus"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
	"github.com/AmyDe/town-crier/api-go/internal/worker"
)

func main() {
	os.Exit(run())
}

// run is main's body split out so its deferred telemetry flush executes before
// the process exits — os.Exit in main would skip every defer. It returns the
// process exit code, propagated by main via os.Exit.
func run() int {
	cfg, err := platform.LoadConfig()
	if err != nil {
		log.Print(err)
		return 1
	}

	logger := platform.NewLogger(os.Stdout, cfg.LogLevel)

	mode := os.Getenv("WORKER_MODE")

	// SetupTelemetry self-disables when OTEL_EXPORTER_OTLP_ENDPOINT is unset, so
	// local/dev boots leave the no-op TracerProvider in place. OTEL_SERVICE_NAME
	// (set to town-crier-worker-go by infra) drives the service name on exported
	// spans. The deferred shutdown force-flushes the final batch before this
	// short-lived process exits — without it the worker can terminate before its
	// spans reach the collector (mirrors the .NET worker's ForceFlush).
	shutdownTelemetry, err := platform.SetupTelemetry(context.Background(), logger)
	if err != nil {
		log.Print(err)
		return 1
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			logger.Error("telemetry shutdown", "error", err)
		}
	}()

	// The Service Bus client (and thus the bootstrapper) is built only when the
	// job carries Service Bus config. Jobs that don't touch Service Bus (digest,
	// hourly-digest, dormant-cleanup) leave the bootstrapper nil; poll-bootstrap
	// then refuses to run rather than crashing.
	var bootstrapper *worker.Bootstrapper
	if cfg.ServiceBusNamespace != "" && cfg.ServiceBusQueueName != "" {
		sbClient, err := servicebus.NewClient(cfg.ServiceBusNamespace, cfg.ServiceBusQueueName, cfg.AzureClientID)
		if err != nil {
			logger.Error("build service bus client", "error", err)
			return 1
		}
		defer func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := sbClient.Close(closeCtx); err != nil {
				logger.Error("service bus client close", "error", err)
			}
		}()
		bootstrapper = worker.NewBootstrapper(sbClient, logger, time.Now)
	}

	// The digest handler (and thus the digest / hourly-digest modes) is built
	// only when the job carries Cosmos config. Jobs that don't touch Cosmos leave
	// the digester a genuinely nil interface; the digest modes then refuse to run
	// rather than crashing. The email and push senders fall back to NoOp when
	// their credentials are absent, so a Cosmos-only job still boots cleanly.
	//
	// digester is declared as the interface (not the concrete *digest.Handler) so
	// the "no Cosmos" case leaves it nil — assigning a typed-nil *digest.Handler
	// would make the interface non-nil and defeat worker.Run's nil guard.
	var digester worker.DigestRunner
	handler, err := buildDigester(cfg, logger)
	if err != nil {
		logger.Error("build digest handler", "error", err)
		return 1
	}
	if handler != nil {
		digester = handler
	}

	// The dormant-cleanup handler is built only when the job carries Cosmos config.
	// Like digester, it is declared as the interface so the "no Cosmos" case leaves
	// it a genuinely nil interface value (a typed-nil *dormant.Handler would defeat
	// worker.Run's nil guard). The Auth0 deleter falls back to a no-op when the M2M
	// credentials are absent, so a Cosmos-only job still boots cleanly.
	var dormantRunner worker.DormantRunner
	dormantHandler, err := buildDormant(cfg, logger)
	if err != nil {
		logger.Error("build dormant cleanup handler", "error", err)
		return 1
	}
	if dormantHandler != nil {
		dormantRunner = dormantHandler
	}

	return worker.Run(context.Background(), mode, bootstrapper, digester, dormantRunner, logger)
}

// buildDigester constructs the digest handler when Cosmos is configured, wiring
// the per-container stores and the email/push senders (real when their
// credentials are present, NoOp otherwise so a job without ACS/APNs boots
// cleanly). It returns (nil, nil) when Cosmos is unconfigured — the digest modes
// then refuse to run rather than nil-panicking. Returning the concrete
// *digest.Handler lets worker.Run accept it via its unexported digestRunner
// interface (structural satisfaction).
func buildDigester(cfg platform.Config, logger *slog.Logger) (*digest.Handler, error) {
	if cfg.CosmosEndpoint == "" {
		return nil, nil //nolint:nilnil // absent Cosmos config is a valid "no digester" state, not an error
	}

	users, err := platform.NewCosmosContainerNamed(cfg, "Users", logger)
	if err != nil {
		return nil, err
	}
	notifs, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosNotificationsContainer, logger)
	if err != nil {
		return nil, err
	}
	zonesContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosWatchZonesContainer, logger)
	if err != nil {
		return nil, err
	}
	stateContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosNotificationStateContainer, logger)
	if err != nil {
		return nil, err
	}
	devicesContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosDeviceRegistrationsContainer, logger)
	if err != nil {
		return nil, err
	}

	profileStore := digestProfiles{
		admin: profiles.NewAdminStore(users),
		point: profiles.NewCosmosStore(users),
	}
	notificationStore := notifications.NewDigestStore(notifs)
	zoneStore := watchzones.NewCosmosStore(zonesContainer)
	stateStore := notificationstate.NewCosmosStore(stateContainer, notifs)
	deviceStore := devicetokens.NewCosmosStore(devicesContainer)

	emailSender := buildEmailSender(cfg, logger)
	pushSender := buildPushSender(cfg, logger)

	return digest.NewHandler(
		profileStore,
		notificationStore,
		zoneStore,
		stateStore,
		deviceStore,
		emailSender,
		pushSender,
		logger,
		time.Now,
	), nil
}

// digestProfiles adapts the two profile stores the digest handler needs — the
// cross-partition digest-day selector (AdminStore) and the per-user point read
// (CosmosStore) — into the single consumer-side profile interface the handler
// depends on.
type digestProfiles struct {
	admin *profiles.AdminStore
	point *profiles.CosmosStore
}

func (p digestProfiles) ByDigestDay(ctx context.Context, day time.Weekday) ([]*profiles.UserProfile, error) {
	return p.admin.ByDigestDay(ctx, day)
}

func (p digestProfiles) Get(ctx context.Context, userID string) (*profiles.UserProfile, error) {
	return p.point.Get(ctx, userID)
}

// buildDormant constructs the dormant-cleanup handler when Cosmos is configured,
// wiring the dormant-account finder, the per-container erasure stores, and the
// Auth0 M2M deleter (real when its credentials are present, NoOp otherwise so a
// job without Auth0 M2M config still erases Cosmos data). It returns (nil, nil)
// when Cosmos is unconfigured — the dormant-cleanup mode then refuses to run
// rather than nil-panicking. Returning the concrete *dormant.Handler lets
// worker.Run accept it via its exported DormantRunner interface.
func buildDormant(cfg platform.Config, logger *slog.Logger) (*dormant.Handler, error) {
	if cfg.CosmosEndpoint == "" {
		return nil, nil //nolint:nilnil // absent Cosmos config is a valid "no dormant handler" state, not an error
	}

	users, err := platform.NewCosmosContainerNamed(cfg, "Users", logger)
	if err != nil {
		return nil, err
	}
	notifs, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosNotificationsContainer, logger)
	if err != nil {
		return nil, err
	}
	zonesContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosWatchZonesContainer, logger)
	if err != nil {
		return nil, err
	}
	savedContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosSavedApplicationsContainer, logger)
	if err != nil {
		return nil, err
	}
	devicesContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosDeviceRegistrationsContainer, logger)
	if err != nil {
		return nil, err
	}
	stateContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosNotificationStateContainer, logger)
	if err != nil {
		return nil, err
	}

	stores := dormant.Stores{
		Notifications:       notificationDeleter{notifications.NewDeleteStore(notifs)},
		WatchZones:          watchZoneDeleter{watchzones.NewCosmosStore(zonesContainer)},
		SavedApplications:   savedApplicationDeleter{savedapplications.NewCosmosStore(savedContainer)},
		DeviceRegistrations: deviceDeleter{devicetokens.NewCosmosStore(devicesContainer)},
		NotificationState:   stateDeleter{notificationstate.NewCosmosStore(stateContainer, notifs)},
		Profiles:            profileDeleter{profiles.NewCosmosStore(users)},
		Auth0:               buildAuth0Deleter(cfg, logger),
	}

	return dormant.New(profiles.NewAdminStore(users), stores, logger, time.Now), nil
}

// buildAuth0Deleter returns the real Auth0 Management (M2M) client when the M2M
// credentials are configured, else a no-op so a job without Auth0 M2M config
// still erases Cosmos data, mirroring .NET's NoOpAuth0ManagementClient fallback.
func buildAuth0Deleter(cfg platform.Config, logger *slog.Logger) dormant.Auth0Deleter {
	if !cfg.Auth0M2MConfigured() {
		logger.Info("auth0 m2m unconfigured; dormant cleanup will skip Auth0 user deletion (NoOp)")
		return profiles.NoOpAuth0Client{}
	}
	return profiles.NewAuth0Client(
		&http.Client{Timeout: 30 * time.Second},
		"https://"+cfg.Auth0Domain,
		cfg.Auth0M2MClientID,
		cfg.Auth0M2MClientSecret,
	)
}

// The adapters below bridge each store's own method name (DeleteAllByUserID /
// Delete / DeleteByUserID) to the dormant package's consumer-side interface
// method names. They keep the dormant handler's contracts explicit without
// renaming the stores' established API.

type notificationDeleter struct{ s *notifications.DeleteStore }

func (d notificationDeleter) DeleteAllNotifications(ctx context.Context, userID string) error {
	return d.s.DeleteAllByUserID(ctx, userID)
}

type watchZoneDeleter struct{ s *watchzones.CosmosStore }

func (d watchZoneDeleter) DeleteAllWatchZones(ctx context.Context, userID string) error {
	return d.s.DeleteAllByUserID(ctx, userID)
}

type savedApplicationDeleter struct{ s *savedapplications.CosmosStore }

func (d savedApplicationDeleter) DeleteAllSavedApplications(ctx context.Context, userID string) error {
	return d.s.DeleteAllByUserID(ctx, userID)
}

type deviceDeleter struct{ s *devicetokens.CosmosStore }

func (d deviceDeleter) DeleteAllDeviceRegistrations(ctx context.Context, userID string) error {
	return d.s.DeleteAllByUserID(ctx, userID)
}

type stateDeleter struct{ s *notificationstate.CosmosStore }

func (d stateDeleter) DeleteNotificationState(ctx context.Context, userID string) error {
	return d.s.DeleteByUserID(ctx, userID)
}

type profileDeleter struct{ s *profiles.CosmosStore }

// DeleteProfile maps to the profile store's Delete, translating its ErrNotFound
// (a 404 from Cosmos) so the dormant handler can tolerate a concurrently-deleted
// profile via errors.Is(err, profiles.ErrNotFound).
func (d profileDeleter) DeleteProfile(ctx context.Context, userID string) error {
	return d.s.Delete(ctx, userID)
}

// buildEmailSender returns the real ACS email sender when a connection string is
// configured, else a NoOp so a job without ACS credentials boots cleanly.
func buildEmailSender(cfg platform.Config, logger *slog.Logger) acsemail.EmailSender {
	conn := cfg.ACSConnectionString.Expose()
	if conn == "" {
		logger.Info("acs connection string unset; digest emails disabled (NoOp sender)")
		return acsemail.NewNoOpSender()
	}
	client, err := acsemail.NewClient(conn, logger, time.Now)
	if err != nil {
		logger.Error("build acs email client; falling back to NoOp sender", "error", err)
		return acsemail.NewNoOpSender()
	}
	return client
}

// buildPushSender returns the real APNs sender when APNs is enabled, else a NoOp
// so a job without a .p8 auth key boots cleanly.
func buildPushSender(cfg platform.Config, logger *slog.Logger) apns.PushSender {
	if !cfg.APNsEnabled {
		logger.Info("apns disabled; digest pushes disabled (NoOp sender)")
		return apns.NewNoOpSender()
	}
	client, err := apns.NewClient(apns.Options{
		Enabled:    cfg.APNsEnabled,
		AuthKey:    cfg.APNsAuthKey.Expose(),
		KeyID:      cfg.APNsKeyID,
		TeamID:     cfg.APNsTeamID,
		BundleID:   cfg.APNsBundleID,
		UseSandbox: cfg.APNsUseSandbox,
	}, logger, time.Now)
	if err != nil {
		logger.Error("build apns client; falling back to NoOp sender", "error", err)
		return apns.NewNoOpSender()
	}
	return client
}
