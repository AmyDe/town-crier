# Polling Lease ETag CAS Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** [`docs/specs/polling-lease-cas.md`](polling-lease-cas.md)

**Goal:** Make the Cosmos polling lease a proper ETag-CAS mutex and serialise the Service Bus orchestrator and bootstrap cron through it, eliminating the race where the bootstrap re-seeds the queue while an orchestrator's handler is mid-run.

**Architecture:** Extend `ICosmosRestClient` with ETag-aware read + `If-Match` / `If-None-Match` write methods. Upgrade `CosmosPollingLeaseStore` to use CAS, returning a `LeaseHandle` carrying the winning ETag. Move lease acquisition from `PollPlanItCommandHandler` up to `PollTriggerOrchestrator` (before `ReceiveAsync`) and add a matching lease gate to `PollTriggerBootstrapper` (before the ARM probe).

**Tech Stack:** .NET 10, Native AOT, TUnit, Cosmos DB REST API (no SDK), Azure Service Bus REST API (no SDK).

**Constraints:**
- No SDKs — HTTP/JSON only (reflection-free for AOT).
- Source gen JSON serialisers (`CosmosJsonSerializerContext`) — every new DTO must be registered.
- `sealed` classes by default; TDD Red-Green-Refactor.
- No `bd edit` (opens vim and blocks). Use `bd create --title / --description / --notes` inline flags.

---

## File Structure

| Path | Role |
|---|---|
| `api/src/town-crier.infrastructure/Cosmos/ICosmosRestClient.cs` | **Modify** — add ETag-aware types + methods |
| `api/src/town-crier.infrastructure/Cosmos/CosmosRestClient.cs` | **Modify** — implement the new methods over existing HTTP plumbing |
| `api/src/town-crier.infrastructure/Cosmos/CosmosReadResult.cs` | **Create** — `CosmosReadResult<T>` record |
| `api/src/town-crier.infrastructure/Cosmos/CosmosDeleteOutcome.cs` | **Create** — `CosmosDeleteOutcome` enum |
| `api/src/town-crier.application/Polling/LeaseHandle.cs` | **Create** — opaque token carrying the winning ETag |
| `api/src/town-crier.application/Polling/LeaseAcquireResult.cs` | **Create** — `Acquired(handle)` / `Held` / `TransientError(ex)` |
| `api/src/town-crier.application/Polling/IPollingLeaseStore.cs` | **Modify** — replace bool/void methods with CAS signatures |
| `api/src/town-crier.infrastructure/Polling/CosmosPollingLeaseStore.cs` | **Modify** — CAS acquire + conditional delete |
| `api/src/town-crier.application/Polling/PollingOptions.cs` | **Modify** — add lease TTLs and retry delay |
| `api/src/town-crier.application/Polling/PollTriggerOrchestrator.cs` | **Modify** — acquire lease before receive; one retry + `finally` release |
| `api/src/town-crier.application/Polling/PollTriggerOrchestratorRunResult.cs` | **Modify** — add `LeaseUnavailable` |
| `api/src/town-crier.application/Polling/PollTriggerBootstrapper.cs` | **Modify** — add `IPollingLeaseStore` dep; acquire before probe |
| `api/src/town-crier.application/Polling/PollTriggerBootstrapResult.cs` | **Modify** — add `LeaseUnavailable` |
| `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs` | **Modify** — drop lease field/param/try-finally; drop `LeaseHeld` branch |
| `api/src/town-crier.application/Polling/PollTerminationReason.cs` | **Modify** — remove `LeaseHeld` value |
| `api/src/town-crier.application/Polling/PollTerminationReasonExtensions.cs` | **Modify** — drop `lease_held` mapping |
| `api/src/town-crier.infrastructure/Polling/PollingServiceExtensions.cs` | **Modify** — DI: bootstrap needs `IPollingLeaseStore`; handler no longer does |
| `api/src/town-crier.worker/Program.cs` | **Modify** — startup assertion: `HandlerBudget` must be set for `poll-sb` mode |
| `api/src/town-crier.application/Observability/PollingInstrumentation.cs` | **Modify** — three new lease counters |
| `api/tests/town-crier.infrastructure.tests/Cosmos/CosmosRestClientTests.cs` | **Modify** — tests for new ETag methods |
| `api/tests/town-crier.infrastructure.tests/Cosmos/FakeCosmosRestClient.cs` | **Modify** — ETag simulation |
| `api/tests/town-crier.infrastructure.tests/Polling/CosmosPollingLeaseStoreTests.cs` | **Modify** — CAS acquire + release scenarios |
| `api/tests/town-crier.application.tests/Polling/FakePollingLeaseStore.cs` | **Modify** — new signature + ETag sim |
| `api/tests/town-crier.application.tests/Polling/PollTriggerOrchestratorTests.cs` | **Modify** — lease retry + `LeaseUnavailable` coverage |
| `api/tests/town-crier.application.tests/Polling/PollTriggerBootstrapperTests.cs` | **Modify** — lease gate coverage |
| `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerLeaseTests.cs` | **Delete** — behaviour removed from handler |
| `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs` | **Modify** — drop lease injection from test builder |
| `api/tests/town-crier.integration-tests/Polling/PollLeaseCasIntegrationTests.cs` | **Create** — contention simulation |

---

## Task 1: Cosmos REST client — ETag-aware read

**Files:**
- Modify: `api/src/town-crier.infrastructure/Cosmos/ICosmosRestClient.cs`
- Modify: `api/src/town-crier.infrastructure/Cosmos/CosmosRestClient.cs`
- Create: `api/src/town-crier.infrastructure/Cosmos/CosmosReadResult.cs`
- Modify: `api/tests/town-crier.infrastructure.tests/Cosmos/CosmosRestClientTests.cs`
- Modify: `api/tests/town-crier.infrastructure.tests/Cosmos/StubHttpHandler.cs` (if needed for ETag header support)

- [ ] **Step 1: Create the `CosmosReadResult<T>` record**

`api/src/town-crier.infrastructure/Cosmos/CosmosReadResult.cs`:

```csharp
namespace TownCrier.Infrastructure.Cosmos;

/// <summary>
/// Result of a document read that surfaces the Cosmos-assigned ETag.
/// <see cref="Document"/> is <c>null</c> when the document does not exist
/// (HTTP 404). <see cref="ETag"/> is <c>null</c> in the same case.
/// </summary>
public sealed record CosmosReadResult<T>(T? Document, string? ETag);
```

- [ ] **Step 2: Write failing test for ETag-returning read**

Add to `CosmosRestClientTests.cs`:

```csharp
[Test]
public async Task ReadDocumentWithETag_ReturnsBodyAndETag_When200()
{
    var handler = new StubHttpHandler((req, _) =>
    {
        var resp = new HttpResponseMessage(HttpStatusCode.OK)
        {
            Content = new StringContent("""{"id":"x","payload":"hello"}""", Encoding.UTF8, "application/json"),
        };
        resp.Headers.ETag = new EntityTagHeaderValue("\"v1\"");
        return resp;
    });
    var client = BuildClient(handler);

    var result = await client.ReadDocumentWithETagAsync(
        "c", "x", "x", TestSerializerContext.Default.TestDocument, default);

    await Assert.That(result.Document).IsNotNull();
    await Assert.That(result.Document!.Payload).IsEqualTo("hello");
    await Assert.That(result.ETag).IsEqualTo("\"v1\"");
}

[Test]
public async Task ReadDocumentWithETag_ReturnsNullBodyAndETag_When404()
{
    var handler = new StubHttpHandler((_, _) => new HttpResponseMessage(HttpStatusCode.NotFound));
    var client = BuildClient(handler);

    var result = await client.ReadDocumentWithETagAsync(
        "c", "missing", "missing", TestSerializerContext.Default.TestDocument, default);

    await Assert.That(result.Document).IsNull();
    await Assert.That(result.ETag).IsNull();
}
```

- [ ] **Step 3: Run the test — verify it fails to compile**

Run: `dotnet test api/tests/town-crier.infrastructure.tests/town-crier.infrastructure.tests.csproj --filter "FullyQualifiedName~ReadDocumentWithETag"`
Expected: compile error — `ReadDocumentWithETagAsync` not defined.

- [ ] **Step 4: Add the interface method**

In `ICosmosRestClient.cs`:

```csharp
Task<CosmosReadResult<T>> ReadDocumentWithETagAsync<T>(
    string collection,
    string id,
    string partitionKey,
    JsonTypeInfo<T> typeInfo,
    CancellationToken ct);
```

- [ ] **Step 5: Implement in `CosmosRestClient`**

