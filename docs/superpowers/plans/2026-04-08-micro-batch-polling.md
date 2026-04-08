# Micro-Batch Round-Robin Polling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign the PlanIt sync job from "poll everything hourly" to frequent micro-batches that process authorities in least-recently-synced order, exiting cleanly on 429.

**Architecture:** The poll handler queries PollState for the least-recently-polled authorities, iterates until PlanIt rate-limits (429), then exits cleanly. The PlanItClient is simplified to remove all retry/backoff logic — a 429 throws immediately. The infra schedule changes from hourly to every 15 minutes with a shorter timeout.

**Tech Stack:** .NET 10, Cosmos DB SDK (via CosmosRestClient), TUnit, Pulumi (C#)

**Spec:** `docs/specs/micro-batch-polling.md`

---

## File Map

| Action | File | Responsibility |
|--------|------|---------------|
| Modify | `api/src/town-crier.application/Polling/IPollStateStore.cs` | Add `GetLeastRecentlyPolledAsync` method |
| Modify | `api/src/town-crier.infrastructure/Polling/CosmosPollStateStore.cs` | Implement `GetLeastRecentlyPolledAsync` via Cosmos query |
| Modify | `api/src/town-crier.infrastructure/Cosmos/CosmosJsonSerializerContext.cs` | Add `List<PollStateDocument>` for query deserialization |
| Modify | `api/src/town-crier.infrastructure/Polling/PollStateDocument.cs` | Make `AuthorityId` required (not nullable) |
| Modify | `api/tests/town-crier.application.tests/Polling/FakePollStateStore.cs` | Implement `GetLeastRecentlyPolledAsync` |
| Modify | `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs` | Round-robin ordering, stop-on-429, add `RateLimited` to result |
| Modify | `api/src/town-crier.application/Polling/PollPlanItResult.cs` | Add `RateLimited` property |
| Modify | `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs` | Rewrite 429 tests for new stop-on-429 behaviour |
| Modify | `api/src/town-crier.application/Observability/PollingMetrics.cs` | Add `RateLimited` counter |
| Modify | `api/src/town-crier.infrastructure/PlanIt/PlanItClient.cs` | Replace `SendWithRetryAsync` with `SendWithThrottleAsync` |
| Modify | `api/tests/town-crier.infrastructure.tests/PlanIt/PlanItClientTests.cs` | Remove retry tests, update for immediate-429 behaviour |
| Delete | `api/src/town-crier.infrastructure/PlanIt/PlanItRetryOptions.cs` | No longer needed |
| Delete | `api/tests/town-crier.infrastructure.tests/PlanIt/PlanItRetryOptionsTests.cs` | No longer needed |
| Delete | `api/tests/town-crier.infrastructure.tests/PlanIt/FakePlanItLogger.cs` | Only used by retry log tests |
| Modify | `api/src/town-crier.application/Polling/PlanItPollingOptions.cs` | Remove `RateLimitCooldown`, keep as empty record (or delete) |
| Delete | `api/tests/town-crier.application.tests/Polling/PlanItPollingOptionsTests.cs` | Tests for removed cooldown property |
| Modify | `api/src/town-crier.worker/Program.cs` | Remove retry/polling options binding, record `RateLimited`, set export interval |
| Modify | `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs` | Remove `PlanItRetryOptions` binding |
| Modify | `infra/EnvironmentStack.cs` | Change cron to `*/15 * * * *`, timeout to 120 |

---

### Task 1: Add `GetLeastRecentlyPolledAsync` to poll state store

**Files:**
- Modify: `api/src/town-crier.application/Polling/IPollStateStore.cs`
- Modify: `api/src/town-crier.infrastructure/Polling/PollStateDocument.cs`
- Modify: `api/src/town-crier.infrastructure/Cosmos/CosmosJsonSerializerContext.cs`
- Modify: `api/src/town-crier.infrastructure/Polling/CosmosPollStateStore.cs`
- Modify: `api/tests/town-crier.application.tests/Polling/FakePollStateStore.cs`
- Test: `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs` (tested via handler in Task 2)

- [ ] **Step 1: Add `GetLeastRecentlyPolledAsync` to `IPollStateStore`**

```csharp
// api/src/town-crier.application/Polling/IPollStateStore.cs
namespace TownCrier.Application.Polling;

public interface IPollStateStore
{
    Task<DateTimeOffset?> GetLastPollTimeAsync(int authorityId, CancellationToken ct);

    Task SaveLastPollTimeAsync(int authorityId, DateTimeOffset pollTime, CancellationToken ct);

    Task DeleteGlobalPollStateAsync(CancellationToken ct);

    Task<IReadOnlyList<int>> GetLeastRecentlyPolledAsync(
        IReadOnlyList<int> candidateAuthorityIds,
        CancellationToken ct);
}
```

- [ ] **Step 2: Make `PollStateDocument.AuthorityId` required**

```csharp
// api/src/town-crier.infrastructure/Polling/PollStateDocument.cs
namespace TownCrier.Infrastructure.Polling;

internal sealed class PollStateDocument
{
    public required string Id { get; init; }

    public required string LastPollTime { get; init; }

    public required int AuthorityId { get; init; }
}
```

- [ ] **Step 3: Add `List<PollStateDocument>` to `CosmosJsonSerializerContext`**

Add this attribute to the context class in `api/src/town-crier.infrastructure/Cosmos/CosmosJsonSerializerContext.cs`:

```csharp
[JsonSerializable(typeof(List<PollStateDocument>))]
```

Add it directly after the existing `[JsonSerializable(typeof(PollStateDocument))]` line.

- [ ] **Step 4: Implement `GetLeastRecentlyPolledAsync` in `CosmosPollStateStore`**

Add this method to `api/src/town-crier.infrastructure/Polling/CosmosPollStateStore.cs`:

```csharp
public async Task<IReadOnlyList<int>> GetLeastRecentlyPolledAsync(
    IReadOnlyList<int> candidateAuthorityIds,
    CancellationToken ct)
{
    if (candidateAuthorityIds.Count == 0)
    {
        return [];
    }

    var docs = await this.client.QueryAsync(
        CosmosContainerNames.PollState,
        "SELECT * FROM c WHERE c.authorityId != null",
        parameters: null,
        partitionKey: null,
        CosmosJsonSerializerContext.Default.ListPollStateDocument,
        ct).ConfigureAwait(false);

    var polledSet = docs.ToDictionary(d => d.AuthorityId, d => d.LastPollTime);

    // Never-polled authorities first, then by oldest lastPollTime
    return candidateAuthorityIds
        .OrderBy(id => polledSet.ContainsKey(id) ? 1 : 0)
        .ThenBy(id => polledSet.TryGetValue(id, out var time) ? time : string.Empty)
        .ToList();
}
```

- [ ] **Step 5: Implement `GetLeastRecentlyPolledAsync` in `FakePollStateStore`**

Add this method to `api/tests/town-crier.application.tests/Polling/FakePollStateStore.cs`:

```csharp
public Task<IReadOnlyList<int>> GetLeastRecentlyPolledAsync(
    IReadOnlyList<int> candidateAuthorityIds,
    CancellationToken ct)
{
    IReadOnlyList<int> sorted = candidateAuthorityIds
        .OrderBy(id => this.pollTimes.ContainsKey(id) ? 1 : 0)
        .ThenBy(id => this.pollTimes.TryGetValue(id, out var time) ? time : DateTimeOffset.MinValue)
        .ToList();
    return Task.FromResult(sorted);
}
```

- [ ] **Step 6: Verify build compiles**

Run: `dotnet build api/`
Expected: Build succeeded

- [ ] **Step 7: Commit**

```bash
git add api/src/town-crier.application/Polling/IPollStateStore.cs \
        api/src/town-crier.infrastructure/Polling/PollStateDocument.cs \
        api/src/town-crier.infrastructure/Cosmos/CosmosJsonSerializerContext.cs \
        api/src/town-crier.infrastructure/Polling/CosmosPollStateStore.cs \
        api/tests/town-crier.application.tests/Polling/FakePollStateStore.cs
git commit -m "feat(api): add GetLeastRecentlyPolledAsync to IPollStateStore

Enables round-robin polling by returning authority IDs sorted by
least-recently-polled. Never-polled authorities sort first."
```

---

### Task 2: Rewrite poll handler for round-robin and stop-on-429

**Files:**
- Modify: `api/src/town-crier.application/Polling/PollPlanItResult.cs`
- Modify: `api/src/town-crier.application/Observability/PollingMetrics.cs`
- Modify: `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs`
- Test: `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs`

- [ ] **Step 1: Add `RateLimited` to `PollPlanItResult`**

```csharp
// api/src/town-crier.application/Polling/PollPlanItResult.cs
namespace TownCrier.Application.Polling;

public sealed record PollPlanItResult(int ApplicationCount, int AuthoritiesPolled, bool RateLimited);
```

- [ ] **Step 2: Add `RateLimited` counter to `PollingMetrics`**

Add to `api/src/town-crier.application/Observability/PollingMetrics.cs` after the existing `PollFailures` field:

```csharp
public static readonly Counter<long> RateLimited =
    Meter.CreateCounter<long>("towncrier.polling.rate_limited");
```

- [ ] **Step 3: Rewrite `PollPlanItCommandHandler`**

Replace the full contents of `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs`:

```csharp
using System.Diagnostics;
using System.Net;
using Microsoft.Extensions.Logging;
using TownCrier.Application.Observability;
using TownCrier.Application.PlanIt;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.WatchZones;

namespace TownCrier.Application.Polling;

public sealed partial class PollPlanItCommandHandler
{
    private readonly IPlanItClient planItClient;
    private readonly IPollStateStore pollStateStore;
    private readonly IPlanningApplicationRepository applicationRepository;
    private readonly TimeProvider timeProvider;
    private readonly IActiveAuthorityProvider activeAuthorityProvider;
    private readonly IWatchZoneRepository watchZoneRepository;
    private readonly INotificationEnqueuer notificationEnqueuer;
    private readonly ILogger<PollPlanItCommandHandler> logger;

    public PollPlanItCommandHandler(
        IPlanItClient planItClient,
        IPollStateStore pollStateStore,
        IPlanningApplicationRepository applicationRepository,
        TimeProvider timeProvider,
        IActiveAuthorityProvider activeAuthorityProvider,
        IWatchZoneRepository watchZoneRepository,
        INotificationEnqueuer notificationEnqueuer,
        ILogger<PollPlanItCommandHandler> logger)
    {
        this.planItClient = planItClient;
        this.pollStateStore = pollStateStore;
        this.applicationRepository = applicationRepository;
        this.timeProvider = timeProvider;
        this.activeAuthorityProvider = activeAuthorityProvider;
        this.watchZoneRepository = watchZoneRepository;
        this.notificationEnqueuer = notificationEnqueuer;
        this.logger = logger;
    }

    public async Task<PollPlanItResult> HandleAsync(PollPlanItCommand command, CancellationToken ct)
    {
        var now = this.timeProvider.GetUtcNow();
        var activeIds = await this.activeAuthorityProvider.GetActiveAuthorityIdsAsync(ct).ConfigureAwait(false);
        var sortedIds = await this.pollStateStore.GetLeastRecentlyPolledAsync(
            activeIds.ToList(), ct).ConfigureAwait(false);

        var count = 0;
        var authoritiesPolled = 0;
        var rateLimited = false;

        foreach (var authorityId in sortedIds)
        {
            using var authorityActivity = PollingInstrumentation.Source.StartActivity("Poll Authority");
            authorityActivity?.SetTag("polling.authority_code", authorityId);
            var authorityStart = Stopwatch.GetTimestamp();

            try
            {
                var lastPollTime = await this.pollStateStore.GetLastPollTimeAsync(authorityId, ct).ConfigureAwait(false);
                lastPollTime ??= now.AddDays(-30);

                var authorityAppCount = 0;
                await foreach (var application in this.planItClient.FetchApplicationsAsync(authorityId, lastPollTime, ct).ConfigureAwait(false))
                {
                    await this.applicationRepository.UpsertAsync(application, ct).ConfigureAwait(false);

                    if (application.Latitude.HasValue && application.Longitude.HasValue)
                    {
                        var matchingZones = await this.watchZoneRepository.FindZonesContainingAsync(
                            application.Latitude.Value, application.Longitude.Value, ct).ConfigureAwait(false);

                        foreach (var zone in matchingZones)
                        {
                            await this.notificationEnqueuer.EnqueueAsync(application, zone, ct).ConfigureAwait(false);
                        }
                    }

                    authorityAppCount++;
                    count++;
                }

                PollingMetrics.AuthorityProcessingDuration.Record(Stopwatch.GetElapsedTime(authorityStart).TotalMilliseconds);
                PollingMetrics.ApplicationsIngested.Add(authorityAppCount);
                authorityActivity?.SetTag("polling.applications_found", authorityAppCount);

                PollingMetrics.AuthoritiesPolled.Add(1);
                await this.pollStateStore.SaveLastPollTimeAsync(authorityId, now, ct).ConfigureAwait(false);
                authoritiesPolled++;
            }
            catch (HttpRequestException ex) when (ex.StatusCode == HttpStatusCode.TooManyRequests)
            {
                PollingMetrics.AuthoritiesSkipped.Add(1);
                PollingMetrics.AuthorityProcessingDuration.Record(Stopwatch.GetElapsedTime(authorityStart).TotalMilliseconds);
                LogRateLimitStop(this.logger, authorityId, ex);
                rateLimited = true;
                break;
            }
            catch (Exception ex) when (ex is not OperationCanceledException)
            {
                PollingMetrics.AuthoritiesSkipped.Add(1);
                PollingMetrics.AuthorityProcessingDuration.Record(Stopwatch.GetElapsedTime(authorityStart).TotalMilliseconds);
                LogAuthorityError(this.logger, authorityId, ex);
            }
        }

        await this.pollStateStore.DeleteGlobalPollStateAsync(ct).ConfigureAwait(false);

        return new PollPlanItResult(count, authoritiesPolled, rateLimited);
    }

    [LoggerMessage(Level = LogLevel.Warning, Message = "Rate limited polling authority {AuthorityId}, stopping polling cycle")]
    private static partial void LogRateLimitStop(ILogger logger, int authorityId, Exception exception);

    [LoggerMessage(Level = LogLevel.Error, Message = "Error polling authority {AuthorityId}, skipping to next authority")]
    private static partial void LogAuthorityError(ILogger logger, int authorityId, Exception exception);
}
```

- [ ] **Step 4: Write failing test — stop on first 429**

Replace the 429-related tests in `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs`. First, remove these existing tests:
- `Should_SkipAuthorityAndContinue_When_FirstRateLimitHit`
- `Should_BreakLoop_When_SecondRateLimitHit`
- `Should_ContinueToNextAuthority_When_RateLimitHitWithCooldown`

Then add:

```csharp
[Test]
public async Task Should_StopAndSetRateLimited_When_429Hit()
{
    var authorityProvider = new FakeActiveAuthorityProvider();
    authorityProvider.Add(100);
    authorityProvider.Add(200);
    authorityProvider.Add(300);

    var planItClient = new FakePlanItClient();
    planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
    planItClient.ThrowForAuthority(200, new HttpRequestException("Rate limited", null, HttpStatusCode.TooManyRequests));
    planItClient.Add(300, new PlanningApplicationBuilder().WithUid("app-3").WithAreaId(300).Build());

    var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

    var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

    // Authority 100 completed, 200 triggered 429, 300 never attempted
    await Assert.That(result.ApplicationCount).IsEqualTo(1);
    await Assert.That(result.AuthoritiesPolled).IsEqualTo(1);
    await Assert.That(result.RateLimited).IsTrue();
    await Assert.That(planItClient.AuthorityIdsRequested).DoesNotContain(300);
}

[Test]
public async Task Should_NotSetRateLimited_When_NoRateLimitHit()
{
    var authorityProvider = new FakeActiveAuthorityProvider();
    authorityProvider.Add(100);

    var planItClient = new FakePlanItClient();
    planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());

    var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

    var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

    await Assert.That(result.RateLimited).IsFalse();
}
```

- [ ] **Step 5: Write failing test — round-robin ordering**

Add to the same test file:

```csharp
[Test]
public async Task Should_PollLeastRecentlyPolledFirst_When_MultipleAuthorities()
{
    var authorityProvider = new FakeActiveAuthorityProvider();
    authorityProvider.Add(100);
    authorityProvider.Add(200);
    authorityProvider.Add(300);

    var pollStateStore = new FakePollStateStore();
    // Authority 300 polled longest ago, 200 most recently, 100 in between
    pollStateStore.SetLastPollTime(300, new DateTimeOffset(2026, 4, 1, 0, 0, 0, TimeSpan.Zero));
    pollStateStore.SetLastPollTime(100, new DateTimeOffset(2026, 4, 3, 0, 0, 0, TimeSpan.Zero));
    pollStateStore.SetLastPollTime(200, new DateTimeOffset(2026, 4, 5, 0, 0, 0, TimeSpan.Zero));

    var planItClient = new FakePlanItClient();

    var handler = CreateHandler(
        planItClient: planItClient,
        pollStateStore: pollStateStore,
        authorityProvider: authorityProvider);

    await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

    // Should be polled in order: 300 (oldest), 100, 200 (newest)
    await Assert.That(planItClient.AuthorityIdsRequested).IsEquivalentTo(new[] { 300, 100, 200 });
}

[Test]
public async Task Should_PollNeverPolledAuthorityFirst_When_MixedPollState()
{
    var authorityProvider = new FakeActiveAuthorityProvider();
    authorityProvider.Add(100);
    authorityProvider.Add(200);

    var pollStateStore = new FakePollStateStore();
    pollStateStore.SetLastPollTime(100, new DateTimeOffset(2026, 4, 5, 0, 0, 0, TimeSpan.Zero));
    // Authority 200 has never been polled

    var planItClient = new FakePlanItClient();

    var handler = CreateHandler(
        planItClient: planItClient,
        pollStateStore: pollStateStore,
        authorityProvider: authorityProvider);

    await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

    // Never-polled authority 200 should be first
    await Assert.That(planItClient.AuthorityIdsRequested[0]).IsEqualTo(200);
}
```

- [ ] **Step 6: Update `CreateHandler` helper**

Update the `CreateHandler` method in the test file to remove the `pollingOptions` parameter (no longer used by the handler):

```csharp
private static PollPlanItCommandHandler CreateHandler(
    FakePlanItClient? planItClient = null,
    FakePollStateStore? pollStateStore = null,
    FakePlanningApplicationRepository? repository = null,
    FakeActiveAuthorityProvider? authorityProvider = null,
    FakeWatchZoneRepository? watchZoneRepository = null,
    FakeNotificationEnqueuer? notificationEnqueuer = null,
    TimeProvider? timeProvider = null)
{
    return new PollPlanItCommandHandler(
        planItClient ?? new FakePlanItClient(),
        pollStateStore ?? new FakePollStateStore(),
        repository ?? new FakePlanningApplicationRepository(),
        timeProvider ?? TimeProvider.System,
        authorityProvider ?? new FakeActiveAuthorityProvider(),
        watchZoneRepository ?? new FakeWatchZoneRepository(),
        notificationEnqueuer ?? new FakeNotificationEnqueuer(),
        NullLogger<PollPlanItCommandHandler>.Instance);
}
```

Also remove the `ZeroCooldownOptions` field at the top of the class, and update any remaining tests that pass `pollingOptions` to remove that argument.

- [ ] **Step 7: Run tests to verify they fail for the right reason**

Run: `dotnet test api/ --filter "PollPlanItCommandHandlerTests"`
Expected: New tests fail (handler still has old constructor/behaviour), existing tests that reference removed parameters fail to compile.

- [ ] **Step 8: Run all tests to verify they pass**

Run: `dotnet test api/ --filter "PollPlanItCommandHandlerTests"`
Expected: All tests pass

- [ ] **Step 9: Commit**

```bash
git add api/src/town-crier.application/Polling/PollPlanItResult.cs \
        api/src/town-crier.application/Observability/PollingMetrics.cs \
        api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs \
        api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs
git commit -m "feat(api): rewrite poll handler for round-robin stop-on-429

Authorities are polled in least-recently-synced order. On a 429,
the handler breaks the loop and returns cleanly with RateLimited=true.
Removes the old skip/cooldown/break-after-2nd-429 logic."
```

---

### Task 3: Simplify PlanItClient — remove retry/backoff logic

**Files:**
- Modify: `api/src/town-crier.infrastructure/PlanIt/PlanItClient.cs`
- Delete: `api/src/town-crier.infrastructure/PlanIt/PlanItRetryOptions.cs`
- Delete: `api/tests/town-crier.infrastructure.tests/PlanIt/PlanItRetryOptionsTests.cs`
- Delete: `api/tests/town-crier.infrastructure.tests/PlanIt/FakePlanItLogger.cs`
- Modify: `api/tests/town-crier.infrastructure.tests/PlanIt/PlanItClientTests.cs`

- [ ] **Step 1: Rewrite `PlanItClient` to remove retry logic**

Replace the full contents of `api/src/town-crier.infrastructure/PlanIt/PlanItClient.cs`:

```csharp
using System.Diagnostics.CodeAnalysis;
using System.Globalization;
using System.Runtime.CompilerServices;
using System.Text.Json;
using TownCrier.Application.PlanIt;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Infrastructure.Observability;

namespace TownCrier.Infrastructure.PlanIt;

public sealed class PlanItClient : IPlanItClient
{
    private const int DefaultPageSize = 100;
    private const int SearchPageSize = 20;

    private readonly HttpClient httpClient;
    private readonly PlanItThrottleOptions throttleOptions;
    private readonly Func<TimeSpan, CancellationToken, Task> delayFunc;

    public PlanItClient(
        HttpClient httpClient,
        PlanItThrottleOptions? throttleOptions = null,
        Func<TimeSpan, CancellationToken, Task>? delayFunc = null)
    {
        this.httpClient = httpClient;
        this.throttleOptions = throttleOptions ?? new PlanItThrottleOptions();
        this.delayFunc = delayFunc ?? Task.Delay;
    }

    public async IAsyncEnumerable<PlanningApplication> FetchApplicationsAsync(
        int authorityId,
        DateTimeOffset? differentStart,
        [EnumeratorCancellation] CancellationToken ct)
    {
        var page = 1;
        int fetched;

        do
        {
            var url = new Uri(BuildUrl(authorityId, differentStart, page), UriKind.Relative);
            using var response = await this.SendWithThrottleAsync(url, authorityId, ct).ConfigureAwait(false);
            response.EnsureSuccessStatusCode();

            var planItResponse = await JsonSerializer.DeserializeAsync(
                await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false),
                PlanItJsonSerializerContext.Default.PlanItResponse,
                ct).ConfigureAwait(false);

            if (planItResponse is null)
            {
                yield break;
            }

            fetched = planItResponse.Records.Count;

            foreach (var record in planItResponse.Records)
            {
                yield return MapToDomain(record);
            }

            page++;
        }
        while (fetched >= DefaultPageSize);
    }

    public async Task<PlanItSearchResult> SearchApplicationsAsync(
        string searchText,
        int authorityId,
        int page,
        CancellationToken ct)
    {
        var url = new Uri(BuildSearchUrl(searchText, authorityId, page), UriKind.Relative);
        using var response = await this.SendWithThrottleAsync(url, authorityId, ct).ConfigureAwait(false);
        response.EnsureSuccessStatusCode();

        var planItResponse = await JsonSerializer.DeserializeAsync(
            await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false),
            PlanItJsonSerializerContext.Default.PlanItResponse,
            ct).ConfigureAwait(false);

        if (planItResponse is null)
        {
            return new PlanItSearchResult([], 0);
        }

        var applications = planItResponse.Records
            .Select(MapToDomain)
            .ToList();

        return new PlanItSearchResult(applications, planItResponse.Total ?? 0);
    }

    private static string BuildSearchUrl(string searchText, int authorityId, int page)
    {
        var encodedQuery = Uri.EscapeDataString(searchText);
        return $"/api/applics/json?pg_sz={SearchPageSize}&sort=-last_different&page={page}&auth={authorityId}&search={encodedQuery}";
    }

    private static string BuildUrl(int authorityId, DateTimeOffset? differentStart, int page)
    {
        var url = $"/api/applics/json?pg_sz={DefaultPageSize}&sort=-last_different&page={page}&auth={authorityId}";

        if (differentStart.HasValue)
        {
            url += $"&different_start={differentStart.Value:yyyy-MM-dd}";
        }

        return url;
    }

    private static PlanningApplication MapToDomain(PlanItApplicationRecord record)
    {
        return new PlanningApplication(
            name: record.Name,
            uid: record.Uid,
            areaName: record.AreaName,
            areaId: record.AreaId,
            address: record.Address,
            postcode: record.Postcode,
            description: record.Description ?? string.Empty,
            appType: record.AppType,
            appState: record.AppState,
            appSize: record.AppSize,
            startDate: ParseDate(record.StartDate),
            decidedDate: ParseDate(record.DecidedDate),
            consultedDate: ParseDate(record.ConsultedDate),
            longitude: record.LocationX,
            latitude: record.LocationY,
            url: record.Url,
            link: record.Link,
            lastDifferent: DateTimeOffset.Parse(record.LastDifferent, CultureInfo.InvariantCulture));
    }

    private static DateOnly? ParseDate(string? value)
    {
        if (string.IsNullOrEmpty(value))
        {
            return null;
        }

        return DateOnly.Parse(value, CultureInfo.InvariantCulture);
    }

    private async Task<HttpResponseMessage> SendWithThrottleAsync(Uri url, int authorityId, CancellationToken ct)
    {
        if (this.throttleOptions.DelayBetweenRequests > TimeSpan.Zero)
        {
            await this.delayFunc(this.throttleOptions.DelayBetweenRequests, ct).ConfigureAwait(false);
        }

        var response = await this.httpClient.GetAsync(url, ct).ConfigureAwait(false);

        if (!response.IsSuccessStatusCode)
        {
            PlanItInstrumentation.HttpErrors.Add(
                1,
                new KeyValuePair<string, object?>("http.response.status_code", (int)response.StatusCode),
                new KeyValuePair<string, object?>("planit.authority_code", authorityId));
        }

        return response;
    }
}
```

- [ ] **Step 2: Delete `PlanItRetryOptions.cs`**

Delete: `api/src/town-crier.infrastructure/PlanIt/PlanItRetryOptions.cs`

- [ ] **Step 3: Delete `PlanItRetryOptionsTests.cs`**

Delete: `api/tests/town-crier.infrastructure.tests/PlanIt/PlanItRetryOptionsTests.cs`

- [ ] **Step 4: Delete `FakePlanItLogger.cs`**

Delete: `api/tests/town-crier.infrastructure.tests/PlanIt/FakePlanItLogger.cs`

- [ ] **Step 5: Rewrite `PlanItClientTests.cs`**

Remove these tests (they test retry/backoff logic that no longer exists):
- `Should_RetryAndSucceed_When_ApiReturns429ThenSuccess`
- `Should_ApplyExponentialBackoff_When_Retrying429`
- `Should_ThrowHttpRequestException_When_MaxRetriesExhausted`
- `Should_ThrottleBeforeEveryRetryAttempt_When_RateLimitedThenSucceeds`
- `Should_UseRetryAfterDelay_When_429ResponseHasDeltaSecondsHeader`
- `Should_LogRetryAfterSource_When_429ResponseHasRetryAfterHeader`
- `Should_LogBackoffSource_When_429ResponseHasNoRetryAfterHeader`
- `Should_UseRetryAfterDelay_When_HeaderContainsHttpDate`
- `Should_FallBackToExponentialBackoff_When_NoRetryAfterHeader`

Add this new test:

```csharp
[Test]
public async Task Should_ThrowImmediately_When_ApiReturns429()
{
    // Arrange
    using var handler = new FakePlanItHandler();
    handler.SetupRateLimitForever("page=1");
    var client = CreateClient(handler);

    // Act & Assert — 429 throws immediately, no retries
    await Assert.ThrowsAsync<HttpRequestException>(
        async () => await ConsumeAsync(client, differentStart: null));

    // Only 1 request — no retries
    await Assert.That(handler.RequestUrls).HasCount().EqualTo(1);
}
```

Update `CreateClient` helper to remove `retryOptions` parameter and `ILogger` parameter:

```csharp
private static PlanItClient CreateClient(
    FakePlanItHandler handler,
    PlanItThrottleOptions? throttleOptions = null,
    List<TimeSpan>? throttleDelays = null)
{
    var httpClient = new HttpClient(handler, disposeHandler: false)
    {
        BaseAddress = new Uri(BaseUrl),
    };

    Func<TimeSpan, CancellationToken, Task>? delayFunc = null;
    if (throttleDelays is not null)
    {
        delayFunc = (delay, _) =>
        {
            throttleDelays.Add(delay);
            return Task.CompletedTask;
        };
    }

    return new PlanItClient(httpClient, throttleOptions, delayFunc);
}
```

Update `Should_RecordHttpErrorMetric_When_ApiReturns429` test — it currently sets up 2 rate limits then success. Change it to test a single 429:

```csharp
[Test]
[NotInParallel]
public async Task Should_RecordHttpErrorMetric_When_ApiReturns429()
{
    // Arrange
    using var handler = new FakePlanItHandler();
    handler.SetupRateLimitForever("page=1");
    var client = CreateClient(handler);

    var recorded = new List<(long Value, int StatusCode, int AuthorityCode)>();
    using var listener = new MeterListener();
    listener.InstrumentPublished = (instrument, listener) =>
    {
        if (instrument.Name == "towncrier.planit.http_errors")
        {
            listener.EnableMeasurementEvents(instrument);
        }
    };
    listener.SetMeasurementEventCallback<long>((instrument, measurement, tags, _) =>
    {
        var statusCode = 0;
        var authorityCode = 0;
        foreach (var tag in tags)
        {
            if (tag.Key == "http.response.status_code")
            {
                statusCode = (int)tag.Value!;
            }

            if (tag.Key == "planit.authority_code")
            {
                authorityCode = (int)tag.Value!;
            }
        }

        recorded.Add((measurement, statusCode, authorityCode));
    });
    listener.Start();

    // Act & Assert — 429 throws, but metric should be recorded
    await Assert.ThrowsAsync<HttpRequestException>(
        async () => await ConsumeAsync(client, differentStart: null, authorityId: 292));

    await Assert.That(recorded).HasCount().EqualTo(1);
    await Assert.That(recorded[0].StatusCode).IsEqualTo(429);
    await Assert.That(recorded[0].AuthorityCode).IsEqualTo(292);
}
```

Update `Should_RecordHttpErrorMetric_When_ApiReturns500` and `Should_NotRecordHttpErrorMetric_When_ApiReturns200` — update `CreateClient` calls to not pass `retryOptions`.

Update remaining throttle tests (`Should_DelayBeforeEachRequest_When_ThrottleConfigured`, etc.) to use the new `CreateClient` signature.

- [ ] **Step 6: Run tests to verify they pass**

Run: `dotnet test api/ --filter "PlanItClientTests"`
Expected: All tests pass

- [ ] **Step 7: Commit**

```bash
git add api/src/town-crier.infrastructure/PlanIt/PlanItClient.cs \
        api/tests/town-crier.infrastructure.tests/PlanIt/PlanItClientTests.cs
git rm api/src/town-crier.infrastructure/PlanIt/PlanItRetryOptions.cs \
       api/tests/town-crier.infrastructure.tests/PlanIt/PlanItRetryOptionsTests.cs \
       api/tests/town-crier.infrastructure.tests/PlanIt/FakePlanItLogger.cs
git commit -m "refactor(api): remove PlanIt retry/backoff logic

PlanItClient now throws immediately on 429 — no retries, no
Retry-After parsing, no exponential backoff. The handler stops
the polling cycle on 429 and exits cleanly."
```

---

### Task 4: Remove `PlanItPollingOptions` and update DI wiring

**Files:**
- Delete: `api/src/town-crier.application/Polling/PlanItPollingOptions.cs`
- Delete: `api/tests/town-crier.application.tests/Polling/PlanItPollingOptionsTests.cs`
- Modify: `api/src/town-crier.worker/Program.cs`
- Modify: `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs`

- [ ] **Step 1: Delete `PlanItPollingOptions.cs`**

Delete: `api/src/town-crier.application/Polling/PlanItPollingOptions.cs`

- [ ] **Step 2: Delete `PlanItPollingOptionsTests.cs`**

Delete: `api/tests/town-crier.application.tests/Polling/PlanItPollingOptionsTests.cs`

- [ ] **Step 3: Update worker `Program.cs`**

In `api/src/town-crier.worker/Program.cs`, make these changes:

Remove the `using Microsoft.Extensions.Configuration;` import (line 3) if no other `Configuration` calls remain.

Remove these lines (95-105):

```csharp
var planItThrottle = new PlanItThrottleOptions();
builder.Configuration.GetSection("PlanIt:Throttle").Bind(planItThrottle);
builder.Services.AddSingleton(planItThrottle);

var planItRetry = new PlanItRetryOptions();
builder.Configuration.GetSection("PlanIt:Retry").Bind(planItRetry);
builder.Services.AddSingleton(planItRetry);

var planItPolling = new PlanItPollingOptions();
builder.Configuration.GetSection("PlanIt:Polling").Bind(planItPolling);
builder.Services.AddSingleton(planItPolling);
```

The `PlanItClient` constructor uses default `PlanItThrottleOptions` when none are injected, so explicit registration isn't needed unless you want to override via config. For now, defaults are fine.

Update the OTel metrics configuration to use a shorter export interval. Change:

```csharp
metrics.AddAzureMonitorMetricExporter();
```

to:

```csharp
metrics.AddAzureMonitorMetricExporter(o => o.ExportIntervalMilliseconds = 5_000);
```

In the poll mode case (around line 146 in the current file), update to handle the new `RateLimited` property on the result. After `PollingMetrics.CycleDuration.Record(...)`, add:

```csharp
if (result.RateLimited)
{
    PollingMetrics.RateLimited.Add(1);
}
```

- [ ] **Step 4: Update API `ServiceCollectionExtensions.cs`**

In `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs`, remove the `PlanItRetryOptions` registration (lines 105-107):

```csharp
var planItRetry = new PlanItRetryOptions();
configuration.GetSection("PlanIt:Retry").Bind(planItRetry);
services.AddSingleton(planItRetry);
```

Keep the `PlanItThrottleOptions` registration — the API's `SearchApplicationsAsync` still uses it.

- [ ] **Step 5: Verify full build and test suite**

Run: `dotnet build api/ && dotnet test api/`
Expected: Build succeeded, all tests pass

- [ ] **Step 6: Commit**

```bash
git rm api/src/town-crier.application/Polling/PlanItPollingOptions.cs \
       api/tests/town-crier.application.tests/Polling/PlanItPollingOptionsTests.cs
git add api/src/town-crier.worker/Program.cs \
        api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs
git commit -m "refactor(api): remove PlanItPollingOptions and PlanItRetryOptions DI

Removes configuration bindings for retry and polling cooldown
options that no longer exist. Sets OTel metric export interval
to 5s for the worker to ensure metrics flush during short runs."
```

---

### Task 5: Update infrastructure — schedule and timeout

**Files:**
- Modify: `infra/EnvironmentStack.cs`

- [ ] **Step 1: Update poll job configuration**

In `infra/EnvironmentStack.cs`, find the polling job definition (around line 320). Change:

```csharp
ReplicaTimeout = 600,
```

to:

```csharp
ReplicaTimeout = 120,
```

And change:

```csharp
CronExpression = "0 * * * *",
```

to:

```csharp
CronExpression = "*/15 * * * *",
```

- [ ] **Step 2: Verify Pulumi build**

Run: `dotnet build infra/`
Expected: Build succeeded

- [ ] **Step 3: Commit**

```bash
git add infra/EnvironmentStack.cs
git commit -m "feat(infra): change poll job to every 15 min with 120s timeout

Supports micro-batch polling: frequent short runs instead of
one large hourly run. The 120s timeout gives comfortable margin
for the ~30-70s expected runtime."
```

---

### Task 6: Full integration verification

- [ ] **Step 1: Run full test suite**

Run: `dotnet test api/`
Expected: All tests pass. No compilation errors.

- [ ] **Step 2: Run formatting check**

Run: `dotnet format api/ --verify-no-changes`
Expected: No formatting issues

- [ ] **Step 3: Fix any formatting issues**

Run: `dotnet format api/`
Then re-run: `dotnet format api/ --verify-no-changes`

- [ ] **Step 4: Final commit if formatting was fixed**

```bash
git add -A
git commit -m "style(api): fix formatting"
```
