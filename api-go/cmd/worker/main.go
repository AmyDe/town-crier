// Command worker runs the Town Crier background-worker modes as short-lived
// Container Apps Jobs. One process per job: WORKER_MODE selects the mode,
// the process runs it once, flushes telemetry, and exits with a status code.
//
// poll-bootstrap, digest, hourly-digest, and dormant-cleanup are implemented;
// poll-sb remains a loud stub that exits 1 until its own bead (tc-yng2) lands.
// The Go image is not deployed to any job until the final cutover, so a stub can
// never strand a real job.
package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/AmyDe/town-crier/api-go/internal/acsemail"
	"github.com/AmyDe/town-crier/api-go/internal/apns"
	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/authorities"
	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/digest"
	"github.com/AmyDe/town-crier/api-go/internal/dormant"
	"github.com/AmyDe/town-crier/api-go/internal/erasure"
	"github.com/AmyDe/town-crier/api-go/internal/metrics"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/notificationstate"
	"github.com/AmyDe/town-crier/api-go/internal/notifydispatch"
	"github.com/AmyDe/town-crier/api-go/internal/offercodes"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/polling"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/savedapplications"
	"github.com/AmyDe/town-crier/api-go/internal/servicebus"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
	"github.com/AmyDe/town-crier/api-go/internal/worker"
	"go.opentelemetry.io/otel"
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

	// NewOTelLogger fans every record out to stdout JSON (ContainerAppConsoleLogs)
	// AND the otelslog bridge -> the global OTel LoggerProvider -> App Insights
	// AppTraces. The bridge targets the GLOBAL LoggerProvider, which is the no-op
	// provider until SetupTelemetry (called next) installs an SDK one; the global
	// provider's delegation upgrades this logger in place, so building it before
	// SetupTelemetry is correct (tc-1x8j / tc-8x8g / ADR 0027). Without it the
	// worker's slog records (e.g. digest "send failed") never reach telemetry —
	// worker spans arrive but logs do not, leaving ACS send failures invisible.
	logger := platform.NewOTelLogger(os.Stdout, cfg.LogLevel)

	mode := os.Getenv("WORKER_MODE")

	// SetupTelemetry self-disables when OTEL_EXPORTER_OTLP_ENDPOINT is unset, so
	// local/dev boots leave the no-op Tracer and Logger providers in place.
	// OTEL_SERVICE_NAME (set to town-crier-worker-go by infra) drives the service
	// name on exported spans and logs. The deferred shutdown force-flushes the
	// final batch before this short-lived process exits — without it the worker
	// can terminate before its spans and logs reach the collector; the deferred
	// shutdown flushes the final batch.
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

	// The business-metric registry is built from the global MeterProvider
	// SetupTelemetry just installed (a no-op provider when telemetry is disabled).
	// It is threaded through the builders below so the poll handler / orchestrator,
	// the PlanIt client, the notification dispatchers and every Cosmos container
	// emit their towncrier.* metrics (tc-21np).
	registry := metrics.New(otel.Meter(metrics.MeterName))

	// The Service Bus client (and thus the bootstrapper and the poll-sb
	// orchestrator) is built only when the job carries Service Bus config. Jobs
	// that don't touch Service Bus (digest, hourly-digest, dormant-cleanup) leave
	// the bootstrapper nil; poll-bootstrap then refuses to run rather than
	// crashing.
	var (
		bootstrapper *worker.Bootstrapper
		sbClient     *servicebus.Client
	)
	if cfg.ServiceBusNamespace != "" && cfg.ServiceBusQueueName != "" {
		sbClient, err = servicebus.NewClient(cfg.ServiceBusNamespace, cfg.ServiceBusQueueName, cfg.AzureClientID)
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
	handler, err := buildDigester(cfg, registry, logger)
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
	dormantHandler, err := buildDormant(cfg, registry, logger)
	if err != nil {
		logger.Error("build dormant cleanup handler", "error", err)
		return 1
	}
	if dormantHandler != nil {
		dormantRunner = dormantHandler
	}

	// The poll-sb orchestrator is built only when the job carries BOTH Service Bus
	// (trigger queue) and Cosmos (lease + poll state + applications) config. A job
	// missing either leaves it a genuinely nil interface; poll-sb then refuses to
	// run rather than crashing. Declared as the interface so the "unconfigured"
	// case stays a nil interface value (a typed-nil adapter would defeat the guard).
	var poller worker.PollOrchestrator
	pollAdapter, err := buildPollOrchestrator(cfg, sbClient, registry, logger)
	if err != nil {
		logger.Error("build poll-sb orchestrator", "error", err)
		return 1
	}
	if pollAdapter != nil {
		poller = pollAdapter
	}

	return worker.Run(context.Background(), mode, bootstrapper, digester, dormantRunner, poller, nil, logger)
}

