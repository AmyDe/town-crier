# Admin List Users Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a paginated admin API endpoint and CLI command to list users by email, user ID, and subscription tier, with optional email search.

**Architecture:** New `QueryPageAsync` method on `ICosmosRestClient` for single-page Cosmos queries with continuation tokens. Repository, handler, and endpoint follow existing CQRS patterns. CLI uses interactive auto-paging.

**Tech Stack:** .NET 10 (Native AOT), Cosmos DB REST API, TUnit, ASP.NET Core Minimal APIs

**Spec:** `docs/superpowers/specs/2026-04-15-admin-list-users-design.md`

---

### Task 1: Infrastructure — PagedQueryResult and QueryPageAsync

**Files:**
- Create: `api/src/town-crier.infrastructure/Cosmos/PagedQueryResult.cs`
- Modify: `api/src/town-crier.infrastructure/Cosmos/ICosmosRestClient.cs:27-41`
- Modify: `api/src/town-crier.infrastructure/Cosmos/CosmosRestClient.cs:203-218`
- Modify: `api/tests/town-crier.infrastructure.tests/Cosmos/FakeCosmosRestClient.cs:13-15`

- [ ] **Step 1: Create the PagedQueryResult record**

Create `api/src/town-crier.infrastructure/Cosmos/PagedQueryResult.cs`:

```csharp
namespace TownCrier.Infrastructure.Cosmos;

public sealed record PagedQueryResult<T>(IReadOnlyList<T> Items, string? ContinuationToken);
```

- [ ] **Step 2: Add QueryPageAsync to the ICosmosRestClient interface**

In `api/src/town-crier.infrastructure/Cosmos/ICosmosRestClient.cs`, add after the `ScalarQueryAsync` method (after line 41):

```csharp
    Task<PagedQueryResult<T>> QueryPageAsync<T>(
        string collection,
        string sql,
        IReadOnlyList<QueryParameter>? parameters,
        string? partitionKey,
        int maxItemCount,
        string? continuationToken,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct);
```

- [ ] **Step 3: Implement QueryPageAsync on CosmosRestClient**

In `api/src/town-crier.infrastructure/Cosmos/CosmosRestClient.cs`, add after the `ScalarQueryAsync` method (after line 218):

```csharp
    public async Task<PagedQueryResult<T>> QueryPageAsync<T>(
        string collection,
        string sql,
        IReadOnlyList<QueryParameter>? parameters,
        string? partitionKey,
        int maxItemCount,
        string? continuationToken,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        using var activity = CosmosInstrumentation.Source.StartActivity("Cosmos QueryPage");
        activity?.SetTag("db.system", "cosmosdb");
        activity?.SetTag("db.cosmosdb.container", collection);
        activity?.SetTag("db.operation.name", "QueryPage");

#pragma warning disable CA2000
        using var request = this.BuildQueryRequest(collection, sql, parameters);
#pragma warning restore CA2000
        await this.AddHeadersAsync(request, partitionKey, ct).ConfigureAwait(false);
        AddQueryHeaders(request, partitionKey);
        request.Headers.TryAddWithoutValidation(
            "x-ms-max-item-count", maxItemCount.ToString(System.Globalization.CultureInfo.InvariantCulture));

        if (continuationToken is not null)
        {
            request.Headers.TryAddWithoutValidation("x-ms-continuation", continuationToken);
        }

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);
        await ThrowOnCosmosErrorAsync(response, sql, ct).ConfigureAwait(false);
        RecordResponseMetrics(activity, response);

        var items = new List<T>();
        var stream = await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false);
        await using (stream.ConfigureAwait(false))
        {
            using var doc = await JsonDocument.ParseAsync(stream, cancellationToken: ct)
                .ConfigureAwait(false);

            foreach (var element in doc.RootElement.GetProperty("Documents").EnumerateArray())
            {
                items.Add(element.Deserialize(typeInfo)!);
            }
        }

        var nextContinuation = response.Headers.TryGetValues("x-ms-continuation", out var values)
            ? values.FirstOrDefault()
            : null;

        return new PagedQueryResult<T>(items, nextContinuation);
    }
```

