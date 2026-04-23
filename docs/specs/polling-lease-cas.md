# Polling lease — ETag CAS mutex

Date: 2026-04-23

## Status

Proposed

## Context

Under ADR 0024 and its 2026-04-22 amendment, prod polling is driven by a Service Bus-coordinated chain in receive-and-delete mode. Two actors publish to the `poll` queue:

- **Event-triggered orchestrator** (`job-tc-poll-prod`, `WORKER_MODE=poll-sb`). Destructively consumes one trigger, runs `PollPlanItCommandHandler`, publishes the next-run with a scheduled enqueue time.
- **Cron-triggered bootstrap** (`job-tc-poll-bootstrap-prod`, `*/30 * * * *`, `WORKER_MODE=poll-bootstrap`). Probes queue depth via the ARM management API; if both `activeMessageCount` and `scheduledMessageCount` are zero, publishes a seed.

There is a race window. The orchestrator's destructive receive empties the queue *before* the handler runs. If the bootstrap cron fires during the handler's execution (tens of seconds to minutes), it sees an empty queue and re-seeds — producing a duplicate chain as soon as the orchestrator publishes its next-run.

The existing `CosmosPollingLeaseStore` was intended to mitigate this, but:

1. The handler acquires the lease *after* the destructive receive, leaving a sub-second gap where the queue is empty and no lease is held.
2. The bootstrap never consults the lease at all — its only guard is the queue-depth probe, which is legitimately empty during the handler run.
3. The lease's `TryAcquireAsync` is a read-then-write pair with no precondition — two near-simultaneous acquirers can both return `true`.

The invariants we want to hold are both:

- **(a)** At most one poll cycle running at any instant.
- **(b)** At most one message (active + scheduled) in the `poll` queue at any instant under steady state.

Constraints:

- All Azure service integrations must use the REST API — the Azure Service Bus and Cosmos SDKs use reflection that is incompatible with Native AOT.
- Service Bus Basic tier caps `LockDuration` at 5 minutes, so peek-lock-based mutual exclusion was ruled out in ADR 0024's amendment.
- `poll-sb` runs on an event-triggered ACA Job with `maxExecutions = 1`, so orchestrator-vs-orchestrator contention is impossible by construction. The only real contention is orchestrator-vs-bootstrap.

## Decision

Make the `Leases/polling` Cosmos document a proper ETag-CAS mutex, and have both the orchestrator and the bootstrap acquire the lease before any action that could mutate the `poll` queue.

Serialization is enforced by Cosmos via `If-Match` / `If-None-Match` preconditions. Cosmos returns `412 Precondition Failed` (or `409 Conflict` for creates) when a concurrent writer has bumped the document's ETag; this turns the lease acquire into an atomic compare-and-swap.

Blob lease was considered and rejected: its 15–60 s fixed-duration cap would force a renewal loop inside the handler, which can run for several minutes. Cosmos lets us own the expiry timestamp as a plain field in the document, so no renewal is needed.

## Architecture

### Components and responsibilities

| Component | Change |
|---|---|
| `CosmosPollingLeaseStore` | Upgrade acquire to **ETag CAS via `If-Match`** (and `If-None-Match: *` for first-ever create). Introduce `LeaseHandle` carrying the ETag of the winning write so `ReleaseAsync` can do a conditional delete. |
| `ICosmosRestClient` | Extend `ReadDocumentAsync` to surface the response ETag; add `TryCreateDocumentAsync` (If-None-Match: *), `TryReplaceDocumentAsync` (If-Match), and `TryDeleteDocumentAsync` (optional If-Match). All return explicit outcomes distinguishing `412` / `409` / `404` / success. |
| `PollTriggerOrchestrator` | Acquire lease **before** `ReceiveAsync`; release in `finally`. New ordering: `acquire → receive → handler → publish → release`. Add `LeaseUnavailable` to `PollTriggerOrchestratorRunResult`. |
| `PollTriggerBootstrapper` | Acquire lease **before** the management-plane probe. Exit without seeding if the lease is held. Release in `finally`. Add `LeaseUnavailable` to `PollTriggerBootstrapResult`. |
| `PollPlanItCommandHandler` | Remove the inline lease acquisition — the orchestrator owns serialization. Remove the `LeaseHeld` termination branch and the `PollTerminationReason.LeaseHeld` enum value. |

### Flow

