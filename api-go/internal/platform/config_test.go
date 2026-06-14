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

func TestLoadConfig_ServiceBus(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		t.Setenv("SERVICE_BUS_NAMESPACE", "sb-town-crier-prod.servicebus.windows.net")
		t.Setenv("SERVICE_BUS_QUEUE_NAME", "poll-triggers")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if cfg.ServiceBusNamespace != "sb-town-crier-prod.servicebus.windows.net" {
			t.Errorf("ServiceBusNamespace: got %q", cfg.ServiceBusNamespace)
		}
		if cfg.ServiceBusQueueName != "poll-triggers" {
			t.Errorf("ServiceBusQueueName: got %q", cfg.ServiceBusQueueName)
		}
	})

	t.Run("absent defaults empty (Service Bus modes unwired)", func(t *testing.T) {
		t.Setenv("SERVICE_BUS_NAMESPACE", "")
		t.Setenv("SERVICE_BUS_QUEUE_NAME", "")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if cfg.ServiceBusNamespace != "" || cfg.ServiceBusQueueName != "" {
			t.Errorf("ServiceBus config: got namespace=%q queue=%q, want both empty",
				cfg.ServiceBusNamespace, cfg.ServiceBusQueueName)
		}
	})
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

func TestLoadConfig_PollingDefaults(t *testing.T) {
	// Clear any inherited overrides so the defaults are exercised.
	for _, k := range []string{
		"PLANIT_BASE_URL", "PLANIT_THROTTLE_DELAY_SECONDS",
		"PLANIT_RETRY_MAX_RETRIES", "PLANIT_RETRY_INITIAL_BACKOFF_SECONDS",
		"PLANIT_RETRY_RATE_LIMIT_BACKOFF_SECONDS",
		"POLLING_MAX_PAGES_PER_AUTHORITY_PER_CYCLE", "POLLING_HANDLER_BUDGET_SECONDS",
		"POLL_REPLICA_TIMEOUT_SECONDS", "POLL_SHUTDOWN_GRACE_SECONDS",
	} {
		t.Setenv(k, "")
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.PlanItBaseURL != "https://www.planit.org.uk/" {
		t.Errorf("PlanItBaseURL default: got %q", cfg.PlanItBaseURL)
	}
	if cfg.PlanItThrottleDelaySeconds != 2 {
		t.Errorf("throttle default: got %v, want 2", cfg.PlanItThrottleDelaySeconds)
	}
	if cfg.PlanItMaxRetries != 3 || cfg.PlanItInitialBackoffSeconds != 1 || cfg.PlanItRateLimitBackoffSeconds != 5 {
		t.Errorf("retry defaults: %d/%v/%v", cfg.PlanItMaxRetries, cfg.PlanItInitialBackoffSeconds, cfg.PlanItRateLimitBackoffSeconds)
	}
	if cfg.PollingMaxPagesPerAuthorityPerCycle != 3 {
		t.Errorf("max pages default: got %d, want 3", cfg.PollingMaxPagesPerAuthorityPerCycle)
	}
	if cfg.PollingHandlerBudgetSeconds != 240 {
		t.Errorf("handler budget default: got %d, want 240", cfg.PollingHandlerBudgetSeconds)
	}
	if cfg.PollReplicaTimeoutSeconds != 600 || cfg.PollShutdownGraceSeconds != 30 {
		t.Errorf("cycle budget defaults: %d/%d", cfg.PollReplicaTimeoutSeconds, cfg.PollShutdownGraceSeconds)
	}
}

func TestLoadConfig_PollingOverrides(t *testing.T) {
	t.Setenv("PLANIT_BASE_URL", "https://stub.planit.test/")
	t.Setenv("POLLING_MAX_PAGES_PER_AUTHORITY_PER_CYCLE", "5")
	t.Setenv("POLLING_HANDLER_BUDGET_SECONDS", "120")
	t.Setenv("POLL_REPLICA_TIMEOUT_SECONDS", "300")
	t.Setenv("POLL_SHUTDOWN_GRACE_SECONDS", "15")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.PlanItBaseURL != "https://stub.planit.test/" {
		t.Errorf("PlanItBaseURL override: got %q", cfg.PlanItBaseURL)
	}
	if cfg.PollingMaxPagesPerAuthorityPerCycle != 5 {
		t.Errorf("max pages override: got %d, want 5", cfg.PollingMaxPagesPerAuthorityPerCycle)
	}
	if cfg.PollingHandlerBudgetSeconds != 120 {
		t.Errorf("handler budget override: got %d, want 120", cfg.PollingHandlerBudgetSeconds)
	}
	if cfg.PollReplicaTimeoutSeconds != 300 || cfg.PollShutdownGraceSeconds != 15 {
		t.Errorf("cycle budget override: %d/%d", cfg.PollReplicaTimeoutSeconds, cfg.PollShutdownGraceSeconds)
	}
}
