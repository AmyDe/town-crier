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

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/google/uuid"

	"github.com/AmyDe/town-crier/api-go/internal/acsemail"
	"github.com/AmyDe/town-crier/api-go/internal/apns"
	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/authorities"
	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/devseed"
	"github.com/AmyDe/town-crier/api-go/internal/digest"
	"github.com/AmyDe/town-crier/api-go/internal/dormant"
	"github.com/AmyDe/town-crier/api-go/internal/erasure"
	"github.com/AmyDe/town-crier/api-go/internal/fcm"
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
	backfill     *polling.PostgresBackfillStateStore
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
		offerCode:    offercodes.NewPostgresStore(pool, logger),
		pollState:    polling.NewPostgresPollStateStore(pool),
		lease:        polling.NewPostgresLeaseStore(pool, time.Now),
		backfill:     polling.NewPostgresBackfillStateStore(pool),
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
		// The bootstrapper shares the same Postgres polling lease the poll-sb
		// orchestrator uses (built into st.lease below) — this is what closes the
		// unleased-bootstrap fork mechanism (GH#938 PR1): only the current lease
		// holder may mutate the trigger queue. WithLeaseMetrics records
		// towncrier.polling.lease.acquired with caller "bootstrap" (registry.go:264,
		// designed for but never wired until now).
		bootstrapper = worker.NewBootstrapper(sbClient, st.lease, logger, time.Now).WithLeaseMetrics(registry)
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

	// The dev-seed job is built only when its dedicated prod-read config
	// (DEV_SEED_PROD_AZURE_CLIENT_ID / DEV_SEED_PROD_POSTGRES_USER) is present.
	// It is created dev-only (tc-grvu.6), so a job missing it (every prod job,
	// and any dev job before that infra bead deploys) leaves devSeeder a
	// genuinely nil interface; dev-seed then refuses to run rather than
	// crashing.
	devSeeder := buildDevSeeder(cfg, st, logger)

	return worker.Run(context.Background(), mode, bootstrapper, digester, dormantRunner, poller, sweepRunner, purger, devSeeder, logger)
}