// buildPollOrchestrator wires the poll-sb orchestrator: the PlanIt client, the
// Cosmos poll-state and etag-CAS lease stores, the cycle-alternating authority
// provider, the ingestion handler, and the next-run scheduler — bridged to the
// receive/publish operations of the shared Service Bus client. It returns
// (nil, nil) when Service Bus or Cosmos config is absent, so poll-sb refuses to
// run rather than nil-panicking. The cycle budget (replicaTimeout − grace) and
// the handler/lease budgets all come from config.
func buildPollOrchestrator(cfg platform.Config, sbClient *servicebus.Client, registry *metrics.Registry, logger *slog.Logger) (*pollOrchestratorAdapter, error) {
	if sbClient == nil || cfg.CosmosEndpoint == "" {
		return nil, nil //nolint:nilnil // absent SB/Cosmos config is a valid "no poller" state, not an error
	}

	planItClient, err := planit.NewClient(planit.Options{
		BaseURL: cfg.PlanItBaseURL,
		Throttle: planit.ThrottleOptions{
			DelayBetweenRequests: secondsToDuration(cfg.PlanItThrottleDelaySeconds),
		},
		Retry: planit.RetryOptions{
			MaxRetries:       cfg.PlanItMaxRetries,
			InitialBackoff:   secondsToDuration(cfg.PlanItInitialBackoffSeconds),
			RateLimitBackoff: secondsToDuration(cfg.PlanItRateLimitBackoffSeconds),
		},
		Metrics: registry,
	})
	if err != nil {
		return nil, err
	}

	appsContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosApplicationsContainer, logger)
	if err != nil {
		return nil, err
	}
	appsContainer.WithMetrics(registry)
	stateContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosPollStateContainer, logger)
	if err != nil {
		return nil, err
	}
	stateContainer.WithMetrics(registry)
	leasesContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosLeasesContainer, logger)
	if err != nil {
		return nil, err
	}
	leasesContainer.WithMetrics(registry)
	zonesContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosWatchZonesContainer, logger)
	if err != nil {
		return nil, err
	}
	zonesContainer.WithMetrics(registry)

	stateStore := polling.NewPollStateStore(stateContainer)
	leaseStore := polling.NewLeaseStore(leaseItemsAdapter{leasesContainer}, time.Now)
	appStore := applications.NewCosmosStore(appsContainer)
	zoneStore := watchzones.NewCosmosStore(zonesContainer)

	cycleSelector := polling.NewMinuteCycleSelector(time.Now)
	authorityProvider := polling.NewCycleAlternatingProvider(
		polling.NewWatchZoneAuthorityProvider(zoneStore),
		polling.NewAllAuthorityProvider(authorities.NewLookup()),
		cycleSelector,
	)

	maxPages := cfg.PollingMaxPagesPerAuthorityPerCycle
	handler := polling.NewPollPlanItHandler(
		planItClient,
		appStore,
		stateStore,
		authorityProvider,
		cycleSelector,
		polling.HandlerOptions{
			MaxPagesPerAuthorityPerCycle: &maxPages,
			HandlerBudget:                secondsToDuration(float64(cfg.PollingHandlerBudgetSeconds)),
		},
		time.Now,
		logger,
	)
	// Record the towncrier.polling.* per-cycle / per-authority KPIs (tc-21np).
	handler.WithMetrics(registry)

	// Wire the poll-path notification fan-out: each upserted application drives a
	// decision-event dispatch (on a non-decision -> decision transition) and a
	// watch-zone notification fan-out.
	// This is the CUTOVER-BLOCKER fan-out (bead tc-uc2p) — without it the
	// Notifications container stays empty and every alert/digest breaks.
	if err := wirePollFanOut(cfg, handler, zoneStore, registry, logger); err != nil {
		return nil, err
	}

	scheduler := polling.NewNextRunScheduler(polling.DefaultSchedulerOptions(), polling.NewRandomJitter())

	// Lease TTL must exceed the handler's worst-case runtime so the lease cannot
	// expire mid-handler (which would let a peer start a duplicate cycle). Size it
	// at the handler budget plus a 30s margin.
	leaseTTL := secondsToDuration(float64(cfg.PollingHandlerBudgetSeconds)) + 30*time.Second

	orchestrator := polling.NewOrchestrator(
		handler,
		sbClient,
		sbClient,
		leaseStore,
		scheduler,
		polling.OrchestratorOptions{
			LeaseTTL:               leaseTTL,
			LeaseAcquireRetryDelay: 1 * time.Second,
		},
		time.Now,
		logger,
	)
	// Record towncrier.polling.lease.acquired with caller "orchestrator" (tc-21np).
	orchestrator.WithLeaseMetrics(registry)

	// The hard cycle budget is replicaTimeout − grace; it bounds the whole
	// RunOnce so the process exits cleanly before Container Apps SIGTERMs the
	// replica.
	cycleBudget := time.Duration(maxInt(1, cfg.PollReplicaTimeoutSeconds-cfg.PollShutdownGraceSeconds)) * time.Second

	return &pollOrchestratorAdapter{orchestrator: orchestrator, cycleBudget: cycleBudget}, nil
}