- [ ] **Step 4: Implement QueryPageAsync on FakeCosmosRestClient**

In `api/tests/town-crier.infrastructure.tests/Cosmos/FakeCosmosRestClient.cs`, add a new dictionary field alongside the existing `cannedQueryResults` (after line 14):

```csharp
    private readonly Dictionary<string, (object Results, string? ContinuationToken)> cannedPageResults = new();
```

Add a setup helper after the existing `SetQueryResults` method (after line 28):

```csharp
    public void SetPageQueryResults<T>(string sqlPrefix, List<T> results, string? continuationToken = null)
    {
        this.cannedPageResults[sqlPrefix] = (results, continuationToken);
    }
```

Add the interface implementation after the `ScalarQueryAsync` method (after line 149):

```csharp
    public Task<PagedQueryResult<T>> QueryPageAsync<T>(
        string collection,
        string sql,
        IReadOnlyList<QueryParameter>? parameters,
        string? partitionKey,
        int maxItemCount,
        string? continuationToken,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        foreach (var (prefix, (value, token)) in this.cannedPageResults)
        {
            if (sql.StartsWith(prefix, StringComparison.Ordinal) && value is List<T> canned)
            {
                return Task.FromResult(new PagedQueryResult<T>(canned, token));
            }
        }

        var results = new List<T>();
        foreach (var ((c, _, pk), json) in this.store)
        {
            if (c != collection)
            {
                continue;
            }

            if (partitionKey is not null && pk != partitionKey)
            {
                continue;
            }

            var item = JsonSerializer.Deserialize(json, typeInfo);
            if (item is not null)
            {
                results.Add(item);
            }
        }

        return Task.FromResult(new PagedQueryResult<T>(results, null));
    }
```

- [ ] **Step 5: Verify the build compiles**

Run: `dotnet build api/`
Expected: Build succeeded with 0 errors.

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.infrastructure/Cosmos/PagedQueryResult.cs \
       api/src/town-crier.infrastructure/Cosmos/ICosmosRestClient.cs \
       api/src/town-crier.infrastructure/Cosmos/CosmosRestClient.cs \
       api/tests/town-crier.infrastructure.tests/Cosmos/FakeCosmosRestClient.cs
git commit -m "feat(api): add QueryPageAsync to Cosmos REST client for single-page queries"
```

---

### Task 2: Repository — ListAsync with TDD

**Files:**
- Create: `api/src/town-crier.application/UserProfiles/UserProfilePage.cs`
- Modify: `api/src/town-crier.application/UserProfiles/IUserProfileRepository.cs:6-20`
- Modify: `api/src/town-crier.infrastructure/UserProfiles/CosmosUserProfileRepository.cs:82-104`
- Modify: `api/tests/town-crier.application.tests/UserProfiles/FakeUserProfileRepository.cs:6-65`
- Create: `api/tests/town-crier.infrastructure.tests/UserProfiles/CosmosUserProfileRepositoryListTests.cs`

- [ ] **Step 1: Create the UserProfilePage result type**

Create `api/src/town-crier.application/UserProfiles/UserProfilePage.cs`:

```csharp
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed record UserProfilePage(IReadOnlyList<UserProfile> Profiles, string? ContinuationToken);
```

- [ ] **Step 2: Add ListAsync to the IUserProfileRepository interface**

In `api/src/town-crier.application/UserProfiles/IUserProfileRepository.cs`, add before the `SaveAsync` method (before line 17):

```csharp
    Task<UserProfilePage> ListAsync(
        string? emailSearch, int pageSize, string? continuationToken, CancellationToken ct);
```

- [ ] **Step 3: Add stub implementation to FakeUserProfileRepository**

In `api/tests/town-crier.application.tests/UserProfiles/FakeUserProfileRepository.cs`, add after the `DeleteAsync` method (after line 58):

```csharp
    public Task<UserProfilePage> ListAsync(
        string? emailSearch, int pageSize, string? continuationToken, CancellationToken ct)
    {
        var profiles = this.store.Values.AsEnumerable();

        if (emailSearch is not null)
        {
            profiles = profiles.Where(p =>
                p.Email is not null &&
                p.Email.Contains(emailSearch, StringComparison.OrdinalIgnoreCase));
        }

        var result = profiles.OrderBy(p => p.Email, StringComparer.OrdinalIgnoreCase).ToList();
        return Task.FromResult(new UserProfilePage(result, null));
    }
