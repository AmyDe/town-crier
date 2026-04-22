# Service Bus-Only Polling Implementation Plan

> **For agentic workers:** This plan is intended to be converted into beads via the `plan-to-beads` skill (per project CLAUDE.md rule) rather than executed inline with TodoWrite. Each task below becomes one bead. Steps use checkbox (`- [ ]`) syntax for tracking inside each bead.

**Goal:** Strip timer-based polling. In prod, the cron safety-net job becomes bootstrap-only (probes the Service Bus trigger queue and publishes a seed if empty); in dev, polling is removed entirely.

**Architecture:** Prod retains one event-triggered `poll` job (`WORKER_MODE=poll-sb`) that services the SB chain end-to-end. A cron job `poll-bootstrap` (`WORKER_MODE=poll-bootstrap`) runs every 30 minutes doing nothing but probing the queue for emptiness and publishing a jittered seed message if so. Dev has no polling job at all.

**Tech Stack:** .NET 10 worker (`api/src/town-crier.worker`), Azure Container Apps Jobs, Azure Service Bus (Basic tier), Pulumi .NET (`infra/EnvironmentStack.cs`), GitHub Actions (`.github/actions/deploy-worker-jobs/action.yml`), TUnit tests.

**Spec:** `docs/specs/sb-only-polling.md`

---

## File Structure

**Create:**
- `docs/adr/0024-service-bus-only-polling.md` — new ADR.

**Modify:**
- `api/src/town-crier.worker/Program.cs` — delete `case "poll":`, add `case "poll-bootstrap":`, remove `WORKER_MODE` default.
- `infra/EnvironmentStack.cs` — rename cron job and set its `WORKER_MODE`, reduce `replicaTimeout`, delete the dev `else` branch.
- `.github/actions/deploy-worker-jobs/action.yml` — replace `poll-safety-net` with `poll-bootstrap` in the worker-jobs bash loop and its leading comment.

**Rename + modify:**
- `api/tests/town-crier.integration-tests/Polling/SafetyNetBootstrapIntegrationTests.cs` → `PollBootstrapIntegrationTests.cs` — class rename, comment refresh. Assertions unchanged (bootstrapper behaviour is not changing).

**Unchanged but notable:**
- `api/tests/town-crier.application.tests/Polling/PollTriggerBootstrapperTests.cs` — unit tests stay as-is.
- `api/src/town-crier.application/Polling/PollTriggerBootstrapper.cs` — no changes.
- `api/src/town-crier.application/Polling/PollTriggerOrchestrator.cs` — no changes.
- `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs` — no changes.

---

## Pre-Tasks (workflow setup)

These are mandated by project CLAUDE.md; `plan-to-beads` will place these inside each bead's TDD cycle.

- [ ] Ensure a bead exists and is claimed (`bd update <id> --claim`) before any code edit.
- [ ] Ensure the session is inside a git worktree (`EnterWorktree`) before any code edit — the `.claude/require-worktree.sh` hook blocks Write/Edit on `.cs`, `.ts`, `.tsx`, `.css`, `.csproj` otherwise.
- [ ] Docs-only edits (`.md`) can be made without a worktree — the hook ignores them.

---

## Task 1: Write ADR 0024

Records the architectural decision before any code changes.

**Files:**
- Create: `docs/adr/0024-service-bus-only-polling.md`

- [ ] **Step 1: Create the ADR file**

Exact content:

```markdown
# 0024. Service Bus-Only Polling

Date: 2026-04-22

## Status

Accepted. Supersedes the cron-poll portion of [0019](0019-extract-polling-to-container-apps-job.md).

## Context

ADR 0019 introduced Container Apps Jobs for the PlanIt polling worker, with a cron schedule as the primary trigger. Since then, the adaptive Service Bus-coordinated polling chain (PRs #275, #276, #278, #280, #283, #285) has gone operational in prod. One event-triggered `poll` job consumes each trigger message, runs the handler, publishes the next trigger with a scheduled enqueue time, then acks. Publish-before-ack ordering and a Cosmos lease guard keep the chain self-healing under replica crashes and restarts.

A parallel cron `poll-safety-net` job has been running every 30 minutes doing a full poll cycle AND re-seeding the SB queue if empty. The full-poll half is now redundant (it does the work the SB chain already does, competes for the Cosmos lease, and costs a 600-second replica timeout every 30 min). The re-seed half remains valuable as a recovery mechanism for the rare case where the chain dies entirely — e.g. max-delivery-count exhaustion moves the live message to the dead-letter queue.

Dev has never been on the SB chain: `ServiceBusPollingInfra` is provisioned only when `env == "prod"`. Dev's cron `poll` job exists purely to keep Cosmos data fresh for manual testing. A future prod→dev Cosmos backfill feature will replace this more cheaply.

## Decision

Prod polling happens only via the Service Bus-triggered `poll` job. A second cron-triggered job, `poll-bootstrap`, runs `*/30 * * * *` with `WORKER_MODE=poll-bootstrap` and does nothing except probe the trigger queue and publish a jittered seed message if the queue is empty. It never invokes `PollPlanItCommandHandler`.

Dev has no polling job at all. Dev Cosmos retains existing data; new data arrives via a future backfill feature (not in scope here).

The `"poll"` worker mode is removed entirely. `WORKER_MODE` is read without a default — an unset value is a deployment accident and the worker exits 1 via the existing `UnknownWorkerMode` log path.

## Consequences

Easier:
- One canonical polling path in prod. No ambiguity about which job produced a given application.
- Cheaper — no redundant full polls, no dev polling cost.
- Clearer telemetry — three distinct OpenTelemetry activity names (`Polling Cycle (SB)`, `Polling Bootstrap`, per-mode digest/cleanup) make filtering trivial.

Harder:
- A dead SB chain now recovers only on the next cron tick. Worst case: max-delivery-count exhaustion (~50 min) plus time-to-next-bootstrap (~30 min) ≈ 80 min of paused polling. Acceptable given the chain is self-healing under all non-catastrophic failures.
- Dev data goes stale until the backfill feature lands. This is the stated tradeoff — manual testing either uses prod-like seeded data or waits for backfill.

Rejected alternatives documented in `docs/specs/sb-only-polling.md` (gate-via-flag, delete-cron-entirely, tighter cadence, bootstrap-on-API-startup, add-SB-to-dev).
```

- [ ] **Step 2: Commit**

```bash
git add docs/adr/0024-service-bus-only-polling.md
git commit -m "docs(adr): 0024 service bus-only polling"
```

---

## Task 2: Add `poll-bootstrap` switch case to `Program.cs`

Adds the new worker mode without removing the old one yet, so the worker can be deployed as a transitional image if needed. The logic is a thin call through to the already-tested `PollTriggerBootstrapper`.

**Files:**
- Modify: `api/src/town-crier.worker/Program.cs` (switch statement, around lines 225–453)

- [ ] **Step 1: Open the worker `Program.cs`**

The switch sits between `var exitCode = 0;` and `await host.StopAsync()`. Cases are ordered: `"poll"`, `"poll-sb"`, `"digest"`, `"hourly-digest"`, `"dormant-cleanup"`, `default`. Add `"poll-bootstrap"` immediately after the `"poll-sb"` case (which it most closely resembles).

- [ ] **Step 2: Insert the new case**

Add this block between `case "poll-sb":` (which currently ends around line 364) and `case "digest":` (around line 366):