// wirePollFanOut builds the poll-path notification fan-out collaborators — the
// decision-event dispatcher and the watch-zone enqueuer — and attaches them to
// the ingestion handler. They share the WatchZones store with the poll's
// authority provider (the same *watchzones.CosmosStore satisfies the
// zone-containment lookup) and open their own Notifications / Users /
// NotificationState / DeviceRegistrations / SavedApplications containers. The
// APNs push sender is real when its credentials are present, NoOp otherwise so
// the poll job boots even without APNs config (the record is still written, so
// the digest pipeline keeps working).
func wirePollFanOut(cfg platform.Config, handler *polling.PollPlanItHandler, zoneStore *watchzones.CosmosStore, registry *metrics.Registry, logger *slog.Logger) error {
	notifsContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosNotificationsContainer, logger)
	if err != nil {
		return err
	}
	notifsContainer.WithMetrics(registry)
	usersContainer, err := platform.NewCosmosContainerNamed(cfg, "Users", logger)
	if err != nil {
		return err
	}
	usersContainer.WithMetrics(registry)
	stateContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosNotificationStateContainer, logger)
	if err != nil {
		return err
	}
	stateContainer.WithMetrics(registry)
	devicesContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosDeviceRegistrationsContainer, logger)
	if err != nil {
		return err
	}
	devicesContainer.WithMetrics(registry)
	savedContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosSavedApplicationsContainer, logger)
	if err != nil {
		return err
	}
	savedContainer.WithMetrics(registry)

	notifStore := notifications.NewDigestStore(notifsContainer)
	profileStore := profiles.NewCosmosStore(usersContainer)
	deviceStore := devicetokens.NewCosmosStore(devicesContainer)
	statePushStore := notificationstate.NewCosmosStore(stateContainer, notifsContainer)
	savedStore := savedapplications.NewCosmosStore(savedContainer)
	pushSender := buildPushSender(cfg, logger)

	// Record towncrier.notifications.created on each dispatcher (tc-21np).
	enqueuer := notifydispatch.NewEnqueuer(
		notifStore, zoneStore, profileStore, deviceStore, statePushStore, pushSender,
		uuid.NewString, time.Now, logger,
	).WithMetrics(registry)
	dispatcher := notifydispatch.NewDecisionDispatcher(
		notifStore, zoneStore, savedStore, profileStore, deviceStore, statePushStore, pushSender,
		uuid.NewString, time.Now, logger,
	).WithMetrics(registry)

	handler.WithFanOut(dispatcher, enqueuer)
	return nil
}

