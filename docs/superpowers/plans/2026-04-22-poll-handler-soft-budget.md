# Poll Handler Soft Budget Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Self-bound `PollPlanItCommandHandler` to a 240 s soft budget so poll-sb runs never exceed the 5-min Service Bus lock, and drop `PlanItClient`'s internal 429 retry so lock budget is not wasted on client-side backoff.

**Architecture:** Add `HandlerBudget` (nullable) to `PollingOptions`. Handler records a deadline at start and checks it at two cooperative checkpoints — between authorities, and between PlanIt pages within an authority. In-flight HTTP / Cosmos work is never interrupted (the soft budget is polled, not enforced via a CT). The page-boundary check reuses the existing `capHit` cursor-save path. `PlanItClient.IsRetryable` drops `HttpStatusCode.TooManyRequests` so 429 throws `PlanItRateLimitException` on the first response.

**Tech Stack:** .NET 10, TUnit, Native AOT-compatible polling stack. Config via `POLLING_HANDLER_BUDGET_SECONDS` (int, default 240, `0` disables).

**Spec:** `docs/specs/poll-handler-soft-budget.md`

---

## Files

Modified:

- `api/src/town-crier.application/Polling/PollingOptions.cs` — add `HandlerBudget`.
- `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs` — add deadline + two checkpoints.
- `api/src/town-crier.infrastructure/PlanIt/PlanItClient.cs` — drop 429 from `IsRetryable`.
- `api/src/town-crier.worker/Program.cs` — read `POLLING_HANDLER_BUDGET_SECONDS`, wire into `PollingOptions` for both `poll-sb` and `poll` branches.
- `api/tests/town-crier.application.tests/Polling/FakeTimeProvider.cs` — add `Advance(TimeSpan)`.
- `api/tests/town-crier.application.tests/Polling/FakePlanItClient.cs` — add `OnFetchComplete` hook.
- `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs` — add four tests.
- `api/tests/town-crier.application.tests/Polling/PollTriggerOrchestratorTests.cs` — add one test.
- `api/tests/town-crier.infrastructure.tests/PlanIt/PlanItClientTests.cs` — replace the "retry 429" test with a "no retry on 429" test.

No new files.

---

## Task 1: Add `HandlerBudget` to `PollingOptions`

**Files:**
- Modify: `api/src/town-crier.application/Polling/PollingOptions.cs`

- [ ] **Step 1: Add the property**

Replace the class body with:

```csharp
namespace TownCrier.Application.Polling;

/// <summary>
/// Runtime configuration for the PlanIt polling cycle. Bound from the
/// <c>Polling</c> configuration section in the worker host.
/// </summary>
public sealed record PollingOptions
{
    /// <summary>
    /// Gets the maximum number of PlanIt pages fetched per authority in a single
    /// poll cycle. <c>null</c> means unbounded (use natural end-of-data exit);
    /// any positive value voluntarily bails pagination so a backlogged authority
    /// cannot monopolise the cycle's rate budget before rotation advances.
    /// See <c>bd tc-l77h</c> for the seed-poll rationale.
    /// </summary>
    public int? MaxPagesPerAuthorityPerCycle { get; init; }

    /// <summary>
    /// Gets the TTL requested when the handler acquires the polling lease. Should
    /// cover a full cycle (default 10 minutes to match the worker's replicaTimeout).
    /// </summary>
    public TimeSpan LeaseTtl { get; init; } = TimeSpan.FromMinutes(10);

    /// <summary>
    /// Gets the soft wall-clock budget for a single handler invocation. When set,
    /// the handler checks at authority and page boundaries whether the deadline
    /// has elapsed and exits cleanly with <see cref="PollTerminationReason.TimeBounded"/>,
    /// saving a resumable cursor if mid-pagination. <c>null</c> disables the budget
    /// (handler only honours the outer CancellationToken). Sized to leave the
    /// orchestrator 60 s to publish-next + complete inside the 5-min Service Bus
    /// message lock. See <c>docs/specs/poll-handler-soft-budget.md</c>.
    /// </summary>
    public TimeSpan? HandlerBudget { get; init; }
}
```

- [ ] **Step 2: Build**

Run: `dotnet build api/town-crier.sln`
Expected: succeeds, no warnings.

- [ ] **Step 3: Commit**

```bash
git add api/src/town-crier.application/Polling/PollingOptions.cs
git commit -m "feat(polling): add HandlerBudget option"
```

