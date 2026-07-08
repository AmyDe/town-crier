package platform

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// defaultCorsOrigin is the fallback when CORS_ALLOWED_ORIGINS is unset.
const defaultCorsOrigin = "http://localhost:5173"

// defaultPostcodesIoBaseURL and defaultGovUkBaseURL are the live UK services the
// geocode and designation clients call. The defaults are the real endpoints,
// so geocoding works without any infra wiring.
const (
	defaultPostcodesIoBaseURL = "https://api.postcodes.io/"
	defaultGovUkBaseURL       = "https://www.planning.data.gov.uk/"
)

// Config holds process configuration loaded from environment variables.
// Container Apps provides env vars; load them, validate them, fail fast at
// startup.
type Config struct {
	Port     string
	LogLevel slog.Level

	// Auth0Domain and Auth0Audience configure JWT validation. They are absent
	// on the dev Go app until infra wires them (GH#418, it3+); an empty value is
	// valid and yields a deny-all validator rather than a startup failure.
	Auth0Domain   string
	Auth0Audience string

	// CorsAllowedOrigins is the set of origins the CORS middleware echoes.
	// Defaults to localhost dev origin.
	CorsAllowedOrigins []string

	// AnonRateLimitRequests and AnonRateLimitWindowSeconds configure the
	// per-IP anonymous rate limiter (middleware.AnonRateLimit, GH#868 Phase 1):
	// the request budget and fixed window applied to every unauthenticated
	// request, keyed on the client IP resolved via internal/clientip. Loaded
	// from ANON_RATE_LIMIT_REQUESTS / ANON_RATE_LIMIT_WINDOW_SECONDS,
	// defaulting to 60 requests per 60-second window so an unset env never
	// leaves anonymous routes (a scraping target for a public geo endpoint,
	// and load that ultimately lands on PlanIt) unmetered.
	AnonRateLimitRequests      int
	AnonRateLimitWindowSeconds int

	// AzureClientID pins the user-assigned managed identity used for AAD auth
	// (azidentity) by the Postgres pool (passwordless Entra token) and the Service
	// Bus client (tc-6ig5). Empty falls back to the ambient managed identity.
	AzureClientID string

	// Auth0M2MClientID / Auth0M2MClientSecret are the machine-to-machine
	// client-credentials used to sync subscription tier and delete users in the
	// Auth0 Management API. When any of these (or Auth0Domain) is absent, the
	// Auth0 client falls back to a no-op.
	Auth0M2MClientID     string
	Auth0M2MClientSecret SecretString

	// ServiceBusNamespace and ServiceBusQueueName address the Azure Service Bus
	// poll-trigger queue the worker probes and seeds (WORKER_MODE=poll-bootstrap
	// and poll-sb). ServiceBusNamespace is the fully-qualified namespace
	// (e.g. sb-town-crier-prod.servicebus.windows.net); ServiceBusQueueName is the
	// trigger queue name. Authentication is the pinned user-assigned managed
	// identity (AzureClientID) — no SAS / connection string, mirroring the Postgres
	// identity model. Both are empty on jobs that don't touch Service Bus (digest,
	// hourly-digest, dormant-cleanup), in which case the Service Bus client is not
	// constructed and the poll modes refuse to run rather than crash. The infra
	// bead tc-uzm1 wires these env vars additively onto the prod poll jobs.
	ServiceBusNamespace string
	ServiceBusQueueName string

	// ShareCardsBlobURL is the Azure Blob account URL the baked share-card PNGs
	// are cached in (the share-cards container, #738 Slice 3 / ADR 0037), e.g.
	// https://sttowncrierdev.blob.core.windows.net. Loaded from
	// SHARE_CARDS_BLOB_URL (the exact name infra emits). Empty means the cache is
	// unwired — mirroring the ServiceBusNamespace empty-means-off convention — so
	// the API boots normally and the og:image handler regenerates on demand.
	// Authentication is the pinned user-assigned managed identity (AzureClientID).
	ShareCardsBlobURL string

	// PostcodesIoBaseURL and GovUkBaseURL address the outbound geocode and
	// designation upstreams. They default to the live UK services, so the clients
	// work without any env wiring; an override points them at a stub.
	PostcodesIoBaseURL string
	GovUkBaseURL       string

	// AdminAPIKey gates the /v1/admin/* endpoints (the X-Admin-Key header).
	// Empty means the admin endpoints reject every request, so an unconfigured
	// deployment exposes no admin surface.
	AdminAPIKey string

	// SiteBuildKey gates the build-time SEO endpoint
	// GET /v1/authorities/{id}/applications (the X-Build-Key header). It is a
	// dedicated, least-privilege key, distinct from AdminAPIKey: the SEO endpoint
	// reads only public planning data, never user PII or subscriptions. Empty
	// means the endpoint rejects every request, so an unconfigured deployment
	// exposes no SEO surface.
	SiteBuildKey string

	// AppleBundleID is the App Store bundle id a verified StoreKit transaction
	// must carry (the /v1/subscriptions/verify bundle check). It defaults to the
	// canonical uk.towncrierapp.mobile — the value the iOS app is built under and
	// App Store Connect issues transactions for. The legacy API defaulted to
	// the wrong uk.co.towncrier.ios; that bug is not carried over (tc-7g3i.12).
	AppleBundleID string

	// AppleEnvironments is the allowlist of StoreKit transaction environments
	// (e.g. "Production", "Sandbox") that the subscription-verify and webhook
	// paths accept. Loaded from APPLE_ENVIRONMENT (comma-separated, whitespace-
	// tolerant). Defaults to ["Production"] — fail-safe so an unconfigured
	// production deployment never accepts a free sandbox transaction. A dev/
	// TestFlight deployment sets APPLE_ENVIRONMENT=Sandbox,Production so both
	// real-money (Production) and TestFlight (Sandbox) purchases work during the
	// testing phase. Matching is case-insensitive at use-time.
	AppleEnvironments []string

	// APNs* configure the direct APNs HTTP/2 push client the digest worker modes
	// use to deliver instant and digest alerts (epic tc-wad3, enabler tc-qlqn).
	// APNsEnabled gates whether the real sender is constructed; when false the
	// worker wires a NoOp sender so a job without a .p8 auth key boots cleanly.
	// APNsAuthKey is the PEM contents of the .p8 auth key (the apns-auth-key
	// secret, ADR 0026); APNsKeyID and APNsTeamID default to Apple's issued
	// identifiers; APNsBundleID is sent as the apns-topic header; APNsUseSandbox
	// routes to the APNs sandbox for TestFlight/dev builds. The infra bead tc-uzm1
	// wires these env vars additively onto the digest jobs.
	APNsEnabled    bool
	APNsAuthKey    SecretString
	APNsKeyID      string
	APNsTeamID     string
	APNsBundleID   string
	APNsUseSandbox bool

	// FCM* configure the direct FCM HTTP v1 push client the worker modes use to
	// deliver instant and digest alerts to Android devices (GH#780), the mirror of
	// the APNs client for iOS. FCMEnabled gates whether the real sender is
	// constructed; when false the worker wires a NoOp sender so a job without a
	// service-account key boots cleanly. FCMProjectID is the Firebase/GCP project
	// id the send URL targets (/v1/projects/{id}/messages:send). FCMServiceAccountJSON
	// is the full service-account key JSON blob (the fcm-service-account secret),
	// carrying the RSA private key, client email, and token URI the JWT-bearer OAuth
	// exchange uses — the mirror of how APNsAuthKey carries the .p8. FCM has no
	// sandbox concept (dev and prod share one Firebase project), so there is no
	// FCM_USE_SANDBOX. A separate infra bead wires these env vars additively onto
	// the push-sending worker jobs (poll-sb, digest, hourly-digest).
	FCMEnabled            bool
	FCMProjectID          string
	FCMServiceAccountJSON SecretString

	// ACSConnectionString is the Azure Communication Services connection string
	// (endpoint=...;accesskey=...) the digest worker modes use to send email via
	// the ACS Email REST client (epic tc-wad3, enabler tc-qyf5). It carries the
	// HMAC-SHA256 access key, so it is a SecretString. Empty means the worker
	// wires a NoOp email sender. The infra bead tc-uzm1 wires the
	// acs-connection-string secret to this env var.
	ACSConnectionString SecretString

	// PlanIt* configure the rate-limited PlanIt HTTP client the poll-sb worker
	// mode uses to fetch planning applications (epic tc-wad3, bead tc-yng2).
	// PlanItBaseURL defaults to the live PlanIt service. The throttle/retry knobs
	// use the planit package defaults. The infra bead tc-uzm1 wires these env
	// vars additively onto the prod poll job.
	PlanItBaseURL                 string
	PlanItThrottleDelaySeconds    float64
	PlanItMaxRetries              int
	PlanItInitialBackoffSeconds   float64
	PlanItRateLimitBackoffSeconds float64

	// Polling* configure the poll-sb ingestion cycle (bead tc-yng2).
	// PollingMaxPagesPerAuthorityPerCycle caps PlanIt pagination per authority
	// (default 3). PollingHandlerBudgetSeconds is the soft per-cycle wall-clock
	// budget (default 240); under ADR 0024's receive-and-delete model it is a
	// safety cap, not a Service-Bus-lock bound — the lease TTL (> handler
	// budget) prevents concurrent runs. PollReplicaTimeoutSeconds and
	// PollShutdownGraceSeconds size the hard cycle budget (replicaTimeout − grace).
	PollingMaxPagesPerAuthorityPerCycle int
	PollingHandlerBudgetSeconds         int
	PollReplicaTimeoutSeconds           int
	PollShutdownGraceSeconds            int

	// NotificationsRetentionDays is the number of days to keep Notifications rows
	// when running the pg-purge job. Loaded from NOTIFICATIONS_RETENTION_DAYS;
	// defaults to 90.
	NotificationsRetentionDays int

	// DeviceRegistrationsRetentionDays is the number of days to keep DeviceRegistrations
	// rows when running the pg-purge job. Loaded from
	// DEVICE_REGISTRATIONS_RETENTION_DAYS; defaults to 180.
	DeviceRegistrationsRetentionDays int

	// PostgresHost and PostgresSSLMode are the discrete connection parameters
	// for the shared Azure Postgres Flexible Server (POSTGRES_HOST /
	// POSTGRES_SSLMODE), the same env vars internal/platform/postgres's
	// NewPoolFromEnv already reads directly for the process's primary pool.
	// They are threaded through Config too because the dev-seed job
	// (WORKER_MODE=dev-seed, epic tc-grvu, GH#808) opens a SECOND pool, bound
	// to town_crier_prod under a distinct read-only role, on the same
	// physical host — cmd/worker/main.go's buildDevSeeder needs the host/SSL
	// mode to build that pool's ConnParams.
	PostgresHost    string
	PostgresSSLMode string

	// DevSeed* configure the hourly dev-seed job (WORKER_MODE=dev-seed, epic
	// tc-grvu, GH#808), which mirrors a small slice of recently-changed prod
	// planning applications into dev so a TestFlight build pointed at dev gets
	// real push notifications to test against (dev otherwise runs no PlanIt
	// poller, ADR 0024). DevSeedLimit caps how many prod applications are
	// pulled per cycle (DEV_SEED_LIMIT, default 5). DevSeedProdPostgresDB is
	// the prod database name the second pool connects to
	// (DEV_SEED_PROD_POSTGRES_DB, default town_crier_prod).
	// DevSeedProdPostgresUser is the dedicated least-privilege Postgres role
	// (DEV_SEED_PROD_POSTGRES_USER, e.g. towncrier_dev_seed_reader, bootstrapped
	// out-of-band by cmd/pgbootstrap -readonly). DevSeedProdAzureClientID pins
	// the dedicated id-town-crier-dev-seed-reader managed identity
	// (DEV_SEED_PROD_AZURE_CLIENT_ID, infra bead tc-grvu.1) used to mint that
	// role's Entra token — a separate identity from AzureClientID, which stays
	// scoped to the process's own (dev) pool. DevSeedProdPostgresUser and
	// DevSeedProdAzureClientID have no default: both empty is the "job
	// unconfigured" signal cmd/worker/main.go's buildDevSeeder checks so the
	// mode refuses to run rather than nil-panicking on a job/environment (e.g.
	// prod) that never wires this config — this mode is created dev-only,
	// tc-grvu.6.
	DevSeedLimit             int
	DevSeedProdPostgresDB    string
	DevSeedProdPostgresUser  string
	DevSeedProdAzureClientID string
}

