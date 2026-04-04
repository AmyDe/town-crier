# Cosmos DB Partition Key Range Fan-Out Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix polling service crash by teaching `CosmosRestClient` to handle partition fan-out for DISTINCT/GROUP BY cross-partition queries.

**Architecture:** When the Cosmos DB gateway returns 400 with `partitionedQueryExecutionInfoVersion`, fetch partition key ranges via REST API, re-execute the query per range with `x-ms-documentdb-partitionkeyrangeid` header, and concatenate results. Repository methods add post-processing (dedup/re-aggregation).

**Tech Stack:** .NET 10, Cosmos DB REST API, TUnit, System.Text.Json

---

### Task 1: Fan-out detection and partition key range querying in CosmosRestClient

**Files:**
- Modify: `api/src/town-crier.infrastructure/Cosmos/CosmosRestClient.cs`
- Modify: `api/tests/town-crier.infrastructure.tests/Cosmos/CosmosRestClientTests.cs`

- [ ] **Step 1: Write failing test — fan-out returns combined results from two partition ranges**

Add this test to `CosmosRestClientTests.cs`:

```csharp
[Test]
public async Task Should_FanOutToPartitionRanges_When_GatewayReturnsPartitionedQueryInfo()
{
    var (client, handler) = CreateClient();

    // 1. Initial cross-partition query → 400 with fan-out info
    handler.EnqueueResponse(HttpStatusCode.BadRequest,
        """{"code":"BadRequest","message":"cross partition query can not be directly served by the gateway","additionalErrorInfo":"{\"partitionedQueryExecutionInfoVersion\":2}"}""");

    // 2. GET pkranges → two ranges
    handler.EnqueueResponse(HttpStatusCode.OK,
        """{"PartitionKeyRanges":[{"id":"0"},{"id":"1"}],"_count":2}""");

    // 3. Query range 0
    handler.EnqueueResponse(HttpStatusCode.OK,
        """{"Documents":[{"id":"d1","name":"A"}],"_count":1}""");

    // 4. Query range 1
    handler.EnqueueResponse(HttpStatusCode.OK,
        """{"Documents":[{"id":"d2","name":"B"}],"_count":1}""");

    var results = await client.QueryAsync(
        "Users",
        "SELECT DISTINCT VALUE c.name FROM c",
        null,
        null, // cross-partition
        TestSerializerContext.Default.TestDocument,
        CancellationToken.None);

    await Assert.That(results).HasCount().EqualTo(2);
    await Assert.That(results[0].Id).IsEqualTo("d1");
    await Assert.That(results[1].Id).IsEqualTo("d2");

    // Verify request flow: probe, pkranges, range-0 query, range-1 query
    await Assert.That(handler.SentRequests).HasCount().EqualTo(4);

    await Assert.That(handler.SentRequests[1].Method).IsEqualTo(HttpMethod.Get);
    await Assert.That(handler.SentRequests[1].RequestUri!.AbsolutePath)
        .IsEqualTo("/dbs/test-db/colls/Users/pkranges");

    await Assert.That(
        handler.SentRequests[2].Headers.GetValues("x-ms-documentdb-partitionkeyrangeid").First())
        .IsEqualTo("0");
    await Assert.That(
        handler.SentRequests[3].Headers.GetValues("x-ms-documentdb-partitionkeyrangeid").First())
        .IsEqualTo("1");
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/christy/Dev/town-crier/api && dotnet test --filter "Should_FanOutToPartitionRanges" --no-restore`

Expected: FAIL — `QueryAsync` throws `HttpRequestException` on the 400 response.

- [ ] **Step 3: Implement fan-out detection and partition range querying**

In `CosmosRestClient.cs`, modify the `QueryAsync` method. Replace the existing do-while loop body with fan-out detection on the first request, then add two new private methods.

Replace the entire `QueryAsync` method (lines 125–176) with:

```csharp
public async Task<List<T>> QueryAsync<T>(
    string collection,
    string sql,
    IReadOnlyList<QueryParameter>? parameters,
    string? partitionKey,
    JsonTypeInfo<T> typeInfo,
    CancellationToken ct)
{
    using var activity = CosmosInstrumentation.Source.StartActivity("Cosmos Query");
    activity?.SetTag("db.system", "cosmosdb");
    activity?.SetTag("db.cosmosdb.container", collection);
    activity?.SetTag("db.operation.name", "Query");

    var results = new List<T>();
    string? continuation = null;

    do
    {
        using var request = this.BuildQueryRequest(collection, sql, parameters);
        await this.AddHeadersAsync(request, partitionKey, ct).ConfigureAwait(false);
        AddQueryHeaders(request, partitionKey);

        if (continuation is not null)
        {
            request.Headers.TryAddWithoutValidation("x-ms-continuation", continuation);
        }

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

        // Detect partition fan-out requirement on the first cross-partition request
        if (continuation is null
            && response.StatusCode == HttpStatusCode.BadRequest
            && partitionKey is null)
        {
            var errorBody = await response.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
            if (errorBody.Contains("partitionedQueryExecutionInfoVersion", StringComparison.Ordinal))
            {
                return await this.QueryWithFanOutAsync(
                    collection, sql, parameters, typeInfo, activity, ct).ConfigureAwait(false);
            }

            throw new HttpRequestException(
                $"Cosmos DB query failed ({(int)response.StatusCode}): {errorBody} | SQL: {sql}");
        }

        await ThrowOnCosmosErrorAsync(response, sql, ct).ConfigureAwait(false);

        RecordResponseMetrics(activity, response);

        var stream = await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false);
        await using (stream.ConfigureAwait(false))
        {
            using var doc = await JsonDocument.ParseAsync(stream, cancellationToken: ct)
                .ConfigureAwait(false);

            foreach (var element in doc.RootElement.GetProperty("Documents").EnumerateArray())
            {
                results.Add(element.Deserialize(typeInfo)!);
            }
        }

        continuation = response.Headers.TryGetValues("x-ms-continuation", out var values)
            ? values.FirstOrDefault()
            : null;
    }
    while (continuation is not null);

    return results;
}
```

Add two new private methods after the `AddQueryHeaders` method (after line 203):

```csharp
private async Task<List<T>> QueryWithFanOutAsync<T>(
    string collection,
    string sql,
    IReadOnlyList<QueryParameter>? parameters,
    JsonTypeInfo<T> typeInfo,
    Activity? activity,
    CancellationToken ct)
{
    var rangeIds = await this.GetPartitionKeyRangesAsync(collection, ct).ConfigureAwait(false);
    var allResults = new List<T>();

    foreach (var rangeId in rangeIds)
    {
        string? continuation = null;
        do
        {
            using var request = this.BuildQueryRequest(collection, sql, parameters);
            await this.AddHeadersAsync(request, null, ct).ConfigureAwait(false);
            AddQueryHeaders(request, null);
            request.Headers.TryAddWithoutValidation(
                "x-ms-documentdb-partitionkeyrangeid", rangeId);

            if (continuation is not null)
            {
                request.Headers.TryAddWithoutValidation("x-ms-continuation", continuation);
            }

            using var response = await this.httpClient.SendAsync(request, ct)
                .ConfigureAwait(false);
            await ThrowOnCosmosErrorAsync(response, sql, ct).ConfigureAwait(false);

            RecordResponseMetrics(activity, response);

            var stream = await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false);
            await using (stream.ConfigureAwait(false))
            {
                using var doc = await JsonDocument.ParseAsync(stream, cancellationToken: ct)
                    .ConfigureAwait(false);

                foreach (var element in doc.RootElement.GetProperty("Documents").EnumerateArray())
                {
                    allResults.Add(element.Deserialize(typeInfo)!);
                }
            }

            continuation = response.Headers.TryGetValues("x-ms-continuation", out var values)
                ? values.FirstOrDefault()
                : null;
        }
        while (continuation is not null);
    }

    return allResults;
}

private async Task<List<string>> GetPartitionKeyRangesAsync(
    string collection, CancellationToken ct)
{
    var resourceLink = $"dbs/{this.databaseName}/colls/{collection}";
    using var request = new HttpRequestMessage(HttpMethod.Get, $"/{resourceLink}/pkranges");
    await this.AddHeadersAsync(request, null, ct).ConfigureAwait(false);

    using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);
    response.EnsureSuccessStatusCode();

    var stream = await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false);
    await using (stream.ConfigureAwait(false))
    {
        using var doc = await JsonDocument.ParseAsync(stream, cancellationToken: ct)
            .ConfigureAwait(false);

        return doc.RootElement.GetProperty("PartitionKeyRanges")
            .EnumerateArray()
            .Select(e => e.GetProperty("id").GetString()!)
            .ToList();
    }
}
```

