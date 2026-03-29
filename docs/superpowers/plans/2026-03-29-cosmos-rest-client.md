# Cosmos DB REST Client Migration — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the AOT-incompatible Cosmos DB SDK v3 with a thin HttpClient-based REST client that talks directly to the Cosmos DB REST API.

**Architecture:** The change is entirely within the infrastructure layer. `CosmosRestClient` handles auth (Entra ID), headers, pagination, and error mapping. All 9 repository implementations are mechanically rewritten to call the REST client instead of the SDK. Polly v8 provides retry resilience for transient Cosmos errors.

**Tech Stack:** .NET 10, `HttpClient` + `IHttpClientFactory`, `System.Text.Json` source generators, `Microsoft.Extensions.Http.Resilience` (Polly v8), `Azure.Identity` (`DefaultAzureCredential`)

**Spec:** `docs/superpowers/specs/2026-03-29-cosmos-rest-client-design.md`

---

## File Map

### New files
| File | Responsibility |
|------|---------------|
| `api/src/town-crier.infrastructure/Cosmos/ICosmosRestClient.cs` | Interface + `QueryParameter` record |
| `api/src/town-crier.infrastructure/Cosmos/CosmosRestClient.cs` | HTTP client: auth, headers, pagination, error mapping |
| `api/src/town-crier.infrastructure/Cosmos/CosmosAuthProvider.cs` | Entra ID token acquisition + caching |
| `api/src/town-crier.infrastructure/Cosmos/CosmosRestOptions.cs` | Config POCO (`AccountEndpoint`, `DatabaseName`) |
| `api/tests/town-crier.infrastructure.tests/Cosmos/CosmosRestClientTests.cs` | Unit tests for REST client |
| `api/tests/town-crier.infrastructure.tests/Cosmos/CosmosAuthProviderTests.cs` | Token caching tests |
| `api/tests/town-crier.infrastructure.tests/Cosmos/StubHttpHandler.cs` | Reusable test stub for HttpMessageHandler |

### Modified files
| File | Change |
|------|--------|
| `api/src/town-crier.infrastructure/town-crier.infrastructure.csproj` | Remove `Microsoft.Azure.Cosmos`, `Newtonsoft.Json`; add `Microsoft.Extensions.Http.Resilience`, `Microsoft.Extensions.Http` |
| `api/src/town-crier.web/town-crier.web.csproj` | Remove `NoWarn` for IL2104, IL3053, IL3000 |
| `api/src/town-crier.infrastructure/Cosmos/CosmosJsonSerializerContext.cs` | Add query body types |
| `api/src/town-crier.infrastructure/Cosmos/CosmosServiceExtensions.cs` | Rewrite for REST client + Polly |
| `api/src/town-crier.infrastructure/UserProfiles/CosmosUserProfileRepository.cs` | Rewrite against `ICosmosRestClient` |
| `api/src/town-crier.infrastructure/WatchZones/CosmosWatchZoneRepository.cs` | Rewrite against `ICosmosRestClient` |
| `api/src/town-crier.infrastructure/PlanningApplications/CosmosPlanningApplicationRepository.cs` | Rewrite against `ICosmosRestClient` |
| `api/src/town-crier.infrastructure/DeviceRegistrations/CosmosDeviceRegistrationRepository.cs` | Rewrite against `ICosmosRestClient` |
| `api/src/town-crier.infrastructure/Notifications/CosmosNotificationRepository.cs` | Rewrite against `ICosmosRestClient` |
| `api/src/town-crier.infrastructure/Groups/CosmosGroupRepository.cs` | Rewrite against `ICosmosRestClient` |
| `api/src/town-crier.infrastructure/Groups/CosmosGroupInvitationRepository.cs` | Rewrite against `ICosmosRestClient` |
| `api/src/town-crier.infrastructure/DecisionAlerts/CosmosDecisionAlertRepository.cs` | Rewrite against `ICosmosRestClient` |
| `api/src/town-crier.infrastructure/SavedApplications/CosmosSavedApplicationRepository.cs` | Rewrite against `ICosmosRestClient` |
| `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs` | Update DI registrations |
| `api/tests/town-crier.infrastructure.tests/town-crier.infrastructure.tests.csproj` | Remove Cosmos SDK reference (transitive), add HTTP test packages if needed |

### Deleted files
| File | Reason |
|------|--------|
| `api/src/town-crier.infrastructure/Cosmos/CosmosClientFactory.cs` | Replaced by `CosmosRestClient` |
| `api/src/town-crier.infrastructure/Cosmos/SystemTextJsonCosmosSerializer.cs` | Was SDK adapter, no longer needed |
| `api/src/town-crier.infrastructure/Cosmos/CosmosQueryExtensions.cs` | Pagination now internal to REST client |
| `api/tests/town-crier.infrastructure.tests/Cosmos/CosmosClientFactoryTests.cs` | Tests for deleted code |
| `api/tests/town-crier.infrastructure.tests/Cosmos/CosmosServiceRegistrationTests.cs` | Replaced by new registration tests |
| `api/tests/town-crier.infrastructure.tests/Cosmos/SystemTextJsonCosmosSerializerTests.cs` | Tests for deleted code |
| `api/tests/town-crier.infrastructure.tests/Cosmos/CosmosQueryExtensionsTests.cs` | Tests for deleted code |
| `api/tests/town-crier.infrastructure.tests/Cosmos/FakeFeedIterator.cs` | SDK test helper no longer needed |
| `api/tests/town-crier.infrastructure.tests/Cosmos/FakeFeedResponse.cs` | SDK test helper no longer needed |

---

## Task 1: Package references and project cleanup

**Files:**
- Modify: `api/src/town-crier.infrastructure/town-crier.infrastructure.csproj`
- Modify: `api/src/town-crier.web/town-crier.web.csproj`

- [ ] **Step 1: Update infrastructure .csproj — remove SDK packages, add HTTP resilience**

Replace the contents of `api/src/town-crier.infrastructure/town-crier.infrastructure.csproj` with:

```xml
<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <TargetFramework>net10.0</TargetFramework>
    <RootNamespace>TownCrier.Infrastructure</RootNamespace>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
    <IsAotCompatible>true</IsAotCompatible>
  </PropertyGroup>

  <ItemGroup>
    <InternalsVisibleTo Include="town-crier.infrastructure.tests" />
  </ItemGroup>

  <ItemGroup>
    <PackageReference Include="Azure.Identity" Version="1.19.0" />
    <PackageReference Include="Microsoft.Extensions.Configuration.Abstractions" Version="10.0.*" />
    <PackageReference Include="Microsoft.Extensions.DependencyInjection.Abstractions" Version="10.0.*" />
    <PackageReference Include="Microsoft.Extensions.Http" Version="10.0.*" />
    <PackageReference Include="Microsoft.Extensions.Http.Resilience" Version="10.0.*" />
    <PackageReference Include="Microsoft.Extensions.Logging.Abstractions" Version="10.0.*" />
    <PackageReference Include="Microsoft.Extensions.Options" Version="10.0.*" />
  </ItemGroup>

  <ItemGroup>
    <ProjectReference Include="..\town-crier.application\town-crier.application.csproj" />
  </ItemGroup>

</Project>
```

- [ ] **Step 2: Remove trimming warning suppressions from web .csproj**

In `api/src/town-crier.web/town-crier.web.csproj`, remove the `<NoWarn>` line and its comment block. The `<PropertyGroup>` should become:

```xml
  <PropertyGroup>
    <TargetFramework>net10.0</TargetFramework>
    <RootNamespace>TownCrier.Web</RootNamespace>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
    <PublishAot>true</PublishAot>
    <InterceptorsNamespaces>$(InterceptorsNamespaces);Microsoft.AspNetCore.Http.Generated</InterceptorsNamespaces>
  </PropertyGroup>
```

- [ ] **Step 3: Verify restore succeeds**

Run: `cd api && dotnet restore`

Expected: Restore succeeds. Build will fail at this point (existing code still references `Microsoft.Azure.Cosmos` types) — that's expected and will be fixed in subsequent tasks.

- [ ] **Step 4: Commit**

```bash
git add api/src/town-crier.infrastructure/town-crier.infrastructure.csproj api/src/town-crier.web/town-crier.web.csproj
git commit -m "chore: replace Cosmos SDK packages with HTTP resilience"
```

---

## Task 2: REST client interface, options, and auth provider

**Files:**
- Create: `api/src/town-crier.infrastructure/Cosmos/ICosmosRestClient.cs`
- Create: `api/src/town-crier.infrastructure/Cosmos/CosmosRestOptions.cs`
- Create: `api/src/town-crier.infrastructure/Cosmos/CosmosAuthProvider.cs`
- Create: `api/tests/town-crier.infrastructure.tests/Cosmos/CosmosAuthProviderTests.cs`

- [ ] **Step 1: Create the interface and QueryParameter**

Create `api/src/town-crier.infrastructure/Cosmos/ICosmosRestClient.cs`:

```csharp
using System.Text.Json.Serialization.Metadata;

namespace TownCrier.Infrastructure.Cosmos;

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

- [ ] **Step 2: Create config options**

Create `api/src/town-crier.infrastructure/Cosmos/CosmosRestOptions.cs`:

```csharp
namespace TownCrier.Infrastructure.Cosmos;

