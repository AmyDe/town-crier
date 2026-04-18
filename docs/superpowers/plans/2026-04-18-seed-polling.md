# Seed Polling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** On alternate 15-minute poll cycles, sync the full 417-authority UK list by least-recently-polled (seed cycle). The remaining cycles continue syncing only watched authorities (watched cycle). Map/applications views are populated with recent activity for every authority within ~26 hours of deployment.

**Architecture:** A new `ICycleSelector` decides which cycle type is current based on `(utcNow.Minute % 30) < 15`. A new `CycleAlternatingAuthorityProvider` implements the existing `IActiveAuthorityProvider` contract, delegating to one of two marker-interface providers (watched or all-authorities). The poll handler is otherwise unchanged except for adding a `cycle.type` tag to each counter increment, and the worker's `Program.cs` tags the root span identically.

**Tech Stack:** .NET 10 (AOT), TUnit, Microsoft.Extensions.DependencyInjection, System.Diagnostics.Metrics, OpenTelemetry.

**Spec:** [docs/superpowers/specs/2026-04-18-seed-polling-design.md](../specs/2026-04-18-seed-polling-design.md)

---

## File Structure

### New files

| Path | Responsibility |
|------|---------------|
| `api/src/town-crier.application/Polling/CycleType.cs` | Enum: `Watched`, `Seed` |
| `api/src/town-crier.application/Polling/ICycleSelector.cs` | Returns current `CycleType` |
| `api/src/town-crier.application/Polling/MinuteBasedCycleSelector.cs` | `ICycleSelector` impl using `TimeProvider` |
| `api/src/town-crier.application/Polling/IWatchZoneActiveAuthorityProvider.cs` | Marker interface extending `IActiveAuthorityProvider` |
| `api/src/town-crier.application/Polling/IAllAuthorityIdProvider.cs` | Marker interface extending `IActiveAuthorityProvider` |
| `api/src/town-crier.application/Polling/CycleAlternatingAuthorityProvider.cs` | Dispatches to watched or all-authority provider based on `ICycleSelector` |
| `api/src/town-crier.infrastructure/Authorities/AllAuthorityIdProvider.cs` | Adapts `IAuthorityProvider` to return the 417 IDs |
| `api/tests/town-crier.application.tests/Polling/MinuteBasedCycleSelectorTests.cs` | Unit tests for minute-boundary routing |
| `api/tests/town-crier.application.tests/Polling/FakeCycleSelector.cs` | Test double: records calls, returns fixed `CycleType` |
| `api/tests/town-crier.application.tests/Polling/CycleAlternatingAuthorityProviderTests.cs` | Unit tests for delegation logic |
| `api/tests/town-crier.infrastructure.tests/Authorities/AllAuthorityIdProviderTests.cs` | Unit tests for ID projection |

### Modified files

| Path | Change |
|------|--------|
| `api/src/town-crier.application/Polling/WatchZoneActiveAuthorityProvider.cs` | Additionally implements `IWatchZoneActiveAuthorityProvider` |
| `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs` | Accepts `ICycleSelector`; tags every counter emission with `cycle.type` |
| `api/src/town-crier.worker/Program.cs` | Registers selector + new providers; tags root activity with `cycle.type` |
| `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs` | `CreateHandler` helper accepts optional `ICycleSelector` (defaults to `FakeCycleSelector(Watched)`) |
| `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerMetricsTests.cs` | Same helper update + new test asserting `cycle.type` tag present on counter |
| `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTracingTests.cs` | Same helper update |

---

## Task 1: Define `CycleType` enum

**Files:**
- Create: `api/src/town-crier.application/Polling/CycleType.cs`

- [ ] **Step 1: Create the enum**

Write the file:

```csharp
namespace TownCrier.Application.Polling;

public enum CycleType
{
    Watched,
    Seed,
}
```

- [ ] **Step 2: Build to verify it compiles**