---

## Task 2: Extend test infrastructure — advancing `FakeTimeProvider` and page-fetch callback

**Files:**
- Modify: `api/tests/town-crier.application.tests/Polling/FakeTimeProvider.cs`
- Modify: `api/tests/town-crier.application.tests/Polling/FakePlanItClient.cs`

- [ ] **Step 1: Make `FakeTimeProvider` advanceable**

Replace the file content with:

```csharp
namespace TownCrier.Application.Tests.Polling;

internal sealed class FakeTimeProvider : TimeProvider
{
    private DateTimeOffset utcNow;

    public FakeTimeProvider(DateTimeOffset utcNow)
    {
        this.utcNow = utcNow;
    }

    public override DateTimeOffset GetUtcNow() => this.utcNow;

    public void Advance(TimeSpan delta)
    {
        this.utcNow = this.utcNow + delta;
    }
}
```

- [ ] **Step 2: Add `OnFetchComplete` hook to `FakePlanItClient`**

At the top of the class (after `LastAuthorityId`):

```csharp
/// <summary>
/// Invoked after each page fetch has assembled its result but before
/// <see cref="FetchApplicationsPageAsync"/> returns. Lets tests advance a
/// fake time provider to simulate wall-clock progression across pages.
/// </summary>
public Action<int, int>? OnFetchComplete { get; set; }
```

And at the end of `FetchApplicationsPageAsync`, just before every `return` statement, call the hook. Replace the final return and the rule-return with these blocks:

For the rule path (around the existing `return new FetchPageResult(page, trimmed, totalForRule, HasMorePages: true);`):

```csharp
            var totalForRule = this.TotalOverride ?? allApps.Count;
            var ruleResult = new FetchPageResult(page, trimmed, totalForRule, HasMorePages: true);
            this.OnFetchComplete?.Invoke(authorityId, page);
            return ruleResult;
        }
```

For the normal path (around the final `return new FetchPageResult(page, pageItems, total, hasMorePages);`):

```csharp
        var hasMorePages = pageItems.Count >= PageSize;
        var total = this.TotalOverride ?? allApps.Count;
        var result = new FetchPageResult(page, pageItems, total, hasMorePages);
        this.OnFetchComplete?.Invoke(authorityId, page);
        return result;
```

- [ ] **Step 3: Build tests project**

Run: `dotnet build api/tests/town-crier.application.tests/town-crier.application.tests.csproj`
Expected: succeeds, no warnings.

- [ ] **Step 4: Run the full handler test suite**

Run: `dotnet test api/tests/town-crier.application.tests/town-crier.application.tests.csproj --filter "FullyQualifiedName~PollPlanItCommandHandler"`
Expected: all existing tests pass (infrastructure change must be non-breaking).

- [ ] **Step 5: Commit**

```bash
git add api/tests/town-crier.application.tests/Polling/FakeTimeProvider.cs api/tests/town-crier.application.tests/Polling/FakePlanItClient.cs
git commit -m "test(polling): make FakeTimeProvider advanceable and hook page fetch"
```

---

## Task 3: Authority-boundary budget check

**Files:**
- Test: `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs`
- Modify: `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs`

- [ ] **Step 1: Write the failing test**

Append to `PollPlanItCommandHandlerTests` (before `CreateHandler`):

```csharp
[Test]
public async Task Should_StopBetweenAuthorities_When_HandlerBudgetExhausted()
{
    var start = new DateTimeOffset(2026, 4, 22, 8, 0, 0, TimeSpan.Zero);
    var timeProvider = new FakeTimeProvider(start);

    var authorityProvider = new FakeActiveAuthorityProvider();
    authorityProvider.Add(100);
    authorityProvider.Add(200);

    var planItClient = new FakePlanItClient
    {
        // Advance 5 minutes on the way out of authority 100's single page.
        OnFetchComplete = (_, _) => timeProvider.Advance(TimeSpan.FromMinutes(5)),
    };
    planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
    planItClient.Add(200, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(200).Build());

    var options = new PollingOptions { HandlerBudget = TimeSpan.FromMinutes(4) };

    var handler = CreateHandler(
        planItClient: planItClient,
        authorityProvider: authorityProvider,
        timeProvider: timeProvider,
        options: options);

    var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

    await Assert.That(result.TerminationReason).IsEqualTo(PollTerminationReason.TimeBounded);
    await Assert.That(result.AuthoritiesPolled).IsEqualTo(1);
    await Assert.That(planItClient.AuthorityIdsRequested).DoesNotContain(200);
}
```