public sealed class CosmosRestOptions
{
    public required string AccountEndpoint { get; init; }
    public required string DatabaseName { get; init; }
}
```

- [ ] **Step 3: Write the auth provider test**

Create `api/tests/town-crier.infrastructure.tests/Cosmos/CosmosAuthProviderTests.cs`:

```csharp
using Azure.Core;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Tests.Cosmos;

public sealed class CosmosAuthProviderTests
{
    [Test]
    public async Task GetAuthorizationHeaderAsync_ReturnsEntraIdFormat()
    {
        var credential = new FakeTokenCredential("test-token-123");
        var provider = new CosmosAuthProvider(credential);

        var header = await provider.GetAuthorizationHeaderAsync(CancellationToken.None);

        await Assert.That(header).IsEqualTo("type%3daad%26ver%3d1.0%26sig%3dtest-token-123");
    }

    [Test]
    public async Task GetAuthorizationHeaderAsync_CachesToken()
    {
        var credential = new FakeTokenCredential("token-1");
        var provider = new CosmosAuthProvider(credential);

        var first = await provider.GetAuthorizationHeaderAsync(CancellationToken.None);
        credential.NextToken = "token-2";
        var second = await provider.GetAuthorizationHeaderAsync(CancellationToken.None);

        // Same token because it's cached (not expired)
        await Assert.That(second).IsEqualTo(first);
    }

    [Test]
    public async Task GetAuthorizationHeaderAsync_RefreshesExpiredToken()
    {
        var credential = new FakeTokenCredential("token-1", expiresInMinutes: 0);
        var provider = new CosmosAuthProvider(credential);

        var first = await provider.GetAuthorizationHeaderAsync(CancellationToken.None);
        credential.NextToken = "token-2";
        credential.ExpiresInMinutes = 60;
        var second = await provider.GetAuthorizationHeaderAsync(CancellationToken.None);

        await Assert.That(first).Contains("token-1");
        await Assert.That(second).Contains("token-2");
    }

    private sealed class FakeTokenCredential : TokenCredential
    {
        public string NextToken { get; set; }
        public int ExpiresInMinutes { get; set; }

        public FakeTokenCredential(string token, int expiresInMinutes = 60)
        {
            NextToken = token;
            ExpiresInMinutes = expiresInMinutes;
        }

        public override AccessToken GetToken(TokenRequestContext requestContext, CancellationToken cancellationToken) =>
            new(NextToken, DateTimeOffset.UtcNow.AddMinutes(ExpiresInMinutes));

        public override ValueTask<AccessToken> GetTokenAsync(TokenRequestContext requestContext, CancellationToken cancellationToken) =>
            ValueTask.FromResult(new AccessToken(NextToken, DateTimeOffset.UtcNow.AddMinutes(ExpiresInMinutes)));
    }
}
```

- [ ] **Step 4: Run tests — verify they fail**

Run: `cd api && dotnet test tests/town-crier.infrastructure.tests --filter "CosmosAuthProviderTests" --no-restore`

Expected: Compilation error — `CosmosAuthProvider` does not exist yet.

- [ ] **Step 5: Implement the auth provider**

Create `api/src/town-crier.infrastructure/Cosmos/CosmosAuthProvider.cs`:

```csharp
using System.Net;
using Azure.Core;

namespace TownCrier.Infrastructure.Cosmos;

internal sealed class CosmosAuthProvider
{
    private static readonly string[] Scopes = ["https://cosmos.azure.com/.default"];
    private readonly TokenCredential credential;
    private AccessToken cachedToken;

    public CosmosAuthProvider(TokenCredential credential)
    {
        ArgumentNullException.ThrowIfNull(credential);
        this.credential = credential;
    }

    public async Task<string> GetAuthorizationHeaderAsync(CancellationToken ct)
    {
        if (cachedToken.ExpiresOn <= DateTimeOffset.UtcNow.AddMinutes(5))
        {
            cachedToken = await credential.GetTokenAsync(
                new TokenRequestContext(Scopes), ct).ConfigureAwait(false);
        }

        // Cosmos DB Entra ID auth format: type=aad&ver=1.0&sig={token} (URL-encoded)
        return WebUtility.UrlEncode($"type=aad&ver=1.0&sig={cachedToken.Token}");
    }
}
```

- [ ] **Step 6: Run tests — verify they pass**

Run: `cd api && dotnet test tests/town-crier.infrastructure.tests --filter "CosmosAuthProviderTests"`

Expected: 3 tests pass.

- [ ] **Step 7: Commit**

```bash
git add api/src/town-crier.infrastructure/Cosmos/ICosmosRestClient.cs \
       api/src/town-crier.infrastructure/Cosmos/CosmosRestOptions.cs \
       api/src/town-crier.infrastructure/Cosmos/CosmosAuthProvider.cs \
       api/tests/town-crier.infrastructure.tests/Cosmos/CosmosAuthProviderTests.cs
git commit -m "feat: add Cosmos REST client interface, options, and auth provider"
```

---

## Task 3: CosmosRestClient implementation

**Files:**
- Modify: `api/src/town-crier.infrastructure/Cosmos/CosmosJsonSerializerContext.cs`
- Create: `api/src/town-crier.infrastructure/Cosmos/CosmosRestClient.cs`
- Create: `api/tests/town-crier.infrastructure.tests/Cosmos/StubHttpHandler.cs`
- Create: `api/tests/town-crier.infrastructure.tests/Cosmos/CosmosRestClientTests.cs`

- [ ] **Step 1: Add query body types to the serializer context**

Replace `api/src/town-crier.infrastructure/Cosmos/CosmosJsonSerializerContext.cs` with:

```csharp
using System.Text.Json.Serialization;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.DecisionAlerts;
using TownCrier.Infrastructure.DeviceRegistrations;
using TownCrier.Infrastructure.Groups;
using TownCrier.Infrastructure.Notifications;
using TownCrier.Infrastructure.PlanningApplications;
using TownCrier.Infrastructure.SavedApplications;
using TownCrier.Infrastructure.UserProfiles;
using TownCrier.Infrastructure.WatchZones;

namespace TownCrier.Infrastructure.Cosmos;

[JsonSerializable(typeof(Coordinates))]
[JsonSerializable(typeof(NotificationPreferences))]
[JsonSerializable(typeof(ZoneNotificationPreferences))]
[JsonSerializable(typeof(DeviceRegistrationDocument))]
[JsonSerializable(typeof(List<DeviceRegistrationDocument>))]
[JsonSerializable(typeof(SavedApplicationDocument))]
[JsonSerializable(typeof(List<SavedApplicationDocument>))]
[JsonSerializable(typeof(WatchZoneDocument))]
[JsonSerializable(typeof(List<WatchZoneDocument>))]
[JsonSerializable(typeof(AuthorityZoneCountResult))]
[JsonSerializable(typeof(int))]
[JsonSerializable(typeof(NotificationDocument))]
[JsonSerializable(typeof(List<NotificationDocument>))]
[JsonSerializable(typeof(PlanningApplicationDocument))]
[JsonSerializable(typeof(List<PlanningApplicationDocument>))]
[JsonSerializable(typeof(GeoJsonPoint))]
[JsonSerializable(typeof(UserProfileDocument))]
[JsonSerializable(typeof(List<UserProfileDocument>))]
[JsonSerializable(typeof(DecisionAlertDocument))]
[JsonSerializable(typeof(GroupDocument))]
[JsonSerializable(typeof(List<GroupDocument>))]
[JsonSerializable(typeof(GroupMemberDocument))]
[JsonSerializable(typeof(List<GroupMemberDocument>))]
[JsonSerializable(typeof(GroupInvitationDocument))]
[JsonSerializable(typeof(List<GroupInvitationDocument>))]
[JsonSerializable(typeof(CosmosQueryBody))]
[JsonSerializable(typeof(string))]
internal sealed partial class CosmosJsonSerializerContext : JsonSerializerContext;

internal sealed record CosmosQueryBody(
    [property: JsonPropertyName("query")] string Query,
    [property: JsonPropertyName("parameters")] IReadOnlyList<CosmosQueryParameter>? Parameters);

internal sealed record CosmosQueryParameter(
    [property: JsonPropertyName("name")] string Name,
    [property: JsonPropertyName("value")] object Value);
```

- [ ] **Step 2: Create StubHttpHandler test helper**

Create `api/tests/town-crier.infrastructure.tests/Cosmos/StubHttpHandler.cs`:

```csharp
using System.Net;

namespace TownCrier.Infrastructure.Tests.Cosmos;

internal sealed class StubHttpHandler : HttpMessageHandler
{
    private readonly Queue<HttpResponseMessage> responses = new();

    public List<HttpRequestMessage> SentRequests { get; } = [];

    public void EnqueueResponse(HttpStatusCode statusCode, string? content = null,
        IEnumerable<KeyValuePair<string, string>>? headers = null)
    {
        var response = new HttpResponseMessage(statusCode);
        if (content is not null)
        {
            response.Content = new StringContent(content, System.Text.Encoding.UTF8, "application/json");
        }

        if (headers is not null)
        {
            foreach (var (key, value) in headers)
            {
                response.Headers.TryAddWithoutValidation(key, value);
            }
        }

        responses.Enqueue(response);
    }

