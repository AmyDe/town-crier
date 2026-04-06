# Per-Authority Poll State Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Store and retrieve `lastPollTime` per authority ID instead of globally, so new authorities get a 30-day lookback and rate-limited authorities resume from where they left off.

**Architecture:** Change the single-document poll state to per-authority documents (`poll-state-{authorityId}`) in the existing Cosmos PollState container. Move the poll-state read inside the per-authority loop in the handler. Clean up the orphaned global document.

**Tech Stack:** .NET 10, Cosmos DB SDK (via CosmosRestClient), TUnit

**Spec:** `docs/superpowers/specs/2026-04-05-per-authority-poll-state-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `api/src/town-crier.application/Polling/IPollStateStore.cs` | Modify | Add `authorityId` param to both methods, add `DeleteGlobalPollStateAsync` |
| `api/src/town-crier.infrastructure/Polling/PollStateDocument.cs` | Modify | Add `AuthorityId` property |
| `api/src/town-crier.infrastructure/Polling/CosmosPollStateStore.cs` | Modify | Per-authority doc IDs, implement cleanup |
| `api/tests/town-crier.application.tests/Polling/FakePollStateStore.cs` | Modify | Dictionary-based storage, track per-authority |
| `api/tests/town-crier.application.tests/Polling/FakePlanItClient.cs` | Modify | Track `DifferentStartByAuthority` dictionary |
| `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs` | Modify | Move poll-state read inside loop, pass authority ID, call cleanup |
| `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs` | Modify | Update existing tests, add new tests |

---

### Task 1: Update interface and document model

**Files:**
- Modify: `api/src/town-crier.application/Polling/IPollStateStore.cs`
- Modify: `api/src/town-crier.infrastructure/Polling/PollStateDocument.cs`

- [ ] **Step 1: Update `IPollStateStore` interface**

Replace the entire file content:

```csharp
namespace TownCrier.Application.Polling;

public interface IPollStateStore
{
    Task<DateTimeOffset?> GetLastPollTimeAsync(int authorityId, CancellationToken ct);

    Task SaveLastPollTimeAsync(int authorityId, DateTimeOffset pollTime, CancellationToken ct);

    Task DeleteGlobalPollStateAsync(CancellationToken ct);
}
```

- [ ] **Step 2: Update `PollStateDocument`**

Replace the entire file content:

```csharp
namespace TownCrier.Infrastructure.Polling;

internal sealed class PollStateDocument
{
    public required string Id { get; init; }

    public required string LastPollTime { get; init; }

    public int? AuthorityId { get; init; }
}
```

`AuthorityId` is nullable so the existing global doc (which has no `AuthorityId`) can still be deserialized during cleanup.

- [ ] **Step 3: Verify the solution builds (expect compile errors in consumers)**

Run: `dotnet build api/src/town-crier.application/town-crier.application.csproj 2>&1 | tail -5`

Expected: Build succeeds for the application project (interface has no consumers in this project).

Run: `dotnet build api/ 2>&1 | grep -c "error CS"`

Expected: Compile errors in `CosmosPollStateStore.cs`, `PollPlanItCommandHandler.cs`, `FakePollStateStore.cs`, and tests. This is correct — we fix them in subsequent tasks.

- [ ] **Step 4: Commit**

```bash
git add api/src/town-crier.application/Polling/IPollStateStore.cs api/src/town-crier.infrastructure/Polling/PollStateDocument.cs
git commit -m "refactor(api): add authorityId parameter to IPollStateStore interface

Per-authority poll state allows new authorities to backfill 30 days
and prevents rate-limited authorities from losing progress.
Consumers will be updated in subsequent commits."
```

---

### Task 2: Update Cosmos store implementation

**Files:**
- Modify: `api/src/town-crier.infrastructure/Polling/CosmosPollStateStore.cs`

- [ ] **Step 1: Replace `CosmosPollStateStore` implementation**

Replace the entire file content:

```csharp
using System.Globalization;
using TownCrier.Application.Polling;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Polling;

public sealed class CosmosPollStateStore : IPollStateStore
{
    private const string GlobalDocumentId = "poll-state";

    private readonly ICosmosRestClient client;