- [ ] **Step 2: Run test — expect RED**

Run: `dotnet test api/tests/town-crier.application.tests/town-crier.application.tests.csproj --filter "Should_StopBetweenAuthorities_When_HandlerBudgetExhausted"`
Expected: FAIL — `result.TerminationReason` is `Natural`, `AuthoritiesPolled` is 2.

- [ ] **Step 3: Implement the checkpoint**

In `PollPlanItCommandHandler.HandleUnderLeaseAsync`, just after the method's opening `var now = ...;` line, add:

```csharp
        var now = this.timeProvider.GetUtcNow();
        var cycleType = this.cycleSelector.GetCurrent();
        var deadline = this.options.HandlerBudget is { } budget
            ? (DateTimeOffset?)(now + budget)
            : null;
```

Then add a local function at the top of the method body (just below the variable declarations):

```csharp
        bool BudgetExhausted() => deadline.HasValue && this.timeProvider.GetUtcNow() >= deadline.Value;
```

Replace the existing authority-loop cancellation check:

```csharp
            if (ct.IsCancellationRequested)
            {
                timeBounded = true;
                break;
            }
```

with:

```csharp
            if (ct.IsCancellationRequested || BudgetExhausted())
            {
                timeBounded = true;
                break;
            }
```

- [ ] **Step 4: Run test — expect GREEN**

Run: `dotnet test api/tests/town-crier.application.tests/town-crier.application.tests.csproj --filter "Should_StopBetweenAuthorities_When_HandlerBudgetExhausted"`
Expected: PASS.

- [ ] **Step 5: Run the whole handler suite**

Run: `dotnet test api/tests/town-crier.application.tests/town-crier.application.tests.csproj --filter "FullyQualifiedName~PollPlanItCommandHandler"`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs
git commit -m "feat(polling): enforce handler budget between authorities"
```

---

## Task 4: Page-boundary budget check with cursor save

**Files:**
- Test: `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs`
- Modify: `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs`

- [ ] **Step 1: Write the failing test**

Append to `PollPlanItCommandHandlerTests` (before `CreateHandler`):

```csharp
[Test]
public async Task Should_StopMidPaginationAndSaveCursor_When_HandlerBudgetExhausted()
{
    var start = new DateTimeOffset(2026, 4, 22, 8, 0, 0, TimeSpan.Zero);
    var timeProvider = new FakeTimeProvider(start);

    var authorityProvider = new FakeActiveAuthorityProvider();
    authorityProvider.Add(100);

    var planItClient = new FakePlanItClient();
    // Seed enough apps for 3 full pages (HasMorePages=true after pages 1, 2).
    for (var i = 0; i < FakePlanItClient.PageSize * 2 + 1; i++)
    {
        planItClient.Add(
            100,
            new PlanningApplicationBuilder().WithUid($"app-{i}").WithAreaId(100).Build());
    }

    // After page 1 completes, jump past the 4-minute budget.
    planItClient.OnFetchComplete = (_, page) =>
    {
        if (page == 1)
        {
            timeProvider.Advance(TimeSpan.FromMinutes(5));
        }
    };

    var pollStateStore = new FakePollStateStore();
    var options = new PollingOptions
    {
        HandlerBudget = TimeSpan.FromMinutes(4),
        MaxPagesPerAuthorityPerCycle = 10,
    };

    var handler = CreateHandler(
        planItClient: planItClient,
        pollStateStore: pollStateStore,
        authorityProvider: authorityProvider,
        timeProvider: timeProvider,
        options: options);

    var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

    await Assert.That(result.TerminationReason).IsEqualTo(PollTerminationReason.TimeBounded);
    await Assert.That(result.ApplicationCount).IsEqualTo(FakePlanItClient.PageSize);
    await Assert.That(planItClient.PagesRequested).HasCount().EqualTo(1);

    var cursor = pollStateStore.GetCursorFor(100);
    await Assert.That(cursor).IsNotNull();
    await Assert.That(cursor!.NextPage).IsEqualTo(2);
}
```

> **Note:** `FakePollStateStore.GetCursorFor` is used by existing tests (search the file to confirm); if not present, use `pollStateStore.LastSavedCursor` / the equivalent accessor already exercised by `Should_SaveCursor_When_PaginationCapHit`. Match the pattern used by that existing test.

- [ ] **Step 2: Run test — expect RED**

Run: `dotnet test api/tests/town-crier.application.tests/town-crier.application.tests.csproj --filter "Should_StopMidPaginationAndSaveCursor_When_HandlerBudgetExhausted"`
Expected: FAIL — cursor is null or `NextPage` is not 2 (handler currently does not break at the page boundary).

- [ ] **Step 3: Implement the checkpoint**

In `PollPlanItCommandHandler.HandleUnderLeaseAsync`, inside the `while (true)` page loop, just after the existing `maxPages` check block. Locate:

```csharp
                    if (maxPages.HasValue && pagesFetched >= maxPages.Value)
                    {
                        capHit = true;
                        break;
                    }

                    page++;