    protected override Task<HttpResponseMessage> SendAsync(
        HttpRequestMessage request, CancellationToken cancellationToken)
    {
        SentRequests.Add(request);
        return Task.FromResult(responses.Count > 0
            ? responses.Dequeue()
            : new HttpResponseMessage(HttpStatusCode.InternalServerError));
    }
}
```

- [ ] **Step 3: Write CosmosRestClient tests**

Create `api/tests/town-crier.infrastructure.tests/Cosmos/CosmosRestClientTests.cs`:

```csharp
using System.Net;
using System.Text.Json;
using System.Text.Json.Serialization;
using System.Text.Json.Serialization.Metadata;
using Azure.Core;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Tests.Cosmos;

public sealed class CosmosRestClientTests
{
    private const string AccountEndpoint = "https://test-account.documents.azure.com:443";
    private const string DatabaseName = "test-db";

    private static (CosmosRestClient Client, StubHttpHandler Handler) CreateClient()
    {
        var handler = new StubHttpHandler();
        var httpClient = new HttpClient(handler) { BaseAddress = new Uri(AccountEndpoint) };
        var credential = new FakeTokenCredential("fake-token");
        var authProvider = new CosmosAuthProvider(credential);
        var options = new CosmosRestOptions
        {
            AccountEndpoint = AccountEndpoint,
            DatabaseName = DatabaseName,
        };
        var client = new CosmosRestClient(httpClient, authProvider, options);
        return (client, handler);
    }

    [Test]
    public async Task ReadDocumentAsync_BuildsCorrectUrl()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.OK, """{"id":"doc1","name":"Test"}""");

        await client.ReadDocumentAsync("Users", "doc1", "doc1",
            TestSerializerContext.Default.TestDocument, CancellationToken.None);

        var request = handler.SentRequests[0];
        await Assert.That(request.RequestUri!.AbsolutePath)
            .IsEqualTo("/dbs/test-db/colls/Users/docs/doc1");
        await Assert.That(request.Method).IsEqualTo(HttpMethod.Get);
    }

    [Test]
    public async Task ReadDocumentAsync_SetsRequiredHeaders()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.OK, """{"id":"doc1","name":"Test"}""");

        await client.ReadDocumentAsync("Users", "doc1", "pk1",
            TestSerializerContext.Default.TestDocument, CancellationToken.None);

        var request = handler.SentRequests[0];
        await Assert.That(request.Headers.GetValues("x-ms-version").First())
            .IsEqualTo("2018-12-31");
        await Assert.That(request.Headers.Contains("x-ms-date")).IsTrue();
        await Assert.That(request.Headers.Contains("Authorization")).IsTrue();
        await Assert.That(request.Headers.GetValues("x-ms-documentdb-partitionkey").First())
            .IsEqualTo("[\"pk1\"]");
    }

    [Test]
    public async Task ReadDocumentAsync_ReturnsNullOn404()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.NotFound);

        var result = await client.ReadDocumentAsync("Users", "doc1", "doc1",
            TestSerializerContext.Default.TestDocument, CancellationToken.None);

        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task UpsertDocumentAsync_SetsUpsertHeader()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.OK);

        await client.UpsertDocumentAsync("Users",
            new TestDocument { Id = "doc1", Name = "Test" }, "doc1",
            TestSerializerContext.Default.TestDocument, CancellationToken.None);

        var request = handler.SentRequests[0];
        await Assert.That(request.Method).IsEqualTo(HttpMethod.Post);
        await Assert.That(request.Headers.GetValues("x-ms-documentdb-is-upsert").First())
            .IsEqualTo("True");
    }

    [Test]
    public async Task DeleteDocumentAsync_SilentOn404()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.NotFound);

        // Should not throw
        await client.DeleteDocumentAsync("Users", "doc1", "doc1", CancellationToken.None);

        await Assert.That(handler.SentRequests[0].Method).IsEqualTo(HttpMethod.Delete);
    }

    [Test]
    public async Task QueryAsync_SetsQueryHeaders()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.OK, """{"Documents":[],"_count":0}""");

        await client.QueryAsync("Users", "SELECT * FROM c", null, "pk1",
            TestSerializerContext.Default.TestDocument, CancellationToken.None);

        var request = handler.SentRequests[0];
        await Assert.That(request.Method).IsEqualTo(HttpMethod.Post);
        await Assert.That(request.Headers.GetValues("x-ms-documentdb-isquery").First())
            .IsEqualTo("True");
        await Assert.That(request.Content!.Headers.ContentType!.MediaType)
            .IsEqualTo("application/query+json");
    }

    [Test]
    public async Task QueryAsync_DrainsContinuationPages()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.OK,
            """{"Documents":[{"id":"d1","name":"A"}],"_count":1}""",
            [new("x-ms-continuation", "page2-token")]);
        handler.EnqueueResponse(HttpStatusCode.OK,
            """{"Documents":[{"id":"d2","name":"B"}],"_count":1}""");

        var results = await client.QueryAsync("Users", "SELECT * FROM c", null, "pk1",
            TestSerializerContext.Default.TestDocument, CancellationToken.None);

        await Assert.That(results).HasCount().EqualTo(2);
        await Assert.That(results[0].Id).IsEqualTo("d1");
        await Assert.That(results[1].Id).IsEqualTo("d2");
    }

    [Test]
    public async Task QueryAsync_WithoutPartitionKey_OmitsHeader()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.OK, """{"Documents":[],"_count":0}""");

        await client.QueryAsync("Users", "SELECT * FROM c", null, null,
            TestSerializerContext.Default.TestDocument, CancellationToken.None);

        var request = handler.SentRequests[0];
        await Assert.That(request.Headers.Contains("x-ms-documentdb-partitionkey")).IsFalse();
        await Assert.That(request.Headers.GetValues("x-ms-documentdb-query-enablecrosspartition").First())
            .IsEqualTo("True");
    }

    [Test]
    public async Task ScalarQueryAsync_ReturnsFirstValue()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.OK, """{"Documents":[42],"_count":1}""");

        var result = await client.ScalarQueryAsync("Users",
            "SELECT VALUE COUNT(1) FROM c", null, "pk1",
            TestSerializerContext.Default.Int32, CancellationToken.None);

        await Assert.That(result).IsEqualTo(42);
    }

    [Test]
    public async Task ReadDocumentAsync_ThrowsOnNonRetryableError()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.BadRequest);

        await Assert.That(() => client.ReadDocumentAsync("Users", "doc1", "doc1",
            TestSerializerContext.Default.TestDocument, CancellationToken.None))
            .ThrowsException().OfType<HttpRequestException>();
    }

    private sealed class FakeTokenCredential : TokenCredential
    {
        private readonly string token;

        public FakeTokenCredential(string token) => this.token = token;

        public override AccessToken GetToken(TokenRequestContext ctx, CancellationToken ct) =>
            new(token, DateTimeOffset.UtcNow.AddHours(1));

        public override ValueTask<AccessToken> GetTokenAsync(TokenRequestContext ctx, CancellationToken ct) =>
            ValueTask.FromResult(new AccessToken(token, DateTimeOffset.UtcNow.AddHours(1)));
    }
}

public sealed class TestDocument
{
    [JsonPropertyName("id")]
    public string Id { get; set; } = "";

    [JsonPropertyName("name")]
    public string Name { get; set; } = "";
}

[JsonSerializable(typeof(TestDocument))]
[JsonSerializable(typeof(int))]
internal sealed partial class TestSerializerContext : JsonSerializerContext;
```

- [ ] **Step 4: Run tests — verify they fail**

Run: `cd api && dotnet test tests/town-crier.infrastructure.tests --filter "CosmosRestClientTests" --no-restore`

Expected: Compilation error — `CosmosRestClient` does not exist yet.

- [ ] **Step 5: Implement CosmosRestClient**

Create `api/src/town-crier.infrastructure/Cosmos/CosmosRestClient.cs`:

```csharp
using System.Net;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization.Metadata;

namespace TownCrier.Infrastructure.Cosmos;

public sealed class CosmosRestClient : ICosmosRestClient
{
    private const string ApiVersion = "2018-12-31";

    private static readonly JsonSerializerOptions QuerySerializerOptions = new()
    {
        PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        TypeInfoResolver = CosmosJsonSerializerContext.Default,
    };

    private readonly HttpClient httpClient;
    private readonly CosmosAuthProvider authProvider;
    private readonly string databaseName;

    public CosmosRestClient(HttpClient httpClient, CosmosAuthProvider authProvider, CosmosRestOptions options)
    {
        ArgumentNullException.ThrowIfNull(httpClient);
        ArgumentNullException.ThrowIfNull(authProvider);
        ArgumentNullException.ThrowIfNull(options);

        this.httpClient = httpClient;
        this.authProvider = authProvider;
        this.databaseName = options.DatabaseName;
    }