```csharp
case "poll-bootstrap":
    {
        // Bootstrap-only safety net. Probes the Service Bus trigger queue
        // and publishes a jittered seed if empty. Never invokes the poll
        // handler. See docs/specs/sb-only-polling.md and ADR 0024.
        using var bootstrapActivity = PollingInstrumentation.Source.StartActivity("Polling Bootstrap");
        try
        {
            // 60 s is generous for a single Service Bus PeekLock receive
            // + optional publish. The Container Apps replicaTimeout is the
            // hard kill ceiling; this budget is the soft self-cancel.
            using var bootstrapCts = new CancellationTokenSource(TimeSpan.FromSeconds(60));

            var bootstrapper = host.Services.GetRequiredService<PollTriggerBootstrapper>();
            var bootstrapResult = await bootstrapper.TryBootstrapAsync(bootstrapCts.Token)
                .ConfigureAwait(false);

            // Tag names match the legacy safety-net path so existing App
            // Insights queries and dashboards keep working.
            bootstrapActivity?.SetTag("polling.safety_net.bootstrap_published", bootstrapResult.Published);
            bootstrapActivity?.SetTag("polling.safety_net.bootstrap_probe_failed", bootstrapResult.ProbeFailed);
        }
#pragma warning disable CA1031 // Worker must return exit code on any failure
        catch (Exception ex)
#pragma warning restore CA1031
        {
            bootstrapActivity?.AddException(ex);
            bootstrapActivity?.SetStatus(ActivityStatusCode.Error, ex.Message);
            WorkerLog.PollCycleFailed(logger, ex);
            exitCode = 1;
        }

        break;
    }
```

- [ ] **Step 3: Build the solution**

```bash
cd api && dotnet build
```

Expected: build succeeds; no new warnings.

- [ ] **Step 4: Run the full test suite**

```bash
cd api && dotnet test
```

Expected: all existing tests pass. No new tests yet — the new case is verified by its downstream dependencies (`PollTriggerBootstrapperTests.cs`, `SafetyNetBootstrapIntegrationTests.cs`) which were not changed.

- [ ] **Step 5: Commit**

```bash
git add api/src/town-crier.worker/Program.cs
git commit -m "feat(worker): add poll-bootstrap mode"
```

---

## Task 3: Rename the integration test file

Renames the class to reflect the new single-purpose job. Assertions do not change — the bootstrapper's behaviour is unchanged.

**Files:**
- Rename: `api/tests/town-crier.integration-tests/Polling/SafetyNetBootstrapIntegrationTests.cs` → `PollBootstrapIntegrationTests.cs`
- Modify (post-rename): class name and XML doc comment inside the file.

- [ ] **Step 1: Rename the file**

```bash
git mv api/tests/town-crier.integration-tests/Polling/SafetyNetBootstrapIntegrationTests.cs \
       api/tests/town-crier.integration-tests/Polling/PollBootstrapIntegrationTests.cs
```

- [ ] **Step 2: Update the class name and XML doc**

Edit the renamed file so lines 11–22 read:

```csharp
/// <summary>
/// End-to-end wiring for the bootstrap-only safety-net job (ADR 0024).
/// Exercises the REAL <see cref="PollTriggerBootstrapper"/>, REAL
/// <see cref="ServiceBusPollTriggerQueue"/>, and REAL
/// <see cref="PollNextRunScheduler"/> backed by a fake
/// <see cref="IServiceBusRestClient"/> at the transport boundary.
/// </summary>
[SuppressMessage(
    "Minor Code Smell",
    "S1075:URIs should not be hardcoded",
    Justification = "Test fixture URIs.")]
public sealed class PollBootstrapIntegrationTests
{
```

All three `[Test]` methods, field values, and helper classes (`ZeroJitter`, `FakeTimeProvider`) remain unchanged.

- [ ] **Step 3: Run the renamed test file**

```bash
cd api && dotnet test --filter "FullyQualifiedName~PollBootstrapIntegrationTests"
```

Expected: three tests pass (`Should_PublishBootstrapTrigger_When_QueueIsEmpty`, `Should_AbandonAndSkipPublish_When_QueueAlreadyHasMessage`, `Should_ReturnFailure_WithoutThrowing_When_TransportFails`).

- [ ] **Step 4: Commit**

