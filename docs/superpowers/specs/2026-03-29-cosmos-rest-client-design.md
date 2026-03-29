# Cosmos DB REST Client Migration

Date: 2026-03-29

## Problem

The Cosmos DB SDK v3 (3.58.0) fails under Native AOT trimming. The SDK's `DocumentClient.Initialize()` internally calls `ConfigurationManager.AppSettings`, which uses `Activator.CreateInstance()` to instantiate `System.Configuration.ClientConfigurationHost`. The Native AOT trimmer strips that constructor, causing `MissingMethodException` at runtime.

This is not an isolated issue — the SDK has deep Newtonsoft.Json and reflection dependencies throughout its transport layer (`Microsoft.Azure.Cosmos.Direct`, `Microsoft.HybridRow`). Patching individual trimmed types would be a whack-a-mole exercise with no confidence of completeness.

**Alternatives investigated and ruled out:**

- **EF Core 10 + Cosmos provider**: EF Core's Cosmos provider does not support Native AOT. Tracked in dotnet/efcore#34446, targeting EF Core 12 (~late 2027).
- **Microsoft.Azure.Cosmos.Aot (0.1.4-preview.2)**: Experimental, version 0.1.x, still depends on Newtonsoft.Json. Not production-grade.
- **Trimmer root preservation**: Suppresses one failure but cannot guarantee coverage of all reflection-based code paths in the SDK.

## Decision

Replace `Microsoft.Azure.Cosmos` with a thin `HttpClient`-based REST client that talks directly to the Cosmos DB REST API. This is AOT-safe by construction — the entire stack is `HttpClient` + `System.Text.Json` source generators.

## Scope

### Changed (infrastructure layer only)

The application layer (handlers, repository interfaces, domain models) is untouched. All changes are within `town-crier.infrastructure` and `town-crier.web`.

**Removed:**
- `Microsoft.Azure.Cosmos` (3.58.0) package reference
- `Newtonsoft.Json` (13.0.3) package reference
- `CosmosClientFactory` — replaced by `CosmosRestClient` DI registration
- `SystemTextJsonCosmosSerializer` — was an SDK adapter, no longer needed
- `CosmosServiceExtensions` — rewritten for the new client
- `CosmosQueryExtensions` (`CollectAsync`, `FirstOrDefaultAsync`, `ScalarAsync`) — pagination now internal to the client
- Trimming warning suppressions (`IL2104`, `IL3053`, `IL3000`) from `town-crier.web.csproj`

**Added:**
- `Microsoft.Extensions.Http.Resilience` package (Polly v8 wrapper, AOT-compatible)
- `CosmosRestClient` — singleton HTTP client handling auth, headers, pagination
- Polly resilience pipeline for 429/408/503/449 retry

**Rewritten:**
- All 9 `Cosmos*Repository` implementations — mechanical translation from SDK calls to REST client calls
- DI registration for Cosmos services

**Unchanged:**
- All 9 `I*Repository` interfaces (application layer ports)
- All 9 `InMemory*Repository` test doubles
- `CosmosJsonSerializerContext` — reused for STJ source generation
- `CosmosContainerNames` — reused for REST URL construction
- All handlers, endpoints, domain models

### Packages

| Removed | Added |
|---------|-------|
| `Microsoft.Azure.Cosmos` 3.58.0 | `Microsoft.Extensions.Http.Resilience` |
| `Newtonsoft.Json` 13.0.3 | |

## REST Client Design

### Interface

```csharp
public interface ICosmosRestClient
{
    Task<T?> ReadDocumentAsync<T>(string collection, string id,
        string partitionKey, JsonTypeInfo<T> typeInfo, CancellationToken ct);

    Task UpsertDocumentAsync<T>(string collection, T document,
        string partitionKey, JsonTypeInfo<T> typeInfo, CancellationToken ct);

    Task DeleteDocumentAsync(string collection, string id,
        string partitionKey, CancellationToken ct);

    Task<List<T>> QueryAsync<T>(string collection, string sql,
        IReadOnlyList<QueryParameter>? parameters, string? partitionKey,
        JsonTypeInfo<T> typeInfo, CancellationToken ct);

    Task<T> ScalarQueryAsync<T>(string collection, string sql,
        IReadOnlyList<QueryParameter>? parameters, string? partitionKey,
        JsonTypeInfo<T> typeInfo, CancellationToken ct);
}

public readonly record struct QueryParameter(string Name, object Value);
```

### URL Pattern

```
https://{account}.documents.azure.com/dbs/{database}/colls/{collection}/docs[/{id}]
```

### Required Headers

