# Poll Handler Soft Budget

Date: 2026-04-22

## Context

Prod incident 2026-04-22 08:19 UTC: `job-tc-poll-prod-tcb2c` Failed because the handler spent ~6 min on one Service Bus message and the 5-min message lock expired. `Complete` returned 404; the execution exited with non-zero status. Telemetry: `PlanItRateLimitException` at 08:25:54 followed immediately by `ServiceBus Complete failed (404): lock supplied is invalid`. Surrounding Succeeded runs complete in 45–90 s.

Two compounding causes:

1. `PlanItClient.SendWithThrottleAsync` retries 429s three times with 5 / 10 / 20 s exponential backoff — up to 35 s per rate-limited request before the exception bubbles out of the client.
2. The handler's only cancellation check is at the authority boundary (between authorities), using the orchestrator's 570 s replica-timeout token. Nothing caps handler wall-clock to less than the 5-min Service Bus lock.

Service Bus is on the Basic tier (deliberate — low volumes don't justify Standard tier pricing), so `LockDuration` cannot be extended beyond 5 min. The handler must self-bound.

## Decision

Introduce a soft 240 s budget inside `PollPlanItCommandHandler`. When the budget elapses:

- No in-flight PlanIt page fetch, Cosmos upsert, or watch-zone fan-out is interrupted.
- At the next page boundary (or authority boundary), the handler saves a resumable cursor if mid-pagination and returns `PollTerminationReason.TimeBounded`.
- The orchestrator has ~60 s headroom to `PublishAtAsync` + `CompleteAsync` before the 5-min lock expires.

Additionally, drop `PlanItClient`'s internal 429 retry. 5xx retries (1 / 2 / 4 s) stay. On 429 the client throws `PlanItRateLimitException` immediately; the scheduler uses the `Retry-After` header to choose the next run time.

The budget is applied to both modes (`poll-sb` and `poll`) via the same config key. Timer mode's observed worst case is 169 s on outage recovery, well within 240 s — no regression expected. `HandlerBudget` stays nullable on `PollingOptions` so the handler's checkpoint logic has a single bypass for tests and future deployments that want unbounded cycles. Timer mode will be removed in a follow-up bead once the SB-coordinated loop is stable; the safety-net job will then become a bootstrap-only job (probe queue → publish seed if empty), not a full poll.

## Scope

In scope:

- `PollingOptions.HandlerBudget` (new, nullable).
- `PollPlanItCommandHandler` checkpoint logic.
- `PlanItClient` 429 retry removal.
- `Program.cs` wiring in `poll-sb` branch.
- TUnit tests.

Out of scope (separate beads):

- Removing timer mode entirely.
- Rewriting the safety-net job to be bootstrap-only.
- Any change to `LockDuration`, tier, or queue topology.
- Per-authority-per-message refactor (approach B from brainstorming).

## Design

### 1. `PollingOptions`

Add a nullable budget:

```csharp
public sealed class PollingOptions
{
    public int? MaxPagesPerAuthorityPerCycle { get; init; }
    public TimeSpan? HandlerBudget { get; init; }  // null = unbounded
}
```

### 2. `PollPlanItCommandHandler.HandleUnderLeaseAsync`

At the top of the method:

```csharp
var now = this.timeProvider.GetUtcNow();
var deadline = this.options.HandlerBudget is { } budget ? now + budget : (DateTimeOffset?)null;
```

Local helper:

```csharp
bool BudgetExhausted() => deadline.HasValue && this.timeProvider.GetUtcNow() >= deadline.Value;
```

Checkpoint A — authority boundary (extends the existing `ct.IsCancellationRequested` check):

```csharp
foreach (var authorityId in sortedIds)
{
    if (ct.IsCancellationRequested || BudgetExhausted())
    {
        timeBounded = true;
        break;
    }
    // ... existing authority processing ...
}
```

Checkpoint B — page boundary (new, inside `while (true)` after `pagesFetched++` / `lastPageFetched = page`, alongside the `HasMorePages` and `maxPages` checks):

```csharp
pagesFetched++;
lastPageFetched = page;

if (!pageResult.HasMorePages) break;

if (maxPages.HasValue && pagesFetched >= maxPages.Value)
{
    capHit = true;
    break;
}

if (ct.IsCancellationRequested || BudgetExhausted())
{
    capHit = true;        // reuse the cursor-save path below
    timeBounded = true;   // drives the TerminationReason
    break;
}

page++;
```

The existing cursor-save block (`if (capHit || (rateLimited && authorityAppCount > 0)) { save cursor at nextPage }`) is unchanged. The existing termination-reason calculation (prefers `RateLimited` > `TimeBounded` > `Natural`) is unchanged.

### 3. `PlanItClient.SendWithThrottleAsync`

Current: 429 is retryable with exponential backoff (5 / 10 / 20 s).

New: 429 causes an immediate throw of `PlanItRateLimitException` via `EnsureSuccessOrThrow`. Remove `HttpStatusCode.TooManyRequests` from `IsRetryable` so the loop does not retry on 429. 5xx retry behaviour (1 / 2 / 4 s, max 3 attempts) is preserved.

The existing handler catch block (`catch (PlanItRateLimitException ex)`) already saves a cursor and breaks the authority loop with `rateLimited = true` — no handler-side change needed.

### 4. `Program.cs`

```csharp
var handlerBudgetSeconds = builder.Configuration.GetValue<int?>("POLLING_HANDLER_BUDGET_SECONDS") ?? 240;
var pollingOptions = new PollingOptions
{
    MaxPagesPerAuthorityPerCycle = builder.Configuration.GetValue<int?>("Polling:MaxPagesPerAuthorityPerCycle") ?? 3,
    HandlerBudget = handlerBudgetSeconds > 0 ? TimeSpan.FromSeconds(handlerBudgetSeconds) : null,
};
```

Applied to both `poll-sb` and `poll` branches (no conditional wiring). Timer mode's observed worst case (169 s) fits comfortably. A `POLLING_HANDLER_BUDGET_SECONDS=0` escape hatch disables the budget entirely (sets `HandlerBudget = null`) for diagnostics or rollback.

### 5. Budget value

- **240 s handler budget** chosen to give **60 s publish+complete headroom** inside the 300 s SB lock.
- Prod publish + complete typically < 500 ms (single REST calls to Service Bus). 60 s is generous even under transient network retries.
- Observed max normal cycle work: 169 s (outage-recovery seed cycle). Comfortably inside 240 s.

## Behaviour

| Scenario | Handler outcome | Cursor | Orchestrator outcome |
| --- | --- | --- | --- |
| Normal completion under budget | `Natural` | Cleared | Publishes next, completes message |
| Mid-pagination at budget | `TimeBounded` | Saved at `lastPageFetched + 1` | Publishes next (NaturalCadence), completes message |
| Between authorities at budget | `TimeBounded` | No cursor for unstarted authorities | Publishes next, completes message |
| 429 from PlanIt | `RateLimited` | Saved (if any apps upserted) | Publishes next (Retry-After delay), completes message |
| In-flight HTTP runs past budget | Finishes page, then checkpoint fires | Saved at next page | Publishes next, completes message |
| In-flight HTTP runs past `ct` (replica timeout) | `OperationCanceledException` propagates | None | Abandons message, throws; redelivered next trigger |

The soft budget is cooperative; `ct` (replica timeout) remains the hard bound for HTTP/Cosmos safety.

## Tests (TUnit)

Handler:

- Budget exhausted between authorities → `TerminationReason = TimeBounded`, skipped authorities have no cursor, already-polled authorities retain their HWM.
- Budget exhausted mid-pagination → `TerminationReason = TimeBounded`, cursor saved at `lastPageFetched + 1`.
- Budget `null` → unbounded cycle (regression guard for timer mode).
- In-flight page fetch returns after budget elapsed → that page's applications are fully upserted; cursor reflects the completed page, not the interrupted one.
- Handler receives `ct.IsCancellationRequested` during the check → same `TimeBounded` path as budget exhaustion.

`PlanItClient`:

- 429 response → throws `PlanItRateLimitException` immediately, no retry.
- 503 response → retries up to 3 times (existing behaviour, regression guard).
- Replace the existing "429 retries 3×" test if present.

Orchestrator:

- Handler returns `TimeBounded` → orchestrator publishes next trigger, completes message, returns `PublishedNext = true, MessageReceived = true`.

## Consequences

Easier:

- Poll-sb cycles become self-bounded; `Failed` executions from lock-expiry are eliminated.
- No wasted lock budget on internal 429 retries; PlanIt 429 recovery is driven entirely by the scheduler using `Retry-After`.
- Timer mode keeps working (unchanged) while the SB loop stabilises.

Harder:

- The handler has a second cancellation signal (budget clock) alongside `ct`. Mitigated by keeping the budget check in one helper and gating it behind `options.HandlerBudget` being set.
- `TimeBounded` mid-pagination now consumes the same cursor slot as `capHit`. Mutually exclusive with `RateLimited` mid-pagination because the handler exits the while loop on either signal — covered by tests.

Not addressed (intentionally):

- No per-authority-per-message decomposition (approach B). Re-evaluate if seed cycles or authority counts grow such that individual authorities no longer fit within the budget.
- No safety-net rewrite. That happens when timer mode is deleted.