```
sync orchestrator:               bootstrap:
  acquire lease                    acquire lease
    ├─ 412/409 → retry once          ├─ 412/409 → exit (peer running)
    ├─ still held → exit             ↓
    ↓                              probe queue depth (ARM)
  receive (destructive)            if zero: publish seed
  handler                          release
  publish next-run
  release
```

### What the lease guarantees

During any `acquire → release` span, no other actor can be inside its own `acquire → release` span. Because both orchestrator and bootstrap perform all their queue mutations inside this span, the two invariants hold:

- **(a)** holds: only one holder of the lease at any instant.
- **(b)** holds: neither actor can publish while the other holds the lease; the orchestrator publishes exactly one next-run per cycle; the bootstrap publishes only when probe shows empty; queue depth stays ≤ 1.

## Lease semantics

### Document schema

Single document at `Leases/polling` (partition key = `polling`):

```jsonc
{
  "id": "polling",
  "holderId": "<GUID>",        // diagnostic only; log on conflict
  "acquiredAtUtc": "...",      // diagnostic
  "expiresAtUtc": "...",       // authoritative for "is this lease dead?"
  "_etag": "..."               // Cosmos server-assigned CAS token
}
```

Decisions are made on `now vs expiresAtUtc` plus the ETag. `holderId` is purely diagnostic.

### Acquire flow

Three outcomes: **Acquired**, **Held**, **TransientError**.

```
READ document
├─ 404 not found → first-ever acquire
│     CREATE with If-None-Match: *
│       ├─ 201 → Acquired(etag)
│       ├─ 409 → raced → Held
│       └─ 5xx → TransientError
│
└─ 200 OK (etag E)
      if expiresAtUtc > now: Held
      else:
        REPLACE with If-Match: E
          ├─ 200 → Acquired(newEtag)
          ├─ 412 → raced → Held
          └─ 5xx → TransientError
```

The caller receives a `LeaseHandle` containing the ETag of the winning write — needed by `ReleaseAsync` to prove ownership.

### Release flow

```
DELETE with If-Match: <acquireEtag>
├─ 204 → released cleanly
├─ 404 → already gone — treat as released (INFO)
└─ 412 → our lease expired; a peer took it (WARN — design signal)
```

A `412` on release is never silently swallowed: it always indicates the caller held the lease past its TTL, which means either the TTL is too tight or `HandlerBudget` wasn't enforced. Both are bugs worth flagging.

### TTL sizing

| Caller | TTL | Rationale |
|---|---|---|
| Orchestrator | `HandlerBudget + 30 s` (default `270 s`) | Must outlive the longest possible handler run; short enough that a crashed replica's orphaned lease clears before the next bootstrap tick |
| Bootstrap | `60 s` | Probe + maybe publish is ~1–2 s; anything longer is a network pathology |

TTL is a per-acquire parameter, not a property of the store.

**Crash-recovery bound:** if the orchestrator's replica `SIGKILL`s mid-handler, the lease is orphaned for at most `TTL − elapsed`. With TTL = 270 s, worst case ≈ 4.5 min of stuck state. The bootstrap tick (every 30 min) absorbs it.

## Orchestrator changes

```csharp
var lease = await this.leaseStore.TryAcquireAsync(
    this.options.OrchestratorLeaseTtl, ct);

if (!lease.Acquired)
{
    await Task.Delay(jitteredBackoff, ct);
    lease = await this.leaseStore.TryAcquireAsync(
        this.options.OrchestratorLeaseTtl, ct);

    if (!lease.Acquired)
    {
        LogLeaseUnavailable(this.logger);
        return new PollTriggerOrchestratorRunResult(
            MessageReceived: false, PublishedNext: false,
            PollResult: null, LeaseUnavailable: true);
    }
}

try
{
    var message = await this.triggerQueue.ReceiveAsync(ct);
    if (message is null)
    {
        LogEmptyQueue(this.logger);
        return new PollTriggerOrchestratorRunResult(
            MessageReceived: false, PublishedNext: false,
            PollResult: null, LeaseUnavailable: false);
    }

    var pollResult = await this.handler.HandleAsync(new PollPlanItCommand(), ct);

    var nextRun = this.scheduler.ComputeNextRun(
        pollResult.TerminationReason, pollResult.RetryAfter,
        this.timeProvider.GetUtcNow());

    await this.triggerQueue.PublishAtAsync(nextRun, ct);

    return new PollTriggerOrchestratorRunResult(
        MessageReceived: true, PublishedNext: true,
        PollResult: pollResult, LeaseUnavailable: false);
}
finally
{
    await this.leaseStore.ReleaseAsync(lease, ct);
}
```

