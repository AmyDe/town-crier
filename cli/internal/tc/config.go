package tc

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Config holds the resolved API connection settings.
type Config struct {
	URL    string
	APIKey string
}

// configFile mirrors the on-disk ~/.config/tc/config.json shape.
type configFile struct {
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
