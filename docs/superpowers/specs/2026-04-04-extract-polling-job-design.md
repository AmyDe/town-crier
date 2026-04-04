# Extract Polling into Container Apps Job

Date: 2026-04-04

## Problem

The PlanIt polling service runs as a `BackgroundService` inside the API container. This forces `MinReplicas=1` to keep polling alive, meaning a container runs 24/7 even with near-zero traffic. The polling only needs a few seconds every 15 minutes.

## Decision

Extract polling into a dedicated Azure Container Apps Job (cron-triggered). The API container reverts to `MinReplicas=0` (scale to zero). A new `town-crier.worker` console app runs one poll cycle per invocation and exits.

## Architecture

### New project: `town-crier.worker`

A minimal .NET 10 console app in `/api/src/town-crier.worker/`:

- Builds a `HostApplicationBuilder` with only the services needed for polling
- Registers: Cosmos clients, PlanIt HTTP client, `PollPlanItCommandHandler`, `WatchZoneActiveAuthorityProvider`, `CosmosPollStateStore`, OpenTelemetry exporters
- Does NOT register: ASP.NET, auth middleware, routing, health endpoints
- Calls `PollPlanItCommandHandler.HandleAsync()` once, then exits with code 0 (success) or 1 (failure)
- Native AOT compatible (same constraints as the web host)

### Simplified `PollPlanItCommandHandler`

Remove features that don't apply to a stateless cron job:

- **Remove scheduling/prioritisation** — poll all active authorities every run. Drop `PollingSchedule`, `PollingPriority`, `PollingScheduleConfig`, and `cycleNumber` parameter from `PollPlanItCommand`.
- **Remove health tracking** — drop `IPollingHealthStore`, `IPollingHealthAlerter`, `PollingHealthConfig`. Application Insights via OpenTelemetry provides failure visibility.
- **Keep**: authority discovery (`WatchZoneActiveAuthorityProvider`), PlanIt fetching, Cosmos upsert, watch zone matching, notification enqueue, `PollingMetrics`, `PollingInstrumentation`.

### Poll state persistence: `CosmosPollStateStore`

Replace `FilePollStateStore` with a Cosmos DB-backed implementation:

- New container: `PollState` in the shared Cosmos account (or reuse an existing metadata container)
- Single document with well-known ID (e.g., `{ "id": "poll-state", "lastPollTime": "2026-04-04T19:00:00Z" }`)
- Partition key: `id` (single document, single partition)
- Implements existing `IPollStateStore` interface
- Read on startup, write after successful cycle completion

### Infrastructure changes (Pulumi)

In `EnvironmentStack.cs`:

- **Revert** `MinReplicas` from 1 to 0 on the API Container App
- **Add** `Pulumi.AzureNative.App.Job` resource:
  - Name: `job-town-crier-poll-{env}`
  - Environment: same shared Container Apps Environment (`cae-town-crier-shared`)
  - Trigger: Cron, `*/15 * * * *`
  - Image: `{acr}/town-crier-worker:{tag}` (separate image from API)
  - CPU: 0.25, Memory: 0.5Gi
  - Replica timeout: 300 seconds (5 min — poll cycle should complete well within this)
  - Managed identities: same `acrPullIdentity` and `cosmosDataIdentity` as the API
  - Environment variables: Cosmos endpoint, database name, Application Insights connection string, AZURE_CLIENT_ID
  - Does NOT need: Auth0 config, CORS, Admin API key

### Dockerfile

New `api/Dockerfile.worker`:

- Same multi-stage pattern as `api/Dockerfile`
- Builds `town-crier.worker` project instead of `town-crier.web`
- Same Alpine base, non-root user, Native AOT publish

### CI/CD changes

In both `cd-dev.yml` and `cd-prod.yml`:

- **Add worker image build step**: `docker build -f Dockerfile.worker -t {acr}/town-crier-worker:{sha}` 
- **Add worker deploy step**: `az containerapp job start` or rely on Pulumi to update the Job image (same `IgnoreChanges` pattern as the API container)
- Worker build can run in parallel with API build

### Remove from API web host

- Delete `PlanItPollingService.cs` (the `BackgroundService`)
- Remove `AddHostedService<PlanItPollingService>()` from `Program.cs`
- Remove polling-specific DI registrations from `ServiceCollectionExtensions.cs` (poll state store, health store, health alerter, health config, schedule config, notification enqueuer)
- Keep shared infrastructure that the API still uses (Cosmos clients, watch zone repo, etc.)

### Code to delete

These types are no longer needed anywhere:

- `PollingSchedule` (domain)
- `PollingPriority` (domain)  
- `PollingScheduleConfig` (domain)
- `InMemoryPollingHealthStore` (infrastructure)
- `LogPollingHealthAlerter` (infrastructure)
- `InMemoryActiveAuthorityProvider` (infrastructure — test-only, unused in prod)
- `FilePollStateStore` (infrastructure — replaced by Cosmos)
- `PollingHealthConfig` (application)
- `PlanItPollingService` (web)

### Code to keep (used by worker)

- `PollPlanItCommandHandler` (simplified)
- `PollPlanItCommand` (simplified — remove `CycleNumber`)
- `PollPlanItResult`
- `WatchZoneActiveAuthorityProvider`
- `PollingMetrics`
- `PollingInstrumentation`
- `PollingHealth` domain entity — review: if health tracking is fully removed, this can go too
- `IPollStateStore` interface + new `CosmosPollStateStore`
- All repository interfaces and Cosmos implementations

## Testing

- Existing `PollPlanItCommandHandler` tests updated to remove scheduling/health assertions
- New `CosmosPollStateStore` tests (integration-style with fake Cosmos client)
- Worker `Program.cs` is thin enough to not need its own tests — the handler is the unit under test

## Follow-up

- `tc-c1b`: Update cron to `0 */3 * * *` once confirmed working
- `tc-fvu`: Can be closed — extracting to a Job with Single revision semantics resolves the duplicate polling concern