// buildPollOrchestrator wires the poll-sb orchestrator: the PlanIt client, the
// Postgres poll-state and lease stores, ADR 0041's three national delta lanes
// (A/B/C), and the next-run scheduler — bridged to the receive/publish
// operations of the shared Service Bus client. It returns (nil, nil) when
// Service Bus config is absent, so poll-sb refuses to run rather than
// nil-panicking. The cycle budget (replicaTimeout − grace) and the
// handler/lease budgets all come from config.
//
// ADR 0041 / GH#962 (bead tc-5m3tw) replaced the per-authority drain this
// function used to build — the LRU authority selection, the watched/seed
// cycle alternation, and the per-authority cursor/high-water-mark handler —
// with a single national query per lane and one global watermark per lane.
// That old wiring (polling.NewMinuteCycleSelector, NewCycleAlternatingProvider,
// NewWatchZoneAuthorityProvider, NewPollPlanItHandler) is left compiling but
// unwired (the ADR's explicit migration posture: rollback is a config change,
// not a revert). NewAllAuthorityProvider is the one exception — it is
// REWIRED below as Lane C's per-authority sweep target, the same pollable-
// authority list the old Seed cycle drained.
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
		// PageSize governs only the legacy per-authority FetchApplicationsPage
		// path (unwired below); the national lanes hardcode pg_sz=300
		// (planit.nationalPageSize) — a fixed safety rule, not a config dial.
		PageSize: cfg.PollingPlanItPageSize,
	})
	if err != nil {
		return nil, err
	}

	// appStore and zoneStore are the Postgres stores every lane's ingester
	// (Upsert + change dedup) and the watch-zone notify fan-out
	// (FindZonesContaining) consume.
	appStore := st.app
	zoneStore := st.zone

	var stateStore polling.PollStateAccess = st.pollState
	var leaseStore polling.LeaseAccess = st.lease

	// Lane A (new applications) and Lane B (decisions): the national
	// churn-masked delta poll. Each lane owns ONE global watermark (persisted
	// in the existing poll_state table via a reserved sentinel authority_id —
	// no schema migration) instead of the retired per-authority cursors.
	laneA := polling.NewNationalLaneHandler(
		planItClient, stateStore, appStore,
		polling.NationalLaneOptions{
			Lane:       polling.LaneA,
			Mask:       planit.MaskStartDate,
			MaskWindow: time.Duration(cfg.PollingLaneAMaskDays) * 24 * time.Hour,
			MaxPages:   nil, // unbounded: the national delta is measured at ~6 pages/day (ADR 0041)
		},
		time.Now, logger,
	)
	laneBMaxPages := cfg.PollingLaneBMaxPages
	laneB := polling.NewNationalLaneHandler(
		planItClient, stateStore, appStore,
		polling.NationalLaneOptions{
			Lane:       polling.LaneB,
			Mask:       planit.MaskDecidedStart,
			MaskWindow: time.Duration(cfg.PollingLaneBMaskDays) * 24 * time.Hour,
			MaxPages:   &laneBMaxPages, // decision volume is unmeasured pre-cutover; do not remove this cap
		},
		time.Now, logger,
	)

	// Lane C (reconciliation): a weekly (config-dialable) per-authority
	// completeness backstop, not on the cutover's critical path. Reuses the
	// static pollable-authority list the old Seed cycle drained.
	//
	// Gated behind POLLING_LANE_C_ENABLED (default ON as of tc-tuge8/GH#971):
	// Lane C shipped broken in v0.21.0 — its per-authority query 400s because
	// it carried no date param at all ("Spatial, date or search restrictions
	// required in query", confirmed from prod's
	// reconciliation.sample_error_body span attribute), and it never recorded
	// its weekly last-run when the sweep was cut off by the cycle budget, so
	// it re-ran and hammered PlanIt every cycle. Both are now fixed —
	// buildReconciliationPath sends a different_start bound (below) and Run
	// persists its cursor uncancellably (PR 1, tc-tuge8) — so the config
	// default now enables Lane C; an operator can still set
	// POLLING_LANE_C_ENABLED=false to force it off.
	var laneC *polling.ReconciliationHandler
	if cfg.PollingLaneCEnabled {
		laneC = polling.NewReconciliationHandler(
			planItClient, stateStore, appStore,
			polling.NewAllAuthorityProvider(authorities.NewLookup()),
			polling.ReconciliationOptions{
				Interval:                  time.Duration(cfg.PollingLaneCIntervalHours) * time.Hour,
				MaxStragglersPerAuthority: cfg.PollingLaneCMaxStragglersPerAuthority,
				AuthoritiesPerCycle:       cfg.PollingLaneCAuthoritiesPerCycle,
				LookbackDays:              cfg.PollingLaneCLookbackDays,
			},
			time.Now, logger,
		)
	}

	handler := polling.NewNationalPollHandler(laneA, laneB, laneC, time.Now, logger)

	// Lane D (GH#967, ADR 0042): the paced historical backfill lane. A
	// national, date-windowed backward sweep that enriches stale/NULL GH#935
	// fields and fills coverage gaps via the existing Ingester — structurally
	// incapable of notifying (its Ingester is always built with nil
	// decision/enqueuer collaborators, and BackfillHandler has no method that
	// could wire one; see internal/polling/backfill.go's package doc).
	//
	// Gated behind POLLING_BACKFILL_ENABLED (default off), mirroring the
	// Lane C rollout precedent (tc-5lu8h): ships dark, soaks, then flips on
	// deliberately. WithBackfill(nil) is the safe default when disabled —
	// NationalPollHandler.Handle nil-guards it exactly like Lane C.
	//
	// Deliberately NOT passed to wirePollFanOut below: there is no fan-out to
	// wire for this lane, by construction.
	var laneD *polling.BackfillHandler
	if cfg.PollingBackfillEnabled {
		laneD = polling.NewBackfillHandler(
			planItClient, st.backfill, appStore,
			polling.BackfillOptions{
				WindowWidthDays:            cfg.PollingBackfillWindowWidthDays,
				MaxPagesPerCycle:           cfg.PollingBackfillMaxPagesPerCycle,
				EmptyWindowsBeforeComplete: cfg.PollingBackfillEmptyWindowsBeforeComplete,
			},
			time.Now, logger,
		).WithMetrics(registry)
	}
	handler.WithBackfill(laneD)

	// Wire the poll-path notification fan-out onto all three lanes: each
	// upserted/hydrated application drives a decision-event dispatch (on a
	// non-decision -> decision transition) and a watch-zone notification
	// fan-out, unchanged from the old drain (GH#784, tc-uc2p — the
	// CUTOVER-BLOCKER fan-out; without it the Notifications table stays
	// empty and every alert/digest breaks).
	wirePollFanOut(cfg, laneA, laneB, laneC, handler, zoneStore, registry, st, logger)

	scheduler := polling.NewNextRunScheduler(polling.DefaultSchedulerOptions(), polling.NewRandomJitter())

	// Lease TTL must exceed the handler's worst-case runtime so the lease cannot
	// expire mid-handler (which would let a peer start a duplicate cycle, forking
	// the trigger chain). leaseTTLFor's +5m margin (GH#938 PR1) replaces a +30s
	// margin that was observed too tight: the 4-minute handler budget is soft
	// (checked between authorities, not preemptive), and an in-flight timeout plus
	// container startup (acquire happens before receive) overran the old slack —
	// a cycle ran ~4.9m against a 4.5m TTL during the 2026-07-12 PlanIt outage.
	// RunOnce's Confirm-before-publish CAS is the second, TOCTOU-safe layer that
	// still catches any residual overrun.
	leaseTTL := leaseTTLFor(secondsToDuration(float64(cfg.PollingHandlerBudgetSeconds)))

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
// decision-event dispatcher, the watch-zone enqueuer, and the push coalescer
// that batches the cycle's queued pushes into at most one per (user, watch
// zone) — and attaches them to all three ADR 0041 lanes (GH#784). They share
// the WatchZones store with Lane C's authority provider (the same
// watchzones.Store satisfies the zone-containment lookup and the coalescer's
// zone-name resolution) and the Postgres Notifications / Users /
// NotificationState / DeviceRegistrations / SavedApplications stores.
// zoneStore is the same Postgres watchzones.Store from buildPollOrchestrator
// so FindZonesContaining already runs against the right backend. The APNs
// push sender is real when its credentials are present, NoOp otherwise so the
// poll job boots even without APNs config (the record is still written, so
// the digest pipeline keeps working). The push coalescer is wired onto the
// top-level handler, not the individual lanes: one Reset/Flush per cycle
// covers every lane's pushes, mirroring the old drain's single flush point.
//
// st may be nil in tests that only exercise the zone-containment path; the store
// fields are extracted under a nil guard so the fan-out wires with no other
// store dependency.
func wirePollFanOut(cfg platform.Config, laneA, laneB *polling.NationalLaneHandler, laneC *polling.ReconciliationHandler, handler *polling.NationalPollHandler, zoneStore watchzones.Store, registry *metrics.Registry, st *stores, logger *slog.Logger) {
	dispatcher, enqueuer, coalescer := buildNotifyFanOut(cfg, zoneStore, st, logger)

	// Record towncrier.notifications.created on each dispatcher (tc-21np). Only
	// the real poll-sb path is on this KPI surface, so metrics wiring is this
	// caller's job, not buildNotifyFanOut's.
	enqueuer = enqueuer.WithMetrics(registry)
	dispatcher = dispatcher.WithMetrics(registry)

	laneA.WithFanOut(dispatcher, enqueuer)
	laneB.WithFanOut(dispatcher, enqueuer)
	// laneC is nil when POLLING_LANE_C_ENABLED is off (its default) — Lane C is
	// disabled until tc-tuge8 fixes it, so there is nothing to wire.
	if laneC != nil {
		laneC.WithFanOut(dispatcher, enqueuer)
	}
	handler.WithPushFlusher(coalescer)

	// Record towncrier.polling.applications_ingested / cycles_completed /
	// oldest_hwm_age_seconds on the three ADR 0041 lanes and the top-level
	// handler (tc-7ef9g) — the same registry already wired onto the
	// notification fan-out above. Without this the new national-lane code
	// path is invisible on those instruments even though ingestion itself
	// works (confirmed via AppDependencies span inspection).
	laneA.WithMetrics(registry)
	laneB.WithMetrics(registry)
	if laneC != nil {
		laneC.WithMetrics(registry)
	}
	handler.WithMetrics(registry)
}

