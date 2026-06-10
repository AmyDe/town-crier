package platform

import (
	"fmt"
	"log/slog"
	"os"
)

// Config holds process configuration loaded from environment variables.
// Container Apps provides env vars; load them, validate them, fail fast at
// startup.
type Config struct {
	Port     string
	LogLevel slog.Level
}

// LoadConfig reads configuration from the environment, applying defaults
// where a variable is unset.
func LoadConfig() (Config, error) {
	cfg := Config{
		Port:     getenv("PORT", "8080"),
		LogLevel: slog.LevelInfo,
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

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
