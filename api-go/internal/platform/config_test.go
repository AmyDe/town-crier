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

func TestLoadConfig_Cosmos(t *testing.T) {
	t.Setenv("COSMOS_ENDPOINT", "https://town-crier.documents.azure.com:443/")
	t.Setenv("COSMOS_DATABASE", "town-crier")
	t.Setenv("AZURE_CLIENT_ID", "11111111-2222-3333-4444-555555555555")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.CosmosEndpoint != "https://town-crier.documents.azure.com:443/" {
		t.Errorf("CosmosEndpoint: got %q", cfg.CosmosEndpoint)
	}
	if cfg.CosmosDatabase != "town-crier" {
		t.Errorf("CosmosDatabase: got %q", cfg.CosmosDatabase)
	}
	if cfg.AzureClientID != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("AzureClientID: got %q", cfg.AzureClientID)
	}
}

func TestLoadConfig_Auth0M2M(t *testing.T) {
	t.Setenv("AUTH0_M2M_CLIENT_ID", "m2m-client")
	t.Setenv("AUTH0_M2M_CLIENT_SECRET", "m2m-secret")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Auth0M2MClientID != "m2m-client" {
		t.Errorf("Auth0M2MClientID: got %q, want m2m-client", cfg.Auth0M2MClientID)
	}
	if cfg.Auth0M2MClientSecret.Expose() != "m2m-secret" {
		t.Errorf("Auth0M2MClientSecret: got %q, want m2m-secret", cfg.Auth0M2MClientSecret.Expose())
	}
	// The secret must redact in any stringified form.
	if got := cfg.Auth0M2MClientSecret.String(); got != "[REDACTED]" {
		t.Errorf("Auth0M2MClientSecret.String(): got %q, want [REDACTED]", got)
	}
}

func TestLoadConfig_M2MConfiguredOnlyWhenAllPresent(t *testing.T) {
	tests := []struct {
		name         string
		domain       string
		clientID     string
		clientSecret string
		want         bool
	}{
		{"all present", "town-crier.eu.auth0.com", "m2m-client", "m2m-secret", true},
		{"missing domain", "", "m2m-client", "m2m-secret", false},
		{"missing client id", "town-crier.eu.auth0.com", "", "m2m-secret", false},
		{"missing secret", "town-crier.eu.auth0.com", "m2m-client", "", false},
		{"all absent", "", "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AUTH0_DOMAIN", tc.domain)
			t.Setenv("AUTH0_M2M_CLIENT_ID", tc.clientID)
			t.Setenv("AUTH0_M2M_CLIENT_SECRET", tc.clientSecret)

			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			if got := cfg.Auth0M2MConfigured(); got != tc.want {
				t.Errorf("Auth0M2MConfigured(): got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLoadConfig_ProDomains(t *testing.T) {
	t.Setenv("SUBSCRIPTION_AUTOGRANT_PRODOMAINS", "towncrier.test, example.org")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.ProDomains != "towncrier.test, example.org" {
		t.Errorf("ProDomains: got %q", cfg.ProDomains)
	}
}

func TestLoadConfig_OutboundBaseURLs(t *testing.T) {
	t.Run("defaults to live UK services", func(t *testing.T) {
		t.Setenv("POSTCODES_IO_BASE_URL", "")
		t.Setenv("GOVUK_PLANNING_DATA_BASE_URL", "")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if cfg.PostcodesIoBaseURL != "https://api.postcodes.io/" {
			t.Errorf("PostcodesIoBaseURL: got %q", cfg.PostcodesIoBaseURL)
		}
		if cfg.GovUkBaseURL != "https://www.planning.data.gov.uk/" {
			t.Errorf("GovUkBaseURL: got %q", cfg.GovUkBaseURL)
		}
	})

	t.Run("overrides honoured", func(t *testing.T) {
		t.Setenv("POSTCODES_IO_BASE_URL", "http://localhost:9001/")
		t.Setenv("GOVUK_PLANNING_DATA_BASE_URL", "http://localhost:9002/")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if cfg.PostcodesIoBaseURL != "http://localhost:9001/" {
			t.Errorf("PostcodesIoBaseURL: got %q", cfg.PostcodesIoBaseURL)
		}
		if cfg.GovUkBaseURL != "http://localhost:9002/" {
			t.Errorf("GovUkBaseURL: got %q", cfg.GovUkBaseURL)
		}
	})
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
