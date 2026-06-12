package platform

import (
	"log/slog"
	"testing"
	"time"
)

func TestCosmosRetryOptions_MatchesBoundedBudget(t *testing.T) {
	t.Parallel()

	opts := cosmosRetryOptions()

	// The .NET CosmosThrottleRetryHandler budget: 3 attempts total (1 try + 2
	// retries), a 750ms per-delay cap, and ~1.5s overall. MaxRetries counts
	// retries only, so 2 => 3 attempts.
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
