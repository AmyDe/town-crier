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

	// AnonRateLimitBurst and AnonRateLimitRefillPerMinute configure the per-IP
	// anonymous token-bucket rate limiter (middleware.AnonRateLimit, GH#868
	// Phase 1, reworked tc-vwobf), keyed on the client IP resolved via
	// internal/clientip: AnonRateLimitBurst is the bucket capacity (how many
	// requests an IP can make back-to-back before it starts drawing on the
	// steady refill rate), AnonRateLimitRefillPerMinute is that steady rate.
	// Loaded from ANON_RATE_LIMIT_BURST / ANON_RATE_LIMIT_REFILL_PER_MINUTE.
	//
	// tc-vwobf: the previous flat 60-requests-per-60-second sliding window
	// throttled ordinary anonymous map browsing — panning the map fans a
	// single pan/zoom settle event out across several endpoints
	// (GET /v1/applications/clusters, GET /v1/me/watch-zones/{id}/
	// applications/clusters, GET /v1/applications/near-point, ...) even
	// though both clients already debounce pan/zoom at ~250ms client-side, so
	// a real session hit 41+ 429s in 9 minutes with no scraping involved.
	//
	// Defaults: burst 120, refill 60/minute (1 token/second).
	//   - Burst=120 is double the old flat budget: generous enough to absorb
	//     a single active map session's fan-out — several endpoints firing
	//     roughly every 250ms while the user is actively panning — as one
	//     instantaneous burst, without needing to wait on the refill rate at
	//     all for a normal browsing session.
	//   - Refill=60/minute exactly matches the old default's long-run rate
	//     (60 requests / 60 seconds), so sustained/scraping-shaped load past
	//     the initial burst is capped at the same steady-state rate as
	//     before; only the front-loaded tolerance changed, not the ceiling on
	//     continuous abuse.
	AnonRateLimitBurst           int
	AnonRateLimitRefillPerMinute int

	// AzureClientID pins the user-assigned managed identity used for AAD auth
	// (azidentity) by the Postgres pool (passwordless Entra token). Empty falls
	// back to the ambient managed identity.
	AzureClientID string

	// Auth0M2MClientID / Auth0M2MClientSecret are the machine-to-machine
	// client-credentials used to sync subscription tier and delete users in the
	// Auth0 Management API. When any of these (or Auth0Domain) is absent, the
	// Auth0 client falls back to a no-op.
	Auth0M2MClientID     string
	Auth0M2MClientSecret SecretString

	// ShareCardsBlobURL is the Azure Blob account URL the baked share-card PNGs
	// are cached in (the share-cards container, #738 Slice 3 / ADR 0037), e.g.
	// https://sttowncrierdev.blob.core.windows.net. Loaded from
	// SHARE_CARDS_BLOB_URL (the exact name infra emits). Empty means the cache is
	// unwired — so the API boots normally and the og:image handler regenerates on demand.
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

	// AppStoreReconcile* configure the WORKER_MODE=appstore-reconcile job (GH
	// #1011, tc-97k35.6): a periodic poll of Apple's Get Notification History
	// API that detects (and, once soaked, recovers) a missed ASSN webhook
	// delivery. AppStoreReconcileEnabled gates whether the job's App Store
	// Server API client is constructed at all; when false (or when the key
	// material is missing/malformed) cmd/worker's builder returns nil and the
	// mode logs and exits 0 rather than crash-looping an optional,
	// still-rolling-out feature. AppStoreServerAPIKey is the PEM contents of
	// the .p8 signing key (the appstore-server-api-key secret);
	// AppStoreServerAPIKeyID and AppStoreServerAPIIssuerID are the matching
	// Apple-issued identifiers. AppStoreServerAPIEnvironment selects
	// "Sandbox" or "Production" — dev talks to Sandbox, prod talks to
	// Production, driven by config, not hardcoded. There is no separate
	// bundle-id config: the client reuses AppleBundleID above.
	// AppStoreReconcileApplyEnabled gates whether a detected gap is actually
	// replayed through the NotificationProcessor; it defaults to false
	// (observe-only) and this bead never flips it — a later, separate
	// decision does. AppStoreReconcileLookbackHours sizes the
	// [now-lookback, now) scan window each cycle (default 30, a few hours of
	// slack over a daily cadence).
	AppStoreReconcileEnabled       bool
	AppStoreServerAPIKey           SecretString
	AppStoreServerAPIKeyID         string
	AppStoreServerAPIIssuerID      string
	AppStoreServerAPIEnvironment   string
	AppStoreReconcileApplyEnabled  bool
	AppStoreReconcileLookbackHours int

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
	// the push-sending worker jobs (poll, digest, hourly-digest).
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

	// NotificationsRetentionDays is the number of days to keep Notifications rows
	// when running the pg-purge job. Loaded from NOTIFICATIONS_RETENTION_DAYS;
	// defaults to 90.
	NotificationsRetentionDays int

	// DeviceRegistrationsRetentionDays is the number of days to keep DeviceRegistrations
	// rows when running the pg-purge job. Loaded from
	// DEVICE_REGISTRATIONS_RETENTION_DAYS; defaults to 180.
	DeviceRegistrationsRetentionDays int

	// PlanItBaseURL is PLANIT_BASE_URL and defaults to the live PlanIt service.
	PlanItBaseURL string

	// PollingDailyCallCap is POLLING_DAILY_CALL_CAP, the PlanIt requests allowed
	// per budget day (prod 240, dev 60). PollingMinRequestSpacingSeconds is the
	// gap between requests. PollingDeltaSlots is the comma-separated HH:MM
	// Europe/London delta slots. PollingDayAllowance is the budget kept back for
	// the next day's deltas. PollingFullReadMaxAgeDays forces a full window read
	// after that many days. PollingRunBudgetMinutes bounds one hourly run.
	// PollingAreaID restricts every query to one PlanIt area (0 = all).
	PollingDailyCallCap             int
	PollingMinRequestSpacingSeconds int
	PollingDeltaSlots               string
	PollingDeltaMaxPages            int
	PollingDayAllowance             int
	PollingFullReadMaxAgeDays       int
	PollingRunBudgetMinutes         int
	PollingAreaID                   int
	PollingOracleEnabled            bool

	// NotifyQuietStart and NotifyQuietEnd are the Europe/London HH:MM quiet
	// hours; NotifyEventSurgeThreshold is the 24h event count above which the
	// dispatcher treats pending events as a surge.
	NotifyQuietStart          string
	NotifyQuietEnd            string
	NotifyEventSurgeThreshold int
}

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

		AnonRateLimitBurst:           getenvInt("ANON_RATE_LIMIT_BURST", 120),
		AnonRateLimitRefillPerMinute: getenvInt("ANON_RATE_LIMIT_REFILL_PER_MINUTE", 60),

		AzureClientID: os.Getenv("AZURE_CLIENT_ID"),

		ShareCardsBlobURL: os.Getenv("SHARE_CARDS_BLOB_URL"),

		Auth0M2MClientID:     os.Getenv("AUTH0_M2M_CLIENT_ID"),
		Auth0M2MClientSecret: NewSecret(os.Getenv("AUTH0_M2M_CLIENT_SECRET")),

		PostcodesIoBaseURL: getenv("POSTCODES_IO_BASE_URL", defaultPostcodesIoBaseURL),
		GovUkBaseURL:       getenv("GOVUK_PLANNING_DATA_BASE_URL", defaultGovUkBaseURL),

		AdminAPIKey:  os.Getenv("ADMIN_API_KEY"),
		SiteBuildKey: os.Getenv("SITE_BUILD_KEY"),

		AppleBundleID:     getenv("APPLE_BUNDLE_ID", defaultAppleBundleID),
		AppleEnvironments: parseAppleEnvironments(os.Getenv("APPLE_ENVIRONMENT")),

		AppStoreReconcileEnabled:       getenvBool("APPSTORE_RECONCILE_ENABLED"),
		AppStoreServerAPIKey:           NewSecret(os.Getenv("APPSTORE_SERVER_API_KEY")),
		AppStoreServerAPIKeyID:         os.Getenv("APPSTORE_SERVER_API_KEY_ID"),
		AppStoreServerAPIIssuerID:      os.Getenv("APPSTORE_SERVER_API_ISSUER_ID"),
		AppStoreServerAPIEnvironment:   getenv("APPSTORE_SERVER_API_ENVIRONMENT", defaultAppleEnvironment),
		AppStoreReconcileApplyEnabled:  getenvBool("APPSTORE_RECONCILE_APPLY_ENABLED"),
		AppStoreReconcileLookbackHours: getenvInt("APPSTORE_RECONCILE_LOOKBACK_HOURS", 30),

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

		NotificationsRetentionDays:       getenvInt("NOTIFICATIONS_RETENTION_DAYS", 90),
		DeviceRegistrationsRetentionDays: getenvInt("DEVICE_REGISTRATIONS_RETENTION_DAYS", 180),

		PlanItBaseURL: getenv("PLANIT_BASE_URL", defaultPlanItBaseURL),

		PollingDailyCallCap:             getenvInt("POLLING_DAILY_CALL_CAP", 240),
		PollingMinRequestSpacingSeconds: getenvInt("POLLING_MIN_REQUEST_SPACING_SECONDS", 60),
		PollingDeltaSlots:               getenv("POLLING_DELTA_SLOTS", "09:00,12:00,15:00,17:00"),
		PollingDeltaMaxPages:            getenvInt("POLLING_DELTA_MAX_PAGES", 20),
		PollingDayAllowance:             getenvInt("POLLING_DAY_ALLOWANCE", 60),
		PollingFullReadMaxAgeDays:       getenvInt("POLLING_FULL_READ_MAX_AGE_DAYS", 7),
		PollingRunBudgetMinutes:         getenvInt("POLLING_RUN_BUDGET_MINUTES", 55),
		PollingOracleEnabled:            getenvBool("POLLING_ORACLE_ENABLED"),

		NotifyQuietStart:          getenv("NOTIFY_QUIET_START", "22:00"),
		NotifyQuietEnd:            getenv("NOTIFY_QUIET_END", "07:00"),
		NotifyEventSurgeThreshold: getenvInt("NOTIFY_EVENT_SURGE_THRESHOLD", 10000),
	}

	if raw := strings.TrimSpace(os.Getenv("POLLING_AREA_ID")); raw != "" {
		areaID, err := strconv.Atoi(raw)
		if err != nil || areaID < 0 {
			return Config{}, fmt.Errorf("parse POLLING_AREA_ID %q: want a non-negative integer", raw)
		}
		cfg.PollingAreaID = areaID
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
