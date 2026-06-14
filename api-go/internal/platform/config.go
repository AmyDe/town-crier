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
}

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

		AppleBundleID: getenv("APPLE_BUNDLE_ID", defaultAppleBundleID),

		APNsEnabled:    getenvBool("APNS_ENABLED"),
		APNsAuthKey:    NewSecret(os.Getenv("APNS_AUTH_KEY")),
		APNsKeyID:      getenv("APNS_KEY_ID", defaultAPNsKeyID),
		APNsTeamID:     getenv("APNS_TEAM_ID", defaultAPNsTeamID),
		APNsBundleID:   getenv("APNS_BUNDLE_ID", defaultAppleBundleID),
		APNsUseSandbox: getenvBool("APNS_USE_SANDBOX"),

		ACSConnectionString: NewSecret(os.Getenv("ACS_CONNECTION_STRING")),
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
