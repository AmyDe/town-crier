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