```bash
git add api/tests/town-crier.integration-tests/Polling/PollBootstrapIntegrationTests.cs
git commit -m "test(polling): rename safety-net bootstrap integration tests"
```

---

## Task 4: Remove the legacy `poll` case and drop the `WORKER_MODE` default

With the bootstrap case landed and the integration test renamed, the timer-based poll branch can go. Removing the configuration default guards against accidental silent deployment with an unset `WORKER_MODE`.

**Files:**
- Modify: `api/src/town-crier.worker/Program.cs`

- [ ] **Step 1: Delete the `case "poll":` block**

Remove the entire block from `case "poll":` (currently around line 227) through `break;` (currently around line 306). This deletes the full poll cycle, the cycle-budget calculation specific to this mode, the lease-held check, and the post-cycle `PollTriggerBootstrapper.TryBootstrapAsync` call (that logic now lives in `case "poll-bootstrap":`).

- [ ] **Step 2: Remove the `WORKER_MODE` default**

Locate the line that currently reads:

```csharp
var mode = builder.Configuration["WORKER_MODE"] ?? "poll";
```

Replace with:

```csharp
var mode = builder.Configuration["WORKER_MODE"];
if (string.IsNullOrEmpty(mode))
{
    WorkerLog.UnknownWorkerMode(logger, "<unset>");
    return 1;
}
```

This makes an unset `WORKER_MODE` fail fast instead of silently falling into a now-deleted case.

- [ ] **Step 3: Build the solution**

```bash
cd api && dotnet build
```

Expected: build succeeds.

- [ ] **Step 4: Run the full test suite**

```bash
cd api && dotnet test
```

Expected: all tests pass. The poll handler, orchestrator, bootstrapper, and scheduler are all exercised by other tests that don't depend on the `"poll"` worker mode.

- [ ] **Step 5: Dead-code grep**

```bash
grep -rn '"poll"' api/src
```

Expected: no results inside `Program.cs`. Other hits (e.g. `"poll-sb"`, `"poll-bootstrap"`, `QueueName = "poll"`) are fine. If anything else is literally `"poll"`, inspect and decide.

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.worker/Program.cs
git commit -m "refactor(worker): remove legacy poll mode and WORKER_MODE default"
```

---

## Task 5: Update Pulumi infra

Renames the cron job, sets its worker mode, tightens its replica timeout, and deletes the dev poll job entirely.

**Files:**
- Modify: `infra/EnvironmentStack.cs` (roughly lines 363–388 — the `if (pollingBus is not null) { ... } else { ... }` block that provisions poll jobs).

- [ ] **Step 1: Locate the polling-job block**

The block is inside the main resource provisioning method. It currently reads (approximate — use the surrounding `// Container Apps Jobs — polling and digest workers share the same shape,` comment to locate it):

```csharp
if (pollingBus is not null)
{
    _ = CreateWorkerJob("poll", cronExpression: null, replicaTimeout: 600, workerMode: "poll-sb",
        env, resourceGroup.Name, containerAppsEnvironmentId,
        acrLoginServer, acrPullIdentityId, cosmosDataIdentityId,
        cosmosAccountEndpoint, cosmosDatabase.Name, cosmosDataIdentityClientId,
        appInsightsConnectionString, acsConnectionString, tags,
        pollingBus);

    _ = CreateWorkerJob("poll-safety-net", cronExpression: "*/30 * * * *", replicaTimeout: 600, workerMode: null,
        env, resourceGroup.Name, containerAppsEnvironmentId,
        acrLoginServer, acrPullIdentityId, cosmosDataIdentityId,
        cosmosAccountEndpoint, cosmosDatabase.Name, cosmosDataIdentityClientId,
        appInsightsConnectionString, acsConnectionString, tags,
        pollingBus);
}
else
{
    var pollCron = env == "dev" ? "0 */3 * * *" : "*/15 * * * *";
    _ = CreateWorkerJob("poll", pollCron, replicaTimeout: 600, workerMode: null,
        env, resourceGroup.Name, containerAppsEnvironmentId,
        acrLoginServer, acrPullIdentityId, cosmosDataIdentityId,
        cosmosAccountEndpoint, cosmosDatabase.Name, cosmosDataIdentityClientId,
        appInsightsConnectionString, acsConnectionString, tags,
        pollingBus: null);
}
```

