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

func TestLoadConfig_AzureClientID(t *testing.T) {
	t.Setenv("AZURE_CLIENT_ID", "11111111-2222-3333-4444-555555555555")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
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

func TestLoadConfig_APNs(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		t.Setenv("APNS_ENABLED", "true")
		t.Setenv("APNS_AUTH_KEY", "-----BEGIN PRIVATE KEY-----\nMIG...\n-----END PRIVATE KEY-----")
		t.Setenv("APNS_KEY_ID", "L2J5PQASN5")
		t.Setenv("APNS_TEAM_ID", "4574VQ7N2X")
		t.Setenv("APNS_BUNDLE_ID", "uk.towncrierapp.mobile")
		t.Setenv("APNS_USE_SANDBOX", "true")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if !cfg.APNsEnabled {
			t.Error("APNsEnabled: got false, want true")
		}
		if cfg.APNsAuthKey.Expose() == "" {
			t.Error("APNsAuthKey: got empty")
		}
		if cfg.APNsKeyID != "L2J5PQASN5" {
			t.Errorf("APNsKeyID: got %q", cfg.APNsKeyID)
		}
		if cfg.APNsTeamID != "4574VQ7N2X" {
			t.Errorf("APNsTeamID: got %q", cfg.APNsTeamID)
		}
		if cfg.APNsBundleID != "uk.towncrierapp.mobile" {
			t.Errorf("APNsBundleID: got %q", cfg.APNsBundleID)
		}
		if !cfg.APNsUseSandbox {
			t.Error("APNsUseSandbox: got false, want true")
		}
	})

	t.Run("absent defaults disabled with canonical key/team/bundle", func(t *testing.T) {
		t.Setenv("APNS_ENABLED", "")
		t.Setenv("APNS_AUTH_KEY", "")
		t.Setenv("APNS_KEY_ID", "")
		t.Setenv("APNS_TEAM_ID", "")
		t.Setenv("APNS_BUNDLE_ID", "")
		t.Setenv("APNS_USE_SANDBOX", "")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if cfg.APNsEnabled {
			t.Error("APNsEnabled: got true, want false by default")
		}
		if cfg.APNsKeyID != "L2J5PQASN5" {
			t.Errorf("APNsKeyID default: got %q, want L2J5PQASN5", cfg.APNsKeyID)
		}
		if cfg.APNsTeamID != "4574VQ7N2X" {
			t.Errorf("APNsTeamID default: got %q, want 4574VQ7N2X", cfg.APNsTeamID)
		}
		if cfg.APNsBundleID != "uk.towncrierapp.mobile" {
			t.Errorf("APNsBundleID default: got %q, want uk.towncrierapp.mobile", cfg.APNsBundleID)
		}
		if cfg.APNsUseSandbox {
			t.Error("APNsUseSandbox: got true, want false by default")
		}
	})
}

func TestLoadConfig_ACSConnectionString(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		t.Setenv("ACS_CONNECTION_STRING", "endpoint=https://acs.example.com/;accesskey=YWJjZA==")
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if cfg.ACSConnectionString.Expose() != "endpoint=https://acs.example.com/;accesskey=YWJjZA==" {
			t.Errorf("ACSConnectionString: got %q", cfg.ACSConnectionString.Expose())
		}
	})

	t.Run("absent defaults empty (email NoOp)", func(t *testing.T) {
		t.Setenv("ACS_CONNECTION_STRING", "")
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if cfg.ACSConnectionString.Expose() != "" {
			t.Errorf("ACSConnectionString: got %q, want empty", cfg.ACSConnectionString.Expose())
		}
	})
}

func TestLoadConfig_AdminAPIKey(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		t.Setenv("ADMIN_API_KEY", "s3cret-admin-key")
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if cfg.AdminAPIKey != "s3cret-admin-key" {
			t.Errorf("AdminAPIKey: got %q", cfg.AdminAPIKey)
		}
	})

	t.Run("absent defaults empty (admin surface disabled)", func(t *testing.T) {
		t.Setenv("ADMIN_API_KEY", "")
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if cfg.AdminAPIKey != "" {
			t.Errorf("AdminAPIKey: got %q, want empty", cfg.AdminAPIKey)
		}
	})
}

func TestLoadConfig_SiteBuildKey(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		t.Setenv("SITE_BUILD_KEY", "s3cret-build-key")
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if cfg.SiteBuildKey != "s3cret-build-key" {
			t.Errorf("SiteBuildKey: got %q", cfg.SiteBuildKey)
		}
	})

	t.Run("absent defaults empty (SEO build surface disabled)", func(t *testing.T) {
		t.Setenv("SITE_BUILD_KEY", "")
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if cfg.SiteBuildKey != "" {
			t.Errorf("SiteBuildKey: got %q, want empty", cfg.SiteBuildKey)
		}
	})
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
		// Default when CORS_ALLOWED_ORIGINS is unset.
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