Mirror the existing `ReadDocumentAsync` shape but return `CosmosReadResult<T>`. ETag lives on `response.Headers.ETag` (`EntityTagHeaderValue`). Return `.Tag` as the string (it includes the quotes — that's the value Cosmos wants back in `If-Match`).

```csharp
public async Task<CosmosReadResult<T>> ReadDocumentWithETagAsync<T>(
    string collection, string id, string partitionKey,
    JsonTypeInfo<T> typeInfo, CancellationToken ct)
{
    using var request = BuildReadRequest(collection, id, partitionKey);
    using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

    if (response.StatusCode == HttpStatusCode.NotFound)
    {
        return new CosmosReadResult<T>(null, null);
    }

    await ThrowOnFailureAsync(response, "Read", ct).ConfigureAwait(false);

    var body = await response.Content.ReadFromJsonAsync(typeInfo, ct).ConfigureAwait(false);
    var etag = response.Headers.ETag?.Tag;
    return new CosmosReadResult<T>(body, etag);
}
```

Refactor point: if `BuildReadRequest` doesn't already exist, extract it from the existing `ReadDocumentAsync` so both methods share the request building. Don't duplicate the header+URL logic.

- [ ] **Step 6: Run test — verify it passes**

Run: `dotnet test api/tests/town-crier.infrastructure.tests/town-crier.infrastructure.tests.csproj --filter "FullyQualifiedName~ReadDocumentWithETag"`
Expected: 2/2 pass.

- [ ] **Step 7: Commit**

```bash
git add api/src/town-crier.infrastructure/Cosmos/ICosmosRestClient.cs \
        api/src/town-crier.infrastructure/Cosmos/CosmosRestClient.cs \
        api/src/town-crier.infrastructure/Cosmos/CosmosReadResult.cs \
        api/tests/town-crier.infrastructure.tests/Cosmos/CosmosRestClientTests.cs
git commit -m "feat(cosmos): add ReadDocumentWithETagAsync returning CosmosReadResult"
```

---

## Task 2: Cosmos REST client — TryCreateDocumentAsync (If-None-Match: *)

**Files:**
- Modify: `api/src/town-crier.infrastructure/Cosmos/ICosmosRestClient.cs`
- Modify: `api/src/town-crier.infrastructure/Cosmos/CosmosRestClient.cs`
- Modify: `api/tests/town-crier.infrastructure.tests/Cosmos/CosmosRestClientTests.cs`

- [ ] **Step 1: Write failing tests**

```csharp
[Test]
public async Task TryCreateDocument_ReturnsTrue_When201()
{
    var handler = new StubHttpHandler((req, _) =>
    {
        // Verify If-None-Match: * was sent.
        var header = req.Headers.IfNoneMatch.Single().Tag;
        if (header != "*") return new HttpResponseMessage(HttpStatusCode.BadRequest);
        return new HttpResponseMessage(HttpStatusCode.Created);
    });
    var client = BuildClient(handler);

    var created = await client.TryCreateDocumentAsync(
        "c", new TestDocument { Id = "x", Payload = "hi" }, "x",
        TestSerializerContext.Default.TestDocument, default);

    await Assert.That(created).IsTrue();
}

[Test]
public async Task TryCreateDocument_ReturnsFalse_When409()
{
    var handler = new StubHttpHandler((_, _) => new HttpResponseMessage(HttpStatusCode.Conflict));
    var client = BuildClient(handler);

    var created = await client.TryCreateDocumentAsync(
        "c", new TestDocument { Id = "x", Payload = "hi" }, "x",
        TestSerializerContext.Default.TestDocument, default);

    await Assert.That(created).IsFalse();
}

[Test]
public async Task TryCreateDocument_Throws_On5xx()
{
    var handler = new StubHttpHandler((_, _) => new HttpResponseMessage(HttpStatusCode.InternalServerError));
    var client = BuildClient(handler);

    await Assert.ThrowsAsync<HttpRequestException>(async () =>
        await client.TryCreateDocumentAsync(
            "c", new TestDocument { Id = "x", Payload = "hi" }, "x",
            TestSerializerContext.Default.TestDocument, default));
}
```

- [ ] **Step 2: Run — verify fail**

Run: `dotnet test --filter "FullyQualifiedName~TryCreateDocument"`
Expected: compile error.

- [ ] **Step 3: Add interface method**

```csharp
Task<bool> TryCreateDocumentAsync<T>(
    string collection,
    T document,
    string partitionKey,
    JsonTypeInfo<T> typeInfo,
    CancellationToken ct);
```

- [ ] **Step 4: Implement**

Cosmos REST creates a document with `POST /dbs/{db}/colls/{coll}/docs`. `If-None-Match: *` makes it fail with `409 Conflict` if any document with the same id already exists (in the same partition).

```csharp
public async Task<bool> TryCreateDocumentAsync<T>(
    string collection, T document, string partitionKey,
    JsonTypeInfo<T> typeInfo, CancellationToken ct)
{
    using var request = BuildCreateRequest(collection, document, partitionKey, typeInfo);
    request.Headers.TryAddWithoutValidation("If-None-Match", "*");
    using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

    if (response.StatusCode == HttpStatusCode.Created) return true;
    if (response.StatusCode == HttpStatusCode.Conflict) return false;

    await ThrowOnFailureAsync(response, "TryCreate", ct).ConfigureAwait(false);
    return false; // unreachable
}
```

If no `BuildCreateRequest` exists, extract the request-building from `UpsertDocumentAsync` first — Upsert is POST without the `x-ms-documentdb-is-upsert: true` header flipped to create-only semantics. Check the existing implementation and match its pattern.

- [ ] **Step 5: Run — verify pass**

Run: `dotnet test --filter "FullyQualifiedName~TryCreateDocument"`
Expected: 3/3 pass.

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.infrastructure/Cosmos/ \
        api/tests/town-crier.infrastructure.tests/Cosmos/CosmosRestClientTests.cs
git commit -m "feat(cosmos): add TryCreateDocumentAsync with If-None-Match: *"
```

---

## Task 3: Cosmos REST client — TryReplaceDocumentAsync (If-Match)

**Files:**
- Modify: `api/src/town-crier.infrastructure/Cosmos/ICosmosRestClient.cs`
- Modify: `api/src/town-crier.infrastructure/Cosmos/CosmosRestClient.cs`
- Modify: `api/tests/town-crier.infrastructure.tests/Cosmos/CosmosRestClientTests.cs`

- [ ] **Step 1: Write failing tests**

```csharp
[Test]
public async Task TryReplaceDocument_ReturnsTrue_When200()
{
    var handler = new StubHttpHandler((req, _) =>
    {
        var ifMatch = req.Headers.IfMatch.Single().Tag;
        if (ifMatch != "\"v1\"") return new HttpResponseMessage(HttpStatusCode.BadRequest);
        return new HttpResponseMessage(HttpStatusCode.OK);
    });
    var client = BuildClient(handler);

    var replaced = await client.TryReplaceDocumentAsync(
        "c", new TestDocument { Id = "x", Payload = "new" }, "x", "\"v1\"",
        TestSerializerContext.Default.TestDocument, default);

    await Assert.That(replaced).IsTrue();
}

[Test]
public async Task TryReplaceDocument_ReturnsFalse_When412()
{
    var handler = new StubHttpHandler((_, _) => new HttpResponseMessage(HttpStatusCode.PreconditionFailed));
    var client = BuildClient(handler);

    var replaced = await client.TryReplaceDocumentAsync(
        "c", new TestDocument { Id = "x", Payload = "new" }, "x", "\"stale\"",
        TestSerializerContext.Default.TestDocument, default);

    await Assert.That(replaced).IsFalse();
}
```

- [ ] **Step 2: Verify fail** — `dotnet test --filter "FullyQualifiedName~TryReplaceDocument"`

- [ ] **Step 3: Add interface method**

```csharp
Task<bool> TryReplaceDocumentAsync<T>(
    string collection,
    T document,
    string partitionKey,
    string ifMatchEtag,
    JsonTypeInfo<T> typeInfo,
    CancellationToken ct);
```

- [ ] **Step 4: Implement**

Cosmos REST replaces with `PUT /dbs/{db}/colls/{coll}/docs/{id}` + `If-Match: <etag>`.

```csharp
public async Task<bool> TryReplaceDocumentAsync<T>(
    string collection, T document, string partitionKey, string ifMatchEtag,
    JsonTypeInfo<T> typeInfo, CancellationToken ct)
{
    ArgumentException.ThrowIfNullOrEmpty(ifMatchEtag);

    using var request = BuildReplaceRequest(collection, document, partitionKey, typeInfo);
    request.Headers.TryAddWithoutValidation("If-Match", ifMatchEtag);
    using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

    if (response.IsSuccessStatusCode) return true;
    if (response.StatusCode == HttpStatusCode.PreconditionFailed) return false;

    await ThrowOnFailureAsync(response, "TryReplace", ct).ConfigureAwait(false);
    return false; // unreachable
}
```

- [ ] **Step 5: Verify pass** — `dotnet test --filter "FullyQualifiedName~TryReplaceDocument"`

- [ ] **Step 6: Commit**

```bash
git commit -am "feat(cosmos): add TryReplaceDocumentAsync with If-Match CAS"
```

---

## Task 4: Cosmos REST client — TryDeleteDocumentAsync (optional If-Match)

**Files:**
- Modify: `api/src/town-crier.infrastructure/Cosmos/ICosmosRestClient.cs`
- Modify: `api/src/town-crier.infrastructure/Cosmos/CosmosRestClient.cs`
- Create: `api/src/town-crier.infrastructure/Cosmos/CosmosDeleteOutcome.cs`
- Modify: `api/tests/town-crier.infrastructure.tests/Cosmos/CosmosRestClientTests.cs`

- [ ] **Step 1: Create the enum**

`api/src/town-crier.infrastructure/Cosmos/CosmosDeleteOutcome.cs`:

```csharp
namespace TownCrier.Infrastructure.Cosmos;

public enum CosmosDeleteOutcome
{
    Deleted = 0,
    NotFound = 1,
    PreconditionFailed = 2,
}
```

- [ ] **Step 2: Write failing tests**

```csharp
[Test]
public async Task TryDeleteDocument_ReturnsDeleted_When204()
{
    var handler = new StubHttpHandler((_, _) => new HttpResponseMessage(HttpStatusCode.NoContent));
    var client = BuildClient(handler);

    var outcome = await client.TryDeleteDocumentAsync("c", "x", "x", ifMatchEtag: null, default);

    await Assert.That(outcome).IsEqualTo(CosmosDeleteOutcome.Deleted);
}

[Test]
public async Task TryDeleteDocument_ReturnsNotFound_When404()
{
    var handler = new StubHttpHandler((_, _) => new HttpResponseMessage(HttpStatusCode.NotFound));
    var client = BuildClient(handler);

    var outcome = await client.TryDeleteDocumentAsync("c", "x", "x", ifMatchEtag: null, default);

    await Assert.That(outcome).IsEqualTo(CosmosDeleteOutcome.NotFound);
}

[Test]
public async Task TryDeleteDocument_ReturnsPreconditionFailed_When412()
{
    var handler = new StubHttpHandler((req, _) =>
    {
        var ifMatch = req.Headers.IfMatch.Single().Tag;
        if (ifMatch != "\"stale\"") return new HttpResponseMessage(HttpStatusCode.BadRequest);
        return new HttpResponseMessage(HttpStatusCode.PreconditionFailed);
    });
    var client = BuildClient(handler);

    var outcome = await client.TryDeleteDocumentAsync("c", "x", "x", ifMatchEtag: "\"stale\"", default);

    await Assert.That(outcome).IsEqualTo(CosmosDeleteOutcome.PreconditionFailed);
}
```

- [ ] **Step 3: Verify fail** — `dotnet test --filter "FullyQualifiedName~TryDeleteDocument"`

- [ ] **Step 4: Add interface method**

```csharp
Task<CosmosDeleteOutcome> TryDeleteDocumentAsync(
    string collection,
    string id,
    string partitionKey,
    string? ifMatchEtag,
    CancellationToken ct);
```

- [ ] **Step 5: Implement**

```csharp
public async Task<CosmosDeleteOutcome> TryDeleteDocumentAsync(
    string collection, string id, string partitionKey, string? ifMatchEtag,
    CancellationToken ct)
{
    using var request = BuildDeleteRequest(collection, id, partitionKey);
    if (!string.IsNullOrEmpty(ifMatchEtag))
    {
        request.Headers.TryAddWithoutValidation("If-Match", ifMatchEtag);
    }
    using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

    return response.StatusCode switch
    {
        HttpStatusCode.NoContent => CosmosDeleteOutcome.Deleted,
        HttpStatusCode.OK => CosmosDeleteOutcome.Deleted, // some tiers return 200
        HttpStatusCode.NotFound => CosmosDeleteOutcome.NotFound,
        HttpStatusCode.PreconditionFailed => CosmosDeleteOutcome.PreconditionFailed,
        _ => await FailAsync(response, ct).ConfigureAwait(false),
    };

    static async Task<CosmosDeleteOutcome> FailAsync(HttpResponseMessage r, CancellationToken ct)
    {
        await ThrowOnFailureAsync(r, "TryDelete", ct).ConfigureAwait(false);
        return CosmosDeleteOutcome.Deleted; // unreachable
    }
}
```

Note: existing `DeleteDocumentAsync` stays — unchanged — for call sites that don't care about CAS.

- [ ] **Step 6: Verify pass** — 3/3 tests pass.

- [ ] **Step 7: Commit**

```bash
git commit -am "feat(cosmos): add TryDeleteDocumentAsync with optional If-Match"
```

---

## Task 5: Update FakeCosmosRestClient with ETag simulation

**Files:**
- Modify: `api/tests/town-crier.infrastructure.tests/Cosmos/FakeCosmosRestClient.cs`

- [ ] **Step 1: Write a test asserting the fake's ETag semantics**

Add a new test file `api/tests/town-crier.infrastructure.tests/Cosmos/FakeCosmosRestClientCasTests.cs` (or extend an existing one):

```csharp
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Tests.Cosmos;

public sealed class FakeCosmosRestClientCasTests
{
    [Test]
    public async Task Create_AssignsEtag_And_ReadReturnsIt()
    {
        var fake = new FakeCosmosRestClient();
        var created = await fake.TryCreateDocumentAsync(
            "c", new TestDocument { Id = "a", Payload = "v0" }, "a",
            TestSerializerContext.Default.TestDocument, default);
        await Assert.That(created).IsTrue();

        var read = await fake.ReadDocumentWithETagAsync(
            "c", "a", "a", TestSerializerContext.Default.TestDocument, default);
        await Assert.That(read.Document!.Payload).IsEqualTo("v0");
        await Assert.That(read.ETag).IsNotNull();
    }

    [Test]
    public async Task Create_SecondCallReturnsFalse_When_DocumentExists()
    {
        var fake = new FakeCosmosRestClient();
        await fake.TryCreateDocumentAsync("c", new TestDocument { Id = "a", Payload = "v0" }, "a",
            TestSerializerContext.Default.TestDocument, default);
        var secondCreate = await fake.TryCreateDocumentAsync(
            "c", new TestDocument { Id = "a", Payload = "v1" }, "a",
            TestSerializerContext.Default.TestDocument, default);
        await Assert.That(secondCreate).IsFalse();
    }

    [Test]
    public async Task Replace_ReturnsTrue_WithMatchingEtag_AndBumpsEtag()
    {
        var fake = new FakeCosmosRestClient();
        await fake.TryCreateDocumentAsync("c", new TestDocument { Id = "a", Payload = "v0" }, "a",
            TestSerializerContext.Default.TestDocument, default);
        var read1 = await fake.ReadDocumentWithETagAsync("c", "a", "a",
            TestSerializerContext.Default.TestDocument, default);

        var ok = await fake.TryReplaceDocumentAsync(
            "c", new TestDocument { Id = "a", Payload = "v1" }, "a", read1.ETag!,
            TestSerializerContext.Default.TestDocument, default);
        await Assert.That(ok).IsTrue();

        var read2 = await fake.ReadDocumentWithETagAsync("c", "a", "a",
            TestSerializerContext.Default.TestDocument, default);
        await Assert.That(read2.ETag).IsNotEqualTo(read1.ETag);
    }

    [Test]
    public async Task Replace_ReturnsFalse_WithStaleEtag()
    {
        var fake = new FakeCosmosRestClient();
        await fake.TryCreateDocumentAsync("c", new TestDocument { Id = "a", Payload = "v0" }, "a",
            TestSerializerContext.Default.TestDocument, default);

        var ok = await fake.TryReplaceDocumentAsync(
            "c", new TestDocument { Id = "a", Payload = "v1" }, "a", "\"stale\"",
            TestSerializerContext.Default.TestDocument, default);
        await Assert.That(ok).IsFalse();
    }

    [Test]
    public async Task Delete_PreconditionFailed_WithStaleEtag()
    {
        var fake = new FakeCosmosRestClient();
        await fake.TryCreateDocumentAsync("c", new TestDocument { Id = "a", Payload = "v0" }, "a",
            TestSerializerContext.Default.TestDocument, default);

        var outcome = await fake.TryDeleteDocumentAsync("c", "a", "a", "\"stale\"", default);
        await Assert.That(outcome).IsEqualTo(CosmosDeleteOutcome.PreconditionFailed);
    }
}
```

- [ ] **Step 2: Verify fail** — fake doesn't implement new methods.

- [ ] **Step 3: Extend FakeCosmosRestClient**

The existing fake stores documents in a dictionary. Add a parallel ETag counter:

```csharp
// Add fields:
private readonly Dictionary<string, string> etags = new();
private long nextEtag = 0;

// New helper:
private string NewEtag() => $"\"v{Interlocked.Increment(ref this.nextEtag)}\"";

// Implement new methods:
public Task<CosmosReadResult<T>> ReadDocumentWithETagAsync<T>(
    string collection, string id, string partitionKey,
    JsonTypeInfo<T> typeInfo, CancellationToken ct)
{
    var key = Key(collection, partitionKey, id);
    if (!this.store.TryGetValue(key, out var json))
    {
        return Task.FromResult(new CosmosReadResult<T>(null, null));
    }
    var doc = JsonSerializer.Deserialize(json, typeInfo);
    this.etags.TryGetValue(key, out var etag);
    return Task.FromResult(new CosmosReadResult<T>(doc, etag));
}

public Task<bool> TryCreateDocumentAsync<T>(
    string collection, T document, string partitionKey,
    JsonTypeInfo<T> typeInfo, CancellationToken ct)
{
    var id = GetId(document, typeInfo);
    var key = Key(collection, partitionKey, id);
    if (this.store.ContainsKey(key))
    {
        return Task.FromResult(false);
    }
    this.store[key] = JsonSerializer.Serialize(document, typeInfo);
    this.etags[key] = NewEtag();
    return Task.FromResult(true);
}

public Task<bool> TryReplaceDocumentAsync<T>(
    string collection, T document, string partitionKey, string ifMatchEtag,
    JsonTypeInfo<T> typeInfo, CancellationToken ct)
{
    var id = GetId(document, typeInfo);
    var key = Key(collection, partitionKey, id);
    if (!this.etags.TryGetValue(key, out var current) || current != ifMatchEtag)
    {
        return Task.FromResult(false);
    }
    this.store[key] = JsonSerializer.Serialize(document, typeInfo);
    this.etags[key] = NewEtag();
    return Task.FromResult(true);
}

public Task<CosmosDeleteOutcome> TryDeleteDocumentAsync(
    string collection, string id, string partitionKey, string? ifMatchEtag,
    CancellationToken ct)
{
    var key = Key(collection, partitionKey, id);
    if (!this.store.ContainsKey(key))
    {
        return Task.FromResult(CosmosDeleteOutcome.NotFound);
    }
    if (ifMatchEtag is not null && this.etags.TryGetValue(key, out var current) && current != ifMatchEtag)
    {
        return Task.FromResult(CosmosDeleteOutcome.PreconditionFailed);
    }
    this.store.Remove(key);
    this.etags.Remove(key);
    return Task.FromResult(CosmosDeleteOutcome.Deleted);
}
```

`GetId` reads the document's `id` via `JsonSerializer.SerializeToElement` then `.GetProperty("id").GetString()` — no reflection needed, compatible with AOT source-generated serializers (this is a test helper, so reflection would be acceptable here, but keep it AOT-clean for parity). If there's already a `Key`/`GetId` pattern, follow it.

- [ ] **Step 4: Verify pass** — 5/5 tests pass.

- [ ] **Step 5: Commit**

```bash
git commit -am "test(cosmos): extend FakeCosmosRestClient with ETag CAS semantics"
```

---

## Task 6: Add `LeaseHandle` + `LeaseAcquireResult` + new `IPollingLeaseStore` shape

**Files:**
- Create: `api/src/town-crier.application/Polling/LeaseHandle.cs`
- Create: `api/src/town-crier.application/Polling/LeaseAcquireResult.cs`
- Modify: `api/src/town-crier.application/Polling/IPollingLeaseStore.cs`

This task changes the interface but does not yet change the implementation or the callers — compile will break until Task 7 + Task 8.

- [ ] **Step 1: Create `LeaseHandle`**

```csharp
namespace TownCrier.Application.Polling;

/// <summary>
/// Opaque token returned by a successful <see cref="IPollingLeaseStore.TryAcquireAsync"/>.
/// Carries the ETag of the winning write so <see cref="IPollingLeaseStore.ReleaseAsync"/>
/// can perform a conditional delete.
/// </summary>
public sealed record LeaseHandle(string ETag);
```

- [ ] **Step 2: Create `LeaseAcquireResult`**

```csharp
namespace TownCrier.Application.Polling;

/// <summary>
/// Outcome of a lease acquire attempt.
/// </summary>
public sealed record LeaseAcquireResult
{
    public bool Acquired => this.Handle is not null;
    public LeaseHandle? Handle { get; init; }
    public bool Held { get; init; }
    public Exception? TransientError { get; init; }

    public static LeaseAcquireResult FromAcquired(LeaseHandle handle) => new() { Handle = handle };
    public static LeaseAcquireResult FromHeld() => new() { Held = true };
    public static LeaseAcquireResult FromTransient(Exception ex) => new() { TransientError = ex };
}
```

- [ ] **Step 3: Replace `IPollingLeaseStore`**

```csharp
namespace TownCrier.Application.Polling;

/// <summary>
/// ETag-CAS distributed lease for the polling cycle. Both
/// <see cref="PollTriggerOrchestrator"/> and <see cref="PollTriggerBootstrapper"/>
/// acquire this lease before any action that could mutate the poll queue.
/// Serialisation is enforced via Cosmos If-Match / If-None-Match preconditions.
/// </summary>
public interface IPollingLeaseStore
{
    /// <summary>
    /// Attempts to acquire the polling lease with the given TTL. Returns an
    /// <see cref="LeaseAcquireResult"/> distinguishing Acquired / Held /
    /// TransientError. Never throws for expected outcomes (held by peer,
    /// raced on create/replace).
    /// </summary>
    Task<LeaseAcquireResult> TryAcquireAsync(TimeSpan ttl, CancellationToken ct);

    /// <summary>
    /// Releases the lease identified by <paramref name="handle"/>. Performs a
    /// conditional delete using the ETag from acquire. Never throws — failures
    /// are logged via the store's own logger, if any.
    /// </summary>
    Task ReleaseAsync(LeaseHandle handle, CancellationToken ct);
}
```

- [ ] **Step 4: Do not compile yet**

There's no test here — this is a signature-only change. The compile will break in `CosmosPollingLeaseStore`, `FakePollingLeaseStore`, `PollPlanItCommandHandler`, and their tests until subsequent tasks land. That's expected — the next task fixes the infrastructure side.

- [ ] **Step 5: Commit**

```bash
git add api/src/town-crier.application/Polling/LeaseHandle.cs \
        api/src/town-crier.application/Polling/LeaseAcquireResult.cs \
        api/src/town-crier.application/Polling/IPollingLeaseStore.cs
git commit -m "feat(polling): introduce LeaseHandle + LeaseAcquireResult; reshape IPollingLeaseStore"
```

---

## Task 7: Implement CAS in `CosmosPollingLeaseStore`

**Files:**
- Modify: `api/src/town-crier.infrastructure/Polling/CosmosPollingLeaseStore.cs`
- Modify: `api/tests/town-crier.infrastructure.tests/Polling/CosmosPollingLeaseStoreTests.cs`
- Modify: `api/src/town-crier.infrastructure/Polling/PollingLeaseDocument.cs` (add `acquiredAtUtc` field if not present)

- [ ] **Step 1: Write failing tests**

Replace the body of `CosmosPollingLeaseStoreTests.cs` with CAS coverage:

```csharp
using TownCrier.Application.Polling;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.Polling;

namespace TownCrier.Infrastructure.Tests.Polling;

public sealed class CosmosPollingLeaseStoreTests
{
    private static readonly DateTimeOffset Now = new(2026, 4, 23, 12, 0, 0, TimeSpan.Zero);
    private static readonly TimeProvider TimeFixed = new FakeTimeProvider(Now);

    private static CosmosPollingLeaseStore BuildStore(FakeCosmosRestClient fake) =>
        new(fake, TimeFixed);

    [Test]
    public async Task TryAcquire_CreatesDocument_When_NoneExists()
    {
        var fake = new FakeCosmosRestClient();
        var store = BuildStore(fake);

        var result = await store.TryAcquireAsync(TimeSpan.FromMinutes(5), default);

        await Assert.That(result.Acquired).IsTrue();
        await Assert.That(result.Handle).IsNotNull();
        await Assert.That(result.Handle!.ETag).IsNotNull();
    }

    [Test]
    public async Task TryAcquire_ReturnsHeld_When_ExistingNotExpired()
    {
        var fake = new FakeCosmosRestClient();
        var store = BuildStore(fake);
        var first = await store.TryAcquireAsync(TimeSpan.FromMinutes(5), default);
        await Assert.That(first.Acquired).IsTrue();

        var second = await store.TryAcquireAsync(TimeSpan.FromMinutes(5), default);

        await Assert.That(second.Acquired).IsFalse();
        await Assert.That(second.Held).IsTrue();
    }

    [Test]
    public async Task TryAcquire_ReacquiresViaCas_When_ExistingExpired()
    {
        var fake = new FakeCosmosRestClient();
        var time = new MutableTimeProvider(Now);
        var store = new CosmosPollingLeaseStore(fake, time);
        var first = await store.TryAcquireAsync(TimeSpan.FromMinutes(5), default);

        time.Advance(TimeSpan.FromMinutes(10)); // lease expired

        var second = await store.TryAcquireAsync(TimeSpan.FromMinutes(5), default);

        await Assert.That(second.Acquired).IsTrue();
        await Assert.That(second.Handle!.ETag).IsNotEqualTo(first.Handle!.ETag);
    }

    [Test]
    public async Task Release_DeletesDocument_When_EtagMatches()
    {
        var fake = new FakeCosmosRestClient();
        var store = BuildStore(fake);
        var result = await store.TryAcquireAsync(TimeSpan.FromMinutes(5), default);

        await store.ReleaseAsync(result.Handle!, default);

        var readAfter = await fake.ReadDocumentWithETagAsync(
            CosmosContainerNames.Leases, "polling", "polling",
            CosmosJsonSerializerContext.Default.PollingLeaseDocument, default);
        await Assert.That(readAfter.Document).IsNull();
    }

    [Test]
    public async Task Release_Swallows_When_EtagIsStale()
    {
        var fake = new FakeCosmosRestClient();
        var store = BuildStore(fake);
        var acquire = await store.TryAcquireAsync(TimeSpan.FromMinutes(5), default);

        // Simulate a peer taking over via a direct replace.
        await fake.TryReplaceDocumentAsync(
            CosmosContainerNames.Leases,
            new PollingLeaseDocument
            {
                Id = "polling",
                HolderId = "peer",
                ExpiresAtUtc = Now.AddMinutes(5).ToString("o"),
            },
            "polling",
            acquire.Handle!.ETag,
            CosmosJsonSerializerContext.Default.PollingLeaseDocument,
            default);

        // Release should not throw; fake records the attempt.
        await store.ReleaseAsync(acquire.Handle, default);
    }
}
```

Depending on the repo's test-time utilities, `MutableTimeProvider` may need to be created as a small helper if none exists; otherwise prefer an existing fake. Do not invent new abstractions — grep for `MutableTimeProvider` / `FakeTimeProvider` and follow existing patterns.

- [ ] **Step 2: Verify fail** — compile errors + test failures.

- [ ] **Step 3: Rewrite `CosmosPollingLeaseStore`**

```csharp
using System.Globalization;
using TownCrier.Application.Polling;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Polling;

/// <summary>
/// ETag-CAS-backed <see cref="IPollingLeaseStore"/> over the Cosmos
/// <c>Leases</c> container. See docs/specs/polling-lease-cas.md.
/// </summary>
public sealed class CosmosPollingLeaseStore : IPollingLeaseStore
{
    private const string LeaseDocumentId = "polling";

    private readonly ICosmosRestClient client;
    private readonly TimeProvider timeProvider;
    private readonly string holderId;

    public CosmosPollingLeaseStore(ICosmosRestClient client, TimeProvider timeProvider)
    {
        ArgumentNullException.ThrowIfNull(client);
        ArgumentNullException.ThrowIfNull(timeProvider);

        this.client = client;
        this.timeProvider = timeProvider;
        this.holderId = Guid.NewGuid().ToString("N");
    }

    public async Task<LeaseAcquireResult> TryAcquireAsync(TimeSpan ttl, CancellationToken ct)
    {
        try
        {
            var now = this.timeProvider.GetUtcNow();
            var read = await this.client.ReadDocumentWithETagAsync(
                CosmosContainerNames.Leases, LeaseDocumentId, LeaseDocumentId,
                CosmosJsonSerializerContext.Default.PollingLeaseDocument, ct).ConfigureAwait(false);

            var desired = new PollingLeaseDocument
            {
                Id = LeaseDocumentId,
                HolderId = this.holderId,
                AcquiredAtUtc = now.ToString("o", CultureInfo.InvariantCulture),
                ExpiresAtUtc = (now + ttl).ToString("o", CultureInfo.InvariantCulture),
            };

            if (read.Document is null)
            {
                var created = await this.client.TryCreateDocumentAsync(
                    CosmosContainerNames.Leases, desired, LeaseDocumentId,
                    CosmosJsonSerializerContext.Default.PollingLeaseDocument, ct).ConfigureAwait(false);
                if (!created) return LeaseAcquireResult.FromHeld();

                // Read back to capture the server-assigned ETag.
                var after = await this.client.ReadDocumentWithETagAsync(
                    CosmosContainerNames.Leases, LeaseDocumentId, LeaseDocumentId,
                    CosmosJsonSerializerContext.Default.PollingLeaseDocument, ct).ConfigureAwait(false);
                return after.ETag is null
                    ? LeaseAcquireResult.FromHeld()
                    : LeaseAcquireResult.FromAcquired(new LeaseHandle(after.ETag));
            }

            if (TryParseExpiry(read.Document.ExpiresAtUtc, out var expiresAt) && expiresAt > now)
            {
                return LeaseAcquireResult.FromHeld();
            }

            var replaced = await this.client.TryReplaceDocumentAsync(
                CosmosContainerNames.Leases, desired, LeaseDocumentId, read.ETag!,
                CosmosJsonSerializerContext.Default.PollingLeaseDocument, ct).ConfigureAwait(false);
            if (!replaced) return LeaseAcquireResult.FromHeld();

            var readAfterReplace = await this.client.ReadDocumentWithETagAsync(
                CosmosContainerNames.Leases, LeaseDocumentId, LeaseDocumentId,
                CosmosJsonSerializerContext.Default.PollingLeaseDocument, ct).ConfigureAwait(false);
            return readAfterReplace.ETag is null
                ? LeaseAcquireResult.FromHeld()
                : LeaseAcquireResult.FromAcquired(new LeaseHandle(readAfterReplace.ETag));
        }
#pragma warning disable CA1031 // Convert transient failures to a result type; swallow above classes of exception
        catch (Exception ex)
#pragma warning restore CA1031
        {
            return LeaseAcquireResult.FromTransient(ex);
        }
    }

    public async Task ReleaseAsync(LeaseHandle handle, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(handle);
        try
        {
            _ = await this.client.TryDeleteDocumentAsync(
                CosmosContainerNames.Leases, LeaseDocumentId, LeaseDocumentId, handle.ETag, ct)
                .ConfigureAwait(false);
            // Caller (orchestrator / bootstrap) logs the outcome via its own logger.
        }
#pragma warning disable CA1031 // Release is best-effort.
        catch
#pragma warning restore CA1031
        {
            // Swallow; TTL is the backstop.
        }
    }

    private static bool TryParseExpiry(string value, out DateTimeOffset expiresAt) =>
        DateTimeOffset.TryParse(value, CultureInfo.InvariantCulture, DateTimeStyles.RoundtripKind, out expiresAt);
}
```

Note: the Read-after-Create/Replace round-trip is a trade-off to keep `ICosmosRestClient`'s existing `TryCreate` / `TryReplace` return types as `bool`. An alternative is to return the new ETag from those methods (via the response headers). If that refactor is preferred, do it here and update Tasks 2 and 3 accordingly — the test coverage is the same.

- [ ] **Step 4: Update `PollingLeaseDocument` if needed**

Check `api/src/town-crier.infrastructure/Polling/PollingLeaseDocument.cs`. It must include:

```csharp
public sealed class PollingLeaseDocument
{
    [JsonPropertyName("id")]           public string Id { get; set; } = "";
    [JsonPropertyName("holderId")]     public string HolderId { get; set; } = "";
    [JsonPropertyName("acquiredAtUtc")]public string AcquiredAtUtc { get; set; } = "";
    [JsonPropertyName("expiresAtUtc")] public string ExpiresAtUtc { get; set; } = "";
}
```

Adding a new property to a source-generated serialized type requires the `CosmosJsonSerializerContext` partial to already list this type (it does — `PollingLeaseDocument` is in use today). Verify no `[JsonSerializable]` attribute change is needed.

- [ ] **Step 5: Verify pass** — all CosmosPollingLeaseStoreTests green.

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.infrastructure/Polling/ \
        api/tests/town-crier.infrastructure.tests/Polling/CosmosPollingLeaseStoreTests.cs
git commit -m "feat(polling): CAS-based CosmosPollingLeaseStore with LeaseHandle"
```

---

## Task 8: Update `FakePollingLeaseStore`

**Files:**
- Modify: `api/tests/town-crier.application.tests/Polling/FakePollingLeaseStore.cs`

- [ ] **Step 1: Rewrite the fake**

```csharp
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakePollingLeaseStore : IPollingLeaseStore
{
    private LeaseHandle? held;
    private int acquireCalls;
    private int releaseCalls;

    public int AcquireCalls => this.acquireCalls;
    public int ReleaseCalls => this.releaseCalls;

    /// <summary>If true, every acquire returns Held until cleared.</summary>
    public bool SimulateHeld { get; set; }

    /// <summary>If set, the next acquire returns TransientError with this exception.</summary>
    public Exception? NextAcquireException { get; set; }

    public Task<LeaseAcquireResult> TryAcquireAsync(TimeSpan ttl, CancellationToken ct)
    {
        Interlocked.Increment(ref this.acquireCalls);

        if (this.NextAcquireException is { } ex)
        {
            this.NextAcquireException = null;
            return Task.FromResult(LeaseAcquireResult.FromTransient(ex));
        }

        if (this.SimulateHeld || this.held is not null)
        {
            return Task.FromResult(LeaseAcquireResult.FromHeld());
        }

        this.held = new LeaseHandle($"\"etag-{Guid.NewGuid():N}\"");
        return Task.FromResult(LeaseAcquireResult.FromAcquired(this.held));
    }

    public Task ReleaseAsync(LeaseHandle handle, CancellationToken ct)
    {
        Interlocked.Increment(ref this.releaseCalls);
        if (this.held is not null && this.held.ETag == handle.ETag)
        {
            this.held = null;
        }
        return Task.CompletedTask;
    }
}
```

- [ ] **Step 2: Compile**

Run `dotnet build api/town-crier.sln` — application.tests should compile. Handler tests still reference `leaseStore.TryAcquireAsync` with old signature; that'll be fixed in Task 12. Expect broken compile there for now.

- [ ] **Step 3: Commit**

```bash
git commit -am "test(polling): update FakePollingLeaseStore to new IPollingLeaseStore shape"
```

---

## Task 9: Add new `PollingOptions` fields

**Files:**
- Modify: `api/src/town-crier.application/Polling/PollingOptions.cs`

- [ ] **Step 1: Edit the record**

```csharp
// Add to PollingOptions (keep existing fields):
public TimeSpan OrchestratorLeaseTtl { get; init; } = TimeSpan.FromMinutes(4.5);
public TimeSpan BootstrapLeaseTtl    { get; init; } = TimeSpan.FromSeconds(60);
public TimeSpan LeaseAcquireRetryDelay { get; init; } = TimeSpan.FromSeconds(1);
```

Leave the existing `LeaseTtl` in place for now — it's referenced by the handler until Task 12. Delete it in Task 13's cleanup.

- [ ] **Step 2: Compile**

`dotnet build api/town-crier.sln` — still broken in handler tests, unchanged elsewhere.

- [ ] **Step 3: Commit**

```bash
git commit -am "feat(polling): add OrchestratorLeaseTtl / BootstrapLeaseTtl / LeaseAcquireRetryDelay options"
```

---

## Task 10: Extend `PollTriggerOrchestratorRunResult` with `LeaseUnavailable`

**Files:**
- Modify: `api/src/town-crier.application/Polling/PollTriggerOrchestratorRunResult.cs`

- [ ] **Step 1: Add the field**

```csharp
namespace TownCrier.Application.Polling;

public sealed record PollTriggerOrchestratorRunResult(
    bool MessageReceived,
    bool PublishedNext,
    PollPlanItResult? PollResult,
    bool LeaseUnavailable = false);
```

The `= false` default keeps existing callers/tests compiling without edits.

- [ ] **Step 2: Compile + commit**

```bash
dotnet build api/town-crier.sln
git commit -am "feat(polling): add LeaseUnavailable to PollTriggerOrchestratorRunResult"
```

---

## Task 11: Rewrite `PollTriggerOrchestrator.RunOnceAsync` — acquire-before-receive

**Files:**
- Modify: `api/src/town-crier.application/Polling/PollTriggerOrchestrator.cs`
- Modify: `api/tests/town-crier.application.tests/Polling/PollTriggerOrchestratorTests.cs`

- [ ] **Step 1: Write failing tests**

Add/modify in `PollTriggerOrchestratorTests.cs`. Replace the existing `LeaseHeld` test (the termination-reason one) with orchestrator-level lease gating:

```csharp
[Test]
public async Task Should_ReturnLeaseUnavailable_When_LeaseHeldAcrossBothAttempts()
{
    var leaseStore = new FakePollingLeaseStore { SimulateHeld = true };
    var triggerQueue = new FakePollTriggerQueue();
    triggerQueue.EnqueueReceivable();
    var handler = new SpyHandler();

    var orchestrator = BuildOrchestrator(triggerQueue, handler, leaseStore);

    var result = await orchestrator.RunOnceAsync(default);

    await Assert.That(result.LeaseUnavailable).IsTrue();
    await Assert.That(result.MessageReceived).IsFalse();
    await Assert.That(handler.HandleCalls).IsEqualTo(0);
    await Assert.That(triggerQueue.ReceiveCalls).IsEqualTo(0);
    await Assert.That(leaseStore.AcquireCalls).IsEqualTo(2); // one retry
}

[Test]
public async Task Should_ProceedAfterRetry_When_LeaseHeldOnFirstAttemptOnly()
{
    var leaseStore = new FakePollingLeaseStore { SimulateHeld = true };
    var triggerQueue = new FakePollTriggerQueue();
    triggerQueue.EnqueueReceivable();
    var handler = new SpyHandler { NextTerminationReason = PollTerminationReason.Natural };

    // Clear SimulateHeld between the first and second acquire call via a spy wrapper.
    var gatedLease = new OneShotGatedLeaseStore(leaseStore);

    var orchestrator = BuildOrchestrator(triggerQueue, handler, gatedLease);

    var result = await orchestrator.RunOnceAsync(default);

    await Assert.That(result.LeaseUnavailable).IsFalse();
    await Assert.That(result.MessageReceived).IsTrue();
    await Assert.That(result.PublishedNext).IsTrue();
    await Assert.That(handler.HandleCalls).IsEqualTo(1);
    await Assert.That(leaseStore.ReleaseCalls).IsEqualTo(1);
}

[Test]
public async Task Should_ReleaseLease_When_HandlerThrows()
{
    var leaseStore = new FakePollingLeaseStore();
    var triggerQueue = new FakePollTriggerQueue();
    triggerQueue.EnqueueReceivable();
    var handler = new SpyHandler { ThrowsOnHandle = new InvalidOperationException("bang") };

    var orchestrator = BuildOrchestrator(triggerQueue, handler, leaseStore);

    await Assert.ThrowsAsync<InvalidOperationException>(async () =>
        await orchestrator.RunOnceAsync(default));

    await Assert.That(leaseStore.ReleaseCalls).IsEqualTo(1);
}

[Test]
public async Task Should_ReleaseLease_When_PublishThrows()
{
    var leaseStore = new FakePollingLeaseStore();
    var triggerQueue = new FakePollTriggerQueue { ThrowOnPublish = new InvalidOperationException("bang") };
    triggerQueue.EnqueueReceivable();
    var handler = new SpyHandler { NextTerminationReason = PollTerminationReason.Natural };

    var orchestrator = BuildOrchestrator(triggerQueue, handler, leaseStore);

    await Assert.ThrowsAsync<InvalidOperationException>(async () =>
        await orchestrator.RunOnceAsync(default));

    await Assert.That(leaseStore.ReleaseCalls).IsEqualTo(1);
}
```

`OneShotGatedLeaseStore` is a local test helper:

```csharp
private sealed class OneShotGatedLeaseStore(FakePollingLeaseStore inner) : IPollingLeaseStore
{
    private int calls;

    public Task<LeaseAcquireResult> TryAcquireAsync(TimeSpan ttl, CancellationToken ct)
    {
        var n = Interlocked.Increment(ref this.calls);
        if (n == 1)
        {
            inner.SimulateHeld = true;
            return inner.TryAcquireAsync(ttl, ct);
        }
        inner.SimulateHeld = false;
        return inner.TryAcquireAsync(ttl, ct);
    }

    public Task ReleaseAsync(LeaseHandle handle, CancellationToken ct) => inner.ReleaseAsync(handle, ct);
}
```

`FakePollTriggerQueue` will need a `ReceiveCalls` counter and a `ThrowOnPublish` field — extend it if absent. `SpyHandler` mirrors the existing test-local handler spy; extend with `ThrowsOnHandle`.

- [ ] **Step 2: Verify fail**

Run: `dotnet test api/tests/town-crier.application.tests/ --filter "FullyQualifiedName~PollTriggerOrchestratorTests"`
Expected: new tests fail (missing orchestrator lease wiring).

- [ ] **Step 3: Rewrite the orchestrator**

```csharp
using Microsoft.Extensions.Logging;

namespace TownCrier.Application.Polling;

public sealed partial class PollTriggerOrchestrator
{
    private readonly PollPlanItCommandHandler handler;
    private readonly IPollTriggerQueue triggerQueue;
    private readonly PollNextRunScheduler scheduler;
    private readonly IPollingLeaseStore leaseStore;
    private readonly PollingOptions options;
    private readonly TimeProvider timeProvider;
    private readonly ILogger<PollTriggerOrchestrator> logger;

    public PollTriggerOrchestrator(
        PollPlanItCommandHandler handler,
        IPollTriggerQueue triggerQueue,
        PollNextRunScheduler scheduler,
        IPollingLeaseStore leaseStore,
        PollingOptions options,
        TimeProvider timeProvider,
        ILogger<PollTriggerOrchestrator> logger)
    {
        this.handler = handler;
        this.triggerQueue = triggerQueue;
        this.scheduler = scheduler;
        this.leaseStore = leaseStore;
        this.options = options;
        this.timeProvider = timeProvider;
        this.logger = logger;
    }

    public async Task<PollTriggerOrchestratorRunResult> RunOnceAsync(CancellationToken ct)
    {
        var acquire = await this.leaseStore.TryAcquireAsync(this.options.OrchestratorLeaseTtl, ct).ConfigureAwait(false);
        if (!acquire.Acquired)
        {
            await Task.Delay(this.options.LeaseAcquireRetryDelay, ct).ConfigureAwait(false);
            acquire = await this.leaseStore.TryAcquireAsync(this.options.OrchestratorLeaseTtl, ct).ConfigureAwait(false);
            if (!acquire.Acquired)
            {
                LogLeaseUnavailable(this.logger);
                return new PollTriggerOrchestratorRunResult(
                    MessageReceived: false, PublishedNext: false, PollResult: null,
                    LeaseUnavailable: true);
            }
        }

        try
        {
            var message = await this.triggerQueue.ReceiveAsync(ct).ConfigureAwait(false);
            if (message is null)
            {
                LogEmptyQueue(this.logger);
                return new PollTriggerOrchestratorRunResult(
                    MessageReceived: false, PublishedNext: false, PollResult: null,
                    LeaseUnavailable: false);
            }

            var pollResult = await this.handler.HandleAsync(new PollPlanItCommand(), ct).ConfigureAwait(false);

            var now = this.timeProvider.GetUtcNow();
            var nextRun = this.scheduler.ComputeNextRun(pollResult.TerminationReason, pollResult.RetryAfter, now);

            await this.triggerQueue.PublishAtAsync(nextRun, ct).ConfigureAwait(false);

            return new PollTriggerOrchestratorRunResult(
                MessageReceived: true, PublishedNext: true, PollResult: pollResult,
                LeaseUnavailable: false);
        }
        finally
        {
            await this.leaseStore.ReleaseAsync(acquire.Handle!, ct).ConfigureAwait(false);
        }
    }

    [LoggerMessage(Level = LogLevel.Information, Message = "Poll trigger queue empty, exiting cleanly (bootstrap will re-seed)")]
    private static partial void LogEmptyQueue(ILogger logger);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Polling lease unavailable after retry — exiting; KEDA will re-trigger when peer releases")]
    private static partial void LogLeaseUnavailable(ILogger logger);
}
```

Note: the `Task.Delay` uses the option value literally (1 s). Add jitter in a small helper if desired — deferred unless the soak window shows synchronisation issues.

- [ ] **Step 4: Verify pass** — all orchestrator tests green, including existing ones.

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(polling): orchestrator acquires lease before receive; one retry + finally release"
```

---

## Task 12: Remove lease wiring from `PollPlanItCommandHandler`

**Files:**
- Modify: `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs`
- Modify: `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs` (builder)
- Delete: `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerLeaseTests.cs`
- Delete: `api/tests/town-crier.application.tests/Polling/FakePollingLeaseStore.cs` (if only handler referenced it — grep first; orchestrator tests likely still use it)
- Modify: `api/src/town-crier.infrastructure/Polling/PollingServiceExtensions.cs`

- [ ] **Step 1: Delete the lease-held-by-handler test file**

```bash
git rm api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerLeaseTests.cs
```

- [ ] **Step 2: Strip `PollPlanItCommandHandler`**

In `PollPlanItCommandHandler.cs`:

- Remove the `leaseStore` field and constructor parameter.
- Remove the `TryAcquireAsync` / `try / finally ReleaseAsync` wrapper from `HandleAsync`.
- Collapse `HandleAsync` into what is currently `HandleUnderLeaseAsync`: rename the private method's body into `HandleAsync`, delete the private method, delete the `#pragma warning disable SA1204` + the split.
- Delete `LogLeaseHeld` and `LogLeaseReleaseFailed` logger methods.

The new top of `HandleAsync`:

```csharp
public async Task<PollPlanItResult> HandleAsync(PollPlanItCommand command, CancellationToken ct)
{
    var now = this.timeProvider.GetUtcNow();
    var cycleType = this.cycleSelector.GetCurrent();
    // ... existing body from HandleUnderLeaseAsync ...
}
```

- [ ] **Step 3: Fix the handler test builder**

In `PollPlanItCommandHandlerTests.cs` find the test-builder helper (around line 1433) and drop the `IPollingLeaseStore? leaseStore = null` parameter and its use. Audit the rest of that file and remove any test asserting `LeaseHeld` behaviour — those were mostly in the deleted `*LeaseTests.cs`.

- [ ] **Step 4: Fix DI wiring**

`api/src/town-crier.infrastructure/Polling/PollingServiceExtensions.cs` at line 28 currently registers `IPollingLeaseStore`. Keep the registration — the orchestrator and bootstrap now consume it — but make sure the handler registration no longer takes it as a constructor param via DI (it shouldn't now that the field is gone).

Add `PollTriggerOrchestrator` registration to take `IPollingLeaseStore`, `PollingOptions`, and `TimeProvider` (verify DI resolves these already; they're likely singletons).

- [ ] **Step 5: Verify build + all tests**

Run: `dotnet test api/town-crier.sln`
Expected: all green. Tests that referenced `LeaseHeld` return paths should be gone.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor(polling): move lease serialisation from handler to orchestrator"
```

---

## Task 13: Extend `PollTriggerBootstrapResult` with `LeaseUnavailable`

**Files:**
- Modify: `api/src/town-crier.application/Polling/PollTriggerBootstrapResult.cs`

- [ ] **Step 1: Add the field**

```csharp
namespace TownCrier.Application.Polling;

public sealed record PollTriggerBootstrapResult(
    bool Published,
    bool ProbeFailed,
    bool LeaseUnavailable = false);
```

- [ ] **Step 2: Compile + commit**

```bash
dotnet build api/town-crier.sln
git commit -am "feat(polling): add LeaseUnavailable to PollTriggerBootstrapResult"
```

---

## Task 14: Rewrite `PollTriggerBootstrapper` — acquire-before-probe

**Files:**
- Modify: `api/src/town-crier.application/Polling/PollTriggerBootstrapper.cs`
- Modify: `api/tests/town-crier.application.tests/Polling/PollTriggerBootstrapperTests.cs`
- Modify: `api/src/town-crier.infrastructure/Polling/PollingServiceExtensions.cs` (DI wiring for new `IPollingLeaseStore` param)

- [ ] **Step 1: Write failing tests**

Add to `PollTriggerBootstrapperTests.cs`:

```csharp
[Test]
public async Task Should_ReturnLeaseUnavailable_When_LeaseHeld()
{
    var leaseStore = new FakePollingLeaseStore { SimulateHeld = true };
    var triggerQueue = new FakePollTriggerQueue();
    var metrics = new FakePollTriggerQueueMetrics();
    // No metrics enqueued — bootstrap should never call GetDepthAsync.
    var bootstrapper = BuildBootstrapper(triggerQueue, metrics, leaseStore);

    var result = await bootstrapper.TryBootstrapAsync(default);

    await Assert.That(result.LeaseUnavailable).IsTrue();
    await Assert.That(result.Published).IsFalse();
    await Assert.That(metrics.ProbeCalls).IsEqualTo(0);
    await Assert.That(triggerQueue.PublishCalls).IsEqualTo(0);
}

[Test]
public async Task Should_PublishAndRelease_When_LeaseAcquiredAndQueueEmpty()
{
    var leaseStore = new FakePollingLeaseStore();
    var triggerQueue = new FakePollTriggerQueue();
    var metrics = new FakePollTriggerQueueMetrics();
    metrics.Enqueue(active: 0, scheduled: 0);
    var bootstrapper = BuildBootstrapper(triggerQueue, metrics, leaseStore);

    var result = await bootstrapper.TryBootstrapAsync(default);

    await Assert.That(result.Published).IsTrue();
    await Assert.That(result.LeaseUnavailable).IsFalse();
    await Assert.That(triggerQueue.PublishCalls).IsEqualTo(1);
    await Assert.That(leaseStore.ReleaseCalls).IsEqualTo(1);
}

[Test]
public async Task Should_ReleaseLease_When_ProbeThrows()
{
    var leaseStore = new FakePollingLeaseStore();
    var triggerQueue = new FakePollTriggerQueue();
    var metrics = new FakePollTriggerQueueMetrics { ThrowOnProbe = new InvalidOperationException("boom") };
    var bootstrapper = BuildBootstrapper(triggerQueue, metrics, leaseStore);

    var result = await bootstrapper.TryBootstrapAsync(default);

    await Assert.That(result.ProbeFailed).IsTrue();
    await Assert.That(leaseStore.ReleaseCalls).IsEqualTo(1);
}
```

`FakePollTriggerQueueMetrics.ProbeCalls` + `ThrowOnProbe` may need adding — follow the shape of `FakePollTriggerQueue`.

Modify existing bootstrap tests: they all already assume the bootstrapper acquires a lease → they need to inject a `FakePollingLeaseStore` (default: acquirable). Update `BuildBootstrapper` helper.

- [ ] **Step 2: Verify fail**

Run: `dotnet test --filter "FullyQualifiedName~PollTriggerBootstrapperTests"`
Expected: compile errors for new constructor arg.

- [ ] **Step 3: Rewrite the bootstrapper**

```csharp
using Microsoft.Extensions.Logging;

namespace TownCrier.Application.Polling;

public sealed partial class PollTriggerBootstrapper
{
    private readonly IPollTriggerQueue triggerQueue;
    private readonly IPollTriggerQueueMetrics metrics;
    private readonly PollNextRunScheduler scheduler;
    private readonly IPollingLeaseStore leaseStore;
    private readonly PollingOptions options;
    private readonly TimeProvider timeProvider;
    private readonly ILogger<PollTriggerBootstrapper> logger;

    public PollTriggerBootstrapper(
        IPollTriggerQueue triggerQueue,
        IPollTriggerQueueMetrics metrics,
        PollNextRunScheduler scheduler,
        IPollingLeaseStore leaseStore,
        PollingOptions options,
        TimeProvider timeProvider,
        ILogger<PollTriggerBootstrapper> logger)
    {
        ArgumentNullException.ThrowIfNull(triggerQueue);
        ArgumentNullException.ThrowIfNull(metrics);
        ArgumentNullException.ThrowIfNull(scheduler);
        ArgumentNullException.ThrowIfNull(leaseStore);
        ArgumentNullException.ThrowIfNull(options);
        ArgumentNullException.ThrowIfNull(timeProvider);
        ArgumentNullException.ThrowIfNull(logger);

        this.triggerQueue = triggerQueue;
        this.metrics = metrics;
        this.scheduler = scheduler;
        this.leaseStore = leaseStore;
        this.options = options;
        this.timeProvider = timeProvider;
        this.logger = logger;
    }

    public async Task<PollTriggerBootstrapResult> TryBootstrapAsync(CancellationToken ct)
    {
        var acquire = await this.leaseStore.TryAcquireAsync(this.options.BootstrapLeaseTtl, ct).ConfigureAwait(false);
        if (!acquire.Acquired)
        {
            LogLeaseHeldByPeer(this.logger);
            return new PollTriggerBootstrapResult(Published: false, ProbeFailed: false, LeaseUnavailable: true);
        }

        try
        {
            PollTriggerQueueDepth depth;
            try
            {
                depth = await this.metrics.GetDepthAsync(ct).ConfigureAwait(false);
            }
#pragma warning disable CA1031
            catch (Exception ex)
#pragma warning restore CA1031
            {
                LogProbeFailed(this.logger, ex);
                return new PollTriggerBootstrapResult(Published: false, ProbeFailed: true);
            }

            if (!depth.IsEmpty)
            {
                LogQueueAlreadySeeded(this.logger, depth.ActiveMessageCount, depth.ScheduledMessageCount);
                return new PollTriggerBootstrapResult(Published: false, ProbeFailed: false);
            }

            var now = this.timeProvider.GetUtcNow();
            var nextRun = this.scheduler.ComputeNextRun(PollTerminationReason.Natural, retryAfter: null, now);

            try
            {
                await this.triggerQueue.PublishAtAsync(nextRun, ct).ConfigureAwait(false);
            }
#pragma warning disable CA1031
            catch (Exception ex)
#pragma warning restore CA1031
            {
                LogPublishFailed(this.logger, ex);
                return new PollTriggerBootstrapResult(Published: false, ProbeFailed: true);
            }

            LogBootstrapPublished(this.logger, nextRun);
            return new PollTriggerBootstrapResult(Published: true, ProbeFailed: false);
        }
        finally
        {
            await this.leaseStore.ReleaseAsync(acquire.Handle!, ct).ConfigureAwait(false);
        }
    }

    [LoggerMessage(Level = LogLevel.Warning, Message = "Safety-net bootstrap queue metrics probe failed; skipping reseed (handler already ran)")]
    private static partial void LogProbeFailed(ILogger logger, Exception ex);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Safety-net bootstrap publish failed; next cron tick will retry")]
    private static partial void LogPublishFailed(ILogger logger, Exception ex);

    [LoggerMessage(Level = LogLevel.Information, Message = "Safety-net bootstrap skipped reseed — poll trigger queue already has {ActiveMessageCount} active and {ScheduledMessageCount} scheduled messages")]
    private static partial void LogQueueAlreadySeeded(ILogger logger, long activeMessageCount, long scheduledMessageCount);

    [LoggerMessage(Level = LogLevel.Information, Message = "Safety-net bootstrap published bootstrap poll trigger scheduled for {ScheduledEnqueueTimeUtc:o}")]
    private static partial void LogBootstrapPublished(ILogger logger, DateTimeOffset scheduledEnqueueTimeUtc);

    [LoggerMessage(Level = LogLevel.Information, Message = "Safety-net bootstrap skipped — peer holds the polling lease (orchestrator is running)")]
    private static partial void LogLeaseHeldByPeer(ILogger logger);
}
```

- [ ] **Step 4: Update DI in `PollingServiceExtensions.cs`**

Make sure the DI container can construct `PollTriggerBootstrapper` with the new dependencies. `PollingOptions` and `IPollingLeaseStore` should already be registered.

- [ ] **Step 5: Verify pass**

Run: `dotnet test api/town-crier.sln`
Expected: all bootstrap + orchestrator tests pass. Previous probe/publish tests still pass because the lease-default fake acquires successfully.

- [ ] **Step 6: Commit**

```bash
git commit -am "feat(polling): bootstrap acquires lease before probe; release in finally"
```

---

## Task 15: Remove `PollTerminationReason.LeaseHeld` + dead branch in orchestrator

**Files:**
- Modify: `api/src/town-crier.application/Polling/PollTerminationReason.cs`
- Modify: `api/src/town-crier.application/Polling/PollTerminationReasonExtensions.cs`
- Modify: `api/src/town-crier.application/Polling/PollTriggerOrchestrator.cs` (delete dead LeaseHeld branch if any survived Task 11)

- [ ] **Step 1: Delete `LeaseHeld` enum value**

```csharp
public enum PollTerminationReason
{
    Natural = 0,
    TimeBounded = 1,
    RateLimited = 2,
    // LeaseHeld removed — serialisation moved from handler to orchestrator (docs/specs/polling-lease-cas.md)
}
```

- [ ] **Step 2: Delete `lease_held` mapping**

```csharp
// In PollTerminationReasonExtensions.cs:
public static string ToTelemetryValue(this PollTerminationReason reason) => reason switch
{
    PollTerminationReason.Natural     => "natural",
    PollTerminationReason.TimeBounded => "time_bounded",
    PollTerminationReason.RateLimited => "rate_limited",
    _ => "unknown",
};
```

- [ ] **Step 3: Audit orchestrator**

Search `PollTriggerOrchestrator.cs` for `LeaseHeld`. If any branch survived Task 11's rewrite, delete it. `PollNextRunScheduler.ComputeNextRun` already falls through to `NaturalCadence` via `_ =>`, no change needed.

- [ ] **Step 4: Verify build + tests**

Run: `dotnet build api/town-crier.sln && dotnet test api/town-crier.sln`
Expected: all green.

- [ ] **Step 5: Commit**

```bash
git commit -am "refactor(polling): remove PollTerminationReason.LeaseHeld (dead)"
```

---

## Task 16: `HandlerBudget` startup assertion in `poll-sb` mode

**Files:**
- Modify: `api/src/town-crier.worker/Program.cs`

- [ ] **Step 1: Find the `poll-sb` entrypoint**

Grep `api/src/town-crier.worker/Program.cs` for `poll-sb` and identify where `PollingOptions` is materialised (bound from config / env vars).

- [ ] **Step 2: Add fail-fast**

Immediately before entering the `poll-sb` run path:

```csharp
if (workerMode == "poll-sb" && options.HandlerBudget is null)
{
    logger.LogCritical("Polling handler budget must be set for poll-sb mode to bound lease TTL. Aborting.");
    return 1;
}
```

The exact name/shape will match existing fail-fast patterns — match the style of the `UnknownWorkerMode` exit used today (ADR 0024 mentions it).

- [ ] **Step 3: Commit**

```bash
git commit -am "feat(worker): fail fast when HandlerBudget is unset in poll-sb mode"
```

---

## Task 17: Lease telemetry — three counters

**Files:**
- Modify: `api/src/town-crier.application/Observability/PollingInstrumentation.cs`
- Modify: `api/src/town-crier.application/Polling/PollTriggerOrchestrator.cs` (instrument)
- Modify: `api/src/town-crier.application/Polling/PollTriggerBootstrapper.cs` (instrument)
- Modify: `api/src/town-crier.infrastructure/Polling/CosmosPollingLeaseStore.cs` (surface 412 on release)

The lease store's `Release` currently swallows outcomes. To emit `released_412`, expose the outcome to the caller — easiest via a return value. Alternative: inject the instrumentation into the lease store. Prefer returning an enum:

- [ ] **Step 1: Add `LeaseReleaseOutcome`**

`api/src/town-crier.application/Polling/LeaseReleaseOutcome.cs`:

```csharp
namespace TownCrier.Application.Polling;

public enum LeaseReleaseOutcome
{
    Released = 0,
    AlreadyGone = 1,
    PreconditionFailed = 2,
    TransientError = 3,
}
```

- [ ] **Step 2: Change `IPollingLeaseStore.ReleaseAsync` to return `Task<LeaseReleaseOutcome>`**

Update `IPollingLeaseStore`, `CosmosPollingLeaseStore`, `FakePollingLeaseStore` in lockstep.

In `CosmosPollingLeaseStore.ReleaseAsync`:

```csharp
public async Task<LeaseReleaseOutcome> ReleaseAsync(LeaseHandle handle, CancellationToken ct)
{
    ArgumentNullException.ThrowIfNull(handle);
    try
    {
        var outcome = await this.client.TryDeleteDocumentAsync(
            CosmosContainerNames.Leases, LeaseDocumentId, LeaseDocumentId, handle.ETag, ct)
            .ConfigureAwait(false);
        return outcome switch
        {
            CosmosDeleteOutcome.Deleted => LeaseReleaseOutcome.Released,
            CosmosDeleteOutcome.NotFound => LeaseReleaseOutcome.AlreadyGone,
            CosmosDeleteOutcome.PreconditionFailed => LeaseReleaseOutcome.PreconditionFailed,
            _ => LeaseReleaseOutcome.TransientError,
        };
    }
#pragma warning disable CA1031
    catch
#pragma warning restore CA1031
    {
        return LeaseReleaseOutcome.TransientError;
    }
}
```

- [ ] **Step 3: Add counters**

In `PollingInstrumentation.cs`:

```csharp
public static readonly Counter<long> LeaseAcquired =
    Meter.CreateCounter<long>("towncrier.polling.lease.acquired");
public static readonly Counter<long> LeaseHeldByPeer =
    Meter.CreateCounter<long>("towncrier.polling.lease.held_by_peer");
public static readonly Counter<long> LeaseReleased412 =
    Meter.CreateCounter<long>("towncrier.polling.lease.released_412");
```

- [ ] **Step 4: Instrument orchestrator**

At the end of a successful acquire: `PollingInstrumentation.LeaseAcquired.Add(1, new("caller", "orchestrator"));`
On second-attempt failure: `PollingInstrumentation.LeaseHeldByPeer.Add(1, new("caller", "orchestrator"));`
In `finally` after release: if outcome is `PreconditionFailed`, `LeaseReleased412.Add(1, new("caller", "orchestrator"));` + WARN log.

- [ ] **Step 5: Instrument bootstrap**

Same tags with `"bootstrap"`.

- [ ] **Step 6: Write a test asserting the counter-observable side**

Add a test that asserts a `MeterListener` collects a single `LeaseAcquired` event during a happy-path orchestrator run. If the repo already has a test pattern for metric assertions (grep for `MeterListener`), follow it. If not, keep it simple and rely on the `FakePollingLeaseStore`'s counters.

- [ ] **Step 7: Verify + commit**

```bash
dotnet test api/town-crier.sln
git commit -am "feat(polling): lease telemetry — acquired / held_by_peer / released_412"
```

---

## Task 18: Integration test — contention simulation

**Files:**
- Create: `api/tests/town-crier.integration-tests/Polling/PollLeaseCasIntegrationTests.cs`

- [ ] **Step 1: Write the integration test**

Use the existing `FakeCosmosRestClient` (now ETag-aware) to back a real `CosmosPollingLeaseStore`. Wire a real `PollTriggerOrchestrator` + `PollTriggerBootstrapper` against a shared `FakePollTriggerQueue`. Spawn both tasks concurrently and randomise start order.

```csharp
using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Polling;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.Polling;

namespace TownCrier.IntegrationTests.Polling;

public sealed class PollLeaseCasIntegrationTests
{
    [Test]
    public async Task QueueDepth_StaysBoundedAtOne_UnderContention()
    {
        for (var iteration = 0; iteration < 100; iteration++)
        {
            await RunOneContentionRound(iteration);
        }
    }

    private static async Task RunOneContentionRound(int seed)
    {
        var cosmos = new FakeCosmosRestClient();
        var time = new FakeTimeProvider(new DateTimeOffset(2026, 4, 23, 12, 0, 0, TimeSpan.Zero));
        var leaseStore = new CosmosPollingLeaseStore(cosmos, time);

        var queue = new FakePollTriggerQueue();
        queue.EnqueueReceivable();

        var metrics = new FakePollTriggerQueueMetrics();
        metrics.Enqueue(active: 1, scheduled: 0); // orchestrator will drain; bootstrap sees non-empty during contention

        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());
        var handler = new SpyHandler { NextTerminationReason = PollTerminationReason.Natural };
        var options = new PollingOptions
        {
            OrchestratorLeaseTtl = TimeSpan.FromMinutes(5),
            BootstrapLeaseTtl = TimeSpan.FromSeconds(60),
            LeaseAcquireRetryDelay = TimeSpan.FromMilliseconds(5),
        };

        var orchestrator = new PollTriggerOrchestrator(
            handler, queue, scheduler, leaseStore, options, time,
            NullLogger<PollTriggerOrchestrator>.Instance);
        var bootstrapper = new PollTriggerBootstrapper(
            queue, metrics, scheduler, leaseStore, options, time,
            NullLogger<PollTriggerBootstrapper>.Instance);

        var rng = new Random(seed);
        var orchestratorFirst = rng.Next(2) == 0;

        Task<PollTriggerOrchestratorRunResult> orchTask;
        Task<PollTriggerBootstrapResult> bootTask;

        if (orchestratorFirst)
        {
            orchTask = Task.Run(() => orchestrator.RunOnceAsync(default));
            await Task.Delay(rng.Next(5), default);
            bootTask = Task.Run(() => bootstrapper.TryBootstrapAsync(default));
        }
        else
        {
            bootTask = Task.Run(() => bootstrapper.TryBootstrapAsync(default));
            await Task.Delay(rng.Next(5), default);
            orchTask = Task.Run(() => orchestrator.RunOnceAsync(default));
        }

        await Task.WhenAll(orchTask, bootTask);

        await Assert.That(queue.PublishCalls).IsLessThanOrEqualTo(1)
            .Because($"iteration {seed}: exactly one publish expected per contention round");
    }
}
```

Depending on the fake queue's internal semantics, `PublishCalls` may need a tighter invariant (e.g. `== 1` if the orchestrator always runs first and publishes its next-run). Aim for: **the sum of orchestrator publishes + bootstrap publishes across both tasks is 1** when the orchestrator wins the race, **or 1** when the bootstrap wins (it sees non-empty because the queue still has the trigger). In no case should the total be 2. Adjust the assertion to match.

- [ ] **Step 2: Verify pass**

Run: `dotnet test api/tests/town-crier.integration-tests/ --filter "FullyQualifiedName~PollLeaseCas"`
Expected: 1/1 pass, 100 iterations succeed.

- [ ] **Step 3: Commit**

```bash
git commit -am "test(polling): lease CAS contention integration test (100 iterations)"
```

---

## Task 19: Pre-flight + PR

**Files:** none (repo-wide)

- [ ] **Step 1: Full solution build + tests**

```bash
dotnet build api/town-crier.sln --configuration Release
dotnet test api/town-crier.sln --configuration Release
dotnet format api/town-crier.sln --verify-no-changes
```

All three must pass.

- [ ] **Step 2: AOT check**

```bash
dotnet publish api/src/town-crier.worker/town-crier.worker.csproj \
    -c Release -r linux-x64 /p:PublishAot=true
```

Expected: no AOT trim warnings introduced by the new Cosmos methods or the `LeaseHandle` types.

- [ ] **Step 3: Beads**

Per CLAUDE.md, each commit in this plan corresponds to a bead. The implementing agent either creates the beads up-front via `plan-to-beads`, or creates one bead per task before starting it and closes it after the commit. Do not skip.

- [ ] **Step 4: Push branch + open PR**

```bash
git push -u origin spec/polling-lease-cas
gh pr create --title "feat(polling): ETag CAS mutex for polling lease" \
    --body "$(cat <<'EOF'
## Summary
- Upgrade `CosmosPollingLeaseStore` to ETag-CAS via Cosmos `If-Match` / `If-None-Match`.
- Move lease acquisition from `PollPlanItCommandHandler` up to `PollTriggerOrchestrator` (acquire before destructive receive).
- Add a matching lease gate to `PollTriggerBootstrapper` (acquire before ARM probe).
- Close the race where the bootstrap cron re-seeds the poll queue while an orchestrator's handler is mid-run.

Spec: docs/specs/polling-lease-cas.md
Plan: docs/specs/2026-04-23-polling-lease-cas-plan.md

## Test plan
- [ ] Unit: CosmosPollingLeaseStore CAS scenarios
- [ ] Unit: orchestrator lease-retry + finally-release
- [ ] Unit: bootstrapper lease gate
- [ ] Integration: 100-iteration contention simulation
- [ ] AOT publish clean

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

- [ ] **Step 5: Post-deploy monitoring (manual, documented in spec §Rollout)**

Watch for 48 hours:
- `activeMessageCount + scheduledMessageCount` ≤ 1 in steady state.
- `towncrier.polling.lease.released_412` = 0.
- No regression in PlanIt 429 rate.

---

## Notes for the implementer

- **File modification order matters.** Tasks 1–5 establish the primitive; Tasks 6–8 reshape the port + fakes; Tasks 9–11 put the orchestrator in charge; Tasks 12–15 clean up the handler and dead enum value; Tasks 16–17 add safety rails + observability; Task 18 is integration; Task 19 ships.
- **Keep each commit green.** If a task's intermediate step breaks the build (Task 6 deliberately does — the interface reshape), explicitly note this in the commit and fix it in the very next task.
- **Don't invent abstractions.** If you find yourself wanting to add a `LeaseManager` wrapper or an `ILeaseClock`, stop — the plan doesn't call for one and the spec's YAGNI call is "lease is a primitive, not a framework".
- **The Cosmos `TryCreate` / `TryReplace` methods can optionally return the new ETag.** If they do, the extra read-after-write in `CosmosPollingLeaseStore` goes away. Judgement call — do the refactor if it's small, skip it otherwise. The spec is agnostic.
- **Do not push force; do not amend public commits.** Per CLAUDE.md and the session-close protocol, commit + push; open the PR; let CI run.