- [ ] **Step 2: Rewrite the block**

Replace the entire `if (pollingBus is not null) { ... } else { ... }` construct with:

```csharp
// Polling jobs exist only when the SB chain is provisioned (prod only).
// The event-triggered "poll" job services the SB chain end-to-end.
// The cron-triggered "poll-bootstrap" job re-seeds the queue if empty
// — see docs/adr/0024-service-bus-only-polling.md.
if (pollingBus is not null)
{
    _ = CreateWorkerJob("poll", cronExpression: null, replicaTimeout: 600, workerMode: "poll-sb",
        env, resourceGroup.Name, containerAppsEnvironmentId,
        acrLoginServer, acrPullIdentityId, cosmosDataIdentityId,
        cosmosAccountEndpoint, cosmosDatabase.Name, cosmosDataIdentityClientId,
        appInsightsConnectionString, acsConnectionString, tags,
        pollingBus);

    _ = CreateWorkerJob("poll-bootstrap", cronExpression: "*/30 * * * *", replicaTimeout: 120, workerMode: "poll-bootstrap",
        env, resourceGroup.Name, containerAppsEnvironmentId,
        acrLoginServer, acrPullIdentityId, cosmosDataIdentityId,
        cosmosAccountEndpoint, cosmosDatabase.Name, cosmosDataIdentityClientId,
        appInsightsConnectionString, acsConnectionString, tags,
        pollingBus);
}
```

No `else` branch. When `pollingBus is null` (currently any non-prod env), no poll jobs are created.

- [ ] **Step 3: Build infra**

```bash
cd infra && dotnet build
```

Expected: build succeeds.

- [ ] **Step 4: Preview the Pulumi plan (optional local sanity check)**

If Pulumi is set up locally:

```bash
cd infra && pulumi preview --stack dev
```

Expected diff: the dev `poll` job is shown as a delete. No other changes unless the stack is stale.

```bash
cd infra && pulumi preview --stack prod
```

Expected diff: `job-tc-poll-safety-net-prod` delete, `job-tc-poll-bootstrap-prod` create. The `job-tc-poll-prod` (event-triggered) is unchanged.

Do not run `pulumi up`. Deployment is via CI/CD only per project CLAUDE.md.

- [ ] **Step 5: Commit**

```bash
git add infra/EnvironmentStack.cs
git commit -m "feat(infra): replace poll-safety-net with poll-bootstrap; remove dev poll job"
```

---

## Task 6: Update CI deploy-worker-jobs

The bash loop in the reusable deploy action iterates over the exact worker-job names. Missing an update here would break the deployment pipeline.

**Files:**
- Modify: `.github/actions/deploy-worker-jobs/action.yml`

- [ ] **Step 1: Update the loop and its leading comment**

Line 2 currently begins:

```yaml
# Iterates over every worker job (poll, poll-safety-net, digest,
```

Change to:

```yaml
# Iterates over every worker job (poll, poll-bootstrap, digest,
```

Line 31 currently reads:

```yaml
        for JOB in poll poll-safety-net digest digest-hourly dormant-cleanup; do
```

Change to:

```yaml
        for JOB in poll poll-bootstrap digest digest-hourly dormant-cleanup; do
```

- [ ] **Step 2: Verify no other CI references remain**

```bash
grep -rn "poll-safety-net" .github/
```

Expected: no results.

- [ ] **Step 3: Commit**

```bash
git add .github/actions/deploy-worker-jobs/action.yml
git commit -m "ci: rename poll-safety-net to poll-bootstrap in deploy loop"
```

---

