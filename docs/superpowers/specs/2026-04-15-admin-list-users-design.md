# Admin List Users API + CLI

Date: 2026-04-15

## Overview

Add an admin API endpoint and CLI command to list users with their email address, user ID, and subscription tier. Supports pagination (default page size 20) with Cosmos DB continuation tokens, optional email substring search, and results ordered by email ascending.

## API Endpoint

```
GET /v1/admin/users?search={term}&pageSize={n}&continuationToken={token}
```

Added to the existing `AdminEndpoints.cs` admin route group. Inherits `AdminApiKeyFilter` (API key via `X-Admin-Key`) and `.AllowAnonymous()` (bypasses JWT) from the group.

### Query Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `search` | No | — | Case-insensitive substring match on email |
| `pageSize` | No | 20 | Items per page |
| `continuationToken` | No | — | Opaque token from a previous response to fetch the next page |

### Response

```json
{
  "items": [
    { "userId": "auth0|abc123", "email": "alice@gmail.com", "tier": "Pro" },
    { "userId": "auth0|def456", "email": "bob@gmail.com", "tier": "Free" }
  ],
  "continuationToken": "eyJ..."
}
```

`continuationToken` is `null` when there are no more pages.

## Application Layer

All types in `TownCrier.Application.Admin`:

| Type | Shape |
|------|-------|
| `ListUsersQuery` | `record(string? SearchTerm, int PageSize, string? ContinuationToken)` |
| `ListUsersQueryHandler` | Sealed class. `HandleAsync` calls repository, maps to result. |
| `ListUsersResult` | `record(IReadOnlyList<ListUsersItem> Items, string? ContinuationToken)` |
| `ListUsersItem` | `record(string UserId, string? Email, SubscriptionTier Tier)` |

The handler is a thin orchestrator with no domain logic — this is a read-only admin projection.

## Infrastructure Layer

### New Cosmos Paging Primitive

The existing `ICosmosRestClient.QueryAsync` drains all continuation pages into a single list. A new method returns one page at a time:

```csharp
// New type
public sealed record PagedQueryResult<T>(IReadOnlyList<T> Items, string? ContinuationToken);

// New method on ICosmosRestClient
Task<PagedQueryResult<T>> QueryPageAsync<T>(
    string collection,
    string sql,
    IReadOnlyList<QueryParameter> parameters,
    string? partitionKey,
    int maxItemCount,
    string? continuationToken,
    JsonTypeInfo<T> jsonTypeInfo,
    CancellationToken ct);
```

Implementation: identical to the first iteration of the existing `QueryAsync` loop, but:
- Sets `x-ms-max-item-count` header to `maxItemCount`
- Passes the incoming continuation token via `x-ms-continuation` request header
- Returns after a **single** Cosmos REST request with the continuation token from the response header

Existing `QueryAsync` is untouched — zero risk to current code paths.

### New Repository Method

```csharp
// On IUserProfileRepository
Task<PagedQueryResult<UserProfile>> ListAsync(
    string? emailSearch,
    int pageSize,
    string? continuationToken,
    CancellationToken ct);
```

**Cosmos SQL (with search):**
```sql
SELECT * FROM c WHERE CONTAINS(c.Email, @search, true) ORDER BY c.Email
```

**Cosmos SQL (without search):**
```sql
SELECT * FROM c ORDER BY c.Email
```

Both are cross-partition queries (`partitionKey: null`).

## CLI — `list-users` Command

### Invocation

```
tc list-users [--search <term>] [--page-size <n>]
```

### Behaviour

1. Parse flags, build query string
2. `GET /v1/admin/users?search=...&pageSize=...`
3. Print fixed-width table:
   ```
   UserId                  Email                         Tier
   auth0|abc123            alice@gmail.com               Pro
   auth0|def456            bob@gmail.com                 Free
   ```
4. If continuation token present, prompt: `Next page? [y/N]`
5. On `y` — repeat from step 2 with `&continuationToken=...`
6. On `N`, empty input, or no more pages — exit with code 0

### ApiClient Changes

Add `GetFromJsonAsync<TResponse>` method alongside the existing `PutAsJsonAsync`. Accepts a path with query string, deserialises the response using AOT-compatible `JsonTypeInfo<TResponse>`.

### Serialisation

New types registered in `TcJsonContext`:

```csharp
public sealed record ListUsersResponse(
    IReadOnlyList<ListUsersItemResponse> Items,
    string? ContinuationToken);

public sealed record ListUsersItemResponse(
    string UserId,
    string? Email,
    string Tier);
```

## Testing Strategy

| Layer | What to test | Approach |
|-------|-------------|----------|
| Handler | Passes search/paging params to repository; maps `UserProfile` list to `ListUsersItem` list; forwards continuation token | `FakeUserProfileRepository` returning pre-built profiles |
| Repository | Builds correct SQL with/without search term; passes `maxItemCount` and continuation token to Cosmos client | `FakeCosmosRestClient` with pre-canned `PagedQueryResult` |
| `QueryPageAsync` | Sets correct headers (`x-ms-max-item-count`, `x-ms-continuation`); returns single page with token | HTTP-level test against `FakeCosmosRestClient` |
| Endpoint | Binds query string params to query record; returns 200 with correct JSON shape | Handler-level fake, verify serialisation |
| CLI arg parsing | `--search`, `--page-size` flags parsed correctly; defaults applied | Unit tests on `ArgParser` / command flag extraction |

## Files Changed

### New Files
- `api/src/town-crier.application/Admin/ListUsersQuery.cs`
- `api/src/town-crier.application/Admin/ListUsersQueryHandler.cs`
- `api/src/town-crier.application/Admin/ListUsersResult.cs`
- `api/src/town-crier.application/Admin/ListUsersItem.cs`
- `cli/src/tc/Commands/ListUsersCommand.cs`
- `cli/src/tc/Json/ListUsersResponse.cs` (or inline in `TcJsonContext.cs`)

### Modified Files
- `api/src/town-crier.infrastructure/Cosmos/ICosmosRestClient.cs` — add `QueryPageAsync`
- `api/src/town-crier.infrastructure/Cosmos/CosmosRestClient.cs` — implement `QueryPageAsync`
- `api/src/town-crier.application/UserProfiles/IUserProfileRepository.cs` — add `ListAsync`
- `api/src/town-crier.infrastructure/UserProfiles/CosmosUserProfileRepository.cs` — implement `ListAsync`
- `api/src/town-crier.web/Endpoints/AdminEndpoints.cs` — add `GET /admin/users`
- `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs` — register `ListUsersQueryHandler`
- `cli/src/tc/Program.cs` — add `"list-users"` case to command switch
- `cli/src/tc/ApiClient.cs` — add `GetFromJsonAsync`
- `cli/src/tc/Json/TcJsonContext.cs` — add serialisation types
- `api/tests/.../FakeCosmosRestClient.cs` — implement `QueryPageAsync`
- `api/tests/.../FakeUserProfileRepository.cs` — implement `ListAsync`

### Test Files (New)
- `api/tests/town-crier.application.tests/Admin/ListUsersQueryHandlerTests.cs`
- `api/tests/town-crier.infrastructure.tests/UserProfiles/CosmosUserProfileRepositoryListTests.cs`
- `cli/tests/tc.tests/ListUsersCommandTests.cs` (arg parsing)
