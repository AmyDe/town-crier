// Command worker runs the Town Crier background-worker modes as short-lived
// Container Apps Jobs. One process per job: WORKER_MODE selects the mode,
// the process runs it once, flushes telemetry, and exits with a status code.
//
// Every store is backed by Postgres + PostGIS; the shared pool is built once at
// boot and a pool failure is fatal.
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
	"github.com/AmyDe/town-crier/api-go/internal/subscriptionevents"
	"github.com/AmyDe/town-crier/api-go/internal/subscriptions"
	"github.com/AmyDe/town-crier/api-go/internal/subscriptionsweep"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
	"github.com/AmyDe/town-crier/api-go/internal/worker"
	"go.opentelemetry.io/otel"
)

// stores holds every Postgres store the worker's modes use, built once from the
// shared pool at boot.
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

// shutdownContext returns a context cancelled by SIGINT or SIGTERM, so a
// Container Apps stop ends the running mode and its deferred cleanup runs.
func shutdownContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
}

// run is main's body split out so its defers run before os.Exit. It returns
// the process exit code.
func run() int {
	ctx, stop := shutdownContext(context.Background())
	defer stop()

	cfg, err := platform.LoadConfig()
	if err != nil {
		log.Print(err)
		return 1
	}

	// The bridge targets the global LoggerProvider, which SetupTelemetry
	// replaces next; the global provider upgrades this logger in place.
	logger := platform.NewOTelLogger(os.Stdout, cfg.LogLevel)

	mode := os.Getenv("WORKER_MODE")

	shutdownTelemetry, err := platform.SetupTelemetry(context.Background(), logger)
	if err != nil {
		log.Print(err)
		return 1
	}
	// The flush uses its own context so it still runs after a stop signal.
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			logger.Error("telemetry shutdown", "error", err)
		}
	}()

	registry := metrics.New(otel.Meter(metrics.MeterName))

	pool, perr := postgres.NewPoolFromEnv(ctx, poolOptions(mode)...)
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

	digester := buildDigester(cfg, registry, st, logger)
	dormantRunner := buildDormant(cfg, st, logger)
	sweepRunner := buildSweep(cfg, st, logger)

	purger := pgpurge.New(
		st.notification,
		st.device,
		time.Duration(cfg.NotificationsRetentionDays)*24*time.Hour,
		time.Duration(cfg.DeviceRegistrationsRetentionDays)*24*time.Hour,
		time.Now,
		logger,
	).WithPollRetention(polling.NewPostgresRetentionStore(pool))

	// Built only for its own mode so a malformed poll setting fails only that
	// job. Declared as the interface so the unbuilt case stays a true nil.
	var poller worker.PollRunner
	if mode == "poll" {
		adapter, err := buildPoller(cfg, pool, registry, st, logger)
		if err != nil {
			logger.Error("build poller", "error", err)
			return 1
		}
		poller = adapter
	}

	var reconciler worker.AppStoreReconcileRunner
	if r := buildAppStoreReconcile(cfg, st, logger); r != nil {
		reconciler = r
	}

	// Built only for its own mode so a gazetteer problem cannot affect any
	// other job.
	var seoCataloger worker.SEOCatalogRunner
	if mode == "seo-catalog" {
		seo, err := buildSEO(pool, logger)
		if err != nil {
			logger.Error("build seo runners", "error", err)
			return 1
		}
		seoCataloger = newSEOCatalogRunner(seoCatalogRunner{seo}, platform.CurrentDeploymentEnvironment(), buildIndexNow(pool, logger))
	}

	// Built only for its own mode so missing backup config fails only that job.
	var backuper worker.BackupRunner
	if mode == "pg-backup" {
		b, err := buildBackup(os.Getenv, pool, logger)
		if err != nil {
			logger.Error("build backup", "error", err)
			return 1
		}
		backuper = b
	}

	return worker.Run(ctx, mode, digester, dormantRunner, poller, sweepRunner, purger, reconciler, seoCataloger, backuper, logger)
}

// buildNotifyFanOut returns the collaborators the poll runner's event
// dispatcher needs. WithMetrics on the enqueuer and decision dispatcher is left
// to the caller; st may be nil in tests.
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