    public async Task<T?> ReadDocumentAsync<T>(string collection, string id,
        string partitionKey, JsonTypeInfo<T> typeInfo, CancellationToken ct)
    {
        var resourceLink = $"dbs/{databaseName}/colls/{collection}/docs/{id}";
        using var request = new HttpRequestMessage(HttpMethod.Get, $"/{resourceLink}");
        await AddHeadersAsync(request, "get", "docs", resourceLink, partitionKey, ct).ConfigureAwait(false);

        using var response = await httpClient.SendAsync(request, ct).ConfigureAwait(false);

        if (response.StatusCode == HttpStatusCode.NotFound)
        {
            return default;
        }

        response.EnsureSuccessStatusCode();

        await using var stream = await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false);
        return await JsonSerializer.DeserializeAsync(stream, typeInfo, ct).ConfigureAwait(false);
    }

    public async Task UpsertDocumentAsync<T>(string collection, T document,
        string partitionKey, JsonTypeInfo<T> typeInfo, CancellationToken ct)
    {
        var resourceLink = $"dbs/{databaseName}/colls/{collection}";
        using var request = new HttpRequestMessage(HttpMethod.Post, $"/{resourceLink}/docs");
        await AddHeadersAsync(request, "post", "docs", resourceLink, partitionKey, ct).ConfigureAwait(false);
        request.Headers.TryAddWithoutValidation("x-ms-documentdb-is-upsert", "True");

        request.Content = new StringContent(
            JsonSerializer.Serialize(document, typeInfo),
            Encoding.UTF8, "application/json");

        using var response = await httpClient.SendAsync(request, ct).ConfigureAwait(false);
        response.EnsureSuccessStatusCode();
    }

    public async Task DeleteDocumentAsync(string collection, string id,
        string partitionKey, CancellationToken ct)
    {
        var resourceLink = $"dbs/{databaseName}/colls/{collection}/docs/{id}";
        using var request = new HttpRequestMessage(HttpMethod.Delete, $"/{resourceLink}");
        await AddHeadersAsync(request, "delete", "docs", resourceLink, partitionKey, ct).ConfigureAwait(false);

        using var response = await httpClient.SendAsync(request, ct).ConfigureAwait(false);

        if (response.StatusCode == HttpStatusCode.NotFound)
        {
            return; // Idempotent delete
        }

        response.EnsureSuccessStatusCode();
    }

    public async Task<List<T>> QueryAsync<T>(string collection, string sql,
        IReadOnlyList<QueryParameter>? parameters, string? partitionKey,
        JsonTypeInfo<T> typeInfo, CancellationToken ct)
    {
        var results = new List<T>();
        string? continuation = null;

        do
        {
            using var request = BuildQueryRequest(collection, sql, parameters, partitionKey, continuation);
            var resourceLink = $"dbs/{databaseName}/colls/{collection}";
            await AddHeadersAsync(request, "post", "docs", resourceLink, partitionKey, ct).ConfigureAwait(false);
            AddQueryHeaders(request, partitionKey);

            if (continuation is not null)
            {
                request.Headers.TryAddWithoutValidation("x-ms-continuation", continuation);
            }

            using var response = await httpClient.SendAsync(request, ct).ConfigureAwait(false);
            response.EnsureSuccessStatusCode();

            await using var stream = await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false);
            using var doc = await JsonDocument.ParseAsync(stream, cancellationToken: ct).ConfigureAwait(false);

            foreach (var element in doc.RootElement.GetProperty("Documents").EnumerateArray())
            {
                results.Add(element.Deserialize(typeInfo)!);
            }

            continuation = response.Headers.TryGetValues("x-ms-continuation", out var values)
                ? values.FirstOrDefault()
                : null;
        }
        while (continuation is not null);

        return results;
    }

    public async Task<T> ScalarQueryAsync<T>(string collection, string sql,
        IReadOnlyList<QueryParameter>? parameters, string? partitionKey,
        JsonTypeInfo<T> typeInfo, CancellationToken ct)
    {
        var results = await QueryAsync(collection, sql, parameters, partitionKey, typeInfo, ct)
            .ConfigureAwait(false);
        return results.FirstOrDefault()!;
    }

    private HttpRequestMessage BuildQueryRequest(string collection, string sql,
        IReadOnlyList<QueryParameter>? parameters, string? partitionKey, string? continuation)
    {
        var resourceLink = $"dbs/{databaseName}/colls/{collection}";
        var request = new HttpRequestMessage(HttpMethod.Post, $"/{resourceLink}/docs");

        var queryParameters = parameters?.Select(p =>
            new CosmosQueryParameter(p.Name, p.Value)).ToList();
        var body = new CosmosQueryBody(sql, queryParameters);

        request.Content = new StringContent(
            JsonSerializer.Serialize(body, CosmosJsonSerializerContext.Default.CosmosQueryBody),
            Encoding.UTF8, "application/query+json");

        return request;
    }

    private static void AddQueryHeaders(HttpRequestMessage request, string? partitionKey)
    {
        request.Headers.TryAddWithoutValidation("x-ms-documentdb-isquery", "True");

        if (partitionKey is null)
        {
            request.Headers.TryAddWithoutValidation("x-ms-documentdb-query-enablecrosspartition", "True");
        }
    }

    private async Task AddHeadersAsync(HttpRequestMessage request, string verb,
        string resourceType, string resourceLink, string? partitionKey, CancellationToken ct)
    {
        var date = DateTime.UtcNow.ToString("R");
        var auth = await authProvider.GetAuthorizationHeaderAsync(ct).ConfigureAwait(false);

        request.Headers.TryAddWithoutValidation("Authorization", auth);
        request.Headers.TryAddWithoutValidation("x-ms-date", date);
        request.Headers.TryAddWithoutValidation("x-ms-version", ApiVersion);

        if (partitionKey is not null)
        {
            request.Headers.TryAddWithoutValidation("x-ms-documentdb-partitionkey", $"[\"{partitionKey}\"]");
        }
    }
}
```

- [ ] **Step 6: Run tests — verify they pass**

Run: `cd api && dotnet test tests/town-crier.infrastructure.tests --filter "CosmosRestClientTests"`

Expected: All 9 tests pass.

- [ ] **Step 7: Commit**

```bash
git add api/src/town-crier.infrastructure/Cosmos/CosmosRestClient.cs \
       api/src/town-crier.infrastructure/Cosmos/CosmosJsonSerializerContext.cs \
       api/tests/town-crier.infrastructure.tests/Cosmos/StubHttpHandler.cs \
       api/tests/town-crier.infrastructure.tests/Cosmos/CosmosRestClientTests.cs
git commit -m "feat: implement CosmosRestClient with query pagination and error handling"
```

---

## Task 4: DI registration and service wiring

**Files:**
- Modify: `api/src/town-crier.infrastructure/Cosmos/CosmosServiceExtensions.cs`
- Modify: `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs`

- [ ] **Step 1: Rewrite CosmosServiceExtensions**

Replace `api/src/town-crier.infrastructure/Cosmos/CosmosServiceExtensions.cs` with:

```csharp
using System.Net;
using Azure.Identity;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Http.Resilience;
using Polly;

namespace TownCrier.Infrastructure.Cosmos;

public static class CosmosServiceExtensions
{
    public static IServiceCollection AddCosmosRestClient(this IServiceCollection services, IConfiguration configuration)
    {
        var accountEndpoint = configuration["Cosmos:AccountEndpoint"]
            ?? throw new InvalidOperationException(
                "Cosmos DB is not configured. Set 'Cosmos:AccountEndpoint'.");

        var databaseName = configuration["Cosmos:DatabaseName"] ?? CosmosContainerNames.DatabaseName;

        var options = new CosmosRestOptions
        {
            AccountEndpoint = accountEndpoint,
            DatabaseName = databaseName,
        };

        services.AddSingleton(options);
        services.AddSingleton(new CosmosAuthProvider(new DefaultAzureCredential()));

        services.AddHttpClient("CosmosRest", client =>
        {
            client.BaseAddress = new Uri(accountEndpoint);
        })
        .AddResilienceHandler("cosmos-retry", builder =>
        {
            builder.AddRetry(new HttpRetryStrategyOptions
            {
                MaxRetryAttempts = 5,
                BackoffType = DelayBackoffType.Exponential,
                UseJitter = true,
                Delay = TimeSpan.FromMilliseconds(500),
                ShouldHandle = static args => ValueTask.FromResult(
                    args.Outcome.Result?.StatusCode is
                        (HttpStatusCode)429 or
                        HttpStatusCode.RequestTimeout or
                        HttpStatusCode.ServiceUnavailable or
                        (HttpStatusCode)449),
                DelayGenerator = static args =>
                {
                    if (args.Outcome.Result?.Headers
                        .TryGetValues("x-ms-retry-after-ms", out var values) == true
                        && int.TryParse(values.FirstOrDefault(), out var ms))
                    {
                        return ValueTask.FromResult<TimeSpan?>(TimeSpan.FromMilliseconds(ms));
                    }

                    return ValueTask.FromResult<TimeSpan?>(null);
                },
            });
        });

        services.AddSingleton<ICosmosRestClient>(sp =>
        {
            var factory = sp.GetRequiredService<IHttpClientFactory>();
            var httpClient = factory.CreateClient("CosmosRest");
            var auth = sp.GetRequiredService<CosmosAuthProvider>();
            return new CosmosRestClient(httpClient, auth, options);
        });

        return services;
    }
}
```

- [ ] **Step 2: Update ServiceCollectionExtensions to use new registration**

In `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs`, make these changes:

1. Remove the `using Microsoft.Azure.Cosmos;` import (if present — it may be a fully-qualified reference).
2. Replace `services.AddCosmosClient(configuration);` with `services.AddCosmosRestClient(configuration);`
3. Change the two factory-lambda registrations that reference `Microsoft.Azure.Cosmos.CosmosClient` to use `ICosmosRestClient`:

Replace:
```csharp
        services.AddSingleton<IDeviceRegistrationRepository>(sp =>
            new CosmosDeviceRegistrationRepository(sp.GetRequiredService<Microsoft.Azure.Cosmos.CosmosClient>()));
        services.AddSingleton<INotificationRepository>(sp =>
            new CosmosNotificationRepository(sp.GetRequiredService<Microsoft.Azure.Cosmos.CosmosClient>()));
