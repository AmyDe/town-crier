package platform

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// defaultCorsOrigin matches .NET's fallback when Cors:AllowedOrigins is unset
// (Program.cs: ?? ["http://localhost:5173"]).
const defaultCorsOrigin = "http://localhost:5173"

// defaultPostcodesIoBaseURL and defaultGovUkBaseURL are the live UK services the
// geocode and designation clients call, matching the .NET fallbacks for
// PostcodesIo:BaseUrl and GovUkPlanningData:BaseUrl. The defaults are the real
// endpoints, so the Go app geocodes against the same upstreams as .NET without
// any infra wiring.
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

	// CorsAllowedOrigins is the set of origins the CORS middleware echoes,
	// mirroring .NET's Cors:AllowedOrigins. Defaults to localhost dev origin.
	CorsAllowedOrigins []string

	// CosmosEndpoint and CosmosDatabase address the Cosmos account and database
	// the profiles store reads and writes. Equivalent to .NET's
	// Cosmos:AccountEndpoint / Cosmos:Database. AzureClientID pins the
	// user-assigned managed identity used for AAD auth (azidentity), matching
	// the .NET CosmosAuthProvider's AZURE_CLIENT_ID requirement (tc-6ig5). All
	// three are empty until infra wires them, in which case the Cosmos client is
	// not constructed and profile routes are unavailable.
	CosmosEndpoint string
	CosmosDatabase string
	AzureClientID  string

	// Auth0M2MClientID / Auth0M2MClientSecret are the machine-to-machine
	// client-credentials used to sync subscription tier and delete users in the
	// Auth0 Management API. When any of these (or Auth0Domain) is absent, the
	// Auth0 client falls back to a no-op, mirroring .NET's NoOpAuth0ManagementClient.
	Auth0M2MClientID     string
	Auth0M2MClientSecret SecretString

	// ServiceBusNamespace and ServiceBusQueueName address the Azure Service Bus
	// poll-trigger queue the worker probes and seeds (WORKER_MODE=poll-bootstrap
	// and poll-sb). ServiceBusNamespace is the fully-qualified namespace
	// (e.g. sb-town-crier-prod.servicebus.windows.net); ServiceBusQueueName is the
	// trigger queue name. Authentication is the pinned user-assigned managed
	// identity (AzureClientID) — no SAS / connection string, mirroring the Cosmos
	// identity model. Both are empty on jobs that don't touch Service Bus (digest,
	// hourly-digest, dormant-cleanup), in which case the Service Bus client is not
	// constructed and the poll modes refuse to run rather than crash. The infra
	// bead tc-uzm1 wires these env vars additively onto the prod poll jobs.
	ServiceBusNamespace string
	ServiceBusQueueName string

	// ProDomains is the comma-separated allow-list of email domains that
	// auto-grant the Pro tier on a verified-email registration. Mirrors .NET's
	// Subscription:AutoGrant:ProDomains. Empty disables auto-grant.
	ProDomains string

	// PostcodesIoBaseURL and GovUkBaseURL address the outbound geocode and
	// designation upstreams. They default to the live UK services (matching
	// .NET's PostcodesIo:BaseUrl / GovUkPlanningData:BaseUrl), so the clients work
	// without any env wiring; an override points them at a stub.
	PostcodesIoBaseURL string
	GovUkBaseURL       string

	// AdminAPIKey gates the /v1/admin/* endpoints (the X-Admin-Key header),
	// mirroring .NET's Admin:ApiKey. Empty means the admin endpoints reject every
	// request, so an unconfigured deployment exposes no admin surface.
	AdminAPIKey string

	// AppleBundleID is the App Store bundle id a verified StoreKit transaction
	// must carry (the /v1/subscriptions/verify bundle check). It defaults to the
	// canonical uk.towncrierapp.mobile — the value the iOS app is built under and
	// App Store Connect issues transactions for. NOTE: the retired .NET API
	// defaulted to the wrong uk.co.towncrier.ios; that bug is not carried over
	// (tc-7g3i.12).
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

	// ACSConnectionString is the Azure Communication Services connection string
	// (endpoint=...;accesskey=...) the digest worker modes use to send email via
	// the ACS Email REST client (epic tc-wad3, enabler tc-qyf5). It carries the
	// HMAC-SHA256 access key, so it is a SecretString. Empty means the worker
	// wires a NoOp email sender. The infra bead tc-uzm1 wires the
	// acs-connection-string secret to this env var.
	ACSConnectionString SecretString

	// PlanIt* configure the rate-limited PlanIt HTTP client the poll-sb worker
	// mode uses to fetch planning applications (epic tc-wad3, bead tc-yng2).
	// PlanItBaseURL defaults to the live PlanIt service (matching .NET's
	// PlanIt:BaseUrl fallback). The throttle/retry knobs mirror .NET's
	// PlanItThrottleOptions / PlanItRetryOptions defaults. The infra bead tc-uzm1
	// wires these env vars additively onto the prod poll job.
	PlanItBaseURL                 string
	PlanItThrottleDelaySeconds    float64
	PlanItMaxRetries              int
	PlanItInitialBackoffSeconds   float64
	PlanItRateLimitBackoffSeconds float64

	// Polling* configure the poll-sb ingestion cycle (bead tc-yng2).
	// PollingMaxPagesPerAuthorityPerCycle caps PlanIt pagination per authority
	// (default 3, matching .NET Polling:MaxPagesPerAuthorityPerCycle).
	// PollingHandlerBudgetSeconds is the soft per-cycle wall-clock budget (default
	// 240); under ADR 0024's receive-and-delete model it is a safety cap, not a
	// Service-Bus-lock bound — the Cosmos lease TTL (> handler budget) prevents
	// concurrent runs. PollReplicaTimeoutSeconds and PollShutdownGraceSeconds size
	// the hard cycle budget (replicaTimeout − grace), matching the .NET worker's
	// POLL_REPLICA_TIMEOUT_SECONDS / POLL_SHUTDOWN_GRACE_SECONDS.
	PollingMaxPagesPerAuthorityPerCycle int
	PollingHandlerBudgetSeconds         int
	PollReplicaTimeoutSeconds           int
	PollShutdownGraceSeconds            int
}