Key points:

- **One retry with jittered backoff** (500–1500 ms) handles the common case: bootstrap briefly holds the lease.
- **Retry failure → exit cleanly.** Message stays active; KEDA re-triggers within 30 s. `LeaseUnavailable` lets telemetry distinguish "idled because lease held" from "idled because queue empty".
- **`finally` always releases**, using the ETag from acquire. Release failure is logged, not rethrown.
- **Handler no longer acquires lease.** The orchestrator is the sole gatekeeper.

## Bootstrap changes

```csharp
var lease = await this.leaseStore.TryAcquireAsync(
    this.options.BootstrapLeaseTtl, ct);

if (!lease.Acquired)
{
    LogLeaseHeldByPeer(this.logger);
    return new PollTriggerBootstrapResult(
        Published: false, ProbeFailed: false, LeaseUnavailable: true);
}

try
{
    PollTriggerQueueDepth depth;
    try { depth = await this.metrics.GetDepthAsync(ct); }
    catch (Exception ex)
    {
        LogProbeFailed(this.logger, ex);
        return new PollTriggerBootstrapResult(Published: false, ProbeFailed: true, LeaseUnavailable: false);
    }

    if (!depth.IsEmpty)
    {
        LogQueueAlreadySeeded(this.logger, depth.ActiveMessageCount, depth.ScheduledMessageCount);
        return new PollTriggerBootstrapResult(Published: false, ProbeFailed: false, LeaseUnavailable: false);
    }

    var nextRun = this.scheduler.ComputeNextRun(
        PollTerminationReason.Natural, retryAfter: null,
        this.timeProvider.GetUtcNow());

    try { await this.triggerQueue.PublishAtAsync(nextRun, ct); }
    catch (Exception ex)
    {
        LogPublishFailed(this.logger, ex);
        return new PollTriggerBootstrapResult(Published: false, ProbeFailed: true, LeaseUnavailable: false);
    }

    LogBootstrapPublished(this.logger, nextRun);
    return new PollTriggerBootstrapResult(Published: true, ProbeFailed: false, LeaseUnavailable: false);
}
finally
{
    await this.leaseStore.ReleaseAsync(lease, ct);
}
```

Key points:

- **No retry on lease-held.** Bootstrap is a 30-min cron; missing one tick because a peer is running is the *correct* outcome.
- **All existing probe/publish failure handling is preserved.** Lease acquire is a new outermost guard, not a rewrite of the body.

## `PollPlanItCommandHandler` — slimmed down

The handler's inline `TryAcquireAsync / try / finally ReleaseAsync` wrapper is removed entirely. `HandleAsync` now *is* `HandleUnderLeaseAsync`. Delete:

- The `leaseStore` field and constructor parameter (update DI wiring in `ServiceCollection` registration).
- `LogLeaseHeld` / `LogLeaseReleaseFailed` loggers.
- `PollTerminationReason.LeaseHeld` (and any test references).
- The `if (pollResult.TerminationReason == LeaseHeld)` branch in the previous orchestrator.

## `PollingOptions` additions

```csharp
public TimeSpan OrchestratorLeaseTtl { get; init; } = TimeSpan.FromMinutes(4.5); // HandlerBudget + 30s
public TimeSpan BootstrapLeaseTtl    { get; init; } = TimeSpan.FromSeconds(60);
public TimeSpan LeaseAcquireRetryDelay { get; init; } = TimeSpan.FromSeconds(1); // jittered ±50%
```

The existing `LeaseTtl` (consumed by the handler) is removed; sweep tests and fixtures for stragglers.

## `ICosmosRestClient` additions

```csharp
public sealed record CosmosReadResult<T>(T? Document, string? ETag);

Task<CosmosReadResult<T>> ReadDocumentAsync<T>(
    string container, string id, string partitionKey,
    JsonTypeInfo<T> typeInfo, CancellationToken ct);

Task<bool> TryCreateDocumentAsync<T>(
    string container, T document, string partitionKey,
    JsonTypeInfo<T> typeInfo, CancellationToken ct);

Task<bool> TryReplaceDocumentAsync<T>(
    string container, T document, string partitionKey, string ifMatchEtag,
    JsonTypeInfo<T> typeInfo, CancellationToken ct);

Task<CosmosDeleteOutcome> TryDeleteDocumentAsync(
    string container, string id, string partitionKey, string? ifMatchEtag,
    CancellationToken ct);

public enum CosmosDeleteOutcome { Deleted, NotFound, PreconditionFailed }
```