```

With:
```csharp
        services.AddSingleton<IDeviceRegistrationRepository, CosmosDeviceRegistrationRepository>();
        services.AddSingleton<INotificationRepository, CosmosNotificationRepository>();
```

- [ ] **Step 3: Commit**

```bash
git add api/src/town-crier.infrastructure/Cosmos/CosmosServiceExtensions.cs \
       api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs
git commit -m "feat: wire up CosmosRestClient DI with Polly retry pipeline"
```

---

## Task 5: Delete SDK files and their tests

**Files:**
- Delete: `api/src/town-crier.infrastructure/Cosmos/CosmosClientFactory.cs`
- Delete: `api/src/town-crier.infrastructure/Cosmos/SystemTextJsonCosmosSerializer.cs`
- Delete: `api/src/town-crier.infrastructure/Cosmos/CosmosQueryExtensions.cs`
- Delete: `api/tests/town-crier.infrastructure.tests/Cosmos/CosmosClientFactoryTests.cs`
- Delete: `api/tests/town-crier.infrastructure.tests/Cosmos/CosmosServiceRegistrationTests.cs`
- Delete: `api/tests/town-crier.infrastructure.tests/Cosmos/SystemTextJsonCosmosSerializerTests.cs`
- Delete: `api/tests/town-crier.infrastructure.tests/Cosmos/CosmosQueryExtensionsTests.cs`
- Delete: `api/tests/town-crier.infrastructure.tests/Cosmos/FakeFeedIterator.cs`
- Delete: `api/tests/town-crier.infrastructure.tests/Cosmos/FakeFeedResponse.cs`

- [ ] **Step 1: Delete all SDK-dependent source files**

```bash
rm -f api/src/town-crier.infrastructure/Cosmos/CosmosClientFactory.cs \
      api/src/town-crier.infrastructure/Cosmos/SystemTextJsonCosmosSerializer.cs \
      api/src/town-crier.infrastructure/Cosmos/CosmosQueryExtensions.cs
```

- [ ] **Step 2: Delete all SDK-dependent test files**

```bash
rm -f api/tests/town-crier.infrastructure.tests/Cosmos/CosmosClientFactoryTests.cs \
      api/tests/town-crier.infrastructure.tests/Cosmos/CosmosServiceRegistrationTests.cs \
      api/tests/town-crier.infrastructure.tests/Cosmos/SystemTextJsonCosmosSerializerTests.cs \
      api/tests/town-crier.infrastructure.tests/Cosmos/CosmosQueryExtensionsTests.cs \
      api/tests/town-crier.infrastructure.tests/Cosmos/FakeFeedIterator.cs \
      api/tests/town-crier.infrastructure.tests/Cosmos/FakeFeedResponse.cs
```

- [ ] **Step 3: Commit**

```bash
git add -u
git commit -m "chore: remove Cosmos SDK adapter files and their tests"
```

---

## Task 6: Migrate UserProfiles and DecisionAlerts repositories

**Files:**
- Modify: `api/src/town-crier.infrastructure/UserProfiles/CosmosUserProfileRepository.cs`
- Modify: `api/src/town-crier.infrastructure/DecisionAlerts/CosmosDecisionAlertRepository.cs`

- [ ] **Step 1: Rewrite CosmosUserProfileRepository**

Replace `api/src/town-crier.infrastructure/UserProfiles/CosmosUserProfileRepository.cs` with:

```csharp
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.UserProfiles;

public sealed class CosmosUserProfileRepository : IUserProfileRepository
{
    private readonly ICosmosRestClient client;

    public CosmosUserProfileRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task<UserProfile?> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        var doc = await client.ReadDocumentAsync(
            CosmosContainerNames.Users, userId, userId,
            CosmosJsonSerializerContext.Default.UserProfileDocument, ct).ConfigureAwait(false);

        return doc?.ToDomain();
    }

    public async Task<IReadOnlyList<UserProfile>> GetAllByTierAsync(SubscriptionTier tier, CancellationToken ct)
    {
        var docs = await client.QueryAsync(
            CosmosContainerNames.Users,
            "SELECT * FROM c WHERE c.tier = @tier",
            [new("@tier", tier.ToString())],
            null,
            CosmosJsonSerializerContext.Default.UserProfileDocument, ct).ConfigureAwait(false);

        return docs.Select(doc => doc.ToDomain()).ToList();
    }

    public async Task<UserProfile?> GetByOriginalTransactionIdAsync(
        string originalTransactionId, CancellationToken ct)
    {
        var docs = await client.QueryAsync(
            CosmosContainerNames.Users,
            "SELECT * FROM c WHERE c.originalTransactionId = @txnId",
            [new("@txnId", originalTransactionId)],
            null,
            CosmosJsonSerializerContext.Default.UserProfileDocument, ct).ConfigureAwait(false);

        return docs.FirstOrDefault()?.ToDomain();
    }

    public async Task SaveAsync(UserProfile profile, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(profile);

        var document = UserProfileDocument.FromDomain(profile);
        await client.UpsertDocumentAsync(
            CosmosContainerNames.Users, document, document.Id,
            CosmosJsonSerializerContext.Default.UserProfileDocument, ct).ConfigureAwait(false);
    }

    public async Task DeleteAsync(string userId, CancellationToken ct)
    {
        await client.DeleteDocumentAsync(
            CosmosContainerNames.Users, userId, userId, ct).ConfigureAwait(false);
    }
}
```

- [ ] **Step 2: Rewrite CosmosDecisionAlertRepository**

Replace `api/src/town-crier.infrastructure/DecisionAlerts/CosmosDecisionAlertRepository.cs` with:

```csharp
using TownCrier.Application.DecisionAlerts;
using TownCrier.Domain.DecisionAlerts;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.DecisionAlerts;

public sealed class CosmosDecisionAlertRepository : IDecisionAlertRepository
{
    private readonly ICosmosRestClient client;

    public CosmosDecisionAlertRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task<DecisionAlert?> GetByUserAndApplicationAsync(
        string userId, string applicationUid, CancellationToken ct)
    {
        var documentId = DecisionAlertDocument.MakeId(userId, applicationUid);

        var doc = await client.ReadDocumentAsync(
            CosmosContainerNames.DecisionAlerts, documentId, userId,
            CosmosJsonSerializerContext.Default.DecisionAlertDocument, ct).ConfigureAwait(false);

        return doc?.ToDomain();
    }

    public async Task SaveAsync(DecisionAlert alert, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(alert);

        var document = DecisionAlertDocument.FromDomain(alert);
        await client.UpsertDocumentAsync(
            CosmosContainerNames.DecisionAlerts, document, document.UserId,
            CosmosJsonSerializerContext.Default.DecisionAlertDocument, ct).ConfigureAwait(false);
    }
}
```

- [ ] **Step 3: Verify build compiles (these two repos)**

Run: `cd api && dotnet build src/town-crier.infrastructure --no-restore 2>&1 | head -20`

Expected: Compilation errors from the other 7 repositories (still referencing `Microsoft.Azure.Cosmos`), but these two files should compile cleanly.

- [ ] **Step 4: Commit**

```bash
git add api/src/town-crier.infrastructure/UserProfiles/CosmosUserProfileRepository.cs \
       api/src/town-crier.infrastructure/DecisionAlerts/CosmosDecisionAlertRepository.cs
git commit -m "feat: migrate UserProfile and DecisionAlert repos to REST client"
```

---

## Task 7: Migrate WatchZones and PlanningApplications repositories

**Files:**
- Modify: `api/src/town-crier.infrastructure/WatchZones/CosmosWatchZoneRepository.cs`
- Modify: `api/src/town-crier.infrastructure/PlanningApplications/CosmosPlanningApplicationRepository.cs`

- [ ] **Step 1: Rewrite CosmosWatchZoneRepository**

Replace `api/src/town-crier.infrastructure/WatchZones/CosmosWatchZoneRepository.cs` with:

```csharp
using TownCrier.Application.WatchZones;
using TownCrier.Domain.WatchZones;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.WatchZones;

public sealed class CosmosWatchZoneRepository : IWatchZoneRepository
{
    private readonly ICosmosRestClient client;

