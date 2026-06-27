// Command worker runs the Town Crier background-worker modes as short-lived
// Container Apps Jobs. One process per job: WORKER_MODE selects the mode,
// the process runs it once, flushes telemetry, and exits with a status code.
//
// poll-bootstrap, poll-sb, digest, hourly-digest, dormant-cleanup,
// subscription-sweep and pg-purge are implemented. Every store is backed by
// Postgres + PostGIS (the single datastore); the shared pool is built once at
// boot and a pool failure is fatal.
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
	"github.com/AmyDe/town-crier/api-go/internal/pgpurge"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres"
	"github.com/AmyDe/town-crier/api-go/internal/polling"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/savedapplications"
	"github.com/AmyDe/town-crier/api-go/internal/servicebus"
	"github.com/AmyDe/town-crier/api-go/internal/subscriptionsweep"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
	"github.com/AmyDe/town-crier/api-go/internal/worker"
	"go.opentelemetry.io/otel"
)

// stores holds every Postgres store the worker's modes use, built once from the
// shared pool at boot. Every field is always populated — Postgres is the only
// datastore.
type stores struct {
	app          *applications.PostgresStore
	zone         *watchzones.PostgresStore
	profile      *profiles.PostgresStore
	profileAdmin *profiles.PostgresAdminStore
	notification *notifications.PostgresStore
	notifState   *notificationstate.PostgresStore
	device       *devicetokens.PostgresStore
	savedApp     *savedapplications.PostgresStore
	offerCode    *offercodes.PostgresStore
	pollState    *polling.PostgresPollStateStore
	lease        *polling.PostgresLeaseStore
}

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
	// the PlanIt client and the notification dispatchers emit their towncrier.*
	// metrics (tc-21np).
	registry := metrics.New(otel.Meter(metrics.MeterName))

	// Build the shared Postgres pool unconditionally — Postgres is the only
	// datastore. One pool is reused by every store and every builder below; it
	// closes on process exit. A pool error is always fatal: the worker cannot run
	// any mode without its store.
	pool, perr := postgres.NewPoolFromEnv(context.Background())
	if perr != nil {
		logger.Error("postgres: build pool", "error", perr)
		return 1
	}
	defer pool.Close()

	st := &stores{
		app:          applications.NewPostgresStore(pool),
		zone:         watchzones.NewPostgresStore(pool),
		profile:      profiles.NewPostgresStore(pool),
		profileAdmin: profiles.NewPostgresAdminStore(pool),
		notification: notifications.NewPostgresStore(pool),
		notifState:   notificationstate.NewPostgresStore(pool),
		device:       devicetokens.NewPostgresStore(pool),
		savedApp:     savedapplications.NewPostgresStore(pool),
		offerCode:    offercodes.NewPostgresStore(pool),
		pollState:    polling.NewPostgresPollStateStore(pool),
		lease:        polling.NewPostgresLeaseStore(pool, time.Now),
	}

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

	// The digest, dormant-cleanup, subscription-sweep and pg-purge handlers build
	// unconditionally now Postgres is always present. The email and push senders
	// fall back to NoOp when their credentials are absent, so a job without
	// ACS/APNs config still boots cleanly.
	digester := buildDigester(cfg, st, logger)
	dormantRunner := buildDormant(cfg, st, logger)
	sweepRunner := buildSweep(cfg, st, logger)

	// The poll-sb orchestrator is built only when the job carries Service Bus
	// (trigger queue) config. A job missing it leaves poller a genuinely nil
	// interface; poll-sb then refuses to run rather than crashing. Declared as the
	// interface so the "unconfigured" case stays a nil interface value (a typed-nil
	// adapter would defeat the guard).
	var poller worker.PollOrchestrator
	pollAdapter, err := buildPollOrchestrator(cfg, sbClient, registry, st, logger)
	if err != nil {
		logger.Error("build poll-sb orchestrator", "error", err)
		return 1
	}
	if pollAdapter != nil {
		poller = pollAdapter
	}

	// The pg-purge runner enforces row retention for Notifications (90 days by
	// default, NOTIFICATIONS_RETENTION_DAYS) and DeviceRegistrations (180 days,
	// DEVICE_REGISTRATIONS_RETENTION_DAYS).
	purger := pgpurge.New(
		st.notification,
		st.device,
		time.Duration(cfg.NotificationsRetentionDays)*24*time.Hour,
		time.Duration(cfg.DeviceRegistrationsRetentionDays)*24*time.Hour,
		time.Now,
		logger,
	)

	return worker.Run(context.Background(), mode, bootstrapper, digester, dormantRunner, poller, sweepRunner, purger, logger)
}