// defaultDevSeedProdPostgresDB is the prod database name the dev-seed job's
// second, read-only pool connects to.
const defaultDevSeedProdPostgresDB = "town_crier_prod"

// defaultPlanItBaseURL is the live PlanIt applications API.
const defaultPlanItBaseURL = "https://www.planit.org.uk/"

// defaultAPNsKeyID and defaultAPNsTeamID are the identifiers Apple issued for
// the Town Crier app's .p8 auth key (epic tc-wad3 notes). Defaulting them keeps
// the digest jobs working with only the auth-key secret wired.
const (
	defaultAPNsKeyID  = "L2J5PQASN5"
	defaultAPNsTeamID = "4574VQ7N2X"
)

// defaultAppleBundleID is the canonical App Store bundle id (uk.towncrierapp.mobile),
// matching the iOS app and the uk.towncrierapp.* product ids.
const defaultAppleBundleID = "uk.towncrierapp.mobile"

// Auth0M2MConfigured reports whether the Auth0 Management (M2M) client can be
// constructed: the domain, client id, and client secret must all be present.
// When false the API uses a no-op Auth0 client.
func (c Config) Auth0M2MConfigured() bool {
	return c.Auth0Domain != "" &&
		c.Auth0M2MClientID != "" &&
		c.Auth0M2MClientSecret.Expose() != ""
}