    public CosmosWatchZoneRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task SaveAsync(WatchZone zone, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(zone);

        var document = WatchZoneDocument.FromDomain(zone);
        await client.UpsertDocumentAsync(
            CosmosContainerNames.WatchZones, document, document.UserId,
            CosmosJsonSerializerContext.Default.WatchZoneDocument, ct).ConfigureAwait(false);
    }

    public async Task<IReadOnlyCollection<WatchZone>> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);

        var docs = await client.QueryAsync(
            CosmosContainerNames.WatchZones,
            "SELECT * FROM c WHERE c.userId = @userId",
            [new("@userId", userId)],
            userId,
            CosmosJsonSerializerContext.Default.WatchZoneDocument, ct).ConfigureAwait(false);

        return docs.Select(doc => doc.ToDomain()).ToList();
    }

    public async Task DeleteAsync(string userId, string zoneId, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);
        ArgumentException.ThrowIfNullOrWhiteSpace(zoneId);

        // REST client's delete is silent on 404 — we need to throw WatchZoneNotFoundException.
        // Check existence first via point read.
        var existing = await client.ReadDocumentAsync(
            CosmosContainerNames.WatchZones, zoneId, userId,
            CosmosJsonSerializerContext.Default.WatchZoneDocument, ct).ConfigureAwait(false);

        if (existing is null)
        {
            throw new WatchZoneNotFoundException();
        }

        await client.DeleteDocumentAsync(
            CosmosContainerNames.WatchZones, zoneId, userId, ct).ConfigureAwait(false);
    }

    public async Task<IReadOnlyCollection<WatchZone>> FindZonesContainingAsync(
        double latitude, double longitude, CancellationToken ct)
    {
        var docs = await client.QueryAsync(
            CosmosContainerNames.WatchZones,
            "SELECT * FROM c WHERE ST_DISTANCE({'type': 'Point', 'coordinates': [c.longitude, c.latitude]}, " +
            "{'type': 'Point', 'coordinates': [@longitude, @latitude]}) <= c.radiusMetres",
            [new("@latitude", latitude), new("@longitude", longitude)],
            null,
            CosmosJsonSerializerContext.Default.WatchZoneDocument, ct).ConfigureAwait(false);

        return docs.Select(doc => doc.ToDomain()).ToList();
    }

    public async Task<IReadOnlyCollection<int>> GetDistinctAuthorityIdsAsync(CancellationToken ct)
    {
        return await client.QueryAsync(
            CosmosContainerNames.WatchZones,
            "SELECT DISTINCT VALUE c.authorityId FROM c",
            null,
            null,
            CosmosJsonSerializerContext.Default.Int32, ct).ConfigureAwait(false);
    }

    public async Task<Dictionary<int, int>> GetZoneCountsByAuthorityAsync(CancellationToken ct)
    {
        var items = await client.QueryAsync(
            CosmosContainerNames.WatchZones,
            "SELECT c.authorityId, COUNT(1) AS zoneCount FROM c GROUP BY c.authorityId",
            null,
            null,
            CosmosJsonSerializerContext.Default.AuthorityZoneCountResult, ct).ConfigureAwait(false);

        return items.ToDictionary(item => item.AuthorityId, item => item.ZoneCount);
    }
}
```

- [ ] **Step 2: Rewrite CosmosPlanningApplicationRepository**

Replace `api/src/town-crier.infrastructure/PlanningApplications/CosmosPlanningApplicationRepository.cs` with:

```csharp
using System.Globalization;
using TownCrier.Application.PlanningApplications;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.PlanningApplications;

public sealed class CosmosPlanningApplicationRepository : IPlanningApplicationRepository
{
    private readonly ICosmosRestClient client;

    public CosmosPlanningApplicationRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task UpsertAsync(PlanningApplication application, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(application);

        var document = PlanningApplicationDocument.FromDomain(application);
        await client.UpsertDocumentAsync(
            CosmosContainerNames.Applications, document, document.AuthorityCode,
            CosmosJsonSerializerContext.Default.PlanningApplicationDocument, ct).ConfigureAwait(false);
    }

    public async Task<PlanningApplication?> GetByUidAsync(string uid, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(uid);

        var docs = await client.QueryAsync(
            CosmosContainerNames.Applications,
            "SELECT * FROM c WHERE c.Uid = @uid",
            [new("@uid", uid)],
            null,
            CosmosJsonSerializerContext.Default.PlanningApplicationDocument, ct).ConfigureAwait(false);

        return docs.FirstOrDefault()?.ToDomain();
    }

    public async Task<IReadOnlyCollection<PlanningApplication>> GetByAuthorityIdAsync(int authorityId, CancellationToken ct)
    {
        var authorityCode = authorityId.ToString(CultureInfo.InvariantCulture);

        var docs = await client.QueryAsync(
            CosmosContainerNames.Applications,
            "SELECT * FROM c",
            null,
            authorityCode,
            CosmosJsonSerializerContext.Default.PlanningApplicationDocument, ct).ConfigureAwait(false);

        return docs.Select(doc => doc.ToDomain()).ToList();
    }

    public async Task<IReadOnlyCollection<PlanningApplication>> FindNearbyAsync(
        string authorityCode, double latitude, double longitude, double radiusMetres, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(authorityCode);

        var docs = await client.QueryAsync(
            CosmosContainerNames.Applications,
            "SELECT * FROM c WHERE ST_DISTANCE(c.location, {\"type\": \"Point\", \"coordinates\": [@lng, @lat]}) <= @radius",
            [new("@lng", longitude), new("@lat", latitude), new("@radius", radiusMetres)],
            authorityCode,
            CosmosJsonSerializerContext.Default.PlanningApplicationDocument, ct).ConfigureAwait(false);

        return docs.Select(doc => doc.ToDomain()).ToList();
    }
}
```

- [ ] **Step 3: Commit**

```bash
git add api/src/town-crier.infrastructure/WatchZones/CosmosWatchZoneRepository.cs \
       api/src/town-crier.infrastructure/PlanningApplications/CosmosPlanningApplicationRepository.cs
git commit -m "feat: migrate WatchZone and PlanningApplication repos to REST client"
```

---

## Task 8: Migrate DeviceRegistrations and Notifications repositories

**Files:**
- Modify: `api/src/town-crier.infrastructure/DeviceRegistrations/CosmosDeviceRegistrationRepository.cs`
- Modify: `api/src/town-crier.infrastructure/Notifications/CosmosNotificationRepository.cs`

- [ ] **Step 1: Rewrite CosmosDeviceRegistrationRepository**

Replace `api/src/town-crier.infrastructure/DeviceRegistrations/CosmosDeviceRegistrationRepository.cs` with:

```csharp
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.DeviceRegistrations;

public sealed class CosmosDeviceRegistrationRepository : IDeviceRegistrationRepository
{
    private readonly ICosmosRestClient client;

    public CosmosDeviceRegistrationRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task<DeviceRegistration?> GetByTokenAsync(string token, CancellationToken ct)
    {
        var docs = await client.QueryAsync(
            CosmosContainerNames.DeviceRegistrations,
            "SELECT * FROM c WHERE c.token = @token",
            [new("@token", token)],
            null,
            CosmosJsonSerializerContext.Default.DeviceRegistrationDocument, ct).ConfigureAwait(false);

        return docs.FirstOrDefault()?.ToDomain();
    }

    public async Task<IReadOnlyList<DeviceRegistration>> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        var docs = await client.QueryAsync(
            CosmosContainerNames.DeviceRegistrations,
            "SELECT * FROM c WHERE c.userId = @userId",
            [new("@userId", userId)],
            userId,
            CosmosJsonSerializerContext.Default.DeviceRegistrationDocument, ct).ConfigureAwait(false);

        return docs.Select(doc => doc.ToDomain()).ToList();
    }

    public async Task SaveAsync(DeviceRegistration registration, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(registration);

        var document = DeviceRegistrationDocument.FromDomain(registration);
        await client.UpsertDocumentAsync(
            CosmosContainerNames.DeviceRegistrations, document, document.UserId,
            CosmosJsonSerializerContext.Default.DeviceRegistrationDocument, ct).ConfigureAwait(false);
    }

    public async Task DeleteByTokenAsync(string token, CancellationToken ct)
    {
        var docs = await client.QueryAsync(
            CosmosContainerNames.DeviceRegistrations,
            "SELECT c.id, c.userId FROM c WHERE c.token = @token",
            [new("@token", token)],
            null,
            CosmosJsonSerializerContext.Default.DeviceRegistrationDocument, ct).ConfigureAwait(false);

        foreach (var document in docs)
        {
            await client.DeleteDocumentAsync(
                CosmosContainerNames.DeviceRegistrations, document.Id, document.UserId, ct).ConfigureAwait(false);
        }
    }
}
```

- [ ] **Step 2: Rewrite CosmosNotificationRepository**

Replace `api/src/town-crier.infrastructure/Notifications/CosmosNotificationRepository.cs` with:

```csharp
using TownCrier.Application.Notifications;
using TownCrier.Domain.Notifications;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Notifications;

public sealed class CosmosNotificationRepository : INotificationRepository
{
    private readonly ICosmosRestClient client;

