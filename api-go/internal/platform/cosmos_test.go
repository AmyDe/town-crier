package platform

import (
	"log/slog"
	"testing"
	"time"
)

func TestCosmosRetryOptions_MatchesBoundedBudget(t *testing.T) {
	t.Parallel()

	opts := cosmosRetryOptions()

	// Retry budget: 3 attempts total (1 try + 2 retries), a 750ms per-delay cap,
	// and ~1.5s overall. MaxRetries counts retries only, so 2 => 3 attempts.
	if opts.MaxRetries != 2 {
		t.Errorf("MaxRetries: got %d, want 2 (3 attempts total)", opts.MaxRetries)
	}
	if opts.MaxRetryDelay != 750*time.Millisecond {
		t.Errorf("MaxRetryDelay: got %v, want 750ms", opts.MaxRetryDelay)
	}
	// TryTimeout bounds a single attempt so the worst case stays within the
	// overall budget rather than hanging on a slow socket.
	if opts.TryTimeout <= 0 || opts.TryTimeout > 1500*time.Millisecond {
		t.Errorf("TryTimeout: got %v, want a positive value within the 1.5s budget", opts.TryTimeout)
	}
}

func TestCosmosBuildReadRetryOptions_LongerSingleAttemptBudgetThanOLTP(t *testing.T) {
	t.Parallel()

	oltp := cosmosRetryOptions()
	build := cosmosBuildReadRetryOptions()

	// The build-time SEO reads (RecentByAuthority / RecentNearby) load up to ~200
	// full documents from a LARGE authority partition, which legitimately exceeds
	// the tight 1.5s OLTP per-try budget — so under the OLTP budget every retry
	// times out and the request 500s (tc-9tov). The build path must get a strictly
	// longer per-attempt budget.
	if build.TryTimeout <= oltp.TryTimeout {
		t.Errorf("build TryTimeout %v must exceed OLTP TryTimeout %v", build.TryTimeout, oltp.TryTimeout)
	}
	// Target the ~8–10s band: long enough for the slow large-partition read,
	// short enough to stay clear of the 15s server WriteTimeout.
	if build.TryTimeout < 8*time.Second || build.TryTimeout > 10*time.Second {
		t.Errorf("build TryTimeout: got %v, want within [8s, 10s]", build.TryTimeout)
	}
	// A single generous attempt. A second attempt at this per-try budget would
	// risk ~18s total and breach the 15s WriteTimeout, and a retry does not cure a
	// consistently-slow large-partition read anyway. MaxRetries < 0 is azcore's
	// spelling of "no retries": setDefaults normalises 0 to the default of 3.
	if build.MaxRetries >= 0 {
		t.Errorf("build MaxRetries: got %d, want < 0 (single attempt; 0 would mean azcore's default of 3 retries)", build.MaxRetries)
	}
	// Worst case (single attempt) must stay comfortably under the 15s HTTP
	// WriteTimeout (platform.NewServer), or the SEO response can't be written.
	const serverWriteTimeout = 15 * time.Second
	if build.TryTimeout >= serverWriteTimeout {
		t.Errorf("build TryTimeout %v must stay under the %v server WriteTimeout", build.TryTimeout, serverWriteTimeout)
	}
}

func TestNewCosmosContainer_RequiresConfig(t *testing.T) {
	t.Parallel()

	// Without an endpoint, the factory must not attempt to build a client — it
	// returns nil so the binary can boot without Cosmos env (profile routes are
	// then unavailable, matching the dev app before infra wires Cosmos).
	cfg := Config{CosmosEndpoint: "", CosmosDatabase: "town-crier", AzureClientID: "id"}
	container, err := NewCosmosContainer(cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewCosmosContainer with no endpoint: got err %v, want nil", err)
	}
	if container != nil {
		t.Errorf("NewCosmosContainer with no endpoint: got %v, want nil container", container)
	}
}

func TestNewCosmosContainerNamed_RequiresConfig(t *testing.T) {
	t.Parallel()

	// The named-container factory shares the no-endpoint short-circuit so the
	// it4 device-token / notification-state containers are simply unwired when
	// Cosmos env is absent, exactly like the Users container.
	cfg := Config{CosmosEndpoint: "", CosmosDatabase: "town-crier", AzureClientID: "id"}
	container, err := NewCosmosContainerNamed(cfg, "DeviceRegistrations", slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewCosmosContainerNamed with no endpoint: got err %v, want nil", err)
	}
	if container != nil {
		t.Errorf("NewCosmosContainerNamed with no endpoint: got %v, want nil container", container)
	}
}

func TestCosmosContainerNames_MirrorDotNet(t *testing.T) {
	t.Parallel()

	// The device-token and notification-state stores read these constants, so a
	// typo here would silently target the wrong container.
	if CosmosDeviceRegistrationsContainer != "DeviceRegistrations" {
		t.Errorf("DeviceRegistrations container = %q", CosmosDeviceRegistrationsContainer)
	}
	if CosmosNotificationStateContainer != "NotificationState" {
		t.Errorf("NotificationState container = %q", CosmosNotificationStateContainer)
	}
	if CosmosNotificationsContainer != "Notifications" {
		t.Errorf("Notifications container = %q", CosmosNotificationsContainer)
	}
}
