# Seed Polling — Alternating Watched / Full-Set Sync

## Context

The poll worker currently syncs only authorities with at least one active watch zone (~8 today). The remaining ~409 UK local planning authorities are never synced, so a new user who creates a watch zone in a never-polled authority sees an empty map until the next poll cycle.

Goal A: map/applications views are populated with recent activity when the user first opens them. Observed baseline (App Insights, 2026-04-17→18): ~8 authorities polled per 15-min cycle, 0 rate-limit hits, ~45 s cycle duration — plenty of headroom for adding a seed pass.

See design doc: [`docs/superpowers/specs/2026-04-18-seed-polling-design.md`](../superpowers/specs/2026-04-18-seed-polling-design.md).
See implementation plan: [`docs/superpowers/plans/2026-04-18-seed-polling.md`](../superpowers/plans/2026-04-18-seed-polling.md).

## Design

Alternate 15-min cycles between two input sets, chosen by the current UTC minute:

```
(utcNow.Minute % 30) < 15  →  watched cycle  (ids from WatchZoneActiveAuthorityProvider)
otherwise                   →  seed cycle     (all 417 authority ids)
```

The modulo tolerates up to ~15 minutes of Container Apps Job startup drift — the job does not have to fire at exact 00/15/30/45 boundaries.

**Effective cadence:**
- Watched authorities: polled every 30 minutes (down from 15).
- Seed authorities: ~8 polls per seed cycle × 48 seed cycles/day = ~384 polls/day → full 417-authority rotation in **~26 hours**.

**Natural LRU separation.** The handler already sorts by `lastPollTime` (least-recently-polled first). On seed cycles, watched authorities polled 15 min earlier sink to the bottom of the seed queue, so unwatched authorities rise to the top. No explicit filter needed.

**First-sync window unchanged.** When a never-polled authority is hit for the first time, `lastPollTime ??= now.AddDays(-1)` still gives a 24-hour backfill. The catalogue deepens naturally over subsequent daily rotations — acceptable given paying users expected no sooner than ~1 month out.

**Telemetry.** All poll counters (`authorities_polled`, `applications_ingested`, `authorities_skipped`, `rate_limited`, `authority_processing_ms`) and both activity spans (root + per-authority) are tagged with `cycle.type` (`"watched"` | `"seed"`).

## Scope

**In scope**
- Alternating selection logic, driven by UTC minute via a new `ICycleSelector`.
- Two marker interfaces (`IWatchZoneActiveAuthorityProvider`, `IAllAuthorityIdProvider`) so the alternating provider can DI-resolve both concretions distinctly.
- New `AllAuthorityIdProvider` adapting `IAuthorityProvider` (already registered via `StaticAuthorityProvider`).
- `cycle.type` tag on handler counters and worker root activity span.

**Out of scope**
- Reactive/lazy backfill on watch zone creation.
- Retroactive notification matching (existing `zone.CreatedAt > application.LastDifferent` filter preserved).
- Per-tier freshness SLAs or priority queues.

## Steps

### Step 1 — Cycle selection primitives

Introduce `CycleType` enum (`Watched`, `Seed`), `ICycleSelector` interface, and `MinuteBasedCycleSelector` implementation. Full TDD: parameterised tests for minute boundaries 0, 14, 15, 29, 30, 44, 45, 59. Foundational; blocks everything else.

Plan reference: [Task 1 & Task 2](../superpowers/plans/2026-04-18-seed-polling.md#task-1-define-cycletype-enum).

### Step 2 — Authority provider interfaces and implementations

Create marker interfaces `IWatchZoneActiveAuthorityProvider` and `IAllAuthorityIdProvider` (both extend `IActiveAuthorityProvider`). Update `WatchZoneActiveAuthorityProvider` to implement the watch-zone marker. Create `AllAuthorityIdProvider` (infrastructure layer) that projects `IAuthorityProvider.GetAllAsync()` to an `IReadOnlyCollection<int>`. Full TDD for the new provider.

Plan reference: [Task 3, Task 4, Task 5](../superpowers/plans/2026-04-18-seed-polling.md#task-3-create-marker-interfaces).

### Step 3 — Cycle-alternating authority provider

Create `CycleAlternatingAuthorityProvider` that implements `IActiveAuthorityProvider` and delegates to either the watched or all-authority provider based on `ICycleSelector`. Introduce `FakeCycleSelector` test double. Full TDD covering both branches.

Depends on Step 1 and Step 2.
Plan reference: [Task 6](../superpowers/plans/2026-04-18-seed-polling.md#task-6-create-cyclealternatingauthorityprovider).

### Step 4 — Handler metric tagging

Extend `PollPlanItCommandHandler` to accept `ICycleSelector`, resolve the cycle type once per run, and tag every counter/histogram emission with `cycle.type`. Update `CreateHandler` helpers in the three existing handler test files to default the new dependency to `FakeCycleSelector(Watched)` — only one new test (asserting the tag is present) is added.

Depends on Step 1.
Plan reference: [Task 7](../superpowers/plans/2026-04-18-seed-polling.md#task-7-add-cycletype-tag-to-handler-counter-emissions).

### Step 5 — Worker DI wiring, span tag, and final verification

Wire the full graph in `api/src/town-crier.worker/Program.cs`: register `IAuthorityProvider`, both marker interfaces, `ICycleSelector`, and swap `IActiveAuthorityProvider` binding from `WatchZoneActiveAuthorityProvider` to `CycleAlternatingAuthorityProvider`. Tag the root "Polling Cycle" activity with `cycle.type`. Run full build, full test suite, `dotnet format --verify-no-changes`.

Depends on Step 3 and Step 4.
Plan reference: [Task 8, Task 9](../superpowers/plans/2026-04-18-seed-polling.md#task-8-wire-up-di-in-the-worker-and-tag-the-root-activity).
