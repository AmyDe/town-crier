# Service Bus-Only Polling

Date: 2026-04-22

## Context

The adaptive Service Bus-coordinated polling chain (PRs #275, #276, #278, #280, #283, #285) is operational in prod. One event-triggered `poll` Container Apps Job consumes each trigger message, runs `PollPlanItCommandHandler`, publishes the next trigger with a scheduled enqueue time, then acks. Publish-before-ack ordering plus a Cosmos lease guard keeps the chain self-healing under replica crashes and restarts.

Alongside it, a cron-triggered `poll-safety-net` job runs `*/30 * * * *` doing two things:

1. A full `PollPlanItCommandHandler` cycle (legacy timer-based polling).
2. `PollTriggerBootstrapper.TryBootstrapAsync` — probes the trigger queue and publishes a seed message if empty.

Now that the SB chain is trusted, the full poll half of the safety-net is redundant: it does the same work the SB chain already does, competes for the Cosmos lease (mostly losing), costs a 600-second replica timeout every 30 min, and muddies the telemetry (two paths calling the same handler). The bootstrap half remains valuable as a recovery mechanism for the rare case where the chain dies entirely (e.g. max-delivery-count exhaustion moves the only live message to DLQ).

Dev has never been on the SB chain — `ServiceBusPollingInfra` is provisioned only when `env == "prod"`. Its cron `poll` job runs every 3 hours purely to keep Cosmos data fresh for manual testing. The intention going forward is to stop dev polling entirely and (as a separate future feature) back-fill dev Cosmos from prod applications once per day.

`docs/specs/poll-handler-soft-budget.md` (2026-04-22) flagged this direction explicitly: *"Timer mode will be removed in a follow-up bead once the SB-coordinated loop is stable; the safety-net job will then become a bootstrap-only job (probe queue → publish seed if empty), not a full poll."* This spec is that follow-up.

## Decision

1. **Prod.** Strip the full poll cycle out of the safety-net job. Rename it `poll-bootstrap` and give it a new `WORKER_MODE=poll-bootstrap` that only runs `PollTriggerBootstrapper.TryBootstrapAsync`. Keep the `*/30 * * * *` cadence.
2. **Dev.** Delete the cron-triggered `poll` job. No Service Bus infra added. Dev Cosmos retains existing data but ingests no new applications until the future prod→dev backfill feature lands.
3. **Code.** Remove the `"poll"` switch branch from `api/src/town-crier.worker/Program.cs` entirely. No environment uses `WORKER_MODE=poll` (or the null default that falls through to it) after this change.

The `poll-sb` orchestrator, `PollPlanItCommandHandler`, `PollNextRunScheduler`, `PollTriggerBootstrapper`, and all polling infrastructure are unchanged. This is purely about which entry points invoke which paths.

## Architecture

After the change:

```
PROD
  [SB queue: poll-triggers] --(event trigger)--> poll (WORKER_MODE=poll-sb)
         |                                          |
         |                                          +--> PollTriggerOrchestrator
         |                                                 - receive msg
         |                                                 - run PollPlanItCommandHandler
         |                                                 - publish next msg (scheduled)
         |                                                 - ack
         |
         +----(cron */30, queue-empty only)----> poll-bootstrap (WORKER_MODE=poll-bootstrap)
                                                    |
                                                    +--> PollTriggerBootstrapper.TryBootstrapAsync
                                                           - probe queue (PeekLock receive)
                                                           - if empty: publish jittered seed
                                                           - if message present: abandon, exit

DEV
  (no poll job at all)
```

Invariants preserved:

- `poll-sb` remains the sole path that calls `PollPlanItCommandHandler`. The Cosmos lease guard and publish-before-ack semantics are untouched.
- Bootstrap is best-effort and idempotent. Any probe or publish failure is swallowed; the next cron tick retries.
- The 240-second handler soft budget / 300-second Service Bus lock pairing (see `poll-handler-soft-budget.md`) applies only to `poll-sb` and is not affected.

## Scope

In scope:

- `api/src/town-crier.worker/Program.cs` — delete `case "poll":`, add `case "poll-bootstrap":`.
- `infra/EnvironmentStack.cs` — rename the cron job, set its `WORKER_MODE`, delete the dev poll job.
- `api/tests/town-crier.integration-tests/Polling/SafetyNetBootstrapIntegrationTests.cs` — rename and trim.
- `.github/workflows/` — update any references to `poll-safety-net`.
- `docs/adr/0024-service-bus-only-polling.md` — new ADR.

Out of scope (separate beads if/when needed):

- Prod→dev Cosmos backfill feature.
- Adding Service Bus polling infra to dev.
- Changes to the adaptive scheduler (`PollNextRunScheduler`), the orchestrator, or the handler.
- Bootstrap-on-API-startup belt-and-braces (considered and rejected below).

## Code Changes

### `api/src/town-crier.worker/Program.cs`

Delete the existing `case "poll":` branch (currently lines ~227–306, covering the full cycle plus the post-cycle bootstrap).

Add a new `case "poll-bootstrap":` branch:

- Starts an OpenTelemetry activity named `"Polling Bootstrap"` (distinct from `"Polling Cycle"` and `"Polling Cycle (SB)"`).
- Uses a short cycle budget. The bootstrap probe is ~1 second of work; a 60-second budget is generous.
- Resolves `PollTriggerBootstrapper`, awaits `TryBootstrapAsync`, sets activity tags `polling.safety_net.bootstrap_published` and `polling.safety_net.bootstrap_probe_failed` (tag names unchanged for dashboard continuity).
- Exit code is always `0` unless the outer catch fires. A failed probe or publish is not a worker failure.

The current `WORKER_MODE` read (`builder.Configuration["WORKER_MODE"] ?? "poll"` near the bottom of `Program.cs`) should change to a non-defaulting read. The configuration is always set by Pulumi now, and silently falling through to a deleted mode would be a deployment accident. Suggestion: read without a default and log-and-exit-1 via the existing `UnknownWorkerMode` log if unset.

### Dead-code sweep

After the change:

- `ICycleSelector`, `CycleType`, `CycleAlternatingAuthorityProvider`, `IActiveAuthorityProvider` are still used by `PollPlanItCommandHandler`, which is still used by `poll-sb`. They stay.
- `IPollingLeaseStore` is still used by the orchestrator. Stays.
- Spot-check: the implementing bead should grep for `"poll"` string literal references and for any code that assumed WORKER_MODE has a default.

## Infra Changes

### `infra/EnvironmentStack.cs`

Inside the `if (pollingBus is not null)` branch (prod-only today):

1. Rename the second `CreateWorkerJob` call from `"poll-safety-net"` to `"poll-bootstrap"`.
2. Change `workerMode: null` to `workerMode: "poll-bootstrap"`.
3. Reduce `replicaTimeout` from `600` to `120`. The job runs a one-second queue probe; 2 minutes is ample and the shorter timeout signals intent.
4. Keep `cronExpression: "*/30 * * * *"` unchanged.

Delete the `else` branch that creates a cron `poll` job when `pollingBus is null` (the current dev path). The parent `if (pollingBus is not null)` condition stays — when `env != "prod"`, no poll jobs are created at all.

No changes to `CreateServiceBusPollingInfra`, `CreateWorkerJob`, or any other helper.

### Pulumi deploy behaviour

- Prod: `poll-safety-net` is deleted; `poll-bootstrap` is created. Brief gap (seconds) during the Pulumi step where neither exists. Harmless — the event-triggered `poll` job is untouched and keeps servicing the SB chain throughout.
- Dev: the cron `poll` job is deleted. Cosmos `PollState` entities remain untouched.

### CI

The deploy-worker-jobs loop (GitHub Actions) currently references `poll-safety-net` (added in #280). Update to `poll-bootstrap`. The implementing bead greps `.github/workflows/` for `poll-safety-net` and updates all references.

## Testing

### Updated

- `api/tests/town-crier.integration-tests/Polling/SafetyNetBootstrapIntegrationTests.cs` — rename to `PollBootstrapIntegrationTests.cs`. Trim to cover only the bootstrap probe/publish flow against the Service Bus emulator. Core assertions stay:
  - Queue empty → probe returns null → publish happens → one message enqueued with a future scheduled enqueue time.
  - Queue non-empty → probe returns message → abandon called → no publish.
  - Probe throws → logged; result reports `ProbeFailed: true`; no crash.

### Unchanged

- `api/tests/town-crier.application.tests/Polling/PollTriggerBootstrapperTests.cs` — unit tests on the bootstrapper with fake `IPollTriggerQueue`. Continue to pass without modification.
- `PollTriggerOrchestratorTests.cs` — untouched; the orchestrator is not changing.

### Optional

- A thin integration test asserting that `WORKER_MODE=poll-bootstrap` resolves the right services and exits 0 on a happy path. The unit tests already cover the logic, so add only if it earns its keep.

## ADR

Write `docs/adr/0024-service-bus-only-polling.md`. Status: Accepted. Supersedes the cron-poll portion of `0019-extract-polling-to-container-apps-job.md` — 0019 introduced the cron-triggered Container Apps Job model; this one narrows it so cron only ever bootstraps, never polls.

Content covers:

- Context: SB-triggered adaptive polling is operational. Cron poll is redundant in prod and unwanted in dev.
- Decision: prod polling only via the SB chain; a single `poll-bootstrap` cron job reseeds an empty queue; dev has no polling.
- Consequences: cheaper (no redundant full polls), clearer (one canonical path), riskier in pathological cases. Dev data goes stale until the prod→dev backfill feature lands.

`0012-dynamic-polling-prioritisation.md` and `0021-resumable-pagination-cursor-for-planit-polling.md` remain in force — both describe the polling algorithm, which is untouched.

## Deploy Risk and Rollback

Deploy order:

1. CD deploys the worker image and the Pulumi changes together, atomically.
2. During the Pulumi step, `poll-safety-net` is deleted before `poll-bootstrap` is created. Gap is seconds. The event-triggered `poll` job is untouched throughout.
3. The dev `poll` job is deleted cleanly. No worker image concern in dev.

Rollback: revert the PR. Pulumi recreates `poll-safety-net` and the dev `poll` job with their old shape. The image revert brings back the `"poll"` switch case.

Failure-mode analysis:

- If the bootstrap probe fails for one or more consecutive cron ticks while the SB chain is also dead, polling pauses for `30 × N` minutes. The probe is a single Service Bus PeekLock receive — failure modes are Service Bus outages or identity/RBAC misconfiguration, both of which would also break the `poll-sb` chain simultaneously and be highly visible.
- If the bootstrap probe succeeds but returns a stuck-in-flight message that a dead handler holds the lock on, the probe abandons and exits. After `LockDuration` (5 min) the message becomes redeliverable. Service Bus max delivery count (default 10) eventually moves it to DLQ; at that point the next cron tick sees an empty queue and reseeds. Worst-case recovery time is bounded by `(max deliveries × lock duration) + (time-to-next-cron-tick) ≈ 50 + 30 = 80 minutes`, which matches the "extenuating circumstances" bar agreed for this design.

## Rejected Alternatives

- **Gate the poll call behind a config flag (`SKIP_POLL_CYCLE=true`), keep `WORKER_MODE=poll`.** Less code churn but semantically ugly — a `"poll"` mode that doesn't poll is a landmine. Rejected.
- **Delete the cron job entirely; rely on the orchestrator's publish-before-ack to keep the chain alive forever.** The chain is self-healing under individual replica crashes but not under catastrophic failures (DLQ exhaustion, Service Bus maintenance, manual queue purges). Rejected — too fragile given the chain only recently went operational.
- **Tighter cron cadence (e.g. `*/10 * * * *`).** Reduces worst-case recovery window but triples the container-start rate. The chain losing its way should be an extenuating event, not a routine one. Rejected per user direction.
- **Bootstrap-on-API-startup.** Belt-and-braces where the API pod reseeds the chain on start. Not needed — the cron tick is cheap and doesn't couple the API lifecycle to SB seed state. Can be added later without undoing this work.
- **Add Service Bus polling infra to dev, then strip dev's timer too.** Matches the "never timer-based polling" rule more literally but requires provisioning dev SB infra that would service essentially no traffic. Rejected in favour of the prod→dev backfill feature.