func TestLoadConfig_AnonRateLimitDefaults(t *testing.T) {
	t.Setenv("ANON_RATE_LIMIT_BURST", "")
	t.Setenv("ANON_RATE_LIMIT_REFILL_PER_MINUTE", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.AnonRateLimitBurst != 120 {
		t.Errorf("AnonRateLimitBurst default: got %d, want 120", cfg.AnonRateLimitBurst)
	}
	if cfg.AnonRateLimitRefillPerMinute != 60 {
		t.Errorf("AnonRateLimitRefillPerMinute default: got %d, want 60", cfg.AnonRateLimitRefillPerMinute)
	}
}

func TestLoadConfig_AnonRateLimitOverrides(t *testing.T) {
	t.Setenv("ANON_RATE_LIMIT_BURST", "30")
	t.Setenv("ANON_RATE_LIMIT_REFILL_PER_MINUTE", "10")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.AnonRateLimitBurst != 30 {
		t.Errorf("AnonRateLimitBurst override: got %d, want 30", cfg.AnonRateLimitBurst)
	}
	if cfg.AnonRateLimitRefillPerMinute != 10 {
		t.Errorf("AnonRateLimitRefillPerMinute override: got %d, want 10", cfg.AnonRateLimitRefillPerMinute)
	}
}

func TestLoadConfig_AppleEnvironments(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want []string
	}{
		{"default is Production when unset", "", []string{"Production"}},
		{"single override", "Sandbox", []string{"Sandbox"}},
		{"comma list with whitespace", " Sandbox , Production ", []string{"Sandbox", "Production"}},
		{"empty entries dropped", "Production,,", []string{"Production"}},
		{"case preserved in value", "sandbox", []string{"sandbox"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APPLE_ENVIRONMENT", tc.env)

			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			if !reflect.DeepEqual(cfg.AppleEnvironments, tc.want) {
				t.Errorf("AppleEnvironments: got %v, want %v", cfg.AppleEnvironments, tc.want)
			}
		})
	}
}

func TestLoadConfig_AppStoreReconcile(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		t.Setenv("APPSTORE_RECONCILE_ENABLED", "true")
		t.Setenv("APPSTORE_SERVER_API_KEY", "-----BEGIN PRIVATE KEY-----\nMIG...\n-----END PRIVATE KEY-----")
		t.Setenv("APPSTORE_SERVER_API_KEY_ID", "ABCDE12345")
		t.Setenv("APPSTORE_SERVER_API_ISSUER_ID", "57246542-96fe-1a63-e053-0824d011072a")
		t.Setenv("APPSTORE_SERVER_API_ENVIRONMENT", "Sandbox")
		t.Setenv("APPSTORE_RECONCILE_APPLY_ENABLED", "true")
		t.Setenv("APPSTORE_RECONCILE_LOOKBACK_HOURS", "48")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if !cfg.AppStoreReconcileEnabled {
			t.Error("AppStoreReconcileEnabled: got false, want true")
		}
		if cfg.AppStoreServerAPIKey.Expose() == "" {
			t.Error("AppStoreServerAPIKey: got empty")
		}
		if cfg.AppStoreServerAPIKeyID != "ABCDE12345" {
			t.Errorf("AppStoreServerAPIKeyID: got %q", cfg.AppStoreServerAPIKeyID)
		}
		if cfg.AppStoreServerAPIIssuerID != "57246542-96fe-1a63-e053-0824d011072a" {
			t.Errorf("AppStoreServerAPIIssuerID: got %q", cfg.AppStoreServerAPIIssuerID)
		}
		if cfg.AppStoreServerAPIEnvironment != "Sandbox" {
			t.Errorf("AppStoreServerAPIEnvironment: got %q", cfg.AppStoreServerAPIEnvironment)
		}
		if !cfg.AppStoreReconcileApplyEnabled {
			t.Error("AppStoreReconcileApplyEnabled: got false, want true")
		}
		if cfg.AppStoreReconcileLookbackHours != 48 {
			t.Errorf("AppStoreReconcileLookbackHours: got %d, want 48", cfg.AppStoreReconcileLookbackHours)
		}
	})

	t.Run("absent defaults to observe-only and unconfigured", func(t *testing.T) {
		t.Setenv("APPSTORE_RECONCILE_ENABLED", "")
		t.Setenv("APPSTORE_SERVER_API_KEY", "")
		t.Setenv("APPSTORE_SERVER_API_KEY_ID", "")
		t.Setenv("APPSTORE_SERVER_API_ISSUER_ID", "")
		t.Setenv("APPSTORE_SERVER_API_ENVIRONMENT", "")
		t.Setenv("APPSTORE_RECONCILE_APPLY_ENABLED", "")
		t.Setenv("APPSTORE_RECONCILE_LOOKBACK_HOURS", "")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if cfg.AppStoreReconcileEnabled {
			t.Error("AppStoreReconcileEnabled: got true, want false by default")
		}
		if cfg.AppStoreServerAPIEnvironment != "Production" {
			t.Errorf("AppStoreServerAPIEnvironment default: got %q, want Production", cfg.AppStoreServerAPIEnvironment)
		}
		if cfg.AppStoreReconcileApplyEnabled {
			t.Error("AppStoreReconcileApplyEnabled: got true, want false by default (observe-only)")
		}
		if cfg.AppStoreReconcileLookbackHours != 30 {
			t.Errorf("AppStoreReconcileLookbackHours default: got %d, want 30", cfg.AppStoreReconcileLookbackHours)
		}
	})
}