```

- [ ] **Step 4: Write failing repository tests**

Create `api/tests/town-crier.infrastructure.tests/UserProfiles/CosmosUserProfileRepositoryListTests.cs`:

```csharp
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.Tests.Cosmos;
using TownCrier.Infrastructure.UserProfiles;

namespace TownCrier.Infrastructure.Tests.UserProfiles;

public sealed class CosmosUserProfileRepositoryListTests
{
    [Test]
    public async Task Should_ReturnProfiles_When_NoSearchTerm()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosUserProfileRepository(client);

        var docs = new List<UserProfileDocument>
        {
            CreateDocument("user-1", "alice@example.com", "Pro"),
            CreateDocument("user-2", "bob@example.com", "Free"),
        };
        client.SetPageQueryResults("SELECT * FROM c ORDER BY", docs);

        // Act
        var result = await repo.ListAsync(null, 20, null, CancellationToken.None);

        // Assert
        await Assert.That(result.Profiles).HasCount().EqualTo(2);
        await Assert.That(result.Profiles[0].UserId).IsEqualTo("user-1");
        await Assert.That(result.Profiles[0].Email).IsEqualTo("alice@example.com");
    }

    [Test]
    public async Task Should_ReturnProfiles_When_SearchTermProvided()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosUserProfileRepository(client);

        var docs = new List<UserProfileDocument>
        {
            CreateDocument("user-1", "alice@gmail.com", "Personal"),
        };
        client.SetPageQueryResults("SELECT * FROM c WHERE CONTAINS", docs);

        // Act
        var result = await repo.ListAsync("gmail", 20, null, CancellationToken.None);

        // Assert
        await Assert.That(result.Profiles).HasCount().EqualTo(1);
        await Assert.That(result.Profiles[0].Email).IsEqualTo("alice@gmail.com");
    }

    [Test]
    public async Task Should_ForwardContinuationToken_When_MorePagesExist()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosUserProfileRepository(client);

        var docs = new List<UserProfileDocument>
        {
            CreateDocument("user-1", "alice@example.com", "Free"),
        };
        client.SetPageQueryResults("SELECT * FROM c ORDER BY", docs, "next-page-token");

        // Act
        var result = await repo.ListAsync(null, 1, null, CancellationToken.None);

        // Assert
        await Assert.That(result.ContinuationToken).IsEqualTo("next-page-token");
    }

    private static UserProfileDocument CreateDocument(string userId, string email, string tier)
    {
        return new UserProfileDocument
        {
            Id = userId,
            UserId = userId,
            Email = email,
            PushEnabled = true,
            DigestDay = DayOfWeek.Monday,
            EmailDigestEnabled = true,
            ZonePreferences = new Dictionary<string, TownCrier.Domain.UserProfiles.ZoneNotificationPreferences>(),
            Tier = tier,
        };
    }
}
```

- [ ] **Step 5: Run tests to verify they fail**

Run: `dotnet test api/tests/town-crier.infrastructure.tests/ --filter "CosmosUserProfileRepositoryListTests"`
Expected: FAIL — `CosmosUserProfileRepository` does not implement `ListAsync`.

- [ ] **Step 6: Implement ListAsync on CosmosUserProfileRepository**

In `api/src/town-crier.infrastructure/UserProfiles/CosmosUserProfileRepository.cs`, add after the `GetByOriginalTransactionIdAsync` method (after line 81):

```csharp
    public async Task<UserProfilePage> ListAsync(
        string? emailSearch,
        int pageSize,
        string? continuationToken,
        CancellationToken ct)
    {
        var sql = emailSearch is not null
            ? "SELECT * FROM c WHERE CONTAINS(c.email, @search, true) ORDER BY c.email"
            : "SELECT * FROM c ORDER BY c.email";

        var parameters = emailSearch is not null
            ? new[] { new QueryParameter("@search", emailSearch) }
            : Array.Empty<QueryParameter>();

        var result = await this.client.QueryPageAsync(
            CosmosContainerNames.Users,
            sql,
            parameters,
            partitionKey: null,
            pageSize,
            continuationToken,
            CosmosJsonSerializerContext.Default.UserProfileDocument,
            ct).ConfigureAwait(false);

        var profiles = result.Items.Select(doc => doc.ToDomain()).ToList();
        return new UserProfilePage(profiles, result.ContinuationToken);
    }
