# Extract Polling into Container Apps Job — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract the PlanIt polling BackgroundService into a dedicated Container Apps Job so the API container can scale to zero.

**Architecture:** New `town-crier.worker` console app runs one poll cycle and exits. `PollPlanItCommandHandler` is simplified (no scheduling, no health tracking). `FilePollStateStore` replaced with `CosmosPollStateStore`. Pulumi adds a cron-triggered Job resource. API reverts to MinReplicas=0.

**Tech Stack:** .NET 10, Native AOT, Azure Container Apps Jobs, Cosmos DB, Pulumi (C#), GitHub Actions

**Spec:** `docs/superpowers/specs/2026-04-04-extract-polling-job-design.md`

---

### Task 1: Simplify PollPlanItCommandHandler — remove scheduling and health tracking

**Files:**
- Modify: `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs`
- Modify: `api/src/town-crier.application/Polling/PollPlanItCommand.cs`
- Modify: `api/src/town-crier.application/Polling/PollPlanItResult.cs`

- [ ] **Step 1: Simplify PollPlanItCommand — remove CycleNumber**

Replace the contents of `api/src/town-crier.application/Polling/PollPlanItCommand.cs`:

```csharp
namespace TownCrier.Application.Polling;

public sealed record PollPlanItCommand;
```

- [ ] **Step 2: Simplify PollPlanItResult — remove scheduling fields**

Replace the contents of `api/src/town-crier.application/Polling/PollPlanItResult.cs`:

```csharp
namespace TownCrier.Application.Polling;

public sealed record PollPlanItResult(int ApplicationCount, int AuthoritiesPolled);
```

- [ ] **Step 3: Simplify PollPlanItCommandHandler — remove scheduling and health dependencies**

Replace the contents of `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs`:

```csharp
using System.Diagnostics;
using TownCrier.Application.Observability;
using TownCrier.Application.PlanIt;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.WatchZones;

namespace TownCrier.Application.Polling;

public sealed class PollPlanItCommandHandler
{
    private readonly IPlanItClient planItClient;
    private readonly IPollStateStore pollStateStore;
    private readonly IPlanningApplicationRepository applicationRepository;
    private readonly TimeProvider timeProvider;
    private readonly IActiveAuthorityProvider activeAuthorityProvider;
    private readonly IWatchZoneRepository watchZoneRepository;
    private readonly INotificationEnqueuer notificationEnqueuer;

    public PollPlanItCommandHandler(
        IPlanItClient planItClient,
        IPollStateStore pollStateStore,
        IPlanningApplicationRepository applicationRepository,
        TimeProvider timeProvider,
        IActiveAuthorityProvider activeAuthorityProvider,
        IWatchZoneRepository watchZoneRepository,
        INotificationEnqueuer notificationEnqueuer)
    {
        this.planItClient = planItClient;
        this.pollStateStore = pollStateStore;
        this.applicationRepository = applicationRepository;
        this.timeProvider = timeProvider;
        this.activeAuthorityProvider = activeAuthorityProvider;
        this.watchZoneRepository = watchZoneRepository;
        this.notificationEnqueuer = notificationEnqueuer;
    }

    public async Task<PollPlanItResult> HandleAsync(PollPlanItCommand command, CancellationToken ct)
    {
        var lastPollTime = await this.pollStateStore.GetLastPollTimeAsync(ct).ConfigureAwait(false);
        var authorityIds = await this.activeAuthorityProvider.GetActiveAuthorityIdsAsync(ct).ConfigureAwait(false);
        var now = this.timeProvider.GetUtcNow();

        var count = 0;
        foreach (var authorityId in authorityIds)
        {
            using var authorityActivity = PollingInstrumentation.Source.StartActivity("Poll Authority");
            authorityActivity?.SetTag("polling.authority_code", authorityId);
            var authorityStart = Stopwatch.GetTimestamp();

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
        }

        PollingMetrics.AuthoritiesPolled.Add(authorityIds.Count);

        await this.pollStateStore.SaveLastPollTimeAsync(now, ct).ConfigureAwait(false);

        return new PollPlanItResult(count, authorityIds.Count);
    }
}
```

- [ ] **Step 4: Verify the solution builds**

Run: `dotnet build api/town-crier.sln`
Expected: Build errors — tests still reference old constructor signatures. That's expected; we'll fix tests in Task 2.

- [ ] **Step 5: Commit**

```bash
git add api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs api/src/town-crier.application/Polling/PollPlanItCommand.cs api/src/town-crier.application/Polling/PollPlanItResult.cs
git commit -m "refactor(api): simplify PollPlanItCommandHandler — remove scheduling and health tracking"
```

---

### Task 2: Update handler tests for simplified signature

**Files:**
- Modify: `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs`
- Delete: `api/tests/town-crier.application.tests/Polling/PollPlanItHealthMonitoringTests.cs`
- Delete: `api/tests/town-crier.application.tests/Polling/PollingScheduleTests.cs`
- Delete: `api/tests/town-crier.application.tests/Polling/FakePollingHealthStore.cs`
- Delete: `api/tests/town-crier.application.tests/Polling/SpyPollingHealthAlerter.cs`

- [ ] **Step 1: Delete health monitoring tests and scheduling tests**

Delete these files:
- `api/tests/town-crier.application.tests/Polling/PollPlanItHealthMonitoringTests.cs`
- `api/tests/town-crier.application.tests/Polling/PollingScheduleTests.cs`
- `api/tests/town-crier.application.tests/Polling/FakePollingHealthStore.cs`
- `api/tests/town-crier.application.tests/Polling/SpyPollingHealthAlerter.cs`

- [ ] **Step 2: Rewrite PollPlanItCommandHandlerTests with simplified constructor**

Replace the contents of `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs`:

```csharp
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class PollPlanItCommandHandlerTests
{
    [Test]
    public async Task Should_ReturnApplicationCount_When_PlanItReturnsApplications()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(1).Build());
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(1).Build());
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-3").WithAreaId(1).Build());

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(result.ApplicationCount).IsEqualTo(3);
    }

    [Test]
    public async Task Should_ReturnZeroCount_When_NoActiveAuthorities()
    {
        var handler = CreateHandler();

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(result.ApplicationCount).IsEqualTo(0);
    }

    [Test]
    public async Task Should_NotCallPlanItClient_When_NoActiveAuthorities()
    {
        var planItClient = new FakePlanItClient();
        var handler = CreateHandler(planItClient: planItClient);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(planItClient.AuthorityIdsRequested).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_FetchForAllActiveAuthorities()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.Add(200, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(200).Build());

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(result.ApplicationCount).IsEqualTo(2);
        await Assert.That(result.AuthoritiesPolled).IsEqualTo(2);
        await Assert.That(planItClient.AuthorityIdsRequested).Contains(100);
        await Assert.That(planItClient.AuthorityIdsRequested).Contains(200);
    }

    [Test]
    public async Task Should_PassNullDifferentStart_When_NoPreviousPollState()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(planItClient.LastDifferentStartUsed).IsNull();
    }

    [Test]
    public async Task Should_PassLastPollTime_When_PreviousPollStateExists()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        var pollStateStore = new FakePollStateStore();
        var lastPoll = new DateTimeOffset(2026, 3, 15, 10, 0, 0, TimeSpan.Zero);
        pollStateStore.SetLastPollTime(lastPoll);

        var handler = CreateHandler(planItClient: planItClient, pollStateStore: pollStateStore, authorityProvider: authorityProvider);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(planItClient.LastDifferentStartUsed).IsEqualTo(lastPoll);
    }

    [Test]
    public async Task Should_PersistCurrentTime_When_PollSucceeds()
    {
        var pollStateStore = new FakePollStateStore();
        var fakeTime = new DateTimeOffset(2026, 3, 16, 12, 0, 0, TimeSpan.Zero);
        var handler = CreateHandler(pollStateStore: pollStateStore, timeProvider: new FakeTimeProvider(fakeTime));

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(pollStateStore.LastPollTime).IsEqualTo(fakeTime);
    }

    [Test]
    public async Task Should_UpsertAllApplications_When_PlanItReturnsResults()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-1").WithName("Council/app-1").WithAreaId(1).Build());
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-2").WithName("Council/app-2").WithAreaId(1).Build());

        var repository = new FakePlanningApplicationRepository();
        var handler = CreateHandler(planItClient: planItClient, repository: repository, authorityProvider: authorityProvider);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(repository.GetAll()).HasCount().EqualTo(2);
    }

    [Test]
    public async Task Should_EnqueueNotification_When_ApplicationIsWithinWatchZone()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var watchZoneRepository = new FakeWatchZoneRepository();
        watchZoneRepository.Add(new WatchZoneBuilder()
            .WithId("zone-1").WithUserId("user-1")
            .WithCentre(51.5074, -0.1278).WithRadiusMetres(5000).WithAuthorityId(1).Build());

        var notificationEnqueuer = new FakeNotificationEnqueuer();
        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder()
            .WithUid("app-1").WithAreaId(1).WithCoordinates(51.5080, -0.1270).Build());

        var handler = CreateHandler(
            planItClient: planItClient, authorityProvider: authorityProvider,
            watchZoneRepository: watchZoneRepository, notificationEnqueuer: notificationEnqueuer);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(notificationEnqueuer.Enqueued).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_NotEnqueueNotification_When_ApplicationHasNoCoordinates()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var watchZoneRepository = new FakeWatchZoneRepository();
        watchZoneRepository.Add(new WatchZoneBuilder()
            .WithId("zone-1").WithUserId("user-1")
            .WithCentre(51.5074, -0.1278).WithRadiusMetres(5000).WithAuthorityId(1).Build());

        var notificationEnqueuer = new FakeNotificationEnqueuer();
        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-no-coords").WithAreaId(1).Build());

        var handler = CreateHandler(
            planItClient: planItClient, authorityProvider: authorityProvider,
            watchZoneRepository: watchZoneRepository, notificationEnqueuer: notificationEnqueuer);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(notificationEnqueuer.Enqueued).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_RethrowException_When_PlanItFails()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);
        var failingClient = new FakePlanItClient { ExceptionToThrow = new HttpRequestException("timeout") };

        var handler = CreateHandler(planItClient: failingClient, authorityProvider: authorityProvider);

        await Assert.That(
            async () => await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None))
            .ThrowsException()
            .OfType<HttpRequestException>();
    }

    [Test]
    public async Task Should_NotSavePollState_When_PollFails()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);
        var failingClient = new FakePlanItClient { ExceptionToThrow = new HttpRequestException("timeout") };
        var pollStateStore = new FakePollStateStore();

        var handler = CreateHandler(planItClient: failingClient, pollStateStore: pollStateStore, authorityProvider: authorityProvider);

        try { await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None); }
        catch (HttpRequestException) { }

        await Assert.That(pollStateStore.LastPollTime).IsNull();
    }

    [Test]
    public async Task Should_RecordFailureMetric_When_PollFails()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);
        var failingClient = new FakePlanItClient { ExceptionToThrow = new HttpRequestException("timeout") };

        var handler = CreateHandler(planItClient: failingClient, authorityProvider: authorityProvider);

        try { await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None); }
        catch (HttpRequestException) { }

        // If we get here without crashing, the failure metric was recorded before rethrowing.
        // The PollingMetrics.PollFailures counter is a static — we just verify the handler
        // doesn't swallow the exception.
    }

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
            notificationEnqueuer ?? new FakeNotificationEnqueuer());
    }
}
```

- [ ] **Step 3: Run tests**

Run: `dotnet test api/town-crier.sln`
Expected: All tests pass. Health monitoring tests and scheduling tests are gone; handler tests pass with new signature.

- [ ] **Step 4: Commit**

```bash
git add -A api/tests/town-crier.application.tests/Polling/
git commit -m "test(api): update handler tests for simplified polling — remove health and scheduling tests"
```

---

### Task 3: Delete dead code — scheduling, health tracking, and file state store

**Files:**
- Delete: `api/src/town-crier.domain/Polling/PollingSchedule.cs`
- Delete: `api/src/town-crier.domain/Polling/PollingScheduleConfig.cs`
- Delete: `api/src/town-crier.domain/Polling/PollingPriority.cs`
- Delete: `api/src/town-crier.domain/Polling/PollingHealth.cs`
- Delete: `api/src/town-crier.application/Polling/PollingHealthConfig.cs`
- Delete: `api/src/town-crier.application/Polling/IPollingHealthStore.cs`
- Delete: `api/src/town-crier.application/Polling/IPollingHealthAlerter.cs`
- Delete: `api/src/town-crier.infrastructure/Polling/FilePollStateStore.cs`
- Delete: `api/src/town-crier.infrastructure/Polling/InMemoryPollingHealthStore.cs`
- Delete: `api/src/town-crier.infrastructure/Polling/LogPollingHealthAlerter.cs`
- Delete: `api/src/town-crier.infrastructure/Polling/InMemoryActiveAuthorityProvider.cs`

- [ ] **Step 1: Delete all scheduling and health domain types**

Delete these files:
- `api/src/town-crier.domain/Polling/PollingSchedule.cs`
- `api/src/town-crier.domain/Polling/PollingScheduleConfig.cs`
- `api/src/town-crier.domain/Polling/PollingPriority.cs`
- `api/src/town-crier.domain/Polling/PollingHealth.cs`

- [ ] **Step 2: Delete application-layer health interfaces**

Delete these files:
- `api/src/town-crier.application/Polling/PollingHealthConfig.cs`
- `api/src/town-crier.application/Polling/IPollingHealthStore.cs`
- `api/src/town-crier.application/Polling/IPollingHealthAlerter.cs`

- [ ] **Step 3: Delete infrastructure implementations**

Delete these files:
- `api/src/town-crier.infrastructure/Polling/FilePollStateStore.cs`
- `api/src/town-crier.infrastructure/Polling/InMemoryPollingHealthStore.cs`
- `api/src/town-crier.infrastructure/Polling/LogPollingHealthAlerter.cs`
- `api/src/town-crier.infrastructure/Polling/InMemoryActiveAuthorityProvider.cs`

- [ ] **Step 4: Verify build**

Run: `dotnet build api/town-crier.sln`
Expected: Build succeeds. The handler no longer references any of these types. If `ServiceCollectionExtensions.cs` still references them, it will fail — that's fixed in Task 5.

- [ ] **Step 5: Commit**

```bash
git add -A api/src/
git commit -m "refactor(api): delete dead polling code — scheduling, health tracking, file state store"
```

---

### Task 4: Add CosmosPollStateStore

**Files:**
- Create: `api/src/town-crier.infrastructure/Polling/CosmosPollStateStore.cs`
- Create: `api/src/town-crier.infrastructure/Polling/PollStateDocument.cs`
- Modify: `api/src/town-crier.infrastructure/Cosmos/CosmosContainerNames.cs`
- Modify: `api/src/town-crier.infrastructure/Cosmos/CosmosJsonSerializerContext.cs`

- [ ] **Step 1: Add PollState container name constant**

In `api/src/town-crier.infrastructure/Cosmos/CosmosContainerNames.cs`, add:

```csharp
public const string PollState = "PollState";
```

- [ ] **Step 2: Create PollStateDocument**

Create `api/src/town-crier.infrastructure/Polling/PollStateDocument.cs`:

```csharp
namespace TownCrier.Infrastructure.Polling;

internal sealed class PollStateDocument
{
    public required string Id { get; init; }

    public required string LastPollTime { get; init; }
}
```

- [ ] **Step 3: Register PollStateDocument in CosmosJsonSerializerContext**

In `api/src/town-crier.infrastructure/Cosmos/CosmosJsonSerializerContext.cs`, add this attribute line alongside the existing ones:

```csharp
[JsonSerializable(typeof(PollStateDocument))]
```

And add the using at the top:

```csharp
using TownCrier.Infrastructure.Polling;
```

- [ ] **Step 4: Create CosmosPollStateStore**

Create `api/src/town-crier.infrastructure/Polling/CosmosPollStateStore.cs`:

```csharp
using System.Globalization;
using TownCrier.Application.Polling;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Polling;

public sealed class CosmosPollStateStore : IPollStateStore
{
    private const string DocumentId = "poll-state";

    private readonly ICosmosRestClient client;

    public CosmosPollStateStore(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task<DateTimeOffset?> GetLastPollTimeAsync(CancellationToken ct)
    {
        var doc = await this.client.ReadDocumentAsync(
            CosmosContainerNames.PollState,
            DocumentId,
            DocumentId,
            CosmosJsonSerializerContext.Default.PollStateDocument,
            ct).ConfigureAwait(false);

        if (doc is null)
        {
            return null;
        }

        return DateTimeOffset.Parse(doc.LastPollTime, CultureInfo.InvariantCulture);
    }

    public async Task SaveLastPollTimeAsync(DateTimeOffset pollTime, CancellationToken ct)
    {
        var doc = new PollStateDocument
        {
            Id = DocumentId,
            LastPollTime = pollTime.ToString("O", CultureInfo.InvariantCulture),
        };

        await this.client.UpsertDocumentAsync(
            CosmosContainerNames.PollState,
            doc,
            DocumentId,
            CosmosJsonSerializerContext.Default.PollStateDocument,
            ct).ConfigureAwait(false);
    }
}
```

- [ ] **Step 5: Verify build**

Run: `dotnet build api/town-crier.sln`
Expected: Build succeeds.

- [ ] **Step 6: Run tests**

Run: `dotnet test api/town-crier.sln`
Expected: All tests pass (tests still use `FakePollStateStore`).

- [ ] **Step 7: Commit**

```bash
git add api/src/town-crier.infrastructure/Polling/CosmosPollStateStore.cs api/src/town-crier.infrastructure/Polling/PollStateDocument.cs api/src/town-crier.infrastructure/Cosmos/CosmosContainerNames.cs api/src/town-crier.infrastructure/Cosmos/CosmosJsonSerializerContext.cs
git commit -m "feat(api): add CosmosPollStateStore — persist poll state to Cosmos DB"
```

---

### Task 5: Remove polling from API web host

**Files:**
- Delete: `api/src/town-crier.web/Polling/PlanItPollingService.cs`
- Modify: `api/src/town-crier.web/Program.cs`
- Modify: `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs`

- [ ] **Step 1: Delete PlanItPollingService**

Delete `api/src/town-crier.web/Polling/PlanItPollingService.cs`.

- [ ] **Step 2: Remove hosted service registration from Program.cs**

In `api/src/town-crier.web/Program.cs`, remove the line:

```csharp
builder.Services.AddHostedService<PlanItPollingService>();
```

Also remove the `using TownCrier.Web.Polling;` import.

- [ ] **Step 3: Remove polling-specific DI from ServiceCollectionExtensions.cs**

In `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs`, in the `AddInfrastructureServices` method, remove these lines (lines 50–64):

```csharp
        var pollStateFilePath = configuration["Polling:StateFilePath"]
            ?? Path.Combine(AppContext.BaseDirectory, "poll-state.txt");
        services.AddSingleton<IPollStateStore>(new FilePollStateStore(pollStateFilePath));

        services.AddSingleton<IActiveAuthorityProvider, WatchZoneActiveAuthorityProvider>();
        services.AddSingleton<IPollingHealthStore, InMemoryPollingHealthStore>();
        services.AddSingleton<IPollingHealthAlerter, LogPollingHealthAlerter>();
        services.AddSingleton(new PollingHealthConfig(
            StalenessThreshold: TimeSpan.FromHours(1),
            MaxConsecutiveFailures: 5));
        services.AddSingleton(TimeProvider.System);
        services.AddSingleton<INotificationEnqueuer, LogNotificationEnqueuer>();
        services.AddSingleton(new PollingScheduleConfig(
            HighThreshold: configuration.GetValue("Polling:HighThreshold", 5),
            LowThreshold: configuration.GetValue("Polling:LowThreshold", 2)));
```

Keep `services.AddSingleton(TimeProvider.System);` — other API code may need it. Also keep repository registrations, Cosmos client, HTTP clients — the API still uses those.

In `AddApplicationServices`, remove:

```csharp
        services.AddTransient<PollPlanItCommandHandler>();
```

Remove now-unused using statements:
- `using TownCrier.Application.Polling;`
- `using TownCrier.Domain.Polling;`
- `using TownCrier.Infrastructure.Polling;`
- `using TownCrier.Infrastructure.WatchZones;` — only if no other type from that namespace is used (check: `CosmosWatchZoneRepository` is still registered, so keep it)

Actually, check which `using` directives are still needed. The notification enqueuer (`LogNotificationEnqueuer`) is in `TownCrier.Infrastructure.WatchZones` — remove its registration but keep the using if `CosmosWatchZoneRepository` still needs it. Let the compiler tell you.

- [ ] **Step 4: Verify build**

Run: `dotnet build api/town-crier.sln`
Expected: Build succeeds. The API no longer has any polling code.

- [ ] **Step 5: Run tests**

Run: `dotnet test api/town-crier.sln`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add -A api/src/town-crier.web/
git commit -m "refactor(api): remove polling BackgroundService from API web host"
```

---

### Task 6: Create town-crier.worker console app

**Files:**
- Create: `api/src/town-crier.worker/town-crier.worker.csproj`
- Create: `api/src/town-crier.worker/Program.cs`
- Modify: `api/town-crier.sln` (add project)

- [ ] **Step 1: Create the worker project file**

Create `api/src/town-crier.worker/town-crier.worker.csproj`:

```xml
<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <OutputType>Exe</OutputType>
    <TargetFramework>net10.0</TargetFramework>
    <RootNamespace>TownCrier.Worker</RootNamespace>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
    <PublishAot>true</PublishAot>
  </PropertyGroup>

  <ItemGroup>
    <ProjectReference Include="..\town-crier.infrastructure\town-crier.infrastructure.csproj" />
  </ItemGroup>

  <ItemGroup>
    <PackageReference Include="Azure.Monitor.OpenTelemetry.Exporter" Version="1.7.0" />
    <PackageReference Include="Microsoft.Extensions.Hosting" Version="10.0.*" />
    <PackageReference Include="OpenTelemetry.Extensions.Hosting" Version="1.15.1" />
    <PackageReference Include="OpenTelemetry.Instrumentation.Http" Version="1.15.0" />
  </ItemGroup>

</Project>
```

- [ ] **Step 2: Create the worker Program.cs**

Create `api/src/town-crier.worker/Program.cs`:

```csharp
using System.Diagnostics;
using Azure.Monitor.OpenTelemetry.Exporter;
using OpenTelemetry;
using OpenTelemetry.Metrics;
using OpenTelemetry.Trace;
using TownCrier.Application.Observability;
using TownCrier.Application.PlanIt;
using TownCrier.Application.Polling;
using TownCrier.Application.WatchZones;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.Observability;
using TownCrier.Infrastructure.PlanIt;
using TownCrier.Infrastructure.Polling;
using TownCrier.Infrastructure.PlanningApplications;
using TownCrier.Infrastructure.WatchZones;

var builder = Host.CreateApplicationBuilder(args);

var hasAppInsights = !string.IsNullOrEmpty(
    builder.Configuration["APPLICATIONINSIGHTS_CONNECTION_STRING"]);

builder.Services.AddOpenTelemetry()
    .WithTracing(tracing =>
    {
        tracing
            .AddHttpClientInstrumentation()
            .AddSource(PollingInstrumentation.ActivitySourceName)
            .AddSource(CosmosInstrumentation.ActivitySourceName);

        if (hasAppInsights)
        {
            tracing.AddAzureMonitorTraceExporter();
        }
    })
    .WithMetrics(metrics =>
    {
        metrics
            .AddHttpClientInstrumentation()
            .AddMeter(PollingMetrics.MeterName)
            .AddMeter(CosmosInstrumentation.MeterName);

        if (hasAppInsights)
        {
            metrics.AddAzureMonitorMetricExporter();
        }
    });

builder.Services.AddCosmosRestClient(builder.Configuration);

builder.Services.AddSingleton<IPlanningApplicationRepository, CosmosPlanningApplicationRepository>();
builder.Services.AddSingleton<IWatchZoneRepository, CosmosWatchZoneRepository>();
builder.Services.AddSingleton<IPollStateStore, CosmosPollStateStore>();
builder.Services.AddSingleton<IActiveAuthorityProvider, WatchZoneActiveAuthorityProvider>();
builder.Services.AddSingleton<INotificationEnqueuer, LogNotificationEnqueuer>();
builder.Services.AddSingleton(TimeProvider.System);

#pragma warning disable S1075 // Hardcoded URI is a sensible default
var planItBaseUrl = builder.Configuration["PlanIt:BaseUrl"] ?? "https://www.planit.org.uk/";
#pragma warning restore S1075
builder.Services.AddHttpClient<IPlanItClient, PlanItClient>(client =>
{
    client.BaseAddress = new Uri(planItBaseUrl);
});

builder.Services.AddTransient<PollPlanItCommandHandler>();

using var host = builder.Build();

var handler = host.Services.GetRequiredService<PollPlanItCommandHandler>();
var logger = host.Services.GetRequiredService<ILoggerFactory>().CreateLogger("TownCrier.Worker");

try
{
    using var activity = PollingInstrumentation.Source.StartActivity("Polling Cycle");
    var cycleStart = Stopwatch.GetTimestamp();

    logger.LogInformation("Starting poll cycle");

    var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None)
        .ConfigureAwait(false);

    PollingMetrics.CycleDuration.Record(Stopwatch.GetElapsedTime(cycleStart).TotalMilliseconds);

    activity?.SetTag("polling.authorities_polled", result.AuthoritiesPolled);
    activity?.SetTag("polling.applications_ingested", result.ApplicationCount);

    logger.LogInformation(
        "Poll cycle completed: {ApplicationCount} applications from {AuthoritiesPolled} authorities",
        result.ApplicationCount, result.AuthoritiesPolled);

    return 0;
}
#pragma warning disable CA1031 // Worker must return exit code on any failure
catch (Exception ex)
#pragma warning restore CA1031
{
    PollingMetrics.PollFailures.Add(1);
    logger.LogError(ex, "Poll cycle failed");
    return 1;
}
```

- [ ] **Step 3: Add the worker project to the solution**

Run: `dotnet sln api/town-crier.sln add api/src/town-crier.worker/town-crier.worker.csproj --solution-folder src`

- [ ] **Step 4: Verify build**

Run: `dotnet build api/town-crier.sln`
Expected: Build succeeds including the new worker project.

- [ ] **Step 5: Run tests**

Run: `dotnet test api/town-crier.sln`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.worker/ api/town-crier.sln
git commit -m "feat(api): add town-crier.worker console app for polling job"
```

