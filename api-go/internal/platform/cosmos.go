package platform

import (
	"context"
	"encoding/json"
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
	return NewCosmosContainerNamed(cfg, cosmosUsersContainer, logger)
}

// NewCosmosContainerNamed is NewCosmosContainer for an arbitrary container in
// the same database — the it4 DeviceRegistrations / NotificationState /
// Notifications stores reuse the same pinned-identity client wiring as Users.
// An empty endpoint short-circuits to a nil container so the routes that depend
// on it stay unwired until infra provisions Cosmos.
func NewCosmosContainerNamed(cfg Config, name string, logger *slog.Logger) (*CosmosContainer, error) {
	if cfg.CosmosEndpoint == "" {
		logger.Warn("cosmos endpoint unset; container routes unavailable", "container", name)
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

	container, err := client.NewContainer(cfg.CosmosDatabase, name)
	if err != nil {
		return nil, fmt.Errorf("open container %q: %w", name, err)
	}
	return &CosmosContainer{container: container}, nil
}

// Cosmos container names mirror the .NET CosmosContainerNames constants so the
// Go stores target the exact same physical containers the .NET API reads and
// writes.
const (
	cosmosUsersContainer = "Users"
	// CosmosDeviceRegistrationsContainer holds one document per (user, token);
	// partition key /userId, document id == token.
	CosmosDeviceRegistrationsContainer = "DeviceRegistrations"
	// CosmosNotificationStateContainer holds one watermark document per user;
	// partition key /userId, document id == userId.
	CosmosNotificationStateContainer = "NotificationState"
	// CosmosNotificationsContainer holds dispatched notifications; partition key
	// /userId. Read-only here for the unread COUNT query.
	CosmosNotificationsContainer = "Notifications"
	// CosmosWatchZonesContainer holds one document per watch zone; partition key
	// /userId, document id == zoneId.
	CosmosWatchZonesContainer = "WatchZones"
	// CosmosApplicationsContainer holds the master planning-application records;
	// partition key /authorityCode (the areaId as a string), document id == name.
	CosmosApplicationsContainer = "Applications"
	// CosmosSavedApplicationsContainer holds one document per saved application;
	// partition key /userId, document id == "{userId}:{applicationUid}".
	CosmosSavedApplicationsContainer = "SavedApplications"
	// CosmosOfferCodesContainer holds one document per offer code; partition key
	// /id == the canonical code, document id == the canonical code.
	CosmosOfferCodesContainer = "OfferCodes"
)

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

// QueryItems runs a single-partition parametrised query and returns the raw
// document bodies. The query must already be scoped to partitionKey (the SDK
// targets that logical partition), so it never fans out cross-partition. params
// binds @name placeholders.
func (c *CosmosContainer) QueryItems(ctx context.Context, partitionKey, query string, params map[string]any) ([][]byte, error) {
	opts := &azcosmos.QueryOptions{QueryParameters: queryParams(params)}
	pager := c.container.NewQueryItemsPager(query, azcosmos.NewPartitionKeyString(partitionKey), opts)
	var items [][]byte
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		items = append(items, page.Items...)
	}
	return items, nil
}

// CountItems runs a SELECT VALUE COUNT(1) scalar query in a single partition and
// returns the integer result, mirroring .NET's ScalarQueryAsync<int>. An empty
// result set (no rows match) yields 0.
func (c *CosmosContainer) CountItems(ctx context.Context, partitionKey, query string, params map[string]any) (int, error) {
	items, err := c.QueryItems(ctx, partitionKey, query, params)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}
	var count int
	if err := json.Unmarshal(items[0], &count); err != nil {
		return 0, fmt.Errorf("decode scalar count: %w", err)
	}
	return count, nil
}

// QueryItemsCrossPartition runs a parametrised query across ALL partitions and
// returns every matching document body, draining all gateway pages. The Gateway
// API supports only simple projections/filters cross-partition, which is all the
// admin find-by-email lookup needs. An empty partition key plus the SDK's
// default cross-partition flag triggers the cross-partition path.
func (c *CosmosContainer) QueryItemsCrossPartition(ctx context.Context, query string, params map[string]any) ([][]byte, error) {
	opts := &azcosmos.QueryOptions{QueryParameters: queryParams(params)}
	pager := c.container.NewQueryItemsPager(query, azcosmos.NewPartitionKey(), opts)
	var items [][]byte
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		items = append(items, page.Items...)
	}
	return items, nil
}

// QueryPageCrossPartition runs a cross-partition query and returns a single
// gateway page of up to pageSize documents plus the continuation token for the
// next page (empty when the query is exhausted). A non-empty continuationToken
// resumes a prior page. Mirrors the .NET CosmosRestClient.QueryPageAsync used by
// the admin user list. The Gateway may return inconsistently sized or empty
// pages with a non-nil token — an inherent property of cross-partition queries
// shared by both APIs.
func (c *CosmosContainer) QueryPageCrossPartition(ctx context.Context, query string, params map[string]any, pageSize int, continuationToken string) ([][]byte, string, error) {
	opts := &azcosmos.QueryOptions{QueryParameters: queryParams(params)}
	if pageSize > 0 {
		opts.PageSizeHint = clampPageSize(pageSize)
	}
	if continuationToken != "" {
		opts.ContinuationToken = &continuationToken
	}
	pager := c.container.NewQueryItemsPager(query, azcosmos.NewPartitionKey(), opts)
	if !pager.More() {
		return nil, "", nil
	}
	page, err := pager.NextPage(ctx)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if page.ContinuationToken != nil {
		next = *page.ContinuationToken
	}
	return page.Items, next, nil
}

// clampPageSize bounds the page size to a safe int32 range for the SDK hint.
func clampPageSize(pageSize int) int32 {
	const maxPageSize = 1000
	if pageSize > maxPageSize {
		return maxPageSize
	}
	return int32(pageSize)
}

// queryParams converts a name/value map into the SDK's parameter slice. The
// map's iteration order is irrelevant — Cosmos binds by @name, not position.
func queryParams(params map[string]any) []azcosmos.QueryParameter {
	if len(params) == 0 {
		return nil
	}
	out := make([]azcosmos.QueryParameter, 0, len(params))
	for name, value := range params {
		out = append(out, azcosmos.QueryParameter{Name: name, Value: value})
	}
	return out
}
