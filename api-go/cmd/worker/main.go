// Command worker runs the Town Crier background-worker modes as short-lived
// Container Apps Jobs. One process per job: WORKER_MODE selects the mode,
// the process runs it once, flushes telemetry, and exits with a status code.
//
// poll, digest, hourly-digest, dormant-cleanup, subscription-sweep, pg-purge
// and appstore-reconcile are implemented. Every store is backed by
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
	"github.com/AmyDe/town-crier/api-go/internal/appstorereconcile"
	"github.com/AmyDe/town-crier/api-go/internal/appstoreserverapi"
	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
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
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres"
	"github.com/AmyDe/town-crier/api-go/internal/polling"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/savedapplications"
	"github.com/AmyDe/town-crier/api-go/internal/subscriptions"
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
	lease        *polling.PostgresLeaseStore
	appleNotif   *subscriptions.PostgresNotificationStore
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
	// It is threaded through the builders below so the PlanIt client and the
	// notification dispatchers emit their towncrier.*
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
		lease:        polling.NewPostgresLeaseStore(pool, time.Now),
		appleNotif:   subscriptions.NewPostgresNotificationStore(pool, time.Now),
	}

	// The digest, dormant-cleanup, subscription-sweep and pg-purge handlers build
	// unconditionally now Postgres is always present. The email and push senders
	// fall back to NoOp when their credentials are absent, so a job without
	// ACS/APNs config still boots cleanly.
	digester := buildDigester(cfg, registry, st, logger)
	dormantRunner := buildDormant(cfg, st, logger)
	sweepRunner := buildSweep(cfg, st, logger)

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

	// The poll runner is built only for WORKER_MODE=poll: it needs PlanIt config
	// and validates the poll settings, so a malformed value fails only that job.
	// Declared as the interface so the unbuilt case stays a genuinely nil value.
	var poller worker.PollRunner
	if mode == "poll" {
		adapter, err := buildPoller(cfg, pool, registry, st, logger)
		if err != nil {
			logger.Error("build poller", "error", err)
			return 1
		}
		poller = adapter
	}

	// The appstore-reconcile runner is built only when APPSTORE_RECONCILE_ENABLED
	// is set and its App Store Server API key material parses. A job missing
	// either leaves reconciler a genuinely nil interface; appstore-reconcile then
	// logs and exits 0 rather than crash-looping — this feature is genuinely
	// optional during rollout (tc-97k35.6, GH#1011).
	var reconciler worker.AppStoreReconcileRunner
	if r := buildAppStoreReconcile(cfg, st, logger); r != nil {
		reconciler = r
	}

	return worker.Run(context.Background(), mode, digester, dormantRunner, poller, sweepRunner, purger, reconciler, logger)
}

// buildNotifyFanOut constructs the decision-dispatch, zone-enqueue and
// push-coalescer collaborators the poll runner's event dispatcher needs.
// registry is wired onto the APNs/FCM push senders; WithMetrics on the
// enqueuer and decision dispatcher is left to the caller. st may be nil in
// tests: the store fields are extracted under a nil guard.
func buildNotifyFanOut(cfg platform.Config, registry *metrics.Registry, zoneStore watchzones.Store, st *stores, logger *slog.Logger) (*notifydispatch.DecisionDispatcher, *notifydispatch.Enqueuer, *notifydispatch.PushCoalescer) {
	var (
		notifStore     *notifications.PostgresStore
		profileStore   *profiles.PostgresStore
		deviceStore    *devicetokens.PostgresStore
		statePushStore *notificationstate.PostgresStore
		savedStore     *savedapplications.PostgresStore
		appStore       *applications.PostgresStore
	)
	if st != nil {
		notifStore = st.notification
		profileStore = st.profile
		deviceStore = st.device
		statePushStore = st.notifState
		savedStore = st.savedApp
		appStore = st.app
	}

	pushDispatcher := buildPlatformDispatcher(cfg, registry, logger)
	coalescer := notifydispatch.NewPushCoalescer(deviceStore, statePushStore, pushDispatcher, zoneStore, logger)
	enqueuer := notifydispatch.NewEnqueuer(
		notifStore, zoneStore, profileStore, coalescer, appStore,
		uuid.NewString, time.Now, logger,
	)
	dispatcher := notifydispatch.NewDecisionDispatcher(
		notifStore, zoneStore, savedStore, profileStore, coalescer,
		uuid.NewString, time.Now, logger,
	)
	return dispatcher, enqueuer, coalescer
}