```

Add `using TownCrier.Application.UserProfiles;` at the top if not already present (it already is on line 1).

- [ ] **Step 7: Run tests to verify they pass**

Run: `dotnet test api/tests/town-crier.infrastructure.tests/ --filter "CosmosUserProfileRepositoryListTests"`
Expected: All 3 tests PASS.

- [ ] **Step 8: Commit**

```bash
git add api/src/town-crier.application/UserProfiles/UserProfilePage.cs \
       api/src/town-crier.application/UserProfiles/IUserProfileRepository.cs \
       api/src/town-crier.infrastructure/UserProfiles/CosmosUserProfileRepository.cs \
       api/tests/town-crier.application.tests/UserProfiles/FakeUserProfileRepository.cs \
       api/tests/town-crier.infrastructure.tests/UserProfiles/CosmosUserProfileRepositoryListTests.cs
git commit -m "feat(api): add paginated ListAsync to user profile repository"
```

---

### Task 3: Application Layer — ListUsersQueryHandler with TDD

**Files:**
- Create: `api/src/town-crier.application/Admin/ListUsersQuery.cs`
- Create: `api/src/town-crier.application/Admin/ListUsersResult.cs`
- Create: `api/src/town-crier.application/Admin/ListUsersQueryHandler.cs`
- Create: `api/tests/town-crier.application.tests/Admin/ListUsersQueryHandlerTests.cs`

- [ ] **Step 1: Create the query record**

Create `api/src/town-crier.application/Admin/ListUsersQuery.cs`:

```csharp
namespace TownCrier.Application.Admin;

public sealed record ListUsersQuery(string? SearchTerm, int PageSize, string? ContinuationToken);
```

- [ ] **Step 2: Create the result types**

Create `api/src/town-crier.application/Admin/ListUsersResult.cs`:

```csharp
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Admin;

public sealed record ListUsersResult(IReadOnlyList<ListUsersItem> Items, string? ContinuationToken);

public sealed record ListUsersItem(string UserId, string? Email, SubscriptionTier Tier);
```

- [ ] **Step 3: Write failing handler tests**

Create `api/tests/town-crier.application.tests/Admin/ListUsersQueryHandlerTests.cs`:

```csharp
using TownCrier.Application.Admin;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.Admin;

public sealed class ListUsersQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnMappedItems_When_ProfilesExist()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        await repository.SaveAsync(
            UserProfile.Register("auth0|user-1", "alice@example.com"), CancellationToken.None);
        await repository.SaveAsync(
            UserProfile.Register("auth0|user-2", "bob@example.com"), CancellationToken.None);

        var handler = new ListUsersQueryHandler(repository);
        var query = new ListUsersQuery(null, 20, null);

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result.Items).HasCount().EqualTo(2);
        await Assert.That(result.Items[0].UserId).IsEqualTo("auth0|user-1");
        await Assert.That(result.Items[0].Email).IsEqualTo("alice@example.com");
        await Assert.That(result.Items[0].Tier).IsEqualTo(SubscriptionTier.Free);
    }

    [Test]
    public async Task Should_FilterByEmail_When_SearchTermProvided()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        await repository.SaveAsync(
            UserProfile.Register("auth0|user-1", "alice@gmail.com"), CancellationToken.None);
        await repository.SaveAsync(
            UserProfile.Register("auth0|user-2", "bob@outlook.com"), CancellationToken.None);

        var handler = new ListUsersQueryHandler(repository);
        var query = new ListUsersQuery("gmail", 20, null);

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result.Items).HasCount().EqualTo(1);
        await Assert.That(result.Items[0].Email).IsEqualTo("alice@gmail.com");
    }

    [Test]
    public async Task Should_ReturnEmptyList_When_NoProfilesExist()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new ListUsersQueryHandler(repository);
        var query = new ListUsersQuery(null, 20, null);

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result.Items).HasCount().EqualTo(0);
        await Assert.That(result.ContinuationToken).IsNull();
    }

    [Test]
    public async Task Should_MapTierCorrectly_When_UserHasProSubscription()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-1", "alice@example.com");
        profile.ActivateSubscription(SubscriptionTier.Pro, DateTimeOffset.UtcNow.AddYears(1));
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new ListUsersQueryHandler(repository);
        var query = new ListUsersQuery(null, 20, null);

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result.Items[0].Tier).IsEqualTo(SubscriptionTier.Pro);
    }
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `dotnet test api/tests/town-crier.application.tests/ --filter "ListUsersQueryHandlerTests"`
Expected: FAIL — `ListUsersQueryHandler` does not exist.

