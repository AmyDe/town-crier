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
	"os"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/acsemail"
	"github.com/AmyDe/town-crier/api-go/internal/apns"
	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/digest"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/notificationstate"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
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

	return worker.Run(context.Background(), mode, bootstrapper, digester, logger)
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