Existing `UpsertDocumentAsync` / `DeleteDocumentAsync` remain — other call sites use them. The new methods are opt-in for CAS.

All of this is REST-native and AOT-safe: `If-Match`, `If-None-Match`, and Cosmos ETags are plain HTTP headers and a string field on the response.

## Failure matrix

### Orchestrator path

| Step | Failure | Queue ends as | Lease ends as | Recovery |
|---|---|---|---|---|
| Acquire | 5xx / network | unchanged (msg still active) | unchanged | KEDA re-triggers ≤ 30 s |
| Acquire | `412` / `409` (peer holds) | unchanged | peer holds | Retry once w/ jitter → exit → KEDA re-trigger |
| Receive | 5xx | unchanged | released (finally) | KEDA re-trigger ≤ 30 s |
| Receive | `null` | empty | released | Bootstrap seeds ≤ 30 min |
| Handler | throws | consumed, no next published | released | Bootstrap seeds ≤ 30 min |
| Handler | hits `HandlerBudget` | consumed, partial cycle | released | Orchestrator publishes next-run from partial result; chain continues |
| Publish | 5xx | consumed, no next published | released | Bootstrap seeds ≤ 30 min |
| Release | `412` | whatever we left | held by peer | Peer's normal flow — log WARN |
| Release | `404` / 5xx | whatever we left | orphaned ≤ TTL | Next acquirer waits ≤ (TTL − elapsed) |
| Replica | SIGKILL pre-receive | unchanged | orphaned | TTL expires (≤ 270 s), KEDA re-triggers |
| Replica | SIGKILL mid-handler | consumed, no next published | orphaned | TTL expires → bootstrap seeds on next tick |
| Replica | SIGKILL post-publish | next-run scheduled (healthy) | orphaned | Chain continues; lease cleared at TTL |

### Bootstrap path

| Step | Failure | Queue ends as | Lease ends as | Recovery |
|---|---|---|---|---|
| Acquire | 5xx / network | unchanged | unchanged | Next cron tick (≤ 30 min) |
| Acquire | `412` / `409` | unchanged | peer holds | Orchestrator completes naturally |
| Probe | ARM 5xx | unchanged | released (finally) | Next cron tick |
| Probe | non-empty | unchanged | released | — (healthy) |
| Publish | 5xx | empty | released | Next cron tick |
| Release | `412` | whatever we left | peer holds | Log WARN (bootstrap ran > 60 s — unexpected) |
| Release | 5xx | whatever we left | orphaned ≤ 60 s | Next acquirer briefly waits |
| Replica | SIGKILL | maybe partial publish | orphaned ≤ 60 s | TTL expires fast; next tick resumes |

### Correlated failures

| Scenario | Behaviour | Recovery |
|---|---|---|
| Cosmos down | Nobody acquires; chain pauses. **No duplicates possible** (nothing can publish). | Cosmos recovers → next KEDA trigger or cron tick resumes |
| Service Bus down | Orchestrator can't receive/publish; bootstrap can't probe/publish. Chain pauses. | SB recovers → normal flow |
| Both down | Total pause | Both recover |
| Clock skew between replicas | Expiry uses local `now`; Azure VM clocks drift ≲ seconds | Negligible in practice |

### Invariants the design depends on

1. **Lease TTL > max handler runtime.** Enforced by `OrchestratorLeaseTtl = HandlerBudget + 30 s`. If `HandlerBudget` is ever disabled for `poll-sb`, the lease can expire mid-handler → a later acquirer + our eventual publish = duplicate. Guard: `Program.cs` asserts `HandlerBudget` is non-null at startup for `poll-sb` mode.
2. **All queue mutations happen inside a held lease span.** Enforced structurally — `ReceiveAsync` / `PublishAtAsync` are inside the orchestrator's `try`; bootstrap's probe+publish inside bootstrap's `try`.
3. **Release proves ownership.** `DELETE` uses `If-Match: <acquireEtag>`. `412` on release is always a design signal.
4. **At most one live trigger.** Derived from (2) + lease mutual exclusion: only one actor is inside its critical section at any instant → at most one publish in flight → queue depth stays ≤ 1.

### Worst-case recovery window