    public CosmosNotificationRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task<Notification?> GetByUserAndApplicationAsync(
        string userId, string applicationName, CancellationToken ct)
    {
        var docs = await client.QueryAsync(
            CosmosContainerNames.Notifications,
            "SELECT * FROM c WHERE c.userId = @userId AND c.applicationName = @appName",
            [new("@userId", userId), new("@appName", applicationName)],
            userId,
            CosmosJsonSerializerContext.Default.NotificationDocument, ct).ConfigureAwait(false);

        return docs.FirstOrDefault()?.ToDomain();
    }

    public async Task<int> CountByUserInMonthAsync(
        string userId, int year, int month, CancellationToken ct)
    {
        var startOfMonth = new DateTimeOffset(year, month, 1, 0, 0, 0, TimeSpan.Zero);
        var startOfNextMonth = startOfMonth.AddMonths(1);

        return await client.ScalarQueryAsync(
            CosmosContainerNames.Notifications,
            "SELECT VALUE COUNT(1) FROM c WHERE c.userId = @userId AND c.createdAt >= @start AND c.createdAt < @end",
            [new("@userId", userId), new("@start", startOfMonth), new("@end", startOfNextMonth)],
            userId,
            CosmosJsonSerializerContext.Default.Int32, ct).ConfigureAwait(false);
    }

    public async Task<int> CountByUserSinceAsync(
        string userId, DateTimeOffset since, CancellationToken ct)
    {
        return await client.ScalarQueryAsync(
            CosmosContainerNames.Notifications,
            "SELECT VALUE COUNT(1) FROM c WHERE c.userId = @userId AND c.createdAt >= @since",
            [new("@userId", userId), new("@since", since)],
            userId,
            CosmosJsonSerializerContext.Default.Int32, ct).ConfigureAwait(false);
    }

    public async Task<(IReadOnlyList<Notification> Items, int Total)> GetByUserPaginatedAsync(
        string userId, int page, int pageSize, CancellationToken ct)
    {
        var total = await client.ScalarQueryAsync(
            CosmosContainerNames.Notifications,
            "SELECT VALUE COUNT(1) FROM c WHERE c.userId = @userId",
            [new("@userId", userId)],
            userId,
            CosmosJsonSerializerContext.Default.Int32, ct).ConfigureAwait(false);

        if (total == 0)
        {
            return (Array.Empty<Notification>(), 0);
        }

        var offset = (page - 1) * pageSize;

        var docs = await client.QueryAsync(
            CosmosContainerNames.Notifications,
            "SELECT * FROM c WHERE c.userId = @userId ORDER BY c._ts DESC OFFSET @offset LIMIT @limit",
            [new("@userId", userId), new("@offset", offset), new("@limit", pageSize)],
            userId,
            CosmosJsonSerializerContext.Default.NotificationDocument, ct).ConfigureAwait(false);

        return (docs.Select(doc => doc.ToDomain()).ToList(), total);
    }

    public async Task SaveAsync(Notification notification, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(notification);

        var document = NotificationDocument.FromDomain(notification);
        await client.UpsertDocumentAsync(
            CosmosContainerNames.Notifications, document, document.UserId,
            CosmosJsonSerializerContext.Default.NotificationDocument, ct).ConfigureAwait(false);
    }
}
```

- [ ] **Step 3: Commit**

```bash
git add api/src/town-crier.infrastructure/DeviceRegistrations/CosmosDeviceRegistrationRepository.cs \
       api/src/town-crier.infrastructure/Notifications/CosmosNotificationRepository.cs
git commit -m "feat: migrate DeviceRegistration and Notification repos to REST client"
```

---

## Task 9: Migrate Groups and SavedApplications repositories

**Files:**
- Modify: `api/src/town-crier.infrastructure/Groups/CosmosGroupRepository.cs`
- Modify: `api/src/town-crier.infrastructure/Groups/CosmosGroupInvitationRepository.cs`
- Modify: `api/src/town-crier.infrastructure/SavedApplications/CosmosSavedApplicationRepository.cs`

- [ ] **Step 1: Rewrite CosmosGroupRepository**

Replace `api/src/town-crier.infrastructure/Groups/CosmosGroupRepository.cs` with:

```csharp
using TownCrier.Application.Groups;
using TownCrier.Domain.Groups;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Groups;

public sealed class CosmosGroupRepository : IGroupRepository
{
    private readonly ICosmosRestClient client;

    public CosmosGroupRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task<Group?> GetByIdAsync(string groupId, CancellationToken ct)
    {
        var docs = await client.QueryAsync(
            CosmosContainerNames.Groups,
            "SELECT * FROM c WHERE c.id = @id AND c.type = 'group'",
            [new("@id", groupId)],
            null,
            CosmosJsonSerializerContext.Default.GroupDocument, ct).ConfigureAwait(false);

        return docs.FirstOrDefault()?.ToDomain();
    }

    public async Task SaveAsync(Group group, CancellationToken ct)
    {
        var document = GroupDocument.FromDomain(group);
        await client.UpsertDocumentAsync(
            CosmosContainerNames.Groups, document, document.OwnerId,
            CosmosJsonSerializerContext.Default.GroupDocument, ct).ConfigureAwait(false);
    }

    public async Task DeleteAsync(string groupId, CancellationToken ct)
    {
        var docs = await client.QueryAsync(
            CosmosContainerNames.Groups,
            "SELECT c.id, c.ownerId FROM c WHERE c.id = @id AND c.type = 'group'",
            [new("@id", groupId)],
            null,
            CosmosJsonSerializerContext.Default.GroupDocument, ct).ConfigureAwait(false);

        var document = docs.FirstOrDefault();
        if (document is not null)
        {
            await client.DeleteDocumentAsync(
                CosmosContainerNames.Groups, document.Id, document.OwnerId, ct).ConfigureAwait(false);
        }
    }

    public async Task<IReadOnlyList<Group>> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        var docs = await client.QueryAsync(
            CosmosContainerNames.Groups,
            "SELECT * FROM c WHERE c.type = 'group' AND ARRAY_CONTAINS(c.members, {\"userId\": @userId}, true)",
            [new("@userId", userId)],
            null,
            CosmosJsonSerializerContext.Default.GroupDocument, ct).ConfigureAwait(false);

        return docs.Select(doc => doc.ToDomain()).ToList();
    }
}
```

- [ ] **Step 2: Rewrite CosmosGroupInvitationRepository**

Replace `api/src/town-crier.infrastructure/Groups/CosmosGroupInvitationRepository.cs` with:

```csharp
using TownCrier.Application.Groups;
using TownCrier.Domain.Groups;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Groups;

public sealed class CosmosGroupInvitationRepository : IGroupInvitationRepository
{
    private readonly ICosmosRestClient client;

    public CosmosGroupInvitationRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task<GroupInvitation?> GetByIdAsync(string invitationId, CancellationToken ct)
    {
        var docs = await client.QueryAsync(
            CosmosContainerNames.Groups,
            "SELECT * FROM c WHERE c.id = @id AND c.type = 'invitation'",
            [new("@id", GroupInvitationDocument.ToDocumentId(invitationId))],
            null,
            CosmosJsonSerializerContext.Default.GroupInvitationDocument, ct).ConfigureAwait(false);

        return docs.FirstOrDefault()?.ToDomain();
    }

    public async Task SaveAsync(GroupInvitation invitation, CancellationToken ct)
    {
        var document = GroupInvitationDocument.FromDomain(invitation);
        await client.UpsertDocumentAsync(
            CosmosContainerNames.Groups, document, document.OwnerId,
            CosmosJsonSerializerContext.Default.GroupInvitationDocument, ct).ConfigureAwait(false);
    }

    public async Task<IReadOnlyList<GroupInvitation>> GetPendingByGroupIdAsync(string groupId, CancellationToken ct)
    {
        var docs = await client.QueryAsync(
            CosmosContainerNames.Groups,
            "SELECT * FROM c WHERE c.type = 'invitation' AND c.groupId = @groupId AND c.status = 'Pending'",
            [new("@groupId", groupId)],
            null,
            CosmosJsonSerializerContext.Default.GroupInvitationDocument, ct).ConfigureAwait(false);

        return docs.Select(doc => doc.ToDomain()).ToList();
    }

    public async Task<IReadOnlyList<GroupInvitation>> GetPendingByEmailAsync(string email, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(email);

#pragma warning disable CA1308 // Emails are normalized to lowercase per industry convention
        var normalizedEmail = email.Trim().ToLowerInvariant();
#pragma warning restore CA1308

        var docs = await client.QueryAsync(
            CosmosContainerNames.Groups,
            "SELECT * FROM c WHERE c.type = 'invitation' AND LOWER(c.inviteeEmail) = @email AND c.status = 'Pending'",
            [new("@email", normalizedEmail)],
            null,
            CosmosJsonSerializerContext.Default.GroupInvitationDocument, ct).ConfigureAwait(false);

        return docs.Select(doc => doc.ToDomain()).ToList();
    }
}
```

- [ ] **Step 3: Rewrite CosmosSavedApplicationRepository**

Replace `api/src/town-crier.infrastructure/SavedApplications/CosmosSavedApplicationRepository.cs` with:

```csharp
using TownCrier.Application.SavedApplications;
using TownCrier.Domain.SavedApplications;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.SavedApplications;