// LoadConfig reads configuration from the environment, applying defaults
// where a variable is unset.
func LoadConfig() (Config, error) {
	cfg := Config{
		Port:               getenv("PORT", "8080"),
		LogLevel:           slog.LevelInfo,
		Auth0Domain:        os.Getenv("AUTH0_DOMAIN"),
		Auth0Audience:      os.Getenv("AUTH0_AUDIENCE"),
		CorsAllowedOrigins: parseOrigins(os.Getenv("CORS_ALLOWED_ORIGINS")),

		AnonRateLimitRequests:      getenvInt("ANON_RATE_LIMIT_REQUESTS", 60),
		AnonRateLimitWindowSeconds: getenvInt("ANON_RATE_LIMIT_WINDOW_SECONDS", 60),

		AzureClientID: os.Getenv("AZURE_CLIENT_ID"),

		ServiceBusNamespace: os.Getenv("SERVICE_BUS_NAMESPACE"),
		ServiceBusQueueName: os.Getenv("SERVICE_BUS_QUEUE_NAME"),

		ShareCardsBlobURL: os.Getenv("SHARE_CARDS_BLOB_URL"),

		Auth0M2MClientID:     os.Getenv("AUTH0_M2M_CLIENT_ID"),
		Auth0M2MClientSecret: NewSecret(os.Getenv("AUTH0_M2M_CLIENT_SECRET")),

		PostcodesIoBaseURL: getenv("POSTCODES_IO_BASE_URL", defaultPostcodesIoBaseURL),
		GovUkBaseURL:       getenv("GOVUK_PLANNING_DATA_BASE_URL", defaultGovUkBaseURL),

		AdminAPIKey:  os.Getenv("ADMIN_API_KEY"),
		SiteBuildKey: os.Getenv("SITE_BUILD_KEY"),

		AppleBundleID:     getenv("APPLE_BUNDLE_ID", defaultAppleBundleID),
		AppleEnvironments: parseAppleEnvironments(os.Getenv("APPLE_ENVIRONMENT")),

		APNsEnabled:    getenvBool("APNS_ENABLED"),
		APNsAuthKey:    NewSecret(os.Getenv("APNS_AUTH_KEY")),
		APNsKeyID:      getenv("APNS_KEY_ID", defaultAPNsKeyID),
		APNsTeamID:     getenv("APNS_TEAM_ID", defaultAPNsTeamID),
		APNsBundleID:   getenv("APNS_BUNDLE_ID", defaultAppleBundleID),
		APNsUseSandbox: getenvBool("APNS_USE_SANDBOX"),

		FCMEnabled:            getenvBool("FCM_ENABLED"),
		FCMProjectID:          os.Getenv("FCM_PROJECT_ID"),
		FCMServiceAccountJSON: NewSecret(os.Getenv("FCM_SERVICE_ACCOUNT_JSON")),

		ACSConnectionString: NewSecret(os.Getenv("ACS_CONNECTION_STRING")),

		PlanItBaseURL:                 getenv("PLANIT_BASE_URL", defaultPlanItBaseURL),
		PlanItThrottleDelaySeconds:    getenvFloat("PLANIT_THROTTLE_DELAY_SECONDS", 2),
		PlanItMaxRetries:              getenvInt("PLANIT_RETRY_MAX_RETRIES", 3),
		PlanItInitialBackoffSeconds:   getenvFloat("PLANIT_RETRY_INITIAL_BACKOFF_SECONDS", 1),
		PlanItRateLimitBackoffSeconds: getenvFloat("PLANIT_RETRY_RATE_LIMIT_BACKOFF_SECONDS", 5),

		PollingMaxPagesPerAuthorityPerCycle: getenvInt("POLLING_MAX_PAGES_PER_AUTHORITY_PER_CYCLE", 3),
		PollingHandlerBudgetSeconds:         getenvInt("POLLING_HANDLER_BUDGET_SECONDS", 240),
		PollReplicaTimeoutSeconds:           getenvInt("POLL_REPLICA_TIMEOUT_SECONDS", 600),
		PollShutdownGraceSeconds:            getenvInt("POLL_SHUTDOWN_GRACE_SECONDS", 30),

		NotificationsRetentionDays:       getenvInt("NOTIFICATIONS_RETENTION_DAYS", 90),
		DeviceRegistrationsRetentionDays: getenvInt("DEVICE_REGISTRATIONS_RETENTION_DAYS", 180),

		PostgresHost:    os.Getenv("POSTGRES_HOST"),
		PostgresSSLMode: os.Getenv("POSTGRES_SSLMODE"),

		DevSeedLimit:             getenvInt("DEV_SEED_LIMIT", 5),
		DevSeedProdPostgresDB:    getenv("DEV_SEED_PROD_POSTGRES_DB", defaultDevSeedProdPostgresDB),
		DevSeedProdPostgresUser:  os.Getenv("DEV_SEED_PROD_POSTGRES_USER"),
		DevSeedProdAzureClientID: os.Getenv("DEV_SEED_PROD_AZURE_CLIENT_ID"),
	}

	if raw := os.Getenv("LOG_LEVEL"); raw != "" {
		var level slog.Level
		if err := level.UnmarshalText([]byte(raw)); err != nil {
			return Config{}, fmt.Errorf("parse LOG_LEVEL %q: %w", raw, err)
		}
		cfg.LogLevel = level
	}

	return cfg, nil
}