Run: `dotnet build api/src/town-crier.application/town-crier.application.csproj`
Expected: Build succeeded. 0 errors.

- [ ] **Step 3: Commit**

```bash
git add api/src/town-crier.application/Polling/CycleType.cs
git commit -m "feat(polling): add CycleType enum"
```

---

## Task 2: Define `ICycleSelector` and `MinuteBasedCycleSelector`

**Files:**
- Create: `api/src/town-crier.application/Polling/ICycleSelector.cs`
- Create: `api/src/town-crier.application/Polling/MinuteBasedCycleSelector.cs`
- Create: `api/tests/town-crier.application.tests/Polling/MinuteBasedCycleSelectorTests.cs`

- [ ] **Step 1: Write the failing test file**

Create `api/tests/town-crier.application.tests/Polling/MinuteBasedCycleSelectorTests.cs`:

```csharp
using Microsoft.Extensions.Time.Testing;
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class MinuteBasedCycleSelectorTests
{
    [Test]
    [Arguments(0, CycleType.Watched)]
    [Arguments(14, CycleType.Watched)]
    [Arguments(15, CycleType.Seed)]
    [Arguments(29, CycleType.Seed)]
    [Arguments(30, CycleType.Watched)]
    [Arguments(44, CycleType.Watched)]
    [Arguments(45, CycleType.Seed)]
    [Arguments(59, CycleType.Seed)]
    public async Task Should_ReturnExpectedCycleType_When_MinuteIsAtBoundary(int minute, CycleType expected)
    {
        // Arrange
        var timeProvider = new FakeTimeProvider(new DateTimeOffset(2026, 4, 18, 12, minute, 0, TimeSpan.Zero));
        var selector = new MinuteBasedCycleSelector(timeProvider);

        // Act
        var result = selector.GetCurrent();

        // Assert
        await Assert.That(result).IsEqualTo(expected);
    }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `dotnet test api/tests/town-crier.application.tests/town-crier.application.tests.csproj --filter "FullyQualifiedName~MinuteBasedCycleSelectorTests"`
Expected: Compilation failure — `ICycleSelector`, `MinuteBasedCycleSelector` don't exist.

- [ ] **Step 3: Create the interface**

Create `api/src/town-crier.application/Polling/ICycleSelector.cs`:

```csharp
namespace TownCrier.Application.Polling;

public interface ICycleSelector
{
    CycleType GetCurrent();
}
```

- [ ] **Step 4: Create the implementation**

Create `api/src/town-crier.application/Polling/MinuteBasedCycleSelector.cs`:

```csharp
namespace TownCrier.Application.Polling;

public sealed class MinuteBasedCycleSelector : ICycleSelector
{
    private readonly TimeProvider timeProvider;

    public MinuteBasedCycleSelector(TimeProvider timeProvider)
    {
        this.timeProvider = timeProvider;
    }