public sealed class CosmosSavedApplicationRepository : ISavedApplicationRepository
{
    private readonly ICosmosRestClient client;

    public CosmosSavedApplicationRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task SaveAsync(SavedApplication savedApplication, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(savedApplication);

        var document = SavedApplicationDocument.FromDomain(savedApplication);
        await client.UpsertDocumentAsync(
            CosmosContainerNames.SavedApplications, document, document.UserId,
            CosmosJsonSerializerContext.Default.SavedApplicationDocument, ct).ConfigureAwait(false);
    }

    public async Task DeleteAsync(string userId, string applicationUid, CancellationToken ct)
    {
        var id = SavedApplicationDocument.MakeId(userId, applicationUid);
        await client.DeleteDocumentAsync(
            CosmosContainerNames.SavedApplications, id, userId, ct).ConfigureAwait(false);
    }

    public async Task<IReadOnlyList<SavedApplication>> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        var docs = await client.QueryAsync(
            CosmosContainerNames.SavedApplications,
            "SELECT * FROM c WHERE c.userId = @userId",
            [new("@userId", userId)],
            userId,
            CosmosJsonSerializerContext.Default.SavedApplicationDocument, ct).ConfigureAwait(false);

        return docs.Select(doc => doc.ToDomain()).ToList();
    }

    public async Task<bool> ExistsAsync(string userId, string applicationUid, CancellationToken ct)
    {
        var id = SavedApplicationDocument.MakeId(userId, applicationUid);

        var doc = await client.ReadDocumentAsync(
            CosmosContainerNames.SavedApplications, id, userId,
            CosmosJsonSerializerContext.Default.SavedApplicationDocument, ct).ConfigureAwait(false);

        return doc is not null;
    }

    public async Task<IReadOnlyList<string>> GetUserIdsByApplicationUidAsync(string applicationUid, CancellationToken ct)
    {
        return await client.QueryAsync(
            CosmosContainerNames.SavedApplications,
            "SELECT VALUE c.userId FROM c WHERE c.applicationUid = @applicationUid",
            [new("@applicationUid", applicationUid)],
            null,
            CosmosJsonSerializerContext.Default.String, ct).ConfigureAwait(false);
    }
}
```

- [ ] **Step 4: Commit**

```bash
git add api/src/town-crier.infrastructure/Groups/CosmosGroupRepository.cs \
       api/src/town-crier.infrastructure/Groups/CosmosGroupInvitationRepository.cs \
       api/src/town-crier.infrastructure/SavedApplications/CosmosSavedApplicationRepository.cs
git commit -m "feat: migrate Group, GroupInvitation, and SavedApplication repos to REST client"
```

---

## Task 10: Build, test, and fix

**Files:**
- Potentially any file from Tasks 1-9 if fixes are needed

- [ ] **Step 1: Full build**

Run: `cd api && dotnet build`

Expected: Build succeeds with zero errors and zero AOT/trimming warnings (IL2104, IL3053, IL3000 should no longer appear).

If there are compilation errors, fix them — they'll be from typos or missed imports in the repository rewrites.

- [ ] **Step 2: Run all tests**

Run: `cd api && dotnet test`

Expected: All tests pass. The handler-level tests use `InMemory*Repository` implementations and should be unaffected. The new `CosmosRestClientTests` and `CosmosAuthProviderTests` should pass. The deleted test files (SDK helpers) should no longer be referenced.

If tests fail, diagnose and fix. Common issues:
- Missing `using` statements in migrated repositories
- `CosmosJsonSerializerContext` missing a type registration (add it)
- Test project still referencing deleted files (clean build)

- [ ] **Step 3: Verify no SDK references remain**

Run: `cd api && grep -r "Microsoft.Azure.Cosmos" --include="*.cs" --include="*.csproj" src/`

Expected: Zero matches. All SDK references should be gone.

- [ ] **Step 4: Verify no Newtonsoft references remain**

Run: `cd api && grep -r "Newtonsoft" --include="*.cs" --include="*.csproj" src/`

Expected: Zero matches.

- [ ] **Step 5: Check for trimming warnings**

Run: `cd api && dotnet publish src/town-crier.web -c Release 2>&1 | grep -i "IL[0-9]"`

Expected: Zero trimming warnings. If any appear, they're from our code (not the SDK) and need fixing.

- [ ] **Step 6: Commit any fixes**

```bash
git add -u
git commit -m "fix: resolve build and test issues from REST client migration"
```

(Skip this step if no fixes were needed.)

---

## Task 11: Update configuration

**Files:**
- Modify: `api/src/town-crier.web/appsettings.Development.json`

- [ ] **Step 1: Add Cosmos config to dev settings**

Add the `Cosmos` section to `api/src/town-crier.web/appsettings.Development.json`:

```json
{
  "Logging": {
    "LogLevel": {
      "Default": "Information",
      "Microsoft.AspNetCore": "Warning"
    }
  },
  "Cors": {
    "AllowedOrigins": [
      "http://localhost:5173"
    ]
  },
  "Cosmos": {
    "AccountEndpoint": "https://tc-shared-cosmos.documents.azure.com:443/",
    "DatabaseName": "town-crier-dev"
  }
}
```

Note: The `AccountEndpoint` value should match the actual dev Cosmos account. The infra stack exports this as `cosmosAccountEndpoint`. In production, this comes from the `Cosmos__AccountEndpoint` environment variable set by Pulumi.

- [ ] **Step 2: Commit**

```bash
git add api/src/town-crier.web/appsettings.Development.json
git commit -m "chore: add Cosmos REST client config to dev settings"
```

---

## Task 12: Update infra to remove connection string support

**Files:**
- Potentially modify: `infra/EnvironmentStack.cs` (if connection string is still being set)

- [ ] **Step 1: Check if infra sets a connection string env var**

Run: `grep -n "ConnectionStrings\|CosmosDb" infra/*.cs`

Expected: No matches for connection string config. The infra already uses `Cosmos__AccountEndpoint` env var. If any connection string references exist, remove them.

- [ ] **Step 2: Verify Cosmos__AccountEndpoint is set in container app config**

Run: `grep -n "Cosmos__AccountEndpoint\|Cosmos__DatabaseName" infra/*.cs`

Expected: `Cosmos__AccountEndpoint` is set. If `Cosmos__DatabaseName` is not set, it will fall back to the constant in `CosmosContainerNames.DatabaseName` ("town-crier"). Add it if the per-environment database name differs (e.g., "town-crier-dev" vs "town-crier").

- [ ] **Step 3: Commit any infra changes**

```bash
git add infra/
git commit -m "chore: update infra config for Cosmos REST client"
```

(Skip if no changes needed.)

---

## Task 13: Write ADR

**Files:**
- Create: `docs/adr/NNNN-cosmos-rest-client.md` (next sequence number)

- [ ] **Step 1: Determine next ADR number**

Run: `ls docs/adr/ | tail -1`

Use the next sequential number.

- [ ] **Step 2: Write the ADR**

Create `docs/adr/NNNN-cosmos-rest-client.md` (replace NNNN with actual number):

```markdown
# NNNN. Replace Cosmos DB SDK with direct REST API client

Date: 2026-03-29

## Status

Accepted (supersedes the Cosmos DB SDK portion of 0001-initial-tech-stack.md)

## Context

The Cosmos DB SDK v3 (Microsoft.Azure.Cosmos 3.58.0) fails under Native AOT trimming. The SDK's DocumentClient.Initialize() internally calls ConfigurationManager.AppSettings, which uses Activator.CreateInstance() — a reflection pattern that the AOT trimmer strips. This causes MissingMethodException at runtime in production.

The SDK has deep dependencies on Newtonsoft.Json and reflection throughout its transport layer (Microsoft.Azure.Cosmos.Direct, Microsoft.HybridRow). Patching individual trimmed types is not viable.

Alternatives investigated:
- EF Core 10 + Cosmos provider: does not support Native AOT (targeted for EF Core 12, ~late 2027)
- Microsoft.Azure.Cosmos.Aot (0.1.4-preview.2): experimental, not production-grade
- Trimmer root preservation: whack-a-mole with no completeness guarantee

## Decision

Replace the Cosmos DB SDK with a thin HttpClient-based REST client that talks directly to the Cosmos DB REST API. Authentication uses Entra ID (DefaultAzureCredential) exclusively — connection string support is removed. Retry resilience uses Polly v8 via Microsoft.Extensions.Http.Resilience.

## Consequences

- Native AOT publishing works without trimming warnings or runtime reflection failures
- Cold start times improve (no SDK initialization overhead, smaller binary)
- We lose SDK-provided features: automatic retry, connection pooling, change feed processor, bulk execution, integrated diagnostics
- Retry handling is now explicit via Polly (429, 408, 503, 449)
- Spatial queries (ST_DISTANCE) continue to work — they are server-side Cosmos SQL functions
- Future SDK AOT support (if/when GA) could allow reverting — the repository interfaces remain unchanged
```

- [ ] **Step 3: Commit**

```bash
git add docs/adr/
git commit -m "docs: add ADR for Cosmos REST client migration"
```