```

Replace with:

```csharp
                    if (maxPages.HasValue && pagesFetched >= maxPages.Value)
                    {
                        capHit = true;
                        break;
                    }

                    if (ct.IsCancellationRequested || BudgetExhausted())
                    {
                        // Mid-pagination budget exhaustion — reuse the capHit cursor-save
                        // path so the next cycle resumes at lastPageFetched + 1, and flag
                        // the outer termination as TimeBounded.
                        capHit = true;
                        timeBounded = true;
                        break;
                    }

                    page++;
```

- [ ] **Step 4: Run test — expect GREEN**

Run: `dotnet test api/tests/town-crier.application.tests/town-crier.application.tests.csproj --filter "Should_StopMidPaginationAndSaveCursor_When_HandlerBudgetExhausted"`
Expected: PASS.

- [ ] **Step 5: Run the full application test project**

Run: `dotnet test api/tests/town-crier.application.tests/town-crier.application.tests.csproj`
Expected: all pass (regression guard).

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs
git commit -m "feat(polling): save cursor and bail mid-pagination when budget elapsed"
```

---

## Task 5: Drop 429 from `PlanItClient` retry

**Files:**
- Test: `api/tests/town-crier.infrastructure.tests/PlanIt/PlanItClientTests.cs`
- Modify: `api/src/town-crier.infrastructure/PlanIt/PlanItClient.cs`

- [ ] **Step 1: Replace the 429-retry test with a no-retry test**

In `PlanItClientTests.cs`, locate the test `Should_RetryOn429WithLongerBackoff_When_RateLimited` (around line 629) and replace it in place with:

```csharp
[Test]
public async Task Should_NotRetryOn429_When_RateLimited()
{
    // Arrange — 429 is expected/handled by the polling handler, not retried
    // internally. Internal retries burn the Service Bus lock budget (see
    // docs/specs/poll-handler-soft-budget.md).
    using var handler = new FakePlanItHandler();
    handler.SetupStatusCodeResponse("page=1", HttpStatusCode.TooManyRequests);
    var delays = new List<TimeSpan>();
    var retryOptions = new PlanItRetryOptions { MaxRetries = 3 };
    var throttleOptions = new PlanItThrottleOptions { DelayBetweenRequestsSeconds = 0 };
    var client = CreateClient(handler, throttleOptions: throttleOptions, retryOptions: retryOptions, delays: delays);

    // Act & Assert — throws immediately, no retry
    await Assert.ThrowsAsync<TownCrier.Application.PlanIt.PlanItRateLimitException>(
        async () => await ConsumeAsync(client, differentStart: null));

    await Assert.That(handler.RequestUrls).HasCount().EqualTo(1);
    var retryDelays = delays.Where(d => d > TimeSpan.Zero).ToList();
    await Assert.That(retryDelays).HasCount().EqualTo(0);
}
```

- [ ] **Step 2: Run test — expect RED**

Run: `dotnet test api/tests/town-crier.infrastructure.tests/town-crier.infrastructure.tests.csproj --filter "Should_NotRetryOn429_When_RateLimited"`
Expected: FAIL — client currently retries the 429 up to `MaxRetries + 1` times and eventually throws. Request count will be 4, not 1.

- [ ] **Step 3: Implement — remove 429 from retryable set**

In `PlanItClient.cs`, locate:

```csharp
    private static bool IsRetryable(HttpStatusCode statusCode)
    {
        return statusCode is
            HttpStatusCode.GatewayTimeout or
            HttpStatusCode.BadGateway or
            HttpStatusCode.ServiceUnavailable or
            HttpStatusCode.RequestTimeout or
            HttpStatusCode.TooManyRequests;
    }
```