| Failure type | Time to next healthy cycle |
|---|---|
| Handler crash or publish failure | ≤ 30 min (next bootstrap tick) |
| Orphaned lease (replica SIGKILL during a live cycle) | ≤ `OrchestratorLeaseTtl` (≈ 270 s) + ≤ 30 min (bootstrap) ≈ 35 min |
| Cosmos or SB outage | Bounded by outage duration + KEDA's 30 s polling interval |

No failure path produces a duplicate. No failure path stalls forever.

## Testing

### `CosmosPollingLeaseStoreTests` (unit)

| Scenario | Expected result |
|---|---|
| Acquire when doc missing → 201 | `Acquired(etag)` |
| Acquire when doc missing → 409 (raced create) | `Held` |
| Acquire when doc exists + expired → 200 with `If-Match` | `Acquired(newEtag)` |
| Acquire when doc exists + expired → 412 (raced) | `Held` |
| Acquire when doc exists + not expired | `Held` (no write attempted) |
| Acquire transient 5xx | `TransientError`, no state change |
| Release happy path → 204 | clean; no log |
| Release with 404 | treat as released; INFO log |
| Release with 412 | WARN log "lease expired under us"; never throws |
| Release with 5xx | swallow + ERROR log; TTL is the backstop |

### `PollTriggerOrchestratorTests` (unit)

- Normal path: acquire → receive → handler → publish → release (verify order via spy).
- Lease unavailable (first acquire returns `Held`) → retry → succeed → normal path.
- Lease unavailable both times → exit with `LeaseUnavailable=true`, `MessageReceived=false`, no handler call, no publish.
- Receive returns `null` → release lease, return `MessageReceived=false`, no handler call.
- Handler throws → `finally` releases lease, exception propagates.
- Publish throws → `finally` releases lease, exception propagates.

### `PollTriggerBootstrapperTests` (unit)

- Lease held → no probe, no publish, `LeaseUnavailable=true`.
- Lease acquired + probe non-empty → release, no publish.
- Lease acquired + probe empty → publish → release.
- Lease acquired + probe throws → release, `ProbeFailed=true`.
- Lease acquired + publish throws → release, appropriate result flags.

### `PollLeaseCasIntegrationTests` (integration)

Uses a fake `ICosmosRestClient` that models ETags + `If-Match` / `If-None-Match` semantics:

- The fake must honour `If-None-Match: *` (returning `409` on second create) and `If-Match: <etag>` (returning `412` on stale).
- Spawn one "orchestrator" + one "bootstrap" against the same fake store; the one that acquires first runs to completion; the other exits cleanly. Run ~100 iterations with randomised ordering — assert queue depth is always ≤ 1 and exactly one cycle runs per iteration.

The fake will be reusable anywhere else in the codebase that needs CAS against Cosmos.

## Rollout

Single PR, shipped direct to prod via normal CI/CD. Justification:

- No schema migration — the existing `Leases/polling` document shape is unchanged; the new code just starts using ETags, which Cosmos has always emitted.
- No external API change.
- Dev has no SB polling path (per ADR 0024), so there's nowhere to soak a staged rollout.
- Feature-flagging adds more complexity than it removes for a behaviourally-contained change.

### Post-deploy monitoring (manual, ~48 h)

1. **Queue depth metric** — `activeMessageCount + scheduledMessageCount` should stay ≤ 1 under steady state (matches the `poll-queue-max-one-message` invariant already monitored).
2. **New lease telemetry:**
   - `polling.lease.acquired{caller="orchestrator|bootstrap"}`
   - `polling.lease.held_by_peer{caller="orchestrator|bootstrap"}`
   - `polling.lease.released_412{caller="orchestrator|bootstrap"}` (should be 0; non-zero ⇒ tune TTL or `HandlerBudget`)
3. **429 rate against PlanIt** — regression would show as a spike.

If a regression is detected, fix forward — the change is contained enough to diagnose and patch without needing to roll back.

## Out of scope

- **Adding CAS support to other Cosmos writes** (e.g. applications dedupe). Worth a follow-up bead once the pattern is proven here.
- **Self-healing via a sentinel scheduled message.** Considered and rejected: the lease already closes both invariants; the extra REST calls per cycle cost more than they save.
- **Blob lease as mutex primitive.** Rejected: 60 s lease cap would force a renewal loop for long-running handlers, adding complexity and failure modes that Cosmos's self-managed `expiresAtUtc` avoids.