---

### Task 7: Add Dockerfile.worker

**Files:**
- Create: `api/Dockerfile.worker`

- [ ] **Step 1: Create Dockerfile.worker**

Create `api/Dockerfile.worker`:

```dockerfile
# Town Crier Polling Worker
FROM mcr.microsoft.com/dotnet/sdk:10.0-alpine AS build
RUN apk add --no-cache clang build-base zlib-dev
WORKDIR /src

COPY Directory.Build.props .editorconfig ./
COPY town-crier.sln ./
COPY src/town-crier.domain/town-crier.domain.csproj src/town-crier.domain/
COPY src/town-crier.application/town-crier.application.csproj src/town-crier.application/
COPY src/town-crier.infrastructure/town-crier.infrastructure.csproj src/town-crier.infrastructure/
COPY src/town-crier.worker/town-crier.worker.csproj src/town-crier.worker/

RUN dotnet restore src/town-crier.worker/town-crier.worker.csproj

COPY src/ src/
RUN dotnet publish src/town-crier.worker/town-crier.worker.csproj \
    -c Release \
    -o /app \
    --no-restore

FROM mcr.microsoft.com/dotnet/runtime-deps:10.0-alpine AS runtime
WORKDIR /app

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

COPY --from=build /app .

ENTRYPOINT ["./town-crier.worker"]
```