    public CycleType GetCurrent()
    {
        var minute = this.timeProvider.GetUtcNow().Minute;
        return (minute % 30) < 15 ? CycleType.Watched : CycleType.Seed;
    }
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `dotnet test api/tests/town-crier.application.tests/town-crier.application.tests.csproj --filter "FullyQualifiedName~MinuteBasedCycleSelectorTests"`
Expected: All 8 parameterised cases PASS.

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.application/Polling/ICycleSelector.cs \
        api/src/town-crier.application/Polling/MinuteBasedCycleSelector.cs \
        api/tests/town-crier.application.tests/Polling/MinuteBasedCycleSelectorTests.cs
git commit -m "feat(polling): add minute-based cycle selector"
```

---

## Task 3: Create marker interfaces

**Files:**
- Create: `api/src/town-crier.application/Polling/IWatchZoneActiveAuthorityProvider.cs`
- Create: `api/src/town-crier.application/Polling/IAllAuthorityIdProvider.cs`

- [ ] **Step 1: Create the watched-provider marker**

Create `api/src/town-crier.application/Polling/IWatchZoneActiveAuthorityProvider.cs`:

```csharp
namespace TownCrier.Application.Polling;

public interface IWatchZoneActiveAuthorityProvider : IActiveAuthorityProvider
{
}
```

- [ ] **Step 2: Create the all-authorities marker**

Create `api/src/town-crier.application/Polling/IAllAuthorityIdProvider.cs`:

```csharp
namespace TownCrier.Application.Polling;

public interface IAllAuthorityIdProvider : IActiveAuthorityProvider
{
}
```

- [ ] **Step 3: Build to verify**

Run: `dotnet build api/src/town-crier.application/town-crier.application.csproj`
Expected: Build succeeded. 0 errors.

- [ ] **Step 4: Commit**

```bash
git add api/src/town-crier.application/Polling/IWatchZoneActiveAuthorityProvider.cs \
        api/src/town-crier.application/Polling/IAllAuthorityIdProvider.cs
git commit -m "feat(polling): add marker interfaces for cycle providers"
```

---

## Task 4: Update `WatchZoneActiveAuthorityProvider` to implement new marker

**Files:**
- Modify: `api/src/town-crier.application/Polling/WatchZoneActiveAuthorityProvider.cs`

- [ ] **Step 1: Update the class declaration**

Open `api/src/town-crier.application/Polling/WatchZoneActiveAuthorityProvider.cs` and change the class declaration.

From:

```csharp
public sealed class WatchZoneActiveAuthorityProvider : IActiveAuthorityProvider
```

To:

```csharp
public sealed class WatchZoneActiveAuthorityProvider : IWatchZoneActiveAuthorityProvider
```

The body and constructor are unchanged. Because `IWatchZoneActiveAuthorityProvider` extends `IActiveAuthorityProvider`, the existing method contract is satisfied.

- [ ] **Step 2: Build and run existing provider tests**

Run: `dotnet test api/tests/town-crier.application.tests/town-crier.application.tests.csproj --filter "FullyQualifiedName~WatchZoneActiveAuthorityProviderTests"`
Expected: All existing tests PASS (no behavioural change).

- [ ] **Step 3: Commit**

```bash
git add api/src/town-crier.application/Polling/WatchZoneActiveAuthorityProvider.cs
git commit -m "refactor(polling): mark WatchZoneActiveAuthorityProvider with specific interface"
```

---

## Task 5: Create `AllAuthorityIdProvider` with tests

`IAuthorityProvider` already exists (`api/src/town-crier.application/Authorities/IAuthorityProvider.cs`) and `StaticAuthorityProvider` is registered in DI — we read the full authority list from it.

**Files:**
- Create: `api/src/town-crier.infrastructure/Authorities/AllAuthorityIdProvider.cs`
- Create: `api/tests/town-crier.infrastructure.tests/Authorities/AllAuthorityIdProviderTests.cs`

- [ ] **Step 1: Write the failing test**

Create `api/tests/town-crier.infrastructure.tests/Authorities/AllAuthorityIdProviderTests.cs`:

```csharp
using TownCrier.Infrastructure.Authorities;

namespace TownCrier.Infrastructure.Tests.Authorities;

public sealed class AllAuthorityIdProviderTests
{
    [Test]
    public async Task Should_ReturnAllAuthorityIds_When_Queried()
    {
        // Arrange
        var authorityProvider = new StaticAuthorityProvider();
        var all = await authorityProvider.GetAllAsync(CancellationToken.None);
        var provider = new AllAuthorityIdProvider(authorityProvider);

        // Act
        var result = await provider.GetActiveAuthorityIdsAsync(CancellationToken.None);

        // Assert — count matches the embedded authority list
        await Assert.That(result).HasCount().EqualTo(all.Count);
    }