## Task 7: Ship

Opens a PR and lets the CI deploy the worker image and infra atomically.

- [ ] **Step 1: Push the branch**

```bash
git push -u origin HEAD
```

- [ ] **Step 2: Open a PR via `gh`**

```bash
gh pr create --title "feat: service bus-only polling (ADR 0024)" --body "$(cat <<'EOF'
## Summary
- Adds `WORKER_MODE=poll-bootstrap` that only runs the SB-queue bootstrap probe.
- Removes the legacy `"poll"` worker mode entirely. `WORKER_MODE` is now required (no default).
- Renames `poll-safety-net` job → `poll-bootstrap` in Pulumi and CI; reduces its `replicaTimeout` from 600 s to 120 s.
- Deletes the dev cron `poll` job. Dev data will be refreshed via a future prod→dev backfill feature.
- Adds ADR 0024; references `docs/specs/sb-only-polling.md`.

## Test plan
- [ ] `dotnet build` in `/api` and `/infra`
- [ ] `dotnet test` in `/api`
- [ ] `dotnet test --filter PollBootstrapIntegrationTests` passes
- [ ] `pulumi preview --stack prod` shows `poll-safety-net` delete + `poll-bootstrap` create
- [ ] `pulumi preview --stack dev` shows `poll` delete, no other changes
- [ ] After deploy: `az containerapp job list -g rg-town-crier-prod -o table` shows `job-tc-poll-bootstrap-prod` in place of `poll-safety-net`
- [ ] After deploy: next cron tick executes in < 5 s and logs `"Safety-net skipped reseed — poll trigger queue already has a pending message"` (the SB chain is alive)
EOF
)"
```

- [ ] **Step 3: Watch the PR Gate**

```bash
gh pr checks --watch
```

Expected: all gate checks pass. Auto-merge is handled by the `auto-enable-automerge` workflow.

- [ ] **Step 4: Verify post-deploy**

After CD completes:

```bash
az containerapp job list -g rg-town-crier-prod -o table
az containerapp job execution list -g rg-town-crier-prod -n job-tc-poll-bootstrap-prod --query "[0].properties" -o json
```

Expected: `job-tc-poll-bootstrap-prod` present, recent execution succeeded with activity name `Polling Bootstrap`. `job-tc-poll-prod` (event-triggered) still present and running.

```bash
az containerapp job list -g rg-town-crier-dev -o table
```

Expected: no `job-tc-poll-dev` in the listing.

---

## Self-Review

Ran against the spec `docs/specs/sb-only-polling.md`:

1. **Spec coverage.** Every item in the spec's "Scope — In scope" list maps to a task:
   - `Program.cs` `"poll"` → `"poll-bootstrap"` — Tasks 2 & 4.
   - `EnvironmentStack.cs` rename + dev poll delete — Task 5.
   - Integration test rename + trim — Task 3.
   - `.github/workflows/` update — Task 6.
   - ADR 0024 — Task 1.
2. **Placeholder scan.** No "TBD"/"TODO"/"implement later" in the plan. Every code block is complete.
3. **Type consistency.** `PollTriggerBootstrapper.TryBootstrapAsync` signature matches existing code; `PollingInstrumentation.Source`, `WorkerLog.PollCycleFailed`, `WorkerLog.UnknownWorkerMode`, and `PollTriggerBootstrapResult` are all existing types referenced in Task 2 and Task 4 with their correct names.
4. **Ambiguity check.** The only judgement call is the 60-second bootstrap CancellationTokenSource budget in Task 2 — documented inline with the "why 60 s" comment so the implementing engineer doesn't second-guess it.
5. **Out-of-scope guard.** Tasks do not touch `PollTriggerOrchestrator`, `PollPlanItCommandHandler`, `PollNextRunScheduler`, `PollTriggerBootstrapper`, `PollingOptions`, or any adapter beneath `Infrastructure/Polling/` — in line with the spec's "Out of scope" list.