// buildDigester wires the digest handler; senders fall back to NoOp when their
// credentials are absent so the job still boots.
func buildDigester(cfg platform.Config, registry *metrics.Registry, st *stores, logger *slog.Logger) *digest.Handler {
	profileStore := digestProfiles{
		admin: st.profileAdmin,
		point: st.profile,
	}

	// InstrumentedSender gives each digest email one "Email send" span, distinct
	// from the underlying "ACS email send" HTTP client spans.
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

// digestProfiles combines the cross-user digest-day selector and the per-user
// read into the digest handler's single profile interface.
type digestProfiles struct {
	admin profiles.AdminProfileStore
	point profiles.Store
}

func (p digestProfiles) ByDigestDay(ctx context.Context, day time.Weekday, notSentSince time.Time) ([]*profiles.UserProfile, error) {
	return p.admin.ByDigestDay(ctx, day, notSentSince)
}

func (p digestProfiles) MarkWeeklyDigestSent(ctx context.Context, userID string, at time.Time) error {
	return p.admin.MarkWeeklyDigestSent(ctx, userID, at)
}

func (p digestProfiles) Get(ctx context.Context, userID string) (*profiles.UserProfile, error) {
	return p.point.Get(ctx, userID)
}

func buildDormant(cfg platform.Config, st *stores, logger *slog.Logger) *dormant.Handler {
	// Every deleter must be the Postgres store: the GDPR cascade must not miss a row.
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

	return dormant.New(st.profileAdmin, deleters, logger, time.Now)
}

// buildAuth0Deleter returns a no-op when Auth0 M2M is unconfigured, so the
// stored data is still erased.
func buildAuth0Deleter(cfg platform.Config, logger *slog.Logger) erasure.Auth0Deleter {
	if !cfg.Auth0M2MConfigured() {
		logger.Info("auth0 m2m unconfigured; dormant cleanup will skip Auth0 user deletion (NoOp)")
		return profiles.NoOpAuth0Client{}
	}
	return newAuth0Client(cfg)
}

func buildSweep(cfg platform.Config, st *stores, logger *slog.Logger) sweepRunner {
	adminStore := st.profileAdmin
	webhooks, err := subscriptionevents.ParseWebhooks(cfg.SubscriptionWebhooks.Expose())
	if err != nil {
		logger.Error("subscription webhooks misconfigured; running with none", "error", err)
		webhooks = nil
	}
	notifier := subscriptionevents.NewNotifier(adminStore, webhooks, subscriptionevents.NewHTTPClient(), logger, time.Now)
	handler := subscriptionsweep.New(adminStore, adminStore, buildAuth0Syncer(cfg, logger), notifier, logger, time.Now)
	return sweepRunner{cycle: handler, notifier: notifier}
}

// buildAuth0Syncer returns a no-op when Auth0 M2M is unconfigured, so the stored
// tier is still reverted. The Auth0 tier metadata is informational: EffectiveTier
// already treats a lapsed user as Free.
func buildAuth0Syncer(cfg platform.Config, logger *slog.Logger) subscriptionsweep.Auth0Syncer {
	if !cfg.Auth0M2MConfigured() {
		logger.Info("auth0 m2m unconfigured; subscription sweep will skip Auth0 tier sync (NoOp)")
		return profiles.NoOpAuth0Client{}
	}
	return newAuth0Client(cfg)
}

// newAuth0Client builds the Auth0 Management (M2M) client. The static span name
// "Auth0 token" keeps cardinality low; the host lands in server.address.
func newAuth0Client(cfg platform.Config) *profiles.Auth0Client {
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

// buildAppStoreReconcile reuses the live webhook's JWSVerifier and
// NotificationProcessor so a recovered notification cannot double-apply against
// a concurrent live delivery. It returns nil, logged and not fatal, when the job
// is disabled or the key material fails to parse, so the other modes sharing
// this process cannot crash-loop.
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

	processor := subscriptions.NewNotificationProcessor(
		jwsVerifier,
		st.profileAdmin,
		buildAuth0Syncer(cfg, logger),
		st.appleNotif,
		cfg.AppleBundleID,
		cfg.AppleEnvironments,
		nil,
		logger,
	)

	lookback := time.Duration(cfg.AppStoreReconcileLookbackHours) * time.Hour
	return appstorereconcile.New(apiClient, jwsVerifier, st.appleNotif, processor, lookback, cfg.AppStoreReconcileApplyEnabled, logger, time.Now)
}

// buildEmailSender returns a NoOp when no ACS connection string is set.
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

// buildPushSender returns a NoOp when APNs is disabled. A nil registry is a safe no-op.
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

// buildFCMSender returns a NoOp when FCM is disabled.
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

// buildPlatformDispatcher splits a recipient's tokens by platform across the
// APNs and FCM senders and prunes tokens either reports invalid.
func buildPlatformDispatcher(cfg platform.Config, registry *metrics.Registry, logger *slog.Logger) *notifydispatch.PlatformDispatcher {
	return notifydispatch.NewPlatformDispatcher(
		buildPushSender(cfg, registry, logger),
		buildFCMSender(cfg, registry, logger),
		logger,
	)
}
