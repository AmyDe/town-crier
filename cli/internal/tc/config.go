package tc

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the resolved API connection settings.
type Config struct {
	URL    string
	APIKey string
}

// configFile mirrors the on-disk ~/.config/tc/config.json shape.
// Environments holds the per-environment settings the polling commands need.
type configFile struct {
	URL          string               `json:"url"`
	APIKey       string               `json:"apiKey"`
	Environments map[string]envConfig `json:"environments"`
}

type envConfig struct {
	URL    string `json:"url"`
	APIKey string `json:"apiKey"`
}

// ErrURLNotConfigured and ErrAPIKeyNotConfigured report that a required setting
// resolved to empty. The caller owns the user-facing wording (which embeds the
// config path) and maps either to exit code 1.
var (
	ErrURLNotConfigured    = errors.New("url not configured")
	ErrAPIKeyNotConfigured = errors.New("api key not configured")
)

// LoadEnvironments reads the "environments" block of the config file at path
// and returns the dev and prod settings. Both must have a url and an apiKey.
func LoadEnvironments(path string) (map[string]Config, error) {
	var file configFile
	// #nosec G304 -- path is the CLI's own config location, not attacker-controlled input.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	out := make(map[string]Config, len(pollingEnvNames))
	for _, name := range pollingEnvNames {
		e := file.Environments[name]
		if e.URL == "" || e.APIKey == "" {
			return nil, fmt.Errorf("set environments.%s.url and environments.%s.apiKey in %s", name, name, path)
		}
		out[name] = Config(e)
	}
	return out, nil
}

// DefaultConfigPath returns ~/.config/tc/config.json.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return filepath.Join(home, ".config", "tc", "config.json")
}

// LoadConfig resolves the API URL and key from the config file at path, with the
// urlOverride and apiKeyOverride (typically --url / --api-key) taking precedence
// when non-nil. A nil override falls back to the file value. It returns
// ErrURLNotConfigured or ErrAPIKeyNotConfigured if either value is ultimately
// empty.
func LoadConfig(path string, urlOverride, apiKeyOverride *string) (Config, error) {
	var fileURL, fileAPIKey string
	// #nosec G304 -- path is the CLI's own config location, not attacker-controlled input.
	if data, err := os.ReadFile(path); err == nil {
		var file configFile
		if err := json.Unmarshal(data, &file); err == nil {
			fileURL = file.URL
			fileAPIKey = file.APIKey
		}
	}

	resolvedURL := fileURL
	if urlOverride != nil {
		resolvedURL = *urlOverride
	}
	resolvedAPIKey := fileAPIKey
	if apiKeyOverride != nil {
		resolvedAPIKey = *apiKeyOverride
	}

	if resolvedURL == "" {
		return Config{}, ErrURLNotConfigured
	}
	if resolvedAPIKey == "" {
		return Config{}, ErrAPIKeyNotConfigured
	}

	return Config{URL: resolvedURL, APIKey: resolvedAPIKey}, nil
}