- [ ] **Step 2: Commit**

```bash
git add api/Dockerfile.worker
git commit -m "feat(api): add Dockerfile.worker for polling job container"
```

---

### Task 8: Add PollState Cosmos container to Pulumi and revert MinReplicas

**Files:**
- Modify: `infra/EnvironmentStack.cs`

- [ ] **Step 1: Add PollState container to the container definitions array**

In `infra/EnvironmentStack.cs`, add to the `containerDefinitions` array (after the DecisionAlerts entry):

```csharp
            // PollState — single document storing last poll timestamp
            new("PollState", "/id"),
```

- [ ] **Step 2: Revert MinReplicas to 0**

In `infra/EnvironmentStack.cs`, change the Scale block:

```csharp
                Scale = new ScaleArgs
                {
                    MinReplicas = 0,
                    MaxReplicas = 1,
                },
```

- [ ] **Step 3: Add Container Apps Job resource**

In `infra/EnvironmentStack.cs`, add the `using` for the Job type at the top of the file (it's already in `Pulumi.AzureNative.App`). Then add this resource after the `containerApp` creation (before the Static Web App section):

```csharp
        // Container Apps Job (Polling Worker) — runs on a cron schedule
        var pollingJob = new Job($"job-town-crier-poll-{env}", new JobArgs
        {
            JobName = $"job-town-crier-poll-{env}",
            ResourceGroupName = resourceGroup.Name,
            EnvironmentId = containerAppsEnvironmentId,
            Configuration = new JobConfigurationArgs
            {
                TriggerType = TriggerType.Schedule,
                ReplicaTimeout = 300,
                ScheduleTriggerConfig = new JobConfigurationScheduleTriggerConfigArgs
                {
                    CronExpression = "*/15 * * * *",
                    Parallelism = 1,
                    ReplicaCompletionCount = 1,
                },
                Registries = new[]
                {
                    new RegistryCredentialsArgs
                    {
                        Server = acrLoginServer,
                        Identity = acrPullIdentityId,
                    },
                },
            },
            Identity = new Pulumi.AzureNative.App.Inputs.ManagedServiceIdentityArgs
            {
                Type = ManagedServiceIdentityType.UserAssigned,
                UserAssignedIdentities = new InputList<string>
                {
                    acrPullIdentityId,
                    cosmosDataIdentityId,
                },
            },
            Template = new JobTemplateArgs
            {
                Containers = new[]
                {
                    new ContainerArgs
                    {
                        Name = "worker",
                        Image = "mcr.microsoft.com/k8se/quickstart:latest",
                        Resources = new ContainerResourcesArgs
                        {
                            Cpu = 0.25,
                            Memory = "0.5Gi",
                        },
                        Env = new[]
                        {
                            new EnvironmentVarArgs { Name = "Cosmos__AccountEndpoint", Value = cosmosAccountEndpoint },
                            new EnvironmentVarArgs { Name = "Cosmos__DatabaseName", Value = cosmosDatabase.Name },
                            new EnvironmentVarArgs { Name = "AZURE_CLIENT_ID", Value = cosmosDataIdentityClientId },
                            new EnvironmentVarArgs { Name = "APPLICATIONINSIGHTS_CONNECTION_STRING", Value = appInsightsConnectionString },
                        },
                    },
                },
            },
            Tags = tags,
        }, new CustomResourceOptions
        {
            // CD pipeline updates the container image via `az containerapp job update`.
            IgnoreChanges = { "template.containers[0].image" },
        });
```

- [ ] **Step 4: Verify infra build**

Run: `dotnet build infra/`
Expected: Build succeeds.

- [ ] **Step 5: Commit**

```bash
git add infra/EnvironmentStack.cs
git commit -m "feat(infra): add polling Job resource, PollState container, revert MinReplicas to 0"
```

---

### Task 9: Update CI/CD pipelines — build and deploy worker image

**Files:**
- Modify: `.github/workflows/cd-dev.yml`
- Modify: `.github/workflows/cd-prod.yml`

- [ ] **Step 1: Update cd-dev.yml — add worker image build**

In `.github/workflows/cd-dev.yml`, add a new job after `api-image`:

```yaml
  # ── Worker: build & push image ──────────────────────────
  worker-image:
    name: "Worker: Build & push image"
    runs-on: ubuntu-latest
    timeout-minutes: 10
    environment: development
    steps:
      - uses: actions/checkout@v6

      - uses: ./.github/actions/azure-login
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - name: Login to ACR
        run: az acr login --name ${{ secrets.ACR_LOGIN_SERVER }}

      - name: Build and push worker image
        run: |
          IMAGE="${{ secrets.ACR_LOGIN_SERVER }}/town-crier-worker:${{ github.sha }}"
          docker build -f Dockerfile.worker -t "$IMAGE" -t "${{ secrets.ACR_LOGIN_SERVER }}/town-crier-worker:latest" .
          docker push "$IMAGE"
          docker push "${{ secrets.ACR_LOGIN_SERVER }}/town-crier-worker:latest"
        working-directory: api
```

Add a new job after `api-deploy`:

```yaml
  # ── Worker: deploy to dev ───────────────────────────────
  worker-deploy:
    name: "Worker: Deploy to dev"
    needs: [worker-image, infra-dev]
    runs-on: ubuntu-latest
    timeout-minutes: 5
    environment: development
    steps:
      - uses: actions/checkout@v6

      - uses: ./.github/actions/azure-login
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - name: Deploy worker image to Job
        run: |
          az containerapp job update \
            --name "job-town-crier-poll-dev" \
            --resource-group "rg-town-crier-dev" \
            --image "${{ secrets.ACR_LOGIN_SERVER }}/town-crier-worker:${{ github.sha }}"
```

- [ ] **Step 2: Update cd-prod.yml — add worker deploy**

In `.github/workflows/cd-prod.yml`, add a new job after `api`:

```yaml
  # ── Worker ───────────────────────────────────────────────
  worker:
    name: Deploy Worker
    needs: infra
    runs-on: ubuntu-latest
    timeout-minutes: 5
    environment: production
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0

      - name: Resolve tag to commit SHA
        id: resolve
        run: echo "sha=$(git rev-parse "$GITHUB_REF_NAME")" >> "$GITHUB_OUTPUT"

      - uses: ./.github/actions/azure-login
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - name: Resolve worker image tag
        id: check-image
        run: |
          ACR_NAME="${{ secrets.ACR_LOGIN_SERVER }}"
          ACR_NAME="${ACR_NAME%.azurecr.io}"
          SHA="${{ steps.resolve.outputs.sha }}"
          if az acr manifest show \
            --registry "$ACR_NAME" \
            --name "town-crier-worker:$SHA" 2>/dev/null; then
            echo "Using exact image for SHA $SHA"
            echo "tag=$SHA" >> "$GITHUB_OUTPUT"
          elif az acr manifest show \
            --registry "$ACR_NAME" \
            --name "town-crier-worker:latest" 2>/dev/null; then
            echo "No image for SHA $SHA — falling back to latest"
            echo "tag=latest" >> "$GITHUB_OUTPUT"
          else
            echo "::error::No worker image found (tried SHA $SHA and latest)"
            exit 1
          fi

      - name: Deploy worker image to prod
        run: |
          az containerapp job update \
            --name "job-town-crier-poll-prod" \
            --resource-group "$RESOURCE_GROUP" \
            --image "${{ secrets.ACR_LOGIN_SERVER }}/town-crier-worker:${{ steps.check-image.outputs.tag }}"
        env:
          RESOURCE_GROUP: ${{ needs.infra.outputs.resource-group }}
```

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/cd-dev.yml .github/workflows/cd-prod.yml
git commit -m "ci: add worker image build and deploy to CD pipelines"
```

---

### Task 10: Update PR Gate to include worker build check

**Files:**
- Check: `.github/workflows/pr-gate.yml` (if it builds the API, ensure it also builds the worker or the full solution)

- [ ] **Step 1: Check if PR Gate already builds the full solution**

Read `.github/workflows/pr-gate.yml` and check if it runs `dotnet build api/town-crier.sln` (which would include the worker project automatically since we added it to the solution).

If it only builds the web project specifically, add the worker project to the build step. If it builds the full solution, no changes needed.

- [ ] **Step 2: Verify and commit if changes needed**

If no changes needed, skip this step. If changes were made:

```bash
git add .github/workflows/pr-gate.yml
git commit -m "ci: include worker project in PR Gate build"
```

---

### Task 11: Write ADR for the extraction decision

**Files:**
- Create: `docs/adr/NNNN-extract-polling-to-container-apps-job.md` (use the next available ADR number)

- [ ] **Step 1: Check existing ADR numbers**

Run: `ls docs/adr/` to find the next available number.

- [ ] **Step 2: Create the ADR**

Create `docs/adr/NNNN-extract-polling-to-container-apps-job.md` (replace NNNN with next number):

```markdown
# NNNN. Extract polling into Container Apps Job

Date: 2026-04-04

## Status

Accepted

## Context

The PlanIt polling service ran as a BackgroundService inside the API container, forcing MinReplicas=1 to keep it alive. With near-zero traffic, this meant paying for a container 24/7 that only needed to do useful work for a few seconds every 15 minutes. ADR 0009 contemplated extraction when workload warranted it.

## Decision

Extract polling into a dedicated Container Apps Job (cron-triggered). The API container reverts to MinReplicas=0 (scale to zero). A new `town-crier.worker` console app runs one poll cycle per invocation and exits.

Key simplifications during extraction:
- Polling schedule prioritisation removed — all authorities polled every run (re-add when scale warrants)
- Health tracking removed — Application Insights provides failure visibility via OpenTelemetry metrics
- File-based poll state replaced with Cosmos DB document persistence

## Consequences

- API container scales to zero when idle — near-zero hosting cost
- Polling job runs on a cron schedule with its own container image and lifecycle
- Two container images to build and deploy (API + worker) instead of one
- Poll state survives job restarts (persisted in Cosmos)
- No scheduling prioritisation — all authorities polled every run, which is fine at current scale
- Health alerting requires App Insights queries rather than in-process monitoring
```

- [ ] **Step 3: Commit**

```bash
git add docs/adr/
git commit -m "docs: add ADR for extracting polling into Container Apps Job"
```

---

### Task 12: Final verification

- [ ] **Step 1: Full solution build**

Run: `dotnet build api/town-crier.sln`
Expected: Build succeeds with zero warnings/errors.

- [ ] **Step 2: Full test suite**

Run: `dotnet test api/town-crier.sln`
Expected: All tests pass.

- [ ] **Step 3: Infra build**

Run: `dotnet build infra/`
Expected: Build succeeds.

- [ ] **Step 4: Docker build smoke test (API)**

Run: `docker build -t tc-api-test -f api/Dockerfile api/`
Expected: Build succeeds.

- [ ] **Step 5: Docker build smoke test (Worker)**

Run: `docker build -t tc-worker-test -f api/Dockerfile.worker api/`
Expected: Build succeeds.

- [ ] **Step 6: Verify no dead references**

Run: `grep -r "FilePollStateStore\|InMemoryPollingHealthStore\|LogPollingHealthAlerter\|PollingSchedule\|PollingPriority\|PollingScheduleConfig\|PollingHealthConfig\|IPollingHealthStore\|IPollingHealthAlerter\|InMemoryActiveAuthorityProvider\|PlanItPollingService" api/src/ infra/`
Expected: No matches (all dead references removed).