// parseOrigins splits a comma-separated origins list, trims whitespace, and
// drops empty entries. An empty or all-empty input falls back to the dev
// default when no origins are configured.
func parseOrigins(raw string) []string {
	origins := make([]string, 0, 1)
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	if len(origins) == 0 {
		return []string{defaultCorsOrigin}
	}
	return origins
}

// defaultAppleEnvironment is the production environment value. A deployment
// that does not configure APPLE_ENVIRONMENT rejects sandbox transactions,
// which is the safe default for production.
const defaultAppleEnvironment = "Production"

// parseAppleEnvironments splits a comma-separated list of Apple StoreKit
// environment names, trims whitespace, and drops empty entries. An empty or
// all-empty input returns [defaultAppleEnvironment], so an unconfigured
// production deployment rejects sandbox transactions by default.
// Values are stored as-is; callers compare case-insensitively.
func parseAppleEnvironments(raw string) []string {
	envs := make([]string, 0, 1)
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			envs = append(envs, trimmed)
		}
	}
	if len(envs) == 0 {
		return []string{defaultAppleEnvironment}
	}
	return envs
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getenvBool reports whether the named env var holds a truthy value. An unset,
// empty, or unparseable value is false, so a misconfigured flag fails safe
// (e.g. APNS_ENABLED defaults to off).
func getenvBool(key string) bool {
	v, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(key)))
	if err != nil {
		return false
	}
	return v
}

// getenvInt returns the named env var parsed as an int, or fallback when unset,
// empty, or unparseable — so a misconfigured value fails safe to the default.
func getenvInt(key string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil {
		return fallback
	}
	return v
}

// getenvFloat returns the named env var parsed as a float64, or fallback when
// unset, empty, or unparseable.
func getenvFloat(key string, fallback float64) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(os.Getenv(key)), 64)
	if err != nil {
		return fallback
	}
	return v
}
