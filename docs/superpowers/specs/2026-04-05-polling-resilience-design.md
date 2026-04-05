# Polling Worker Resilience

Date: 2026-04-05

## Context

The Container Apps Job (`job-town-crier-poll-{env}`) that polls PlanIt every 15 minutes has a 100% failure rate since its creation (59 consecutive failures on dev, 4 on prod). Root cause investigation via Log Analytics revealed:

1. **PlanIt rate-limits the worker (HTTP 429)** after ~2,400 successful requests in a burst
2. **Crash prevents saving progress** — poll state and metrics are recorded after the entire loop, so a mid-loop exception loses all progress
3. **Cascading re-fetch** — without saved progress, the next cycle re-fetches everything from scratch, burning the same rate budget again
4. **Zombie prod revision (0000023)** still running the old `PlanItPollingService` background service, competing for PlanIt rate budget from the same outbound IP
5. **No OTel flush** — the short-lived worker exits before the batch exporter flushes, making failures invisible in App Insights
6. **Excessive frequency** — polling every 15 minutes is unnecessary and amplifies rate-limit pressure

## Design

### 1. Throttle PlanIt requests

Add a configurable delay between each PlanIt HTTP request. Default: 1 second.

**Changes:**
- New `PlanItThrottleOptions` record with `DelayBetweenRequests` property (default `TimeSpan.FromSeconds(1)`)
- Inject into `PlanItClient` constructor
- Call `await Task.Delay(DelayBetweenRequests, ct)` before each `httpClient.GetAsync()` in `SendWithRetryAsync`
- Register in worker DI as singleton

This throttles both page fetches within an authority and the first request for each new authority.

### 2. Save progress per-authority

Move poll state persistence and metric recording inside the foreach loop in `PollPlanItCommandHandler.HandleAsync`.

**Changes:**
- After each authority completes successfully, call `pollStateStore.SaveLastPollTimeAsync(now, ct)`
- Record `PollingMetrics.ApplicationsIngested` per authority (already done)
- Record `PollingMetrics.AuthoritiesPolled.Add(1)` per authority instead of batch at end
- On next cycle, `GetLastPollTimeAsync()` returns the timestamp from the last successful authority, so we don't re-fetch data we already have

### 3. Handle 429 with skip-then-stop

Wrap each authority poll in a try/catch inside the foreach loop.

**Changes:**
- Catch `HttpRequestException` where `StatusCode == 429` inside the foreach loop
- Track a `rateLimitHitCount` counter
- On first 429: log warning with authority ID, increment `PollingMetrics.AuthoritiesSkipped`, continue to next authority
- On second 429: log error, break out of loop (progress already saved per fix #2)
- Non-429 exceptions propagate as before (crash the worker)
- Exit code 0 when cycle completes (even with skips); exit code 1 only on unhandled exceptions

### 4. Kill zombie revision

Deactivate `ca-town-crier-api-prod--0000023` which predates the polling extraction (PR #158) and is still running the old `PlanItPollingService` background service with 1 replica.

**Changes:**
- Run `az containerapp revision deactivate` for revision 0000023
- One-off operational fix, no code change

### 5. Add OTel ForceFlush before exit

Ensure telemetry reaches App Insights before the short-lived worker process exits.

**Changes:**
- After the try/catch block in `Program.cs`, resolve `MeterProvider` and `TracerProvider` from DI
- Call `ForceFlush(TimeSpan.FromSeconds(10))` on both before returning the exit code
- Wrap in try/catch to avoid masking the original error if flush fails

### 6. Change cron schedule to hourly

Reduce polling frequency from every 15 minutes to every hour.

**Changes:**
- Update `EnvironmentStack.cs` cron expression from `*/15 * * * *` to `0 * * * *`
- Update `ReplicaTimeout` from 300 to 600 seconds (10 minutes) to accommodate throttled polling of many authorities

## Testing

- **Throttle**: Unit test that `SendWithRetryAsync` delays between requests (inject fake delay function, verify call count and timing)
- **Per-authority save**: Unit test that poll state is saved after each authority, and that a crash mid-loop preserves prior progress
- **429 handling**: Unit test that first 429 skips authority and continues; second 429 breaks the loop; metrics are recorded correctly
- **OTel flush**: Verify `ForceFlush` is called in the success and failure paths (integration-level, or just code review)
- **Cron schedule**: Verify via `az containerapp job show` after deployment

## Non-goals

- Adaptive rate limiting (start fast, slow on 429) — rejected in favour of fixed conservative throttle
- Per-authority poll state tracking (tracking which specific authorities were polled) — the global timestamp is sufficient since PlanIt's `different_start` parameter handles this
- Retry budget sharing across environments — dev and prod are independent