// defaultPlanItBaseURL is the live PlanIt applications API, matching .NET's
// PlanIt:BaseUrl fallback.
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
// When false the API uses a no-op Auth0 client, exactly as .NET registers
// NoOpAuth0ManagementClient when any of the three is absent.
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

		CosmosEndpoint: os.Getenv("COSMOS_ENDPOINT"),
		CosmosDatabase: os.Getenv("COSMOS_DATABASE"),
		AzureClientID:  os.Getenv("AZURE_CLIENT_ID"),

		ServiceBusNamespace: os.Getenv("SERVICE_BUS_NAMESPACE"),
		ServiceBusQueueName: os.Getenv("SERVICE_BUS_QUEUE_NAME"),

		Auth0M2MClientID:     os.Getenv("AUTH0_M2M_CLIENT_ID"),
		Auth0M2MClientSecret: NewSecret(os.Getenv("AUTH0_M2M_CLIENT_SECRET")),

		ProDomains: os.Getenv("SUBSCRIPTION_AUTOGRANT_PRODOMAINS"),

		PostcodesIoBaseURL: getenv("POSTCODES_IO_BASE_URL", defaultPostcodesIoBaseURL),
		GovUkBaseURL:       getenv("GOVUK_PLANNING_DATA_BASE_URL", defaultGovUkBaseURL),

		AdminAPIKey: os.Getenv("ADMIN_API_KEY"),

		AppleBundleID:     getenv("APPLE_BUNDLE_ID", defaultAppleBundleID),
		AppleEnvironments: parseAppleEnvironments(os.Getenv("APPLE_ENVIRONMENT")),

		APNsEnabled:    getenvBool("APNS_ENABLED"),
		APNsAuthKey:    NewSecret(os.Getenv("APNS_AUTH_KEY")),
		APNsKeyID:      getenv("APNS_KEY_ID", defaultAPNsKeyID),
		APNsTeamID:     getenv("APNS_TEAM_ID", defaultAPNsTeamID),
		APNsBundleID:   getenv("APNS_BUNDLE_ID", defaultAppleBundleID),
		APNsUseSandbox: getenvBool("APNS_USE_SANDBOX"),

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
// default, matching .NET's behaviour when no origins are configured.
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