    public CosmosPollStateStore(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task<DateTimeOffset?> GetLastPollTimeAsync(int authorityId, CancellationToken ct)
    {
        var documentId = FormatDocumentId(authorityId);
        var doc = await this.client.ReadDocumentAsync(
            CosmosContainerNames.PollState,
            documentId,
            documentId,
            CosmosJsonSerializerContext.Default.PollStateDocument,
            ct).ConfigureAwait(false);

        if (doc is null)
        {
            return null;
        }

        return DateTimeOffset.Parse(doc.LastPollTime, CultureInfo.InvariantCulture);
    }

    public async Task SaveLastPollTimeAsync(int authorityId, DateTimeOffset pollTime, CancellationToken ct)
    {
        var documentId = FormatDocumentId(authorityId);
        var doc = new PollStateDocument
        {
            Id = documentId,
            LastPollTime = pollTime.ToString("O", CultureInfo.InvariantCulture),
            AuthorityId = authorityId,
        };

        await this.client.UpsertDocumentAsync(
            CosmosContainerNames.PollState,
            doc,
            documentId,
            CosmosJsonSerializerContext.Default.PollStateDocument,
            ct).ConfigureAwait(false);
    }

    public async Task DeleteGlobalPollStateAsync(CancellationToken ct)
    {
        await this.client.DeleteDocumentAsync(
            CosmosContainerNames.PollState,
            GlobalDocumentId,
            GlobalDocumentId,
            ct).ConfigureAwait(false);
    }

    private static string FormatDocumentId(int authorityId)
    {
        return string.Create(CultureInfo.InvariantCulture, $"poll-state-{authorityId}");
    }
}
```

Key changes:
- `FormatDocumentId` produces `"poll-state-{authorityId}"` (e.g. `"poll-state-314"`)
- Document ID is used as both ID and partition key (same pattern as before)
- `DeleteGlobalPollStateAsync` deletes the old `"poll-state"` doc — `DeleteDocumentAsync` already swallows 404

- [ ] **Step 2: Verify infrastructure project builds**

Run: `dotnet build api/src/town-crier.infrastructure/town-crier.infrastructure.csproj`

Expected: Build succeeds.

- [ ] **Step 3: Commit**

```bash
git add api/src/town-crier.infrastructure/Polling/CosmosPollStateStore.cs
git commit -m "refactor(api): implement per-authority poll state in Cosmos store

Documents keyed as poll-state-{authorityId} instead of single
global poll-state document. Includes DeleteGlobalPollStateAsync
for cleaning up the orphaned global document."
```

---

### Task 3: Update fake and test helpers

**Files:**
- Modify: `api/tests/town-crier.application.tests/Polling/FakePollStateStore.cs`
- Modify: `api/tests/town-crier.application.tests/Polling/FakePlanItClient.cs`

- [ ] **Step 1: Replace `FakePollStateStore`**

Replace the entire file content:

```csharp
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakePollStateStore : IPollStateStore
{
    private readonly Dictionary<int, DateTimeOffset> pollTimes = [];

    public int SaveCallCount { get; private set; }

    public bool DeleteGlobalCalled { get; private set; }

    public DateTimeOffset? GetLastPollTimeFor(int authorityId)
    {
        return this.pollTimes.TryGetValue(authorityId, out var time) ? time : null;
    }

    public void SetLastPollTime(int authorityId, DateTimeOffset pollTime)
    {
        this.pollTimes[authorityId] = pollTime;
    }

    public Task<DateTimeOffset?> GetLastPollTimeAsync(int authorityId, CancellationToken ct)
    {
        DateTimeOffset? result = this.pollTimes.TryGetValue(authorityId, out var time) ? time : null;
        return Task.FromResult(result);
    }

    public Task SaveLastPollTimeAsync(int authorityId, DateTimeOffset pollTime, CancellationToken ct)
    {
        this.pollTimes[authorityId] = pollTime;
        this.SaveCallCount++;
        return Task.CompletedTask;
    }

    public Task DeleteGlobalPollStateAsync(CancellationToken ct)
    {
        this.DeleteGlobalCalled = true;
        return Task.CompletedTask;
    }
}
```

- [ ] **Step 2: Add `DifferentStartByAuthority` to `FakePlanItClient`**

In `api/tests/town-crier.application.tests/Polling/FakePlanItClient.cs`, add a dictionary to track per-authority different_start values. Add this property after the existing `LastDifferentStartUsed` property (line 12):

```csharp
public Dictionary<int, DateTimeOffset?> DifferentStartByAuthority { get; } = [];
```

Then in the `FetchApplicationsAsync` method, after line 55 (`this.LastDifferentStartUsed = differentStart;`), add:

```csharp
this.DifferentStartByAuthority[authorityId] = differentStart;
```

- [ ] **Step 3: Verify test project compiles (expect errors in test file only)**

Run: `dotnet build api/tests/town-crier.application.tests/ 2>&1 | grep "error CS"`

Expected: Errors only in `PollPlanItCommandHandlerTests.cs` (old `pollStateStore.LastPollTime` references). The fakes themselves should compile.

- [ ] **Step 4: Commit**

```bash
git add api/tests/town-crier.application.tests/Polling/FakePollStateStore.cs api/tests/town-crier.application.tests/Polling/FakePlanItClient.cs
git commit -m "refactor(api): update test fakes for per-authority poll state

FakePollStateStore now uses Dictionary<int, DateTimeOffset> for
per-authority tracking. FakePlanItClient tracks DifferentStartByAuthority."
```

---

### Task 4: Update handler to use per-authority poll state

**Files:**
- Modify: `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs`

- [ ] **Step 1: Update `HandleAsync` method**

Replace the `HandleAsync` method body (lines 42–111) with:

```csharp
    public async Task<PollPlanItResult> HandleAsync(PollPlanItCommand command, CancellationToken ct)
    {
        var now = this.timeProvider.GetUtcNow();
        var authorityIds = await this.activeAuthorityProvider.GetActiveAuthorityIdsAsync(ct).ConfigureAwait(false);

        var count = 0;
        var authoritiesPolled = 0;
        var rateLimitHitCount = 0;
        foreach (var authorityId in authorityIds)
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
                rateLimitHitCount++;
                PollingMetrics.AuthoritiesSkipped.Add(1);
                PollingMetrics.AuthorityProcessingDuration.Record(Stopwatch.GetElapsedTime(authorityStart).TotalMilliseconds);

                if (rateLimitHitCount >= 2)
                {
                    LogRateLimitBreak(this.logger, authorityId, ex);
                    break;
                }

                LogRateLimitSkip(this.logger, authorityId, ex);
            }
            catch (Exception ex) when (ex is not OperationCanceledException)
            {
                PollingMetrics.AuthoritiesSkipped.Add(1);
                PollingMetrics.AuthorityProcessingDuration.Record(Stopwatch.GetElapsedTime(authorityStart).TotalMilliseconds);
                LogAuthorityError(this.logger, authorityId, ex);
            }
        }

        await this.pollStateStore.DeleteGlobalPollStateAsync(ct).ConfigureAwait(false);

        return new PollPlanItResult(count, authoritiesPolled);
    }