// buildPollOrchestrator wires the poll-sb orchestrator: the PlanIt client, the
// Postgres poll-state and lease stores, the cycle-alternating authority provider,
// the ingestion handler, and the next-run scheduler — bridged to the
// receive/publish operations of the shared Service Bus client. It returns
// (nil, nil) when Service Bus config is absent, so poll-sb refuses to run rather
// than nil-panicking. The cycle budget (replicaTimeout − grace) and the
// handler/lease budgets all come from config.
func buildPollOrchestrator(cfg platform.Config, sbClient *servicebus.Client, registry *metrics.Registry, st *stores, logger *slog.Logger) (*pollOrchestratorAdapter, error) {
	if sbClient == nil {
		return nil, nil //nolint:nilnil // absent Service Bus config is a valid "no poller" state, not an error
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

	// appStore and zoneStore are the Postgres stores the poll handler (Upsert +
	// change dedup) and the watch-zone notify fan-out (FindZonesContaining) consume.
	appStore := st.app
	zoneStore := st.zone

	var stateStore polling.PollStateAccess = st.pollState
	var leaseStore polling.LeaseAccess = st.lease

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
	// Notifications table stays empty and every alert/digest breaks.
	wirePollFanOut(cfg, handler, zoneStore, registry, st, logger)

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
// authority provider (the same watchzones.Store satisfies the zone-containment
// lookup) and the Postgres Notifications / Users / NotificationState /
// DeviceRegistrations / SavedApplications stores. zoneStore is the same Postgres
// watchzones.Store from buildPollOrchestrator so FindZonesContaining already runs
// against the right backend. The APNs push sender is real when its credentials
// are present, NoOp otherwise so the poll job boots even without APNs config (the
// record is still written, so the digest pipeline keeps working).
//
// st may be nil in tests that only exercise the zone-containment path; the store
// fields are extracted under a nil guard so the fan-out wires with no other
// store dependency.
func wirePollFanOut(cfg platform.Config, handler *polling.PollPlanItHandler, zoneStore watchzones.Store, registry *metrics.Registry, st *stores, logger *slog.Logger) {
	var (
		notifStore     *notifications.PostgresStore
		profileStore   *profiles.PostgresStore
		deviceStore    *devicetokens.PostgresStore
		statePushStore *notificationstate.PostgresStore
		savedStore     *savedapplications.PostgresStore
	)
	if st != nil {
		notifStore = st.notification
		profileStore = st.profile
		deviceStore = st.device
		statePushStore = st.notifState
		savedStore = st.savedApp
	}

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

// buildDigester constructs the digest handler, wiring the per-feature Postgres
// stores and the email/push senders (real when their credentials are present,
// NoOp otherwise so a job without ACS/APNs boots cleanly).
func buildDigester(cfg platform.Config, st *stores, logger *slog.Logger) *digest.Handler {
	// digestProfiles combines the cross-user admin selector (ByDigestDay) and the
	// point-read store (Get).
	profileStore := digestProfiles{
		admin: st.profileAdmin,
		point: st.profile,
	}

	emailSender := buildEmailSender(cfg, logger)
	pushSender := buildPushSender(cfg, logger)

	return digest.NewHandler(
		profileStore,
		st.notification,
		st.zone,
		st.notifState,
		st.device,
		emailSender,
		pushSender,
		logger,
		time.Now,
	)
}

// digestProfiles adapts the two profile stores the digest handler needs — the
// cross-user digest-day selector (AdminProfileStore) and the per-user point read
// (Store) — into the single consumer-side profile interface the handler depends
// on.
type digestProfiles struct {
	admin profiles.AdminProfileStore
	point profiles.Store
}

func (p digestProfiles) ByDigestDay(ctx context.Context, day time.Weekday) ([]*profiles.UserProfile, error) {
	return p.admin.ByDigestDay(ctx, day)
}

func (p digestProfiles) Get(ctx context.Context, userID string) (*profiles.UserProfile, error) {
	return p.point.Get(ctx, userID)
}

// buildDormant constructs the dormant-cleanup handler, wiring the dormant-account
// finder, the per-feature Postgres erasure stores (GDPR cascade completeness is a
// hard requirement), and the Auth0 M2M deleter (real when its credentials are
// present, NoOp otherwise so a job without Auth0 M2M config still erases data).
func buildDormant(cfg platform.Config, st *stores, logger *slog.Logger) *dormant.Handler {
	// Every erasure.Deleters member is the Postgres store: a Postgres row must never
	// be missed by the GDPR cascade. Notifications uses the Postgres store directly
	// (which has DeleteAllByUserID) — so the same store serves both the fan-out path
	// and erasure.
	deleters := erasure.Deleters{
		Notifications:       st.notification,
		WatchZones:          st.zone,
		SavedApplications:   st.savedApp,
		DeviceRegistrations: st.device,
		NotificationState:   erasure.NotificationStateChild(st.notifState),
		OfferCodes:          st.offerCode,
		Profile:             st.profile,
		Auth0:               buildAuth0Deleter(cfg, logger),
		ProfileAbsent:       func(e error) bool { return errors.Is(e, profiles.ErrNotFound) },
	}

	// The FINDER (dormant-account scan) uses the same Postgres admin store as every
	// deleter and as buildSweep's finder.
	finder := st.profileAdmin
	return dormant.New(finder, deleters, logger, time.Now)
}

// buildAuth0Deleter returns the real Auth0 Management (M2M) client when the M2M
// credentials are configured, else a no-op so a job without Auth0 M2M config
// still erases the stored data.
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

// buildSweep constructs the subscription-sweep handler, wiring the lapsed-paid
// finder and profile saver (both the Postgres admin store) and the Auth0 M2M
// syncer (real when its credentials are present, NoOp otherwise so a job without
// Auth0 M2M config still reverts the stored tier).
func buildSweep(cfg platform.Config, st *stores, logger *slog.Logger) *subscriptionsweep.Handler {
	// adminStore backs both LapsedPaid (Finder) and Save (Saver).
	adminStore := st.profileAdmin
	return subscriptionsweep.New(adminStore, adminStore, buildAuth0Syncer(cfg, logger), logger, time.Now)
}

// buildAuth0Syncer returns the real Auth0 Management (M2M) client when the M2M
// credentials are configured, else a no-op so a job without Auth0 M2M config still
// reverts the stored tier. The read path (EffectiveTier) already treats a lapsed
// user as Free everywhere, so the Auth0 subscription_tier metadata the sweep keeps
// in step is informational, not load-bearing.
func buildAuth0Syncer(cfg platform.Config, logger *slog.Logger) subscriptionsweep.Auth0Syncer {
	if !cfg.Auth0M2MConfigured() {
		logger.Info("auth0 m2m unconfigured; subscription sweep will skip Auth0 tier sync (NoOp)")
		return profiles.NoOpAuth0Client{}
	}
	// Wrap the transport so Auth0 token/PATCH calls emit OTel client spans
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
