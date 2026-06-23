package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// cosmosTracerName is the instrumentation scope for the Cosmos client spans. The
// tracer is fetched per call from the global TracerProvider (otel.Tracer), so it
// self-disables to no-op spans when telemetry is off (OTEL_EXPORTER_OTLP_ENDPOINT
// unset) — see telemetry.go. tc-8x8g / ADR 0027: without these client spans the
// Go API emits zero AppDependencies, leaving Cosmos latency and throttling
// invisible in App Insights.
const cosmosTracerName = "github.com/AmyDe/town-crier/api-go/internal/platform"

// cosmosMetricsRecorder is the consumer-side slice of the metrics registry the
// Cosmos container records towncrier.cosmos.request_charge_ru on, once per
// successful operation. *metrics.Registry satisfies it; nil leaves the histogram
// dark (the default for tests and Cosmos-less boots).
type cosmosMetricsRecorder interface {
	CosmosRequestCharge(ctx context.Context, ru float64, operation, container string)
}

// traceCosmosOp wraps a single outbound azcosmos call in an OpenTelemetry client
// span so it surfaces as an App Insights dependency. The span is a child of
// whatever span rides on ctx (the incoming request span), and the ctx carrying
// the span is passed to op so the SDK call links into the same trace. Attributes
// follow the OTel DB semantic conventions so App Insights populates the
// dependency Type/Target; on error the span records the error and is marked
// Error. The span ends regardless of outcome.
//
// fn returns the operation's RU charge (read from the SDK ItemResponse /
// QueryResponse); on success the charge is recorded as the
// db.cosmosdb.request_charge span attribute and on the
// towncrier.cosmos.request_charge_ru histogram (tc-21np).
func traceCosmosOp(ctx context.Context, c *CosmosContainer, op string, fn func(context.Context) (float64, error)) error {
	tracer := otel.Tracer(cosmosTracerName)
	ctx, span := tracer.Start(ctx, "Cosmos "+op+" "+c.name,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "cosmosdb"),
			attribute.String("db.operation", op),
			attribute.String("db.operation.name", op),
			attribute.String("db.cosmosdb.container", c.name),
			attribute.String("server.address", c.accountHost),
		),
	)
	defer span.End()

	ru, err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	span.SetAttributes(attribute.Float64("db.cosmosdb.request_charge", ru))
	if c.metrics != nil {
		c.metrics.CosmosRequestCharge(ctx, ru, op, c.name)
	}
	return nil
}