Replace with:

```csharp
    private static bool IsRetryable(HttpStatusCode statusCode)
    {
        // 429 is NOT retried here. The polling handler catches
        // PlanItRateLimitException, saves a cursor, and returns RateLimited so
        // the scheduler reschedules via the Retry-After header. Internal
        // retries would burn the Service Bus message lock budget
        // (docs/specs/poll-handler-soft-budget.md).
        return statusCode is
            HttpStatusCode.GatewayTimeout or
            HttpStatusCode.BadGateway or
            HttpStatusCode.ServiceUnavailable or
            HttpStatusCode.RequestTimeout;
    }
```

The existing `ComputeBackoff`'s `RateLimitBackoff` branch is now dead for 429 (because 429 won't enter the retry path), but keep the branch — retry options are still used for 5xx and the branch does no harm. Do not remove `RateLimitBackoffSeconds` from `PlanItRetryOptions`.

- [ ] **Step 4: Run test — expect GREEN**

Run: `dotnet test api/tests/town-crier.infrastructure.tests/town-crier.infrastructure.tests.csproj --filter "Should_NotRetryOn429_When_RateLimited"`
Expected: PASS.

- [ ] **Step 5: Run the full infrastructure test project**

Run: `dotnet test api/tests/town-crier.infrastructure.tests/town-crier.infrastructure.tests.csproj`
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.infrastructure/PlanIt/PlanItClient.cs api/tests/town-crier.infrastructure.tests/PlanIt/PlanItClientTests.cs
git commit -m "fix(planit): do not retry 429 internally — hand off to scheduler"
```

---

## Task 6: Wire `POLLING_HANDLER_BUDGET_SECONDS` in the worker

**Files:**
- Modify: `api/src/town-crier.worker/Program.cs`

- [ ] **Step 1: Update the `pollingOptions` construction**

Locate (around line 178):

```csharp
var pollingOptions = new PollingOptions
{
    // Default 3 pages = 300 apps max per authority per cycle. Bounds the
    // per-authority rate budget so a backlogged authority can't monopolise a
    // seed cycle. Null disables the cap (unbounded pagination). See bd tc-l77h.
    MaxPagesPerAuthorityPerCycle = builder.Configuration.GetValue<int?>("Polling:MaxPagesPerAuthorityPerCycle") ?? 3,
};
```

Replace with:

```csharp
// Default 240 s leaves 60 s headroom inside the 300 s Service Bus message
// lock for the orchestrator's publish-next + complete. Set to 0 to disable
// (diagnostics only). See docs/specs/poll-handler-soft-budget.md.
var handlerBudgetSeconds = builder.Configuration.GetValue<int?>("POLLING_HANDLER_BUDGET_SECONDS") ?? 240;
var pollingOptions = new PollingOptions
{
    // Default 3 pages = 300 apps max per authority per cycle. Bounds the
    // per-authority rate budget so a backlogged authority can't monopolise a
    // seed cycle. Null disables the cap (unbounded pagination). See bd tc-l77h.
    MaxPagesPerAuthorityPerCycle = builder.Configuration.GetValue<int?>("Polling:MaxPagesPerAuthorityPerCycle") ?? 3,
    HandlerBudget = handlerBudgetSeconds > 0 ? TimeSpan.FromSeconds(handlerBudgetSeconds) : null,
};
```

- [ ] **Step 2: Build the worker**

Run: `dotnet build api/src/town-crier.worker/town-crier.worker.csproj`
Expected: succeeds, no warnings.

- [ ] **Step 3: Build and test the whole solution**

Run: `dotnet test api/town-crier.sln`
Expected: all tests pass, solution builds.

- [ ] **Step 4: Commit**

```bash
git add api/src/town-crier.worker/Program.cs
git commit -m "feat(worker): wire POLLING_HANDLER_BUDGET_SECONDS into PollingOptions"
```

---

## Task 7: Orchestrator publishes-next and completes after `TimeBounded`

**Files:**
- Test: `api/tests/town-crier.application.tests/Polling/PollTriggerOrchestratorTests.cs`

- [ ] **Step 1: Read the existing orchestrator test setup**

Open `PollTriggerOrchestratorTests.cs` and locate the `Should_PublishAndComplete_When_PollSucceeds` (or equivalently-named) happy-path test and its helper `CreateOrchestrator`. The new test reuses the same fakes.

- [ ] **Step 2: Write the failing test**

Append this test to `PollTriggerOrchestratorTests` using the same construction pattern as the existing happy-path test:

```csharp
[Test]
public async Task Should_PublishNextAndComplete_When_HandlerReturnsTimeBounded()
{
    var triggerQueue = new FakePollTriggerQueue();
    triggerQueue.SetNextReceivedMessage(new FakePollTriggerMessage());

    var pollHandler = CreateFakeHandlerReturning(new PollPlanItResult(
        ApplicationCount: 5,
        AuthoritiesPolled: 1,
        RateLimited: false,
        TerminationReason: PollTerminationReason.TimeBounded,
        AuthorityErrors: 0));

    var orchestrator = CreateOrchestrator(triggerQueue: triggerQueue, handler: pollHandler);

    var runResult = await orchestrator.RunOnceAsync(CancellationToken.None);

    await Assert.That(runResult.MessageReceived).IsTrue();
    await Assert.That(runResult.PublishedNext).IsTrue();
    await Assert.That(triggerQueue.PublishedMessages).HasCount().EqualTo(1);
    await Assert.That(triggerQueue.CompletedMessages).HasCount().EqualTo(1);
    await Assert.That(triggerQueue.AbandonedMessages).HasCount().EqualTo(0);
}
```

> **Note:** The helper names (`CreateFakeHandlerReturning`, `FakePollTriggerQueue`, `FakePollTriggerMessage`) mirror whatever the existing orchestrator tests use. If the file's pattern is different (e.g., a builder, or a handler implemented inline as a subclass), use that pattern verbatim — do not introduce new fakes. Search the file for how `PollTerminationReason.Natural` is staged today and apply the same pattern with `TimeBounded`.

- [ ] **Step 3: Run test — expect GREEN**

Run: `dotnet test api/tests/town-crier.application.tests/town-crier.application.tests.csproj --filter "Should_PublishNextAndComplete_When_HandlerReturnsTimeBounded"`
Expected: PASS — this test is a regression guard; the orchestrator already publishes-next for any non-`LeaseHeld` outcome (see `PollTriggerOrchestrator.cs:57-78`). If it fails, the orchestrator logic has regressed and must be fixed before merging.

- [ ] **Step 4: Run the full orchestrator test class**

Run: `dotnet test api/tests/town-crier.application.tests/town-crier.application.tests.csproj --filter "FullyQualifiedName~PollTriggerOrchestratorTests"`
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add api/tests/town-crier.application.tests/Polling/PollTriggerOrchestratorTests.cs
git commit -m "test(polling): orchestrator publishes-next and completes on TimeBounded"
```