- [ ] **Step 5: Implement the handler**

Create `api/src/town-crier.application/Admin/ListUsersQueryHandler.cs`:

```csharp
using TownCrier.Application.UserProfiles;

namespace TownCrier.Application.Admin;

public sealed class ListUsersQueryHandler
{
    private readonly IUserProfileRepository repository;

    public ListUsersQueryHandler(IUserProfileRepository repository)
    {
        this.repository = repository;
    }

    public async Task<ListUsersResult> HandleAsync(ListUsersQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var page = await this.repository.ListAsync(
            query.SearchTerm,
            query.PageSize,
            query.ContinuationToken,
            ct).ConfigureAwait(false);

        var items = page.Profiles
            .Select(p => new ListUsersItem(p.UserId, p.Email, p.Tier))
            .ToList();

        return new ListUsersResult(items, page.ContinuationToken);
    }
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `dotnet test api/tests/town-crier.application.tests/ --filter "ListUsersQueryHandlerTests"`
Expected: All 4 tests PASS.

- [ ] **Step 7: Run the full test suite**

Run: `dotnet test api/`
Expected: All tests PASS (including existing tests — confirms no regressions).

- [ ] **Step 8: Commit**

```bash
git add api/src/town-crier.application/Admin/ListUsersQuery.cs \
       api/src/town-crier.application/Admin/ListUsersResult.cs \
       api/src/town-crier.application/Admin/ListUsersQueryHandler.cs \
       api/tests/town-crier.application.tests/Admin/ListUsersQueryHandlerTests.cs
git commit -m "feat(api): add ListUsersQueryHandler for paginated admin user listing"
```

---

### Task 4: API Endpoint Wiring

**Files:**
- Modify: `api/src/town-crier.web/Endpoints/AdminEndpoints.cs:6-30`
- Modify: `api/src/town-crier.web/AppJsonSerializerContext.cs:1-55`
- Modify: `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs:170`

- [ ] **Step 1: Register the handler in DI**

In `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs`, add after the `GrantSubscriptionCommandHandler` registration (after line 170):

```csharp
        services.AddTransient<ListUsersQueryHandler>();
```

- [ ] **Step 2: Add serialization types to AppJsonSerializerContext**

In `api/src/town-crier.web/AppJsonSerializerContext.cs`, add after the `[JsonSerializable(typeof(GrantSubscriptionResult))]` line (after line 54):

```csharp
[JsonSerializable(typeof(ListUsersResult))]
```

- [ ] **Step 3: Add the GET endpoint**

In `api/src/town-crier.web/Endpoints/AdminEndpoints.cs`, add after the existing `admin.MapPut("/subscriptions", ...)` block (after line 28):

```csharp
        admin.MapGet("/users", async (
            string? search,
            int? pageSize,
            string? continuationToken,
            ListUsersQueryHandler handler,
            CancellationToken ct) =>
        {
            var query = new ListUsersQuery(search, pageSize ?? 20, continuationToken);
            var result = await handler.HandleAsync(query, ct).ConfigureAwait(false);
            return Results.Ok(result);
        });