| Header | Value |
|--------|-------|
| `Authorization` | `type=aad&ver=1.0&sig={accessToken}` |
| `x-ms-date` | `DateTime.UtcNow.ToString("R")` |
| `x-ms-version` | `2018-12-31` |
| `x-ms-documentdb-partitionkey` | `["{value}"]` |
| `Content-Type` | `application/json` (docs) or `application/query+json` (queries) |
| `x-ms-documentdb-isquery` | `True` (queries only) |
| `x-ms-documentdb-is-upsert` | `True` (upserts only) |

## Authentication

Managed identity only (Entra ID) via `DefaultAzureCredential`, for both dev and production.

- Dev: `az login` credentials chain through `DefaultAzureCredential`
- Prod: Container App managed identity

Auth header format: `type=aad&ver=1.0&sig={accessToken}`

Token is cached and refreshed on near-expiry via standard `TokenCredential.GetTokenAsync` pattern. `Azure.Identity` 1.19.0 is confirmed AOT-compatible.

Connection string / master key auth is removed entirely.

Configuration:

```json
{
  "Cosmos": {
    "AccountEndpoint": "https://xxx.documents.azure.com:443/",
    "DatabaseName": "town-crier"
  }
}
```

## Serialization

### Request serialization

Existing `CosmosJsonSerializerContext` with all 39 registered types handles document serialization. Each client method takes `JsonTypeInfo<T>` from the caller.

### Response envelope (queries)

The REST API wraps query results:

```json
{
  "Documents": [...],
  "_rid": "dbs/xxxx/colls/yyyy",
  "_count": 3
}
```

Deserialized in two steps to avoid registering generic envelope types:

1. Parse response into `JsonDocument` (always AOT-safe)
2. Extract `Documents` array, deserialize each element with the caller's `JsonTypeInfo<T>`

### Continuation tokens

Returned as `x-ms-continuation` response header. The client loops until no continuation header is present.

### Query body

```json
{
  "query": "SELECT * FROM c WHERE c.userId = @uid",
  "parameters": [{"name": "@uid", "value": "user123"}]
}
```

`CosmosQueryBody` and `CosmosQueryParameter` registered in the serializer context. Parameter values are serialized with `JsonSerializer.Serialize()` via the source-generated context.

## Resilience (Polly)

`Microsoft.Extensions.Http.Resilience` with `IHttpClientFactory`.

### Retry strategy

| Status Code | Meaning | Action |
|-------------|---------|--------|
| 429 | Request rate too large | Retry after `x-ms-retry-after-ms` header |
| 408 | Request timeout | Retry |
| 503 | Service unavailable | Retry |
| 449 | Retry with (write conflict) | Retry |
| All others | Permanent failure | Don't retry |

- Max retries: 5
- Backoff: exponential with jitter, base delay 500ms
- 429 responses honour the `x-ms-retry-after-ms` header when present
- No circuit breaker (Cosmos DB is the only data store; nothing to fail over to)

## Repository Migration

All 9 repositories are mechanical translations. Operation mapping:

| Operation | Before (SDK) | After (REST) |
|-----------|-------------|-------------|
| Point read | `container.ReadItemAsync<T>(id, pk)` | `client.ReadDocumentAsync(collection, id, pk, typeInfo)` |
| Upsert | `container.UpsertItemAsync(doc, pk)` | `client.UpsertDocumentAsync(collection, doc, pk, typeInfo)` |
| Delete | `container.DeleteItemAsync(id, pk)` + catch 404 | `client.DeleteDocumentAsync(collection, id, pk)` (404 silent) |
| Query | `container.GetItemQueryIterator` + `CollectAsync` | `client.QueryAsync(collection, sql, params, pk, typeInfo)` |
| Scalar | `iterator.ScalarAsync` | `client.ScalarQueryAsync(collection, sql, params, pk, typeInfo)` |

Spatial queries (`ST_DISTANCE`) work identically — they're server-side Cosmos SQL functions, evaluated the same whether the query arrives via SDK or REST.

## Testing

### CosmosRestClient unit tests (stub HttpMessageHandler)

- Correct URL paths for each operation
- Correct headers on each request type
- Auth header format
- Query body serialization (SQL + parameters)
- Pagination draining (two responses with continuation token)
- 404 on read returns null, 404 on delete succeeds silently
- Non-retryable errors (400, 403, 409) throw immediately

### Auth token caching tests

- Token reused until near-expiry
- Token refreshed when expiring
- Mock `TokenCredential`

### Integration smoke test

- Single test hitting real dev Cosmos DB account
- Create, read, query, delete cycle
- Gated behind environment variable / test category (not in CI by default)

### Unchanged

- All handler tests (use `InMemory*Repository` — unaffected)
- All domain model tests
- Test builder patterns