// pollOrchestratorAdapter flattens polling.OrchestratorRunResult into the
// worker.PollRunResult the dispatcher tags and exit-codes on, and applies the
// hard cycle-budget timeout around the orchestrator's single run. It satisfies
// worker.PollOrchestrator.
type pollOrchestratorAdapter struct {
	orchestrator *polling.Orchestrator
	cycleBudget  time.Duration
}

func (a *pollOrchestratorAdapter) RunOnce(ctx context.Context) (worker.PollRunResult, error) {
	cycleCtx, cancel := context.WithTimeout(ctx, a.cycleBudget)
	defer cancel()

	res, err := a.orchestrator.RunOnce(cycleCtx)
	if err != nil {
		return worker.PollRunResult{MessageReceived: res.MessageReceived}, err
	}

	out := worker.PollRunResult{
		MessageReceived:  res.MessageReceived,
		PublishedNext:    res.PublishedNext,
		LeaseUnavailable: res.LeaseUnavailable,
	}
	if res.PollResult != nil {
		out.ApplicationCount = res.PollResult.ApplicationCount
		out.AuthoritiesPolled = res.PollResult.AuthoritiesPolled
		out.AuthorityErrors = res.PollResult.AuthorityErrors
		out.Termination = res.PollResult.TerminationReason.TelemetryValue()
	}
	return out, nil
}

// leaseItemsAdapter bridges *platform.CosmosContainer's etag-CAS methods to the
// polling lease store's consumer interface, translating the platform CAS
// sentinel errors into the polling sentinels the store branches on.
type leaseItemsAdapter struct {
	c *platform.CosmosContainer
}

func (a leaseItemsAdapter) ReadLeaseWithETag(ctx context.Context, id string) ([]byte, string, bool, error) {
	return a.c.ReadItemWithETag(ctx, id, id)
}

func (a leaseItemsAdapter) CreateLease(ctx context.Context, id string, item []byte) (string, error) {
	etag, err := a.c.CreateItemReturningETag(ctx, id, item)
	if errors.Is(err, platform.ErrCASConflict) {
		return "", polling.ErrLeaseConflict
	}
	return etag, err
}

func (a leaseItemsAdapter) ReplaceLeaseWithETag(ctx context.Context, id string, item []byte, etag string) (string, error) {
	newETag, err := a.c.ReplaceItemWithETag(ctx, id, id, item, etag)
	if errors.Is(err, platform.ErrCASPreconditionFailed) {
		return "", polling.ErrLeasePreconditionFailed
	}
	return newETag, err
}

func (a leaseItemsAdapter) DeleteLeaseWithETag(ctx context.Context, id, etag string) error {
	err := a.c.DeleteItemWithETag(ctx, id, id, etag)
	switch {
	case errors.Is(err, platform.ErrCASNotFound):
		return polling.ErrLeaseNotFound
	case errors.Is(err, platform.ErrCASPreconditionFailed):
		return polling.ErrLeasePreconditionFailed
	default:
		return err
	}
}

// secondsToDuration converts a fractional-seconds config value to a Duration.
func secondsToDuration(s float64) time.Duration {
	return time.Duration(s * float64(time.Second))
}