// buildNotifyFanOut constructs the decision-dispatch, zone-enqueue and
// push-coalescer collaborators the notification fan-out needs. It is shared by
// wirePollFanOut (the real poll-sb path, prod-only) and buildDevSeeder (the
// dev-seed job, dev-only, tc-grvu.5/GH#808) so both feed applications through
// byte-for-byte the same notification pipeline, whatever their application
// source (PlanIt poll vs. the read-only prod mirror). Metrics wiring
// (WithMetrics) is left to the caller: dev-seed is a QA aid, not part of the
// towncrier.notifications.* KPI surface poll-sb's real cycle feeds, so it
// deliberately skips it.
//
// st may be nil in tests that only exercise the zone-containment path; the
// store fields are extracted under a nil guard so the fan-out wires with no
// other store dependency.
func buildNotifyFanOut(cfg platform.Config, zoneStore watchzones.Store, st *stores, logger *slog.Logger) (*notifydispatch.DecisionDispatcher, *notifydispatch.Enqueuer, *notifydispatch.PushCoalescer) {
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

	pushDispatcher := buildPlatformDispatcher(cfg, logger)
	coalescer := notifydispatch.NewPushCoalescer(deviceStore, statePushStore, pushDispatcher, zoneStore, logger)
	enqueuer := notifydispatch.NewEnqueuer(
		notifStore, zoneStore, profileStore, coalescer,
		uuid.NewString, time.Now, logger,
	)
	dispatcher := notifydispatch.NewDecisionDispatcher(
		notifStore, zoneStore, savedStore, profileStore, coalescer,
		uuid.NewString, time.Now, logger,
	)
	return dispatcher, enqueuer, coalescer
}