Add `using System.Diagnostics;` to the top if not already present (it is — line 1). Add `using System.Linq;` if needed (it's implicitly available in .NET 10).

- [ ] **Step 4: Run the fan-out test to verify it passes**

Run: `cd /Users/christy/Dev/town-crier/api && dotnet test --filter "Should_FanOutToPartitionRanges" --no-restore`

Expected: PASS

- [ ] **Step 5: Write failing test — continuation pages within a partition range during fan-out**

Add to `CosmosRestClientTests.cs`:

```csharp
[Test]
public async Task Should_DrainContinuationPages_When_FanOutQueryHasMultiplePages()
{
    var (client, handler) = CreateClient();

    // 1. Initial query → 400 with fan-out info
    handler.EnqueueResponse(HttpStatusCode.BadRequest,
        """{"code":"BadRequest","message":"cross partition query can not be directly served by the gateway","additionalErrorInfo":"{\"partitionedQueryExecutionInfoVersion\":2}"}""");

    // 2. GET pkranges → one range
    handler.EnqueueResponse(HttpStatusCode.OK,
        """{"PartitionKeyRanges":[{"id":"0"}],"_count":1}""");

    // 3. Range 0 page 1 with continuation
    handler.EnqueueResponse(HttpStatusCode.OK,
        """{"Documents":[{"id":"d1","name":"A"}],"_count":1}""",
        [new("x-ms-continuation", "page2-token")]);

    // 4. Range 0 page 2
    handler.EnqueueResponse(HttpStatusCode.OK,
        """{"Documents":[{"id":"d2","name":"B"}],"_count":1}""");

    var results = await client.QueryAsync(
        "Users",
        "SELECT DISTINCT VALUE c.name FROM c",
        null,
        null,
        TestSerializerContext.Default.TestDocument,
        CancellationToken.None);

    await Assert.That(results).HasCount().EqualTo(2);
    await Assert.That(results[0].Id).IsEqualTo("d1");
    await Assert.That(results[1].Id).IsEqualTo("d2");

    // Verify: probe, pkranges, range-0 page 1, range-0 page 2
    await Assert.That(handler.SentRequests).HasCount().EqualTo(4);
    await Assert.That(handler.SentRequests[3].Headers.GetValues("x-ms-continuation").First())
        .IsEqualTo("page2-token");
}
```

- [ ] **Step 6: Run test to verify it passes** (should pass with existing implementation)

Run: `cd /Users/christy/Dev/town-crier/api && dotnet test --filter "Should_DrainContinuationPages_When_FanOutQuery" --no-restore`

Expected: PASS

- [ ] **Step 7: Write failing test — regular 400 still throws when not fan-out**

Add to `CosmosRestClientTests.cs`:

```csharp
[Test]
public async Task Should_ThrowNormally_When_CrossPartitionBadRequestWithoutFanOutMarker()
{
    var (client, handler) = CreateClient();

    handler.EnqueueResponse(HttpStatusCode.BadRequest,
        """{"code":"BadRequest","message":"syntax error in query"}""");

    var exception = await Assert.ThrowsAsync<HttpRequestException>(async () =>
        await client.QueryAsync(
            "Users",
            "SELECT * FORM c", // intentional typo
            null,
            null,
            TestSerializerContext.Default.TestDocument,
            CancellationToken.None));

    await Assert.That(exception!.Message).Contains("syntax error in query");
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `cd /Users/christy/Dev/town-crier/api && dotnet test --filter "Should_ThrowNormally_When_CrossPartitionBadRequest" --no-restore`

Expected: PASS

- [ ] **Step 9: Run all CosmosRestClient tests to verify no regressions**

Run: `cd /Users/christy/Dev/town-crier/api && dotnet test --filter "CosmosRestClientTests" --no-restore`

Expected: All tests PASS (including existing tests for normal queries, continuation, cross-partition header, etc.)

- [ ] **Step 10: Commit**

```bash
cd /Users/christy/Dev/town-crier && git add api/src/town-crier.infrastructure/Cosmos/CosmosRestClient.cs api/tests/town-crier.infrastructure.tests/Cosmos/CosmosRestClientTests.cs && git commit -m "feat(api): add partition key range fan-out for cross-partition queries

Cosmos DB gateway cannot serve DISTINCT/GROUP BY across partitions.
Detect the 400 response with partitionedQueryExecutionInfoVersion,
fetch partition key ranges, and re-execute per range."
```

---

### Task 2: Repository post-processing for DISTINCT dedup and GROUP BY re-aggregation

**Files:**
- Modify: `api/src/town-crier.infrastructure/WatchZones/CosmosWatchZoneRepository.cs`

- [ ] **Step 1: Add `.Distinct().ToList()` to `GetDistinctAuthorityIdsAsync`**

In `CosmosWatchZoneRepository.cs`, replace the `GetDistinctAuthorityIdsAsync` method (lines 89–98):

```csharp
public async Task<IReadOnlyCollection<int>> GetDistinctAuthorityIdsAsync(CancellationToken ct)
{
    var results = await this.client.QueryAsync(
        CosmosContainerNames.WatchZones,
        "SELECT DISTINCT VALUE c.authorityId FROM c",
        parameters: null,
        partitionKey: null,
        CosmosJsonSerializerContext.Default.Int32,
        ct).ConfigureAwait(false);

    return results.Distinct().ToList();
}
```

- [ ] **Step 2: Add group-by re-aggregation to `GetZoneCountsByAuthorityAsync`**

Replace the `GetZoneCountsByAuthorityAsync` method (lines 100–111):

```csharp
public async Task<Dictionary<int, int>> GetZoneCountsByAuthorityAsync(CancellationToken ct)
{
    var items = await this.client.QueryAsync(
        CosmosContainerNames.WatchZones,
        "SELECT c.authorityId, COUNT(1) AS zoneCount FROM c GROUP BY c.authorityId",
        parameters: null,
        partitionKey: null,
        CosmosJsonSerializerContext.Default.AuthorityZoneCountResult,
        ct).ConfigureAwait(false);

    return items
        .GroupBy(item => item.AuthorityId)
        .ToDictionary(g => g.Key, g => g.Sum(item => item.ZoneCount));
}
```

- [ ] **Step 3: Run existing repository tests to verify no regressions**

Run: `cd /Users/christy/Dev/town-crier/api && dotnet test --filter "CosmosWatchZoneRepositoryTests" --no-restore`

Expected: All tests PASS

- [ ] **Step 4: Run full test suite**

Run: `cd /Users/christy/Dev/town-crier/api && dotnet test --no-restore`

Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
cd /Users/christy/Dev/town-crier && git add api/src/town-crier.infrastructure/WatchZones/CosmosWatchZoneRepository.cs && git commit -m "fix(api): add dedup and re-aggregation for fan-out query results

Per-range DISTINCT results may overlap across partition ranges.
Per-range GROUP BY returns partial aggregates needing re-summation."
```
