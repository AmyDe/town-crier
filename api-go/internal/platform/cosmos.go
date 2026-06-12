package platform

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
)

// cosmosMaxRetries is the number of retries (not attempts) the SDK performs on a
// throttled/transient response. Two retries plus the initial try gives the
// 3-attempt budget of the .NET CosmosThrottleRetryHandler.
const cosmosMaxRetries = 2

// cosmosMaxRetryDelay caps a single backoff delay, matching the .NET handler's
// 750ms ceiling.
const cosmosMaxRetryDelay = 750 * time.Millisecond

// cosmosTryTimeout bounds one attempt so the worst-case total (3 attempts plus
// capped backoff) stays within the ~1.5s overall budget rather than hanging.
const cosmosTryTimeout = 1500 * time.Millisecond

// cosmosRetryOptions returns the azcore retry policy tuned to the .NET bounded
// retry budget: 3 attempts, 750ms per-delay cap, ~1.5s overall.
func cosmosRetryOptions() policy.RetryOptions {
	return policy.RetryOptions{
		MaxRetries:    cosmosMaxRetries,
		MaxRetryDelay: cosmosMaxRetryDelay,
		TryTimeout:    cosmosTryTimeout,
	}
}

// CosmosContainer is the azcosmos-backed implementation of the profile store's
// item accessor. It performs single-partition point read/upsert/delete on the
// Users container, keyed on the user id. SDK types never leak past its methods.
type CosmosContainer struct {
	container *azcosmos.ContainerClient
}

// NewCosmosContainer builds a Cosmos client authenticated by the pinned
// user-assigned managed identity (AZURE_CLIENT_ID) and returns the Users
// container accessor. When CosmosEndpoint is empty the function returns a nil
// container without error, so the binary boots without Cosmos env (profile
// routes are then simply unwired). The SDK opens connections lazily on first
// call, preserving cold-start latency.
func NewCosmosContainer(cfg Config, logger *slog.Logger) (*CosmosContainer, error) {
	if cfg.CosmosEndpoint == "" {
		logger.Warn("cosmos endpoint unset; profile routes unavailable")
		return nil, nil //nolint:nilnil // absent config is a valid "no container" state, not an error
	}

	credOpts := &azidentity.ManagedIdentityCredentialOptions{}
	if cfg.AzureClientID != "" {
		credOpts.ID = azidentity.ClientID(cfg.AzureClientID)
	}
	cred, err := azidentity.NewManagedIdentityCredential(credOpts)
	if err != nil {
		return nil, fmt.Errorf("build managed-identity credential: %w", err)
	}

	clientOpts := &azcosmos.ClientOptions{}
	clientOpts.Retry = cosmosRetryOptions()
	client, err := azcosmos.NewClient(cfg.CosmosEndpoint, cred, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("build cosmos client: %w", err)
	}

	container, err := client.NewContainer(cfg.CosmosDatabase, cosmosUsersContainer)
	if err != nil {
		return nil, fmt.Errorf("open container %q: %w", cosmosUsersContainer, err)
	}
	return &CosmosContainer{container: container}, nil
}

// cosmosUsersContainer is the Users container name, matching the .NET
// CosmosContainerNames.Users.
const cosmosUsersContainer = "Users"

// ReadItem point-reads the document with the given id from its partition.
func (c *CosmosContainer) ReadItem(ctx context.Context, partitionKey, id string) ([]byte, error) {
	resp, err := c.container.ReadItem(ctx, azcosmos.NewPartitionKeyString(partitionKey), id, nil)
	if err != nil {
		return nil, err
	}
	return resp.Value, nil
}

// UpsertItem upserts the document into the given partition.
func (c *CosmosContainer) UpsertItem(ctx context.Context, partitionKey string, item []byte) error {
	_, err := c.container.UpsertItem(ctx, azcosmos.NewPartitionKeyString(partitionKey), item, nil)
	return err
}

// DeleteItem deletes the document with the given id from its partition.
func (c *CosmosContainer) DeleteItem(ctx context.Context, partitionKey, id string) error {
	_, err := c.container.DeleteItem(ctx, azcosmos.NewPartitionKeyString(partitionKey), id, nil)
	return err
}