    [Test]
    public async Task Should_ReturnDistinctIds_When_Queried()
    {
        // Arrange
        var authorityProvider = new StaticAuthorityProvider();
        var provider = new AllAuthorityIdProvider(authorityProvider);

        // Act
        var result = await provider.GetActiveAuthorityIdsAsync(CancellationToken.None);

        // Assert
        await Assert.That(result.Distinct().Count()).IsEqualTo(result.Count);
    }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `dotnet test api/tests/town-crier.infrastructure.tests/town-crier.infrastructure.tests.csproj --filter "FullyQualifiedName~AllAuthorityIdProviderTests"`
Expected: Compilation failure — `AllAuthorityIdProvider` doesn't exist.

- [ ] **Step 3: Create the implementation**

Create `api/src/town-crier.infrastructure/Authorities/AllAuthorityIdProvider.cs`:

```csharp
using TownCrier.Application.Authorities;
using TownCrier.Application.Polling;

namespace TownCrier.Infrastructure.Authorities;

public sealed class AllAuthorityIdProvider : IAllAuthorityIdProvider
{
    private readonly IAuthorityProvider authorityProvider;

    public AllAuthorityIdProvider(IAuthorityProvider authorityProvider)
    {
        this.authorityProvider = authorityProvider;
    }

    public async Task<IReadOnlyCollection<int>> GetActiveAuthorityIdsAsync(CancellationToken ct)
    {
        var authorities = await this.authorityProvider.GetAllAsync(ct).ConfigureAwait(false);
        return authorities.Select(a => a.Id).ToList().AsReadOnly();
    }
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `dotnet test api/tests/town-crier.infrastructure.tests/town-crier.infrastructure.tests.csproj --filter "FullyQualifiedName~AllAuthorityIdProviderTests"`
Expected: Both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add api/src/town-crier.infrastructure/Authorities/AllAuthorityIdProvider.cs \
        api/tests/town-crier.infrastructure.tests/Authorities/AllAuthorityIdProviderTests.cs
git commit -m "feat(polling): add all-authority id provider for seed cycles"
```

---

## Task 6: Create `CycleAlternatingAuthorityProvider`

**Files:**
- Create: `api/src/town-crier.application/Polling/CycleAlternatingAuthorityProvider.cs`
- Create: `api/tests/town-crier.application.tests/Polling/FakeCycleSelector.cs`
- Create: `api/tests/town-crier.application.tests/Polling/CycleAlternatingAuthorityProviderTests.cs`

- [ ] **Step 1: Create the test double**

Create `api/tests/town-crier.application.tests/Polling/FakeCycleSelector.cs`:

```csharp
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakeCycleSelector : ICycleSelector
{
    public FakeCycleSelector(CycleType cycleType = CycleType.Watched)
    {
        this.Current = cycleType;
    }

    public CycleType Current { get; set; }

    public int GetCurrentCallCount { get; private set; }

    public CycleType GetCurrent()
    {
        this.GetCurrentCallCount++;
        return this.Current;
    }
}
```

- [ ] **Step 2: Write the failing test file**

Create `api/tests/town-crier.application.tests/Polling/CycleAlternatingAuthorityProviderTests.cs`:

```csharp
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class CycleAlternatingAuthorityProviderTests
{
    [Test]
    public async Task Should_DelegateToWatchedProvider_When_CycleIsWatched()
    {
        // Arrange
        var watched = new FakeWatchZoneActiveAuthorityProvider();
        watched.Add(100);
        watched.Add(200);
        var all = new FakeAllAuthorityIdProvider();
        all.Add(300);
        all.Add(400);
        var selector = new FakeCycleSelector(CycleType.Watched);
        var provider = new CycleAlternatingAuthorityProvider(watched, all, selector);

        // Act
        var result = await provider.GetActiveAuthorityIdsAsync(CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(2);
        await Assert.That(result).Contains(100);
        await Assert.That(result).Contains(200);
    }

    [Test]
    public async Task Should_DelegateToAllProvider_When_CycleIsSeed()
    {
        // Arrange
        var watched = new FakeWatchZoneActiveAuthorityProvider();
        watched.Add(100);
        var all = new FakeAllAuthorityIdProvider();
        all.Add(300);
        all.Add(400);
        all.Add(500);
        var selector = new FakeCycleSelector(CycleType.Seed);
        var provider = new CycleAlternatingAuthorityProvider(watched, all, selector);

        // Act
        var result = await provider.GetActiveAuthorityIdsAsync(CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(3);
        await Assert.That(result).Contains(300);
        await Assert.That(result).Contains(400);
        await Assert.That(result).Contains(500);
    }
}

internal sealed class FakeWatchZoneActiveAuthorityProvider : IWatchZoneActiveAuthorityProvider
{
    private readonly List<int> ids = [];

    public void Add(int id) => this.ids.Add(id);

    public Task<IReadOnlyCollection<int>> GetActiveAuthorityIdsAsync(CancellationToken ct)
        => Task.FromResult<IReadOnlyCollection<int>>(this.ids.AsReadOnly());
}

internal sealed class FakeAllAuthorityIdProvider : IAllAuthorityIdProvider
{
    private readonly List<int> ids = [];

    public void Add(int id) => this.ids.Add(id);

    public Task<IReadOnlyCollection<int>> GetActiveAuthorityIdsAsync(CancellationToken ct)
        => Task.FromResult<IReadOnlyCollection<int>>(this.ids.AsReadOnly());
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `dotnet test api/tests/town-crier.application.tests/town-crier.application.tests.csproj --filter "FullyQualifiedName~CycleAlternatingAuthorityProviderTests"`
Expected: Compilation failure — `CycleAlternatingAuthorityProvider` doesn't exist.

- [ ] **Step 4: Create the implementation**

Create `api/src/town-crier.application/Polling/CycleAlternatingAuthorityProvider.cs`:

```csharp
namespace TownCrier.Application.Polling;

public sealed class CycleAlternatingAuthorityProvider : IActiveAuthorityProvider
{
    private readonly IWatchZoneActiveAuthorityProvider watchZoneProvider;
    private readonly IAllAuthorityIdProvider allAuthorityProvider;
    private readonly ICycleSelector cycleSelector;