// buildDigester constructs the digest handler, wiring the per-feature Postgres
// stores and the email/push senders (real when their credentials are present,
// NoOp otherwise so a job without ACS/APNs boots cleanly).
func buildDigester(cfg platform.Config, registry *metrics.Registry, st *stores, logger *slog.Logger) *digest.Handler {
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
	dispatcher := buildPlatformDispatcher(cfg, registry, logger)

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

// buildAppStoreReconcile constructs the appstore-reconcile handler: the App
// Store Server API client (JWT-authed via the .p8 signing key), the same
// subscriptions.JWSVerifier and subscriptions.NotificationProcessor the live
// POST /v1/webhooks/appstore handler uses (so a recovered notification can
// never double-apply against a concurrent live delivery), and the Postgres
// idempotency store built into st.appleNotif.
//
// It returns nil — logged, not fatal — when APPSTORE_RECONCILE_ENABLED is
// unset or the key material fails to parse, following the same
// "unconfigured optional job" posture: this feature is
// genuinely optional during rollout (tc-97k35.6, GH#1011), so a malformed or
// absent key must not crash-loop the job's OTHER modes (digest,
// dormant-cleanup, etc.) that share this process.
func buildAppStoreReconcile(cfg platform.Config, st *stores, logger *slog.Logger) *appstorereconcile.Handler {
	if !cfg.AppStoreReconcileEnabled {
		logger.Info("appstore-reconcile unconfigured (APPSTORE_RECONCILE_ENABLED unset); appstore-reconcile mode will refuse to run")
		return nil
	}

	appleRoots, err := subscriptions.LoadAppleRootCertificates()
	if err != nil {
		logger.Error("appstore-reconcile: load apple root certificates; appstore-reconcile mode will refuse to run", "error", err)
		return nil
	}
	jwsVerifier, err := subscriptions.NewJWSVerifier(appleRoots, time.Now)
	if err != nil {
		logger.Error("appstore-reconcile: build jws verifier; appstore-reconcile mode will refuse to run", "error", err)
		return nil
	}

	apiClient, err := appstoreserverapi.NewClient(appstoreserverapi.Options{
		SigningKey:  cfg.AppStoreServerAPIKey.Expose(),
		KeyID:       cfg.AppStoreServerAPIKeyID,
		IssuerID:    cfg.AppStoreServerAPIIssuerID,
		BundleID:    cfg.AppleBundleID,
		Environment: cfg.AppStoreServerAPIEnvironment,
	}, time.Now)
	if err != nil {
		logger.Error("appstore-reconcile: build app store server api client; appstore-reconcile mode will refuse to run", "error", err)
		return nil
	}

	// adminStore backs profileByTxn (GetByOriginalTransactionID + Save), the
	// same store the live webhook path uses to locate a subscriber
	// cross-partition.
	adminStore := st.profileAdmin
	processor := subscriptions.NewNotificationProcessor(
		jwsVerifier,
		adminStore,
		buildAuth0Syncer(cfg, logger),
		st.appleNotif,
		cfg.AppleEnvironments,
		logger,
	)

	lookback := time.Duration(cfg.AppStoreReconcileLookbackHours) * time.Hour
	return appstorereconcile.New(apiClient, jwsVerifier, st.appleNotif, processor, lookback, cfg.AppStoreReconcileApplyEnabled, logger, time.Now)
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
// so a job without a .p8 auth key boots cleanly. registry wires
// towncrier.push.delivery_failed (tc-97k35.4) onto the real client; a nil
// registry is a safe no-op (matches every other WithMetrics call site).
func buildPushSender(cfg platform.Config, registry *metrics.Registry, logger *slog.Logger) apns.PushSender {
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
	return client.WithMetrics(registry)
}

// buildFCMSender returns the real FCM sender when FCM is enabled, else a NoOp so
// a job without a service-account key boots cleanly (the mirror of
// buildPushSender for Android delivery). registry wires
// towncrier.push.delivery_failed (tc-97k35.4) onto the real client.
func buildFCMSender(cfg platform.Config, registry *metrics.Registry, logger *slog.Logger) fcm.PushSender {
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
	return client.WithMetrics(registry)
}

// buildPlatformDispatcher wires the platform-aware push dispatcher over the APNs
// (iOS) and FCM (Android) senders. Both send sites — the poll-cycle coalescer
// and the weekly-digest handler — swap their single push sender for this one
// dispatcher, which splits a recipient's tokens by platform and prunes the union
// of tokens either sender reports invalid.
func buildPlatformDispatcher(cfg platform.Config, registry *metrics.Registry, logger *slog.Logger) *notifydispatch.PlatformDispatcher {
	return notifydispatch.NewPlatformDispatcher(
		buildPushSender(cfg, registry, logger),
		buildFCMSender(cfg, registry, logger),
		logger,
	)
}
