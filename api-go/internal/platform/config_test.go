package platform

import (
	"log/slog"
	"reflect"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name      string
		port      string
		logLevel  string
		wantPort  string
		wantLevel slog.Level
		wantErr   bool
	}{
		{"defaults", "", "", "8080", slog.LevelInfo, false},
		{"port override", "9090", "", "9090", slog.LevelInfo, false},
		{"debug level", "", "debug", "8080", slog.LevelDebug, false},
		{"warn level", "", "WARN", "8080", slog.LevelWarn, false},
		{"invalid level", "", "noisy", "", slog.LevelInfo, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PORT", tc.port)
			t.Setenv("LOG_LEVEL", tc.logLevel)

			cfg, err := LoadConfig()

			if (err != nil) != tc.wantErr {
				t.Fatalf("got err=%v, wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if cfg.Port != tc.wantPort {
				t.Errorf("Port: got %q, want %q", cfg.Port, tc.wantPort)
			}
			if cfg.LogLevel != tc.wantLevel {
				t.Errorf("LogLevel: got %v, want %v", cfg.LogLevel, tc.wantLevel)
			}
		})
	}
}

func TestLoadConfig_Auth0(t *testing.T) {
	t.Setenv("AUTH0_DOMAIN", "town-crier.eu.auth0.com")
	t.Setenv("AUTH0_AUDIENCE", "https://api.towncrierapp.uk")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Auth0Domain != "town-crier.eu.auth0.com" {
		t.Errorf("Auth0Domain: got %q, want %q", cfg.Auth0Domain, "town-crier.eu.auth0.com")
	}
	if cfg.Auth0Audience != "https://api.towncrierapp.uk" {
		t.Errorf("Auth0Audience: got %q, want %q", cfg.Auth0Audience, "https://api.towncrierapp.uk")
	}
}

func TestLoadConfig_Auth0DefaultsEmpty(t *testing.T) {
	// The dev Go app ships without Auth0 env vars until infra wires them
	// (GH#418, it3+). Absent config must load cleanly (empty strings), letting
	// the validator become deny-all rather than failing startup.
	t.Setenv("AUTH0_DOMAIN", "")
	t.Setenv("AUTH0_AUDIENCE", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Auth0Domain != "" || cfg.Auth0Audience != "" {
		t.Errorf("Auth0 config: got domain=%q audience=%q, want both empty", cfg.Auth0Domain, cfg.Auth0Audience)
	}
}

func TestLoadConfig_CorsAllowedOrigins(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want []string
	}{
		// Default mirrors .NET's fallback when Cors:AllowedOrigins is unset.
		{"default localhost", "", []string{"http://localhost:5173"}},
		{"single origin", "https://towncrierapp.uk", []string{"https://towncrierapp.uk"}},
		{
			"comma separated with spaces trimmed",
			"https://towncrierapp.uk, http://localhost:5173",
			[]string{"https://towncrierapp.uk", "http://localhost:5173"},
		},
		{"empty entries dropped", "https://towncrierapp.uk,,", []string{"https://towncrierapp.uk"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CORS_ALLOWED_ORIGINS", tc.env)

			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			if !reflect.DeepEqual(cfg.CorsAllowedOrigins, tc.want) {
				t.Errorf("CorsAllowedOrigins: got %v, want %v", cfg.CorsAllowedOrigins, tc.want)
			}
		})
	}
}