---

## Task 8: Solution build, format, full test run

**Files:** (no code changes — validation gate)

- [ ] **Step 1: Build**

Run: `dotnet build api/town-crier.sln --no-incremental`
Expected: succeeds, no warnings.

- [ ] **Step 2: Format verify**

Run: `dotnet format api/town-crier.sln --verify-no-changes`
Expected: exit 0. If it fails, run `dotnet format api/town-crier.sln` and commit any changes with `chore: format`.

- [ ] **Step 3: Full test run**

Run: `dotnet test api/town-crier.sln`
Expected: all tests pass.

- [ ] **Step 4: Confirm no uncommitted changes**

Run: `git status`
Expected: working tree clean.

---

## Self-Review Notes

- **Spec coverage:**
  - `PollingOptions.HandlerBudget` — Task 1.
  - Authority-boundary checkpoint — Task 3.
  - Page-boundary checkpoint + cursor — Task 4.
  - `PlanItClient` drop 429 retry — Task 5.
  - `Program.cs` wiring (both modes, env var, `0` escape hatch) — Task 6.
  - Tests per spec's "Tests" section — Tasks 3, 4, 5, 7 (budget null / timer mode regression is implicitly covered by the existing test suite in Task 8, since no existing test sets `HandlerBudget` and all must still pass).

- **Placeholders:** none. Every code block is complete.

- **Type consistency:** `HandlerBudget` is `TimeSpan?`, consistent across spec and tasks. `BudgetExhausted()` is a local helper, not a member. `OnFetchComplete` signature `Action<int, int>` matches the test wiring.

- **Deferred (spec says "out of scope"):** timer-mode removal, safety-net rewrite, `LockDuration` change, per-authority-per-message. No tasks for these.