    public CycleAlternatingAuthorityProvider(
        IWatchZoneActiveAuthorityProvider watchZoneProvider,
        IAllAuthorityIdProvider allAuthorityProvider,
        ICycleSelector cycleSelector)
    {
        this.watchZoneProvider = watchZoneProvider;
        this.allAuthorityProvider = allAuthorityProvider;
        this.cycleSelector = cycleSelector;
    }

    public Task<IReadOnlyCollection<int>> GetActiveAuthorityIdsAsync(CancellationToken ct)
    {
        return this.cycleSelector.GetCurrent() switch
        {
            CycleType.Seed => this.allAuthorityProvider.GetActiveAuthorityIdsAsync(ct),
            _ => this.watchZoneProvider.GetActiveAuthorityIdsAsync(ct),
        };
    }
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `dotnet test api/tests/town-crier.application.tests/town-crier.application.tests.csproj --filter "FullyQualifiedName~CycleAlternatingAuthorityProviderTests"`
Expected: Both tests PASS.

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.application/Polling/CycleAlternatingAuthorityProvider.cs \
        api/tests/town-crier.application.tests/Polling/FakeCycleSelector.cs \
        api/tests/town-crier.application.tests/Polling/CycleAlternatingAuthorityProviderTests.cs
git commit -m "feat(polling): add cycle-alternating authority provider"
```

---

## Task 7: Add `cycle.type` tag to handler counter emissions

The handler gains an `ICycleSelector` dependency. Every counter emission and the per-authority activity span are tagged with `cycle.type`. Existing test helpers are updated to default the new dependency to `FakeCycleSelector(Watched)` so only new behaviour needs new test code.

**Files:**
- Modify: `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs`
- Modify: `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs` (CreateHandler only)
- Modify: `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerMetricsTests.cs` (CreateHandler + new test)
- Modify: `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTracingTests.cs` (CreateHandler only)

- [ ] **Step 1: Write the failing tag-presence test**

Add this test to the end of `PollPlanItCommandHandlerMetricsTests.cs` (before `private static PollPlanItCommandHandler CreateHandler`):

```csharp
[Test]
public async Task Should_TagAuthoritiesPolled_With_CycleType()
{
    // Arrange
    var authorityProvider = new FakeActiveAuthorityProvider();
    authorityProvider.Add(100);
    var planItClient = new FakePlanItClient();
    planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
    var cycleSelector = new FakeCycleSelector(CycleType.Seed);
    var handler = CreateHandler(
        planItClient: planItClient,
        authorityProvider: authorityProvider,
        cycleSelector: cycleSelector);

    var recordedTags = new List<string?>();
    using var listener = new MeterListener();
    listener.InstrumentPublished = (instrument, listener) =>
    {
        if (instrument.Name == "towncrier.polling.authorities_polled")
        {
            listener.EnableMeasurementEvents(instrument);
        }
    };
    listener.SetMeasurementEventCallback<long>((instrument, measurement, tags, _) =>
    {
        foreach (var tag in tags)
        {
            if (tag.Key == "cycle.type")
            {
                recordedTags.Add(tag.Value?.ToString());
            }
        }
    });
    listener.Start();

    // Act
    await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

    // Assert
    await Assert.That(recordedTags).HasCount().EqualTo(1);
    await Assert.That(recordedTags[0]).IsEqualTo("seed");
}
```

You will also need to import `TownCrier.Application.Polling` at the top of the file if it's not already imported (it is — the existing tests use it).

- [ ] **Step 2: Update `CreateHandler` in MetricsTests to accept an optional `ICycleSelector`**

Replace the existing `CreateHandler` at the bottom of `PollPlanItCommandHandlerMetricsTests.cs` with:

```csharp
private static PollPlanItCommandHandler CreateHandler(
    FakePlanItClient? planItClient = null,
    FakePollStateStore? pollStateStore = null,
    FakePlanningApplicationRepository? repository = null,
    FakeActiveAuthorityProvider? authorityProvider = null,
    FakeWatchZoneRepository? watchZoneRepository = null,
    FakeNotificationEnqueuer? notificationEnqueuer = null,
    TimeProvider? timeProvider = null,
    ICycleSelector? cycleSelector = null)
{
    return new PollPlanItCommandHandler(
        planItClient ?? new FakePlanItClient(),
        pollStateStore ?? new FakePollStateStore(),
        repository ?? new FakePlanningApplicationRepository(),
        timeProvider ?? TimeProvider.System,
        authorityProvider ?? new FakeActiveAuthorityProvider(),
        watchZoneRepository ?? new FakeWatchZoneRepository(),
        notificationEnqueuer ?? new FakeNotificationEnqueuer(),
        cycleSelector ?? new FakeCycleSelector(CycleType.Watched),
        NullLogger<PollPlanItCommandHandler>.Instance);
}
```

- [ ] **Step 3: Mirror the change in `PollPlanItCommandHandlerTests.cs`**

Apply the same signature change to `CreateHandler` at the bottom of `PollPlanItCommandHandlerTests.cs`:

Add the `ICycleSelector? cycleSelector = null` parameter at the end of the parameter list, and add `cycleSelector ?? new FakeCycleSelector(CycleType.Watched),` just before `NullLogger<PollPlanItCommandHandler>.Instance` in the constructor call.

- [ ] **Step 4: Mirror the change in `PollPlanItCommandHandlerTracingTests.cs`**

Apply the same signature change to `CreateHandler` at the bottom of `PollPlanItCommandHandlerTracingTests.cs` — identical to Step 3.

- [ ] **Step 5: Run the new test to verify it fails**

Run: `dotnet test api/tests/town-crier.application.tests/town-crier.application.tests.csproj --filter "FullyQualifiedName~Should_TagAuthoritiesPolled_With_CycleType"`
Expected: Compilation failure (handler constructor doesn't accept `ICycleSelector` yet), or assertion failure once compiled (tag not emitted yet).

- [ ] **Step 6: Update the handler constructor and storage**

Edit `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs`.

Add field and constructor parameter. The constructor signature becomes:

```csharp
private readonly ICycleSelector cycleSelector;

public PollPlanItCommandHandler(
    IPlanItClient planItClient,
    IPollStateStore pollStateStore,
    IPlanningApplicationRepository applicationRepository,
    TimeProvider timeProvider,
    IActiveAuthorityProvider activeAuthorityProvider,
    IWatchZoneRepository watchZoneRepository,
    INotificationEnqueuer notificationEnqueuer,
    ICycleSelector cycleSelector,
    ILogger<PollPlanItCommandHandler> logger)
{
    this.planItClient = planItClient;
    this.pollStateStore = pollStateStore;
    this.applicationRepository = applicationRepository;
    this.timeProvider = timeProvider;
    this.activeAuthorityProvider = activeAuthorityProvider;
    this.watchZoneRepository = watchZoneRepository;
    this.notificationEnqueuer = notificationEnqueuer;
    this.cycleSelector = cycleSelector;
    this.logger = logger;
}
```

- [ ] **Step 7: Resolve the cycle type once at the start of `HandleAsync`**

Add at the very top of `HandleAsync`, just after `var now = this.timeProvider.GetUtcNow();`:

```csharp
var cycleType = this.cycleSelector.GetCurrent();
var cycleTypeValue = cycleType.ToString().ToLowerInvariant();
var cycleTypeTag = new KeyValuePair<string, object?>("cycle.type", cycleTypeValue);
```

- [ ] **Step 8: Tag every counter and histogram emission**

Replace each metric call inside `HandleAsync` with a tagged variant:

| Current | Replace with |
|---------|--------------|
| `PollingMetrics.RateLimited.Add(1);` | `PollingMetrics.RateLimited.Add(1, cycleTypeTag);` |
| `PollingMetrics.AuthorityProcessingDuration.Record(Stopwatch.GetElapsedTime(authorityStart).TotalMilliseconds);` | `PollingMetrics.AuthorityProcessingDuration.Record(Stopwatch.GetElapsedTime(authorityStart).TotalMilliseconds, cycleTypeTag);` |
| `PollingMetrics.AuthoritiesPolled.Add(1);` | `PollingMetrics.AuthoritiesPolled.Add(1, cycleTypeTag);` |
| `PollingMetrics.ApplicationsIngested.Add(authorityAppCount);` | `PollingMetrics.ApplicationsIngested.Add(authorityAppCount, cycleTypeTag);` |
| `PollingMetrics.AuthoritiesSkipped.Add(1);` | `PollingMetrics.AuthoritiesSkipped.Add(1, cycleTypeTag);` |

Also add the tag to the per-authority activity near line 55 where `authorityActivity?.SetTag("polling.authority_code", authorityId);` already exists:

```csharp
authorityActivity?.SetTag("polling.authority_code", authorityId);
authorityActivity?.SetTag("cycle.type", cycleTypeValue);
```

- [ ] **Step 9: Run all handler tests to verify nothing regressed**

Run: `dotnet test api/tests/town-crier.application.tests/town-crier.application.tests.csproj --filter "FullyQualifiedName~PollPlanItCommandHandler"`
Expected: All tests PASS, including the new `Should_TagAuthoritiesPolled_With_CycleType`.

- [ ] **Step 10: Run the full application test project**

Run: `dotnet test api/tests/town-crier.application.tests/town-crier.application.tests.csproj`
Expected: All tests PASS.

- [ ] **Step 11: Commit**

```bash
git add api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs \
        api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs \
        api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerMetricsTests.cs \
        api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTracingTests.cs
git commit -m "feat(polling): tag poll metrics with cycle.type"
```

---

## Task 8: Wire up DI in the worker and tag the root activity

**Files:**
- Modify: `api/src/town-crier.worker/Program.cs`

- [ ] **Step 1: Replace the `IActiveAuthorityProvider` registration with the cycle-alternating setup**

In `api/src/town-crier.worker/Program.cs`, find:

```csharp
builder.Services.AddSingleton<IActiveAuthorityProvider, WatchZoneActiveAuthorityProvider>();
```

Replace with:

```csharp
builder.Services.AddSingleton<IWatchZoneActiveAuthorityProvider, WatchZoneActiveAuthorityProvider>();
builder.Services.AddSingleton<IAllAuthorityIdProvider, AllAuthorityIdProvider>();
builder.Services.AddSingleton<IAuthorityProvider, StaticAuthorityProvider>();
builder.Services.AddSingleton<ICycleSelector, MinuteBasedCycleSelector>();
builder.Services.AddSingleton<IActiveAuthorityProvider, CycleAlternatingAuthorityProvider>();
```

Add any missing `using` statements at the top:

```csharp
using TownCrier.Application.Authorities;
using TownCrier.Infrastructure.Authorities;
```

(`TownCrier.Application.Polling` and `TownCrier.Infrastructure.Polling` are already imported.)

- [ ] **Step 2: Tag the root activity with `cycle.type`**

Find the polling case (starts at `case "poll":`) and locate:

```csharp
using var activity = PollingInstrumentation.Source.StartActivity("Polling Cycle");
try
{
    var cycleStart = Stopwatch.GetTimestamp();
    WorkerLog.PollCycleStarting(logger);
    var pollHandler = host.Services.GetRequiredService<PollPlanItCommandHandler>();
```

Insert just before `WorkerLog.PollCycleStarting(logger);`:

```csharp
var cycleSelector = host.Services.GetRequiredService<ICycleSelector>();
var cycleType = cycleSelector.GetCurrent();
activity?.SetTag("cycle.type", cycleType.ToString().ToLowerInvariant());
```

- [ ] **Step 3: Build the worker**

Run: `dotnet build api/src/town-crier.worker/town-crier.worker.csproj`
Expected: Build succeeded. 0 errors.

- [ ] **Step 4: Run all tests to ensure nothing else regressed**

Run: `dotnet test`
Expected: All tests PASS.

- [ ] **Step 5: Run the formatter**

Run: `dotnet format api/town-crier.sln`
Expected: No changes reported (or trivial whitespace, which is fine).

Then verify:

Run: `dotnet format api/town-crier.sln --verify-no-changes`
Expected: Exit code 0.

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.worker/Program.cs
git commit -m "feat(polling): wire alternating seed/watched cycles in worker"
```

---

## Task 9: Final verification

- [ ] **Step 1: Full solution build**

Run: `dotnet build api/town-crier.sln`
Expected: Build succeeded. 0 errors, 0 warnings.

- [ ] **Step 2: Full test suite**

Run: `dotnet test api/town-crier.sln`
Expected: All tests PASS.

- [ ] **Step 3: Formatting check**

Run: `dotnet format api/town-crier.sln --verify-no-changes`
Expected: Exit code 0.

- [ ] **Step 4: Manual smoke (observability)**

After the next deployment, verify in App Insights that:

```
customMetrics
| where timestamp > ago(1h)
| where name == "towncrier.polling.authorities_polled"
| summarize count() by tostring(customDimensions["cycle.type"])
```

returns rows for both `watched` and `seed`, with seed increments beginning after the first seed cycle fires.

No code commit for this step.