// buildDevSeeder constructs the dev-seed job's collaborators: a second,
// read-only pgxpool.Pool authenticated as the dedicated
// towncrier_dev_seed_reader Postgres role via its own managed identity
// (DEV_SEED_PROD_AZURE_CLIENT_ID, infra bead tc-grvu.1 — a distinct identity
// from AzureClientID, which stays scoped to this process's own pool), wrapped
// in applications.PostgresStore to read prod's most-recently-changed
// applications, fed through a polling.Ingester built over the SAME
// decision-dispatch/enqueue/push-coalescer collaborators wirePollFanOut builds
// for the real poll path (via the shared buildNotifyFanOut, bound here to
// dev's own stores), into a devseed.Seeder.
//
// It returns nil when DEV_SEED_PROD_AZURE_CLIENT_ID or
// DEV_SEED_PROD_POSTGRES_USER is unset — the "unconfigured optional job"
// posture buildPollOrchestrator/buildPushSender already use — so dev-seed
// refuses to run rather than nil-panicking. This mode is created dev-only
// (tc-grvu.6): every prod job, and any dev job before that infra bead deploys,
// takes this path. A credential or pool build error is also treated as
// unconfigured (logged, nil returned) rather than fatal, since a malformed
// managed-identity/DSN input at boot must not crash the OTHER modes this same
// binary dispatches (digest, dormant-cleanup, etc.) when they share a process.
func buildDevSeeder(cfg platform.Config, st *stores, logger *slog.Logger) worker.DevSeedRunner {
	if cfg.DevSeedProdAzureClientID == "" || cfg.DevSeedProdPostgresUser == "" {
		logger.Info("dev-seed unconfigured (DEV_SEED_PROD_AZURE_CLIENT_ID / DEV_SEED_PROD_POSTGRES_USER unset); dev-seed mode will refuse to run")
		return nil
	}

	cred, err := azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
		ID: azidentity.ClientID(cfg.DevSeedProdAzureClientID),
	})
	if err != nil {
		logger.Error("dev-seed: build managed-identity credential; dev-seed mode will refuse to run", "error", err)
		return nil
	}

	prodPool, err := postgres.NewTokenCredentialPool(context.Background(), postgres.ConnParams{
		Host:    cfg.PostgresHost,
		DB:      cfg.DevSeedProdPostgresDB,
		User:    cfg.DevSeedProdPostgresUser,
		SSLMode: cfg.PostgresSSLMode,
	}, cred)
	if err != nil {
		logger.Error("dev-seed: build prod read-only pool; dev-seed mode will refuse to run", "error", err)
		return nil
	}

	prodApps := applications.NewPostgresStore(prodPool)

	// registry is deliberately nil here: buildNotifyFanOut's metrics wiring is
	// wirePollFanOut's job (the poll-sb KPI surface); dev-seed's ingestion is a
	// QA aid and skips it.
	decision, enqueuer, coalescer := buildNotifyFanOut(cfg, st.zone, st, logger)
	ingester := polling.NewIngester(st.app, decision, enqueuer)

	return devseed.NewSeeder(st.zone, prodApps, ingester, coalescer, cfg.DevSeedLimit, logger)
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
		out.OldestHWMAgeSeconds = res.PollResult.OldestHWMAgeSeconds
		out.OldestHWMNeverPolled = res.PollResult.OldestHWMNeverPolled
		out.CycleType = res.PollResult.CycleType
	}
	return out, nil
}

