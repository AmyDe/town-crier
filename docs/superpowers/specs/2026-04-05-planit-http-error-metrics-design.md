# PlanIt HTTP Error Metrics Design

Date: 2026-04-05

## Goal

Surface every non-2xx HTTP response from PlanIt as an OTel counter, tagged by status code and authority. Add two Azure dashboard tiles: one dedicated to 429 rate limits (primary early-warning signal), and one for all other HTTP errors.

## Metric

| Field | Value |
|-------|-------|
| Meter | `TownCrier.PlanIt` |
| Counter | `towncrier.planit.http_errors` (long) |
| Tag: `http.response.status_code` | Numeric status code (e.g. `429`, `500`) |
| Tag: `planit.authority_code` | Authority ID the request was for |

Every non-2xx response is counted **at the point of receipt**, before retry logic. A request that receives three 429s before succeeding produces 3 counts.

## Implementation

### 1. PlanItInstrumentation class

New file: `api/src/town-crier.infrastructure/Observability/PlanItInstrumentation.cs`

Follows the `CosmosInstrumentation` pattern:

```csharp
public static class PlanItInstrumentation
{
    public const string MeterName = "TownCrier.PlanIt";
    private static readonly Meter Meter = new(MeterName);

    public static readonly Counter<long> HttpErrors =
        Meter.CreateCounter<long>(
            "towncrier.planit.http_errors",
            description: "Non-2xx HTTP responses from PlanIt API");
}
```

### 2. Instrumentation point in PlanItClient

`SendWithRetryAsync` is the single method that sends HTTP requests. It already inspects the status code for 429 handling. Changes:

- Add `int authorityId` parameter to `SendWithRetryAsync`.
- Record the metric for **every** non-2xx response before retry/return:

```
SendWithRetryAsync(Uri url, int authorityId, CancellationToken ct)
    for each attempt:
        response = httpClient.GetAsync(url)

        if response is not success:
            PlanItInstrumentation.HttpErrors.Add(1,
                ("http.response.status_code", (int)response.StatusCode),
                ("planit.authority_code", authorityId))

        if response is not 429:
            return response          // caller calls EnsureSuccessStatusCode()

        // existing 429 retry logic continues unchanged
```

This means:
- 429s are counted on every retry attempt, then existing backoff logic runs.
- Other non-2xx (400, 500, 502, etc.) are counted once, then returned to the caller which throws via `EnsureSuccessStatusCode()`.
- 2xx responses are returned without recording.

Call sites updated to pass `authorityId` through:
- `FetchApplicationsAsync` line 47: `SendWithRetryAsync(url, authorityId, ct)`
- `SearchApplicationsAsync` line 79: `SendWithRetryAsync(url, authorityId, ct)`

### 3. OTel meter registration

Add `PlanItInstrumentation.MeterName` to both:
- `api/src/town-crier.worker/Program.cs` — alongside existing `PollingMetrics` and `CosmosInstrumentation` meters
- `api/src/town-crier.web/Program.cs` — alongside existing meters

### 4. Dashboard tiles

Two new tiles in `infra/SharedStack.cs`, added as row 4 below the existing 3-row layout:

| Position | Metric | Filter | Title |
|----------|--------|--------|-------|
| X=0, Y=12, ColSpan=6, RowSpan=4 | `towncrier.planit.http_errors` | `http.response.status_code == 429` | PlanIt 429s |
| X=6, Y=12, ColSpan=6, RowSpan=4 | `towncrier.planit.http_errors` | `http.response.status_code != 429` | PlanIt Errors |

Both use KQL tiles (not the simple `MetricTile` helper) because they need a `where` filter on the status code dimension. The KQL queries:

**PlanIt 429s:**
```kql
customMetrics
| where name == "towncrier.planit.http_errors"
| extend status = tostring(customDimensions["http.response.status_code"])
| where status == "429"
| summarize Value=sum(value) by timestamp=bin(timestamp, 1h)
| render timechart
```

**PlanIt Errors:**
```kql
customMetrics
| where name == "towncrier.planit.http_errors"
| extend status = tostring(customDimensions["http.response.status_code"])
| where status != "429"
| summarize Value=sum(value) by timestamp=bin(timestamp, 1h), status
| render timechart
```

The second query splits by status code so different error types show as separate series.

## Files Changed

| File | Change |
|------|--------|
| `api/src/town-crier.infrastructure/Observability/PlanItInstrumentation.cs` | **New** — meter + counter |
| `api/src/town-crier.infrastructure/PlanIt/PlanItClient.cs` | Add `authorityId` to `SendWithRetryAsync`, record metric on non-2xx |
| `api/src/town-crier.worker/Program.cs` | Register `PlanItInstrumentation.MeterName` |
| `api/src/town-crier.web/Program.cs` | Register `PlanItInstrumentation.MeterName` |
| `infra/SharedStack.cs` | Add two KQL tiles in row 4 |

## Testing

- Unit test `PlanItClient` with a fake `HttpMessageHandler` that returns various status codes (429, 400, 500). Verify the counter increments with correct tags.
- Existing polling handler tests are unaffected — the metric recording is internal to `PlanItClient`.