// parseAccountHost extracts the bare host (no port) from a Cosmos account
// endpoint URL so it can populate the dependency Target (server.address). An
// empty or unparseable endpoint yields an empty host rather than an error — the
// span is still useful without a Target.
func parseAccountHost(endpoint string) string {
	if endpoint == "" {
		return ""
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// cosmosMaxRetries is the number of retries (not attempts) the SDK performs on a
// throttled/transient response. Two retries plus the initial try gives the
// 3-attempt budget.
const cosmosMaxRetries = 2

// cosmosMaxRetryDelay caps a single backoff delay at 750ms.
const cosmosMaxRetryDelay = 750 * time.Millisecond

// cosmosTryTimeout bounds one attempt so the worst-case total (3 attempts plus
// capped backoff) stays within the ~1.5s overall budget rather than hanging.
const cosmosTryTimeout = 1500 * time.Millisecond

// cosmosRetryOptions returns the azcore retry policy tuned to the bounded retry
// budget: 3 attempts, 750ms per-delay cap, ~1.5s overall.
func cosmosRetryOptions() policy.RetryOptions {
	return policy.RetryOptions{
		MaxRetries:    cosmosMaxRetries,
		MaxRetryDelay: cosmosMaxRetryDelay,
		TryTimeout:    cosmosTryTimeout,
	}
}

// cosmosBuildReadTryTimeout bounds one attempt of a latency-tolerant build-time
// read. The SEO prerender's bounded top-N queries (RecentByAuthority /
// RecentNearby, up to ~200 full documents) legitimately exceed the 1.5s OLTP
// cosmosTryTimeout on a LARGE authority partition (e.g. a big-city authority),
// so under that tight budget every retry times out and the request 500s
// (tc-9tov). 9s gives a single generous attempt that covers the slow
// large-partition read while staying comfortably under the 15s server
// WriteTimeout (NewServer in server.go). Only the build-key-gated SEO reads opt
// in, via QueryItemsLongRead — user-facing OLTP endpoints keep the 1.5s budget.
const cosmosBuildReadTryTimeout = 9 * time.Second

// cosmosBuildReadRetryOptions returns the per-call retry override for
// latency-tolerant build-time reads: a single generous attempt, no retries.
// MaxRetries is -1 because azcore's setDefaults normalises 0 to its default of 3
// retries; -1 normalises to 0, i.e. one attempt. A second attempt at the 9s
// per-try budget would risk ~18s total and breach the 15s server WriteTimeout,
// so a retry is deliberately omitted — and a retry does not cure a
// consistently-slow large-partition read anyway (the root cause is the
// per-attempt budget, not transient failure).
func cosmosBuildReadRetryOptions() policy.RetryOptions {
	return policy.RetryOptions{
		MaxRetries: -1, // -1 => 0 retries (single attempt); 0 would mean azcore's default of 3
		TryTimeout: cosmosBuildReadTryTimeout,
	}
}

// cosmosTokenAcquireTimeout bounds a managed-identity token fetch independently
// of cosmosTryTimeout. The 1.5s per-Cosmos-try timeout (cosmosTryTimeout) wraps
// the bearer-token auth policy, so it would otherwise cancel a COLD token fetch
// on a short-lived Container Apps Job replica before azidentity can cache a
// token — failing every retry (tc-u7mp). 30s comfortably covers a cold identity-
// endpoint round trip; once cached the bearer policy reuses the token, so this
// only affects the very first fetch.
const cosmosTokenAcquireTimeout = 30 * time.Second

// detachedTokenCredential decouples token acquisition from the caller's
// (per-Cosmos-try) deadline. context.WithoutCancel keeps the trace/values but
// drops the parent's cancellation + deadline; a fresh cosmosTokenAcquireTimeout
// then bounds the fetch so it can't hang forever.
type detachedTokenCredential struct {
	inner   azcore.TokenCredential
	timeout time.Duration
}

func (c detachedTokenCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	tokenCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.timeout)
	defer cancel()
	return c.inner.GetToken(tokenCtx, opts)
}

// CosmosContainer is the azcosmos-backed implementation of the profile store's
// item accessor. It performs single-partition point read/upsert/delete on the
// Users container, keyed on the user id. SDK types never leak past its methods.
type CosmosContainer struct {
	container *azcosmos.ContainerClient
	// name and accountHost label the OTel dependency spans (tc-8x8g): the
	// container name becomes db.cosmosdb.container and the account host becomes
	// server.address (the dependency Target in App Insights).
	name        string
	accountHost string
	// metrics records towncrier.cosmos.request_charge_ru per op, wired via
	// WithMetrics. nil until wired, so the histogram stays dark in tests and
	// Cosmos-less boots.
	metrics cosmosMetricsRecorder
}

// WithMetrics wires the recorder the container records the per-op RU charge on.
// A post-construction setter so the many NewCosmosContainerNamed call sites are
// unaffected; cmd/api and cmd/worker call it once per container after building
// it. Returns the container for chaining. A nil container (Cosmos unconfigured)
// is a safe no-op so call sites can chain unconditionally.
func (c *CosmosContainer) WithMetrics(rec cosmosMetricsRecorder) *CosmosContainer {
	if c == nil {
		return nil
	}
	c.metrics = rec
	return c
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

	// Wrap the MI credential so its token fetch runs under cosmosTokenAcquireTimeout
	// detached from the per-Cosmos-try deadline, otherwise a cold token fetch on a
	// short-lived job replica is strangled by cosmosTryTimeout (tc-u7mp).
	detachedCred := detachedTokenCredential{inner: cred, timeout: cosmosTokenAcquireTimeout}

	clientOpts := &azcosmos.ClientOptions{}
	clientOpts.Retry = cosmosRetryOptions()
	client, err := azcosmos.NewClient(cfg.CosmosEndpoint, detachedCred, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("build cosmos client: %w", err)
	}

	container, err := client.NewContainer(cfg.CosmosDatabase, name)
	if err != nil {
		return nil, fmt.Errorf("open container %q: %w", name, err)
	}
	return &CosmosContainer{
		container:   container,
		name:        name,
		accountHost: parseAccountHost(cfg.CosmosEndpoint),
	}, nil
}

// Cosmos container names for the production physical containers the Go stores
// read and write.
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
	// /code == the canonical code, document id == the canonical code.
	CosmosOfferCodesContainer = "OfferCodes"
	// CosmosAppleNotificationsContainer holds one document per processed App Store
	// Server Notification; partition key /id == the notificationUUID, document id
	// == the notificationUUID. Backs webhook idempotency.
	CosmosAppleNotificationsContainer = "AppleNotifications"
	// CosmosPollStateContainer holds one document per authority's poll state;
	// partition key /id == "poll-state-{authorityId}", document id == same. The
	// poll-sb cycle reads/writes per-authority high-water marks and cursors here.
	CosmosPollStateContainer = "PollState"
	// CosmosLeasesContainer holds the single "polling" lease document gating
	// concurrent poll cycles; partition key /id == the lease id. CAS via etag.
	CosmosLeasesContainer = "Leases"
)