```

Key changes from the original:
- Removed: `var lastPollTime = await this.pollStateStore.GetLastPollTimeAsync(ct)` before the loop
- Added: `var lastPollTime = await this.pollStateStore.GetLastPollTimeAsync(authorityId, ct)` inside the try block, per authority
- Changed: `SaveLastPollTimeAsync(now, ct)` → `SaveLastPollTimeAsync(authorityId, now, ct)`
- Added: `await this.pollStateStore.DeleteGlobalPollStateAsync(ct)` after the loop

- [ ] **Step 2: Verify application project builds**

Run: `dotnet build api/src/town-crier.application/town-crier.application.csproj`

Expected: Build succeeds.

- [ ] **Step 3: Commit**

```bash
git add api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs
git commit -m "feat(api): use per-authority poll state in polling handler

Poll state is now read and written per authority ID inside the loop.
New authorities automatically get a 30-day lookback. Rate-limited
authorities retain their own last poll time. The orphaned global
poll-state document is cleaned up after each cycle."
```

---

### Task 5: Update existing tests

**Files:**
- Modify: `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs`

- [ ] **Step 1: Update `Should_UseDefault30DayLookback_When_NoPreviousPollState`**

This test verifies new authorities use 30-day lookback. The FakePlanItClient's `LastDifferentStartUsed` still captures the last call. The test needs no assertion change — only the handler logic changed (poll state read moved inside loop). No changes needed to this test.

- [ ] **Step 2: Update `Should_PassLastPollTime_When_PreviousPollStateExists`**

Change the `SetLastPollTime` call to include the authority ID. Replace lines 96–98:

Old:
```csharp
        var pollStateStore = new FakePollStateStore();
        var lastPoll = new DateTimeOffset(2026, 3, 15, 10, 0, 0, TimeSpan.Zero);
        pollStateStore.SetLastPollTime(lastPoll);
