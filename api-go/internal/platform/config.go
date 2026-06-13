package platform

import (
	"fmt"
	"log/slog"
	"os"
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
}

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

		Auth0M2MClientID:     os.Getenv("AUTH0_M2M_CLIENT_ID"),
		Auth0M2MClientSecret: NewSecret(os.Getenv("AUTH0_M2M_CLIENT_SECRET")),

		ProDomains: os.Getenv("SUBSCRIPTION_AUTOGRANT_PRODOMAINS"),

		PostcodesIoBaseURL: getenv("POSTCODES_IO_BASE_URL", defaultPostcodesIoBaseURL),
		GovUkBaseURL:       getenv("GOVUK_PLANNING_DATA_BASE_URL", defaultGovUkBaseURL),
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