// maxInt returns the larger of a and b.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// buildDigester constructs the digest handler when Cosmos is configured, wiring
// the per-container stores and the email/push senders (real when their
// credentials are present, NoOp otherwise so a job without ACS/APNs boots
// cleanly). It returns (nil, nil) when Cosmos is unconfigured — the digest modes
// then refuse to run rather than nil-panicking. Returning the concrete
// *digest.Handler lets worker.Run accept it via its unexported digestRunner
// interface (structural satisfaction).
func buildDigester(cfg platform.Config, registry *metrics.Registry, logger *slog.Logger) (*digest.Handler, error) {
	if cfg.CosmosEndpoint == "" {
		return nil, nil //nolint:nilnil // absent Cosmos config is a valid "no digester" state, not an error
	}

	users, err := platform.NewCosmosContainerNamed(cfg, "Users", logger)
	if err != nil {
		return nil, err
	}
	users.WithMetrics(registry)
	notifs, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosNotificationsContainer, logger)
	if err != nil {
		return nil, err
	}
	notifs.WithMetrics(registry)
	zonesContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosWatchZonesContainer, logger)
	if err != nil {
		return nil, err
	}
	zonesContainer.WithMetrics(registry)
	stateContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosNotificationStateContainer, logger)
	if err != nil {
		return nil, err
	}
	stateContainer.WithMetrics(registry)
	devicesContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosDeviceRegistrationsContainer, logger)
	if err != nil {
		return nil, err
	}
	devicesContainer.WithMetrics(registry)

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
func buildDormant(cfg platform.Config, registry *metrics.Registry, logger *slog.Logger) (*dormant.Handler, error) {
	if cfg.CosmosEndpoint == "" {
		return nil, nil //nolint:nilnil // absent Cosmos config is a valid "no dormant handler" state, not an error
	}

	users, err := platform.NewCosmosContainerNamed(cfg, "Users", logger)
	if err != nil {
		return nil, err
	}
	users.WithMetrics(registry)
	notifs, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosNotificationsContainer, logger)
	if err != nil {
		return nil, err
	}
	notifs.WithMetrics(registry)
	zonesContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosWatchZonesContainer, logger)
	if err != nil {
		return nil, err
	}
	zonesContainer.WithMetrics(registry)
	savedContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosSavedApplicationsContainer, logger)
	if err != nil {
		return nil, err
	}
	savedContainer.WithMetrics(registry)
	devicesContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosDeviceRegistrationsContainer, logger)
	if err != nil {
		return nil, err
	}
	devicesContainer.WithMetrics(registry)
	stateContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosNotificationStateContainer, logger)
	if err != nil {
		return nil, err
	}
	stateContainer.WithMetrics(registry)
	offerCodesContainer, err := platform.NewCosmosContainerNamed(cfg, platform.CosmosOfferCodesContainer, logger)
	if err != nil {
		return nil, err
	}
	offerCodesContainer.WithMetrics(registry)

	// The four DeleteAllByUserID stores satisfy erasure.ChildDeleter directly, the
	// profile store's Delete satisfies erasure.ProfileDeleter directly, and the
	// notification-state store (whose method is DeleteByUserID) is bridged by
	// erasure.NotificationStateChild — so no per-step adapter types are needed. The
	// offer-code store anonymises the redeemer back-reference (redeemedByUserId +
	// redeemedAt) without deleting the admin-issued code (bead tc-5jyh).
	deleters := erasure.Deleters{
		Notifications:       notifications.NewDeleteStore(notifs),
		WatchZones:          watchzones.NewCosmosStore(zonesContainer),
		SavedApplications:   savedapplications.NewCosmosStore(savedContainer),
		DeviceRegistrations: devicetokens.NewCosmosStore(devicesContainer),
		NotificationState:   erasure.NotificationStateChild(notificationstate.NewCosmosStore(stateContainer, notifs)),
		OfferCodes:          offercodes.NewCosmosStore(offerCodesContainer),
		Profile:             profiles.NewCosmosStore(users),
		Auth0:               buildAuth0Deleter(cfg, logger),
		ProfileAbsent:       func(e error) bool { return errors.Is(e, profiles.ErrNotFound) },
	}

	return dormant.New(profiles.NewAdminStore(users), deleters, logger, time.Now), nil
}

// buildAuth0Deleter returns the real Auth0 Management (M2M) client when the M2M
// credentials are configured, else a no-op so a job without Auth0 M2M config
// still erases Cosmos data.
func buildAuth0Deleter(cfg platform.Config, logger *slog.Logger) erasure.Auth0Deleter {
	if !cfg.Auth0M2MConfigured() {
		logger.Info("auth0 m2m unconfigured; dormant cleanup will skip Auth0 user deletion (NoOp)")
		return profiles.NoOpAuth0Client{}
	}
	// Wrap the transport so Auth0 token/DELETE calls emit OTel client spans
	// (Type=HTTP in AppDependencies) named "Auth0 token"; the host lands in
	// server.address.
	auth0HTTP := platform.WrapHTTPClient(
		&http.Client{Timeout: 30 * time.Second},
		func(string, *http.Request) string { return "Auth0 token" },
	)
	return profiles.NewAuth0Client(
		auth0HTTP,
		"https://"+cfg.Auth0Domain,
		cfg.Auth0M2MClientID,
		cfg.Auth0M2MClientSecret,
	)
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