```

- [ ] **Step 4: Verify the build and all tests pass**

Run: `dotnet build api/ && dotnet test api/`
Expected: Build succeeded, all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add api/src/town-crier.web/Endpoints/AdminEndpoints.cs \
       api/src/town-crier.web/AppJsonSerializerContext.cs \
       api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs
git commit -m "feat(api): wire GET /v1/admin/users endpoint for paginated user listing"
```

---

### Task 5: CLI — list-users Command

**Files:**
- Modify: `cli/src/tc/ApiClient.cs:6-34`
- Modify: `cli/src/tc/Json/TcJsonContext.cs:1-7`
- Create: `cli/src/tc/Json/ListUsersResponse.cs`
- Create: `cli/src/tc/Commands/ListUsersCommand.cs`
- Modify: `cli/src/tc/Program.cs:40-44`

- [ ] **Step 1: Add GetFromJsonAsync to ApiClient**

In `cli/src/tc/ApiClient.cs`, add the following `using` at the top:

```csharp
using System.Net.Http.Json;
```

(This `using` is already present on line 1 — verify before adding.)

Add the new method after the `PutAsJsonAsync` method (after line 28):

```csharp
    public async Task<TResponse?> GetFromJsonAsync<TResponse>(
        string path,
        JsonTypeInfo<TResponse> typeInfo,
        CancellationToken ct)
    {
        using var response = await this.client.GetAsync(path, ct).ConfigureAwait(false);

        if (!response.IsSuccessStatusCode)
        {
            var body = await response.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
            throw new HttpRequestException(
                $"API error ({(int)response.StatusCode}): {body}");
        }

        return await response.Content.ReadFromJsonAsync(typeInfo, ct).ConfigureAwait(false);
    }
```

- [ ] **Step 2: Create the CLI JSON response types**

Create `cli/src/tc/Json/ListUsersResponse.cs`:

```csharp
namespace Tc.Json;

internal sealed class ListUsersResponse
{
    public required IReadOnlyList<ListUsersItemResponse> Items { get; init; }

    public string? ContinuationToken { get; init; }
}

internal sealed class ListUsersItemResponse
{
    public required string UserId { get; init; }

    public string? Email { get; init; }

    public required string Tier { get; init; }
}
```

- [ ] **Step 3: Register the response type in TcJsonContext**

Replace the content of `cli/src/tc/Json/TcJsonContext.cs` with:

```csharp
using System.Text.Json.Serialization;

namespace Tc.Json;

[JsonSourceGenerationOptions(PropertyNamingPolicy = JsonKnownNamingPolicy.CamelCase)]
[JsonSerializable(typeof(ConfigFile))]
[JsonSerializable(typeof(GrantSubscriptionRequest))]
[JsonSerializable(typeof(ListUsersResponse))]
internal sealed partial class TcJsonContext : JsonSerializerContext;
```

The `[JsonSourceGenerationOptions(PropertyNamingPolicy = JsonKnownNamingPolicy.CamelCase)]` attribute is required because the API serializes with camelCase (ASP.NET Core default). Without it, the source generator expects PascalCase property names in JSON, and deserialization of `continuationToken`, `userId` etc. would fail silently (properties stay null/default).

Existing types are unaffected: `ConfigFile` uses explicit `[JsonPropertyName]` which overrides the policy; `GrantSubscriptionRequest` switches from PascalCase to camelCase serialization but the API accepts both (ASP.NET Core deserializes case-insensitively).

- [ ] **Step 4: Implement the ListUsersCommand**

Create `cli/src/tc/Commands/ListUsersCommand.cs`:

```csharp
using System.Globalization;
using System.Text;
using Tc.Json;

namespace Tc.Commands;

internal static class ListUsersCommand
{
    public static async Task<int> RunAsync(ApiClient client, ParsedArgs args, CancellationToken ct)
    {
        var search = args.GetOptional("search");
        var pageSizeStr = args.GetOptional("page-size");
        var pageSize = 20;

        if (pageSizeStr is not null
            && (!int.TryParse(pageSizeStr, NumberStyles.None, CultureInfo.InvariantCulture, out pageSize)
                || pageSize <= 0))
        {
            await Console.Error.WriteLineAsync("Invalid --page-size: must be a positive integer")
                .ConfigureAwait(false);
            return 1;
        }

        string? continuationToken = null;

        do
        {
            var path = BuildPath(search, pageSize, continuationToken);

            ListUsersResponse? response;
            try
            {
                response = await client.GetFromJsonAsync(path, TcJsonContext.Default.ListUsersResponse, ct)
                    .ConfigureAwait(false);
            }
            catch (HttpRequestException ex)
            {
                await Console.Error.WriteLineAsync(ex.Message).ConfigureAwait(false);
                return 2;
            }

            if (response is null)
            {
                await Console.Error.WriteLineAsync("Empty response from API").ConfigureAwait(false);
                return 2;
            }

            PrintTable(response);
            continuationToken = response.ContinuationToken;

            if (continuationToken is null)
            {
                break;
            }

            await Console.Out.WriteAsync("Next page? [y/N] ").ConfigureAwait(false);
            var input = Console.ReadLine();
            if (!string.Equals(input?.Trim(), "y", StringComparison.OrdinalIgnoreCase))
            {
                break;
            }
        }
        while (true);

        return 0;
    }

    private static string BuildPath(string? search, int pageSize, string? continuationToken)
    {
        var sb = new StringBuilder("/v1/admin/users?pageSize=");
        sb.Append(pageSize.ToString(CultureInfo.InvariantCulture));

        if (search is not null)
        {
            sb.Append("&search=");
            sb.Append(Uri.EscapeDataString(search));
        }

        if (continuationToken is not null)
        {
            sb.Append("&continuationToken=");
            sb.Append(Uri.EscapeDataString(continuationToken));
        }

        return sb.ToString();
    }

    private static void PrintTable(ListUsersResponse response)
    {
        Console.WriteLine($"{"UserId",-24} {"Email",-32} {"Tier",-10}");
        Console.WriteLine(new string('-', 66));

        foreach (var item in response.Items)
        {
            Console.WriteLine(
                $"{item.UserId,-24} {item.Email ?? "(none)",-32} {item.Tier,-10}");
        }
    }
}
```

- [ ] **Step 5: Wire the command into Program.cs**

In `cli/src/tc/Program.cs`, add the new command to the switch expression (line 40-44). Replace the existing switch:

```csharp
return parsed.Command switch
{
    "grant-subscription" => await GrantSubscriptionCommand.RunAsync(client, parsed, cts.Token).ConfigureAwait(false),
    "list-users" => await ListUsersCommand.RunAsync(client, parsed, cts.Token).ConfigureAwait(false),
    _ => await UnknownCommandAsync(parsed.Command).ConfigureAwait(false),
};
```

- [ ] **Step 6: Update the help text**

In `cli/src/tc/Program.cs`, replace the help text in `PrintHelpAsync` (line 55-71):

```csharp
static async Task PrintHelpAsync()
{
    await Console.Out.WriteLineAsync("""
        tc — Town Crier admin CLI

        Usage: tc <command> [options]

        Commands:
          grant-subscription   Grant or change a user's subscription tier
          list-users           List users with email, ID, and subscription tier
          help                 Show this help message
          version              Print version

        list-users options:
          --search <term>      Filter by email substring (case-insensitive)
          --page-size <n>      Results per page (default: 20)

        Global options:
          --url <url>          API base URL (overrides config file)
          --api-key <key>      Admin API key (overrides config file)

        Config file: ~/.config/tc/config.json
        """).ConfigureAwait(false);
}
```

- [ ] **Step 7: Verify the CLI builds and tests pass**

Run: `dotnet build cli/ && dotnet test cli/`
Expected: Build succeeded, all existing CLI tests PASS.

- [ ] **Step 8: Run full solution build and test**

Run: `dotnet build api/ && dotnet test api/ && dotnet build cli/ && dotnet test cli/`
Expected: All builds succeed, all tests PASS across both projects.

- [ ] **Step 9: Commit**

```bash
git add cli/src/tc/ApiClient.cs \
       cli/src/tc/Json/ListUsersResponse.cs \
       cli/src/tc/Json/TcJsonContext.cs \
       cli/src/tc/Commands/ListUsersCommand.cs \
       cli/src/tc/Program.cs
git commit -m "feat(cli): add list-users command with paginated output and email search"
```