// ReadItem point-reads the document with the given id from its partition.
func (c *CosmosContainer) ReadItem(ctx context.Context, partitionKey, id string) ([]byte, error) {
	var value []byte
	err := traceCosmosOp(ctx, c, "ReadItem", func(ctx context.Context) (float64, error) {
		resp, err := c.container.ReadItem(ctx, azcosmos.NewPartitionKeyString(partitionKey), id, nil)
		if err != nil {
			return 0, err
		}
		value = resp.Value
		return float64(resp.RequestCharge), nil
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

// UpsertItem upserts the document into the given partition.
func (c *CosmosContainer) UpsertItem(ctx context.Context, partitionKey string, item []byte) error {
	return traceCosmosOp(ctx, c, "UpsertItem", func(ctx context.Context) (float64, error) {
		resp, err := c.container.UpsertItem(ctx, azcosmos.NewPartitionKeyString(partitionKey), item, nil)
		if err != nil {
			return 0, err
		}
		return float64(resp.RequestCharge), nil
	})
}

// DeleteItem deletes the document with the given id from its partition.
func (c *CosmosContainer) DeleteItem(ctx context.Context, partitionKey, id string) error {
	return traceCosmosOp(ctx, c, "DeleteItem", func(ctx context.Context) (float64, error) {
		resp, err := c.container.DeleteItem(ctx, azcosmos.NewPartitionKeyString(partitionKey), id, nil)
		if err != nil {
			return 0, err
		}
		return float64(resp.RequestCharge), nil
	})
}

// QueryItems runs a single-partition parametrised query and returns the raw
// document bodies. The query must already be scoped to partitionKey (the SDK
// targets that logical partition), so it never fans out cross-partition. params
// binds @name placeholders.
func (c *CosmosContainer) QueryItems(ctx context.Context, partitionKey, query string, params map[string]any) ([][]byte, error) {
	var items [][]byte
	err := traceCosmosOp(ctx, c, "QueryItems", func(ctx context.Context) (float64, error) {
		opts := &azcosmos.QueryOptions{QueryParameters: queryParams(params)}
		pager := c.container.NewQueryItemsPager(query, azcosmos.NewPartitionKeyString(partitionKey), opts)
		var ru float64
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return 0, err
			}
			ru += float64(page.RequestCharge)
			items = append(items, page.Items...)
		}
		return ru, nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

// QueryItemsLongRead is QueryItems with a longer per-attempt Cosmos budget for
// latency-tolerant build-time reads (the SEO prerender's bounded top-N queries
// over a LARGE authority partition). It overrides the tight 1.5s OLTP TryTimeout
// for this single call only — via policy.WithRetryOptions on the request
// context, which the azcore retry policy reads per call (req.Raw().Context()) —
// so the user-facing endpoints sharing this client keep their 1.5s budget. The
// override flows through traceCosmosOp's span context into the SDK pager. See
// cosmosBuildReadRetryOptions for the budget and tc-9tov for the motivation.
func (c *CosmosContainer) QueryItemsLongRead(ctx context.Context, partitionKey, query string, params map[string]any) ([][]byte, error) {
	ctx = policy.WithRetryOptions(ctx, cosmosBuildReadRetryOptions())
	return c.QueryItems(ctx, partitionKey, query, params)
}

// CountItems runs a SELECT VALUE COUNT(1) scalar query in a single partition and
// returns the integer result. An empty result set (no rows match) yields 0.
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
	var items [][]byte
	err := traceCosmosOp(ctx, c, "QueryItemsCrossPartition", func(ctx context.Context) (float64, error) {
		opts := &azcosmos.QueryOptions{QueryParameters: queryParams(params)}
		pager := c.container.NewQueryItemsPager(query, azcosmos.NewPartitionKey(), opts)
		var ru float64
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return 0, err
			}
			ru += float64(page.RequestCharge)
			items = append(items, page.Items...)
		}
		return ru, nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

// QueryPageCrossPartition runs a cross-partition query and returns a single
// gateway page of up to pageSize documents plus the continuation token for the
// next page (empty when the query is exhausted). A non-empty continuationToken
// resumes a prior page. The Gateway may return inconsistently sized or empty
// pages with a non-nil token — an inherent property of cross-partition queries.
func (c *CosmosContainer) QueryPageCrossPartition(ctx context.Context, query string, params map[string]any, pageSize int, continuationToken string) ([][]byte, string, error) {
	var (
		items [][]byte
		next  string
	)
	err := traceCosmosOp(ctx, c, "QueryPageCrossPartition", func(ctx context.Context) (float64, error) {
		opts := &azcosmos.QueryOptions{QueryParameters: queryParams(params)}
		if pageSize > 0 {
			opts.PageSizeHint = clampPageSize(pageSize)
		}
		if continuationToken != "" {
			opts.ContinuationToken = &continuationToken
		}
		pager := c.container.NewQueryItemsPager(query, azcosmos.NewPartitionKey(), opts)
		if !pager.More() {
			return 0, nil
		}
		page, err := pager.NextPage(ctx)
		if err != nil {
			return 0, err
		}
		items = page.Items
		if page.ContinuationToken != nil {
			next = *page.ContinuationToken
		}
		return float64(page.RequestCharge), nil
	})
	if err != nil {
		return nil, "", err
	}
	return items, next, nil
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