// secondsToDuration converts a fractional-seconds config value to a Duration.
func secondsToDuration(s float64) time.Duration {
	return time.Duration(s * float64(time.Second))
}

// leaseTTLFor derives the polling lease TTL from the handler budget: budget
// plus a 5-minute margin, covering soft-budget overshoot (the budget is checked
// between authorities, not preemptive) plus container cold-start before the
// lease is even acquired (GH#938 PR1's "honest TTL").
func leaseTTLFor(handlerBudget time.Duration) time.Duration {
	return handlerBudget + 5*time.Minute
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

	// Wrapped in InstrumentedSender so every digest email gets exactly one
	// "Email send" span (tagged email.kind) distinct from the underlying "ACS
	// email send" HTTP client spans (tc-3jx8d).
	emailSender := acsemail.NewInstrumentedSender(buildEmailSender(cfg, logger))
	dispatcher := buildPlatformDispatcher(cfg, logger)

	return digest.NewHandler(
		profileStore,
		st.notification,
		st.zone,
		st.notifState,
		st.device,
		emailSender,
		dispatcher,
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

// buildFCMSender returns the real FCM sender when FCM is enabled, else a NoOp so
// a job without a service-account key boots cleanly (the mirror of
// buildPushSender for Android delivery).
func buildFCMSender(cfg platform.Config, logger *slog.Logger) fcm.PushSender {
	if !cfg.FCMEnabled {
		logger.Info("fcm disabled; android pushes disabled (NoOp sender)")
		return fcm.NewNoOpSender()
	}
	client, err := fcm.NewClient(fcm.Options{
		Enabled:            cfg.FCMEnabled,
		ProjectID:          cfg.FCMProjectID,
		ServiceAccountJSON: cfg.FCMServiceAccountJSON.Expose(),
	}, logger, time.Now)
	if err != nil {
		logger.Error("build fcm client; falling back to NoOp sender", "error", err)
		return fcm.NewNoOpSender()
	}
	logger.Info("fcm enabled", "projectId", cfg.FCMProjectID)
	return client
}

// buildPlatformDispatcher wires the platform-aware push dispatcher over the APNs
// (iOS) and FCM (Android) senders. Both send sites — the poll-cycle coalescer
// and the weekly-digest handler — swap their single push sender for this one
// dispatcher, which splits a recipient's tokens by platform and prunes the union
// of tokens either sender reports invalid.
func buildPlatformDispatcher(cfg platform.Config, logger *slog.Logger) *notifydispatch.PlatformDispatcher {
	return notifydispatch.NewPlatformDispatcher(
		buildPushSender(cfg, logger),
		buildFCMSender(cfg, logger),
		logger,
	)
}