```

New:
```csharp
        var pollStateStore = new FakePollStateStore();
        var lastPoll = new DateTimeOffset(2026, 3, 15, 10, 0, 0, TimeSpan.Zero);
        pollStateStore.SetLastPollTime(1, lastPoll);
```

- [ ] **Step 3: Update `Should_PersistCurrentTime_When_PollSucceeds`**

Replace the assertion on line 121. Old:
```csharp
        await Assert.That(pollStateStore.LastPollTime).IsEqualTo(fakeTime);
```

New:
```csharp
        await Assert.That(pollStateStore.GetLastPollTimeFor(1)).IsEqualTo(fakeTime);
```

(Authority ID is 1 because `FakeActiveAuthorityProvider` defaults to empty and the test adds authority `1` on line 110.)

- [ ] **Step 4: Update `Should_SavePollStateAfterEachAuthority_When_MultipleAuthoritiesPolled`**

No changes needed — `SaveCallCount` is still tracked the same way and the test only checks the count (3 saves for 3 authorities).

- [ ] **Step 5: Update `Should_NotSavePollState_When_OnlyAuthorityFails`**

Replace the assertion on line 280. Old:
```csharp
        await Assert.That(pollStateStore.LastPollTime).IsNull();
```

New:
```csharp
        await Assert.That(pollStateStore.GetLastPollTimeFor(1)).IsNull();
```

- [ ] **Step 6: Update `Should_ContinueAndPreserveProgress_When_MiddleAuthorityFails`**

Replace the assertion on line 262. Old:
```csharp
        await Assert.That(pollStateStore.LastPollTime).IsEqualTo(fakeTime);
```

New:
```csharp
        await Assert.That(pollStateStore.GetLastPollTimeFor(100)).IsEqualTo(fakeTime);
        await Assert.That(pollStateStore.GetLastPollTimeFor(300)).IsEqualTo(fakeTime);
        await Assert.That(pollStateStore.GetLastPollTimeFor(200)).IsNull();
```

This is stronger than before — it verifies authority 200 (the failed one) didn't get its timestamp advanced, while 100 and 300 did.

- [ ] **Step 7: Run all existing tests**

Run: `dotnet test api/tests/town-crier.application.tests/ --filter "PollPlanItCommandHandler"`

Expected: All 14 existing tests pass.

- [ ] **Step 8: Commit**

```bash
git add api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs
git commit -m "test(api): update existing poll handler tests for per-authority state

Mechanical update: SetLastPollTime and assertions now use authority IDs.
ContinueAndPreserveProgress test now verifies per-authority isolation."
```

---

### Task 6: Add new tests for per-authority behaviour

**Files:**
- Modify: `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs`

- [ ] **Step 1: Add test — new authority uses 30-day lookback when others have state**

Add this test after the existing tests, before the `CreateHandler` method:

```csharp
    [Test]
    public async Task Should_Use30DayLookback_When_NewAuthorityHasNoPollState()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);

        var pollStateStore = new FakePollStateStore();
        var existingPollTime = new DateTimeOffset(2026, 4, 5, 10, 0, 0, TimeSpan.Zero);
        pollStateStore.SetLastPollTime(100, existingPollTime);
        // Authority 200 has no poll state — should get 30-day lookback

        var now = new DateTimeOffset(2026, 4, 5, 12, 0, 0, TimeSpan.Zero);
        var planItClient = new FakePlanItClient();

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider,
            timeProvider: new FakeTimeProvider(now));

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Authority 100 should use its existing poll time
        await Assert.That(planItClient.DifferentStartByAuthority[100]).IsEqualTo(existingPollTime);

        // Authority 200 should use 30-day lookback
        var expected30DaysAgo = new DateTimeOffset(2026, 3, 6, 12, 0, 0, TimeSpan.Zero);
        await Assert.That(planItClient.DifferentStartByAuthority[200]).IsEqualTo(expected30DaysAgo);
    }
