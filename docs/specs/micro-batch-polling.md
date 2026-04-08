# Micro-Batch Round-Robin Polling

Date: 2026-04-08

## Problem

The poll worker tries to sync all active authorities in a single 10-minute run. PlanIt (a free, hobby-run API) rate-limits after ~6 requests per burst with `Retry-After` values of 5-9 minutes. The worker honours the header literally, sits idle, and gets killed by the 600s `replicaTimeout`. Result: 33+ hours of zero successful sync runs, a flatlined dashboard, and unnecessary load on PlanIt.

## Design Principles

- **Be a good citizen.** PlanIt is a free service run by one person. When it says "slow down," stop — don't retry, don't work around it.
- **Let PlanIt set the pace.** The 429 is the natural batch boundary. No artificial batch size cap.
- **Least-recently-synced first.** Fair coverage across all authorities regardless of how many there are.
- **Short, predictable runs.** Finish fast, flush metrics, exit cleanly.

## Schedule & Container Config

| Setting | Current | New |
|---------|---------|-----|
| Cron | `0 * * * *` (hourly) | `*/15 * * * *` (every 15 min) |
| replicaTimeout | 600s | 120s |
| CPU / Memory | 0.25 / 0.5Gi | unchanged |

## Round-Robin Ordering

A new method on `IPollStateStore`:

```csharp
Task<IReadOnlyList<int>> GetLeastRecentlyPolledAsync(
    IReadOnlyList<int> candidateAuthorityIds,
    CancellationToken ct);
```

Implementation: query PollState container with `SELECT * FROM c WHERE c.authorityId IN (...)` for the candidate IDs, sort by `lastPollTime` ascending in memory. Authorities in the candidate set but absent from PollState sort first (never been polled). Return the full sorted list — the handler iterates until PlanIt stops it.

At 100 authorities this is a single cross-partition query (~3-5 RU). The candidate set comes from `activeAuthorityProvider.GetActiveAuthorityIdsAsync()` which still supplies the authority IDs that have watch zones.

## Polling Loop

```
activeIds = activeAuthorityProvider.GetActiveAuthorityIdsAsync()
sortedIds = pollStateStore.GetLeastRecentlyPolledAsync(activeIds)

for each authorityId in sortedIds:
    try:
        fetch all pages of applications (with 2s throttle between requests)
        upsert each to Cosmos
        match against watch zones, enqueue notifications
        save lastPollTime for this authority
        record metrics (authorities_polled, applications_ingested)
    catch HttpRequestException(429):
        record rate_limited metric
        break — exit the loop, do not process remaining authorities
    catch other Exception:
        log error, record authorities_skipped metric
        continue to next authority
```

The 429 catch is on the handler, not the client. The client throws immediately on 429 — no retries.

## PlanItClient Changes

`SendWithRetryAsync` is replaced with a simple send-with-throttle:

```csharp
private async Task<HttpResponseMessage> SendWithThrottleAsync(Uri url, int authorityId, CancellationToken ct)
{
    if (throttleOptions.DelayBetweenRequests > TimeSpan.Zero)
        await delayFunc(throttleOptions.DelayBetweenRequests, ct);

    var response = await httpClient.GetAsync(url, ct);

    if (!response.IsSuccessStatusCode)
        PlanItInstrumentation.HttpErrors.Add(1, ...);

    return response;
}
```

`FetchApplicationsAsync` calls this instead of `SendWithRetryAsync`. On a 429 response, `EnsureSuccessStatusCode()` throws `HttpRequestException` with `StatusCode = 429`, which propagates up to the handler's catch block.

### Removed

- `SendWithRetryAsync` (retry loop, backoff calculation)
- `ParseRetryAfterHeader`
- `CalculateBackoffDelay`
- `PlanItRetryOptions` (MaxRetries, BaseDelay)
- `PlanItPollingOptions.RateLimitCooldown`
- The `rateLimitHitCount` / "break after 2nd 429" logic in the handler

### Retained

- `PlanItThrottleOptions.DelayBetweenRequests` (2s default) — pacing between requests
- `PlanItInstrumentation.HttpErrors` — records all non-2xx responses with status code and authority
- Per-authority error isolation in the handler (catch non-429 exceptions, log, continue)

## Exit Behaviour

The handler returns normally after the loop (whether it completed all authorities or broke on a 429). Back in `Program.cs`:

- Record `PollingMetrics.CycleDuration`
- If the loop broke due to 429: record `PollingMetrics.RateLimited` counter
- `ForceFlush` MeterProvider and TracerProvider
- Exit code 0

The job always exits 0 unless an unexpected exception escapes the top-level catch. Container Apps marks it as Succeeded.

## Metrics

### New

| Metric | Type | Description |
|--------|------|-------------|
| `towncrier.polling.rate_limited` | Counter<long> | Incremented when a run stops due to PlanIt 429 |

### Existing (unchanged)

| Metric | Type |
|--------|------|
| `towncrier.polling.authorities_polled` | Counter<long> |
| `towncrier.polling.authorities_skipped` | Counter<long> |
| `towncrier.polling.applications_ingested` | Counter<long> |
| `towncrier.polling.failures` | Counter<long> |
| `towncrier.polling.authority_processing_ms` | Histogram<double> |
| `towncrier.polling.cycle_duration_ms` | Histogram<double> |

### OTel Export Interval

Reduce the Azure Monitor metric export interval from the default 30s to 5s for the worker:

```csharp
metrics.AddAzureMonitorMetricExporter(o => o.ExportIntervalMilliseconds = 5_000);
```

This ensures metrics are streamed out during execution rather than relying solely on `ForceFlush` at exit.

## Throughput Model

| Scenario | Requests/run | Runs/day | Authorities/day |
|----------|-------------|----------|-----------------|
| Steady state (1 page/authority) | ~6 | 96 | ~576 |
| Mixed (some multi-page) | ~6 | 96 | ~200-400 |
| First sync (30-day backlog) | ~6 | 96 | fewer until backlog drains |

576 single-page authorities per day comfortably covers the target of 100+ authorities. Multi-page authorities and first syncs temporarily reduce coverage but the round-robin ensures no authority is starved.

## Infrastructure Change

In `infra/EnvironmentStack.cs`, update the poll job definition:

- `CronExpression`: `"0 * * * *"` → `"*/15 * * * *"`
- `ReplicaTimeout`: `600` → `120`

## What This Spec Does NOT Cover

- Paid-member priority ordering (future enhancement — requires cross-container join with user subscriptions)
- Alerting on sustained rate limiting (can be built from the `rate_limited` metric later)
- Removing `DeleteGlobalPollStateAsync` (legacy cleanup — separate housekeeping task)