```

- [ ] **Step 2: Run to verify it fails**

Run: `dotnet test api/tests/town-crier.application.tests/ --filter "Should_Use30DayLookback_When_NewAuthorityHasNoPollState"`

Expected: PASS (the handler implementation from Task 4 already supports this).

Note: This test would have FAILED against the old global-state implementation, where authority 200 would have received the same `existingPollTime` as authority 100. We're writing it to prevent regression.

- [ ] **Step 3: Add test — rate-limited authority retains its own poll time**

```csharp
    [Test]
    public async Task Should_RetainPerAuthorityPollTime_When_AuthorityIsRateLimited()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);

        var pollStateStore = new FakePollStateStore();
        var authority100Time = new DateTimeOffset(2026, 4, 4, 10, 0, 0, TimeSpan.Zero);
        var authority200Time = new DateTimeOffset(2026, 4, 3, 8, 0, 0, TimeSpan.Zero);
        pollStateStore.SetLastPollTime(100, authority100Time);
        pollStateStore.SetLastPollTime(200, authority200Time);

        var now = new DateTimeOffset(2026, 4, 5, 12, 0, 0, TimeSpan.Zero);
        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.ThrowForAuthority(200, new HttpRequestException("Rate limited", null, HttpStatusCode.TooManyRequests));

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider,
            timeProvider: new FakeTimeProvider(now));

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Authority 100 should be advanced to now
        await Assert.That(pollStateStore.GetLastPollTimeFor(100)).IsEqualTo(now);

        // Authority 200 should retain its original poll time (rate limited, not advanced)
        await Assert.That(pollStateStore.GetLastPollTimeFor(200)).IsEqualTo(authority200Time);
    }
```

- [ ] **Step 4: Run to verify it passes**

Run: `dotnet test api/tests/town-crier.application.tests/ --filter "Should_RetainPerAuthorityPollTime_When_AuthorityIsRateLimited"`

Expected: PASS.

- [ ] **Step 5: Add test — global poll state cleanup**

```csharp
    [Test]
    public async Task Should_DeleteGlobalPollState_When_CycleCompletes()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var pollStateStore = new FakePollStateStore();
        var handler = CreateHandler(pollStateStore: pollStateStore, authorityProvider: authorityProvider);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(pollStateStore.DeleteGlobalCalled).IsTrue();
    }

    [Test]
    public async Task Should_DeleteGlobalPollState_When_NoActiveAuthorities()
    {
        var pollStateStore = new FakePollStateStore();
        var handler = CreateHandler(pollStateStore: pollStateStore);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(pollStateStore.DeleteGlobalCalled).IsTrue();
    }
```

- [ ] **Step 6: Run all new tests**

Run: `dotnet test api/tests/town-crier.application.tests/ --filter "Should_DeleteGlobalPollState|Should_RetainPerAuthorityPollTime|Should_Use30DayLookback_When_NewAuthority"`

Expected: All 4 new tests PASS.

- [ ] **Step 7: Run full test suite**

Run: `dotnet test api/`

Expected: All tests pass.

- [ ] **Step 8: Commit**

```bash
git add api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs
git commit -m "test(api): add per-authority poll state regression tests

New authority 30-day lookback, rate-limited authority time retention,
and global poll-state cleanup are now covered."
```

---

### Task 7: Format check and final verification

- [ ] **Step 1: Run formatter**

Run: `dotnet format api/ --verify-no-changes`

If formatting issues found, run: `dotnet format api/` and commit the fix.

- [ ] **Step 2: Run full build**

Run: `dotnet build api/`

Expected: Build succeeds with no warnings.

- [ ] **Step 3: Run full test suite one final time**

Run: `dotnet test api/`

Expected: All tests pass.
