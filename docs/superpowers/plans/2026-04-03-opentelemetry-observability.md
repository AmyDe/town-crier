# OpenTelemetry Observability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add full-stack observability (traces, metrics, logs) via OpenTelemetry, exported to Azure Monitor / Application Insights.

**Architecture:** Manual OTEL wiring with AOT-compatible packages. Three .NET meters (Api, Polling, Cosmos) and two ActivitySources (Polling, Cosmos) feed into Azure Monitor Exporter. React frontend uses App Insights JS SDK with cross-origin trace correlation.

**Tech Stack:** OpenTelemetry .NET 1.15, Azure.Monitor.OpenTelemetry.Exporter 1.7, @microsoft/applicationinsights-web, Pulumi AzureNative (Application Insights), TUnit

**Spec:** `docs/superpowers/specs/2026-04-03-opentelemetry-observability-design.md`

---

## File Structure

### Create
| File | Responsibility |
|------|---------------|
| `api/src/town-crier.infrastructure/Observability/CosmosInstrumentation.cs` | ActivitySource + Metrics for Cosmos REST calls |
| `api/src/town-crier.application/Observability/PollingInstrumentation.cs` | ActivitySource for polling pipeline spans |
| `api/src/town-crier.application/Observability/PollingMetrics.cs` | Metrics counters/histograms for polling pipeline |
| `api/src/town-crier.application/Observability/ApiMetrics.cs` | Business metrics (watch zones, notifications, subscriptions) |
| `api/tests/town-crier.web.tests/Observability/ErrorResponseMiddlewareTests.cs` | Tests for updated error middleware (exception logging) |
| `web/src/telemetry.ts` | App Insights JS SDK initialization |
| `web/src/components/ErrorBoundary.tsx` | React error boundary reporting to App Insights |
| `web/src/components/ErrorBoundary.module.css` | Styles for error fallback UI |

### Modify
| File | Change |
|------|--------|
| `infra/SharedStack.cs` | Add Application Insights resource, export connection string |
| `infra/EnvironmentStack.cs` | Pass App Insights connection string to Container App env vars |
| `api/src/town-crier.web/town-crier.web.csproj` | Add 4 OTEL NuGet packages |
| `api/src/town-crier.web/Program.cs` | Wire OTEL traces/metrics/logs, remove `AddJsonConsole()` |
| `api/src/town-crier.web/Extensions/WebApplicationExtensions.cs` | Remove CorrelationId + RequestLogging middleware |
| `api/src/town-crier.web/Observability/ErrorResponseMiddleware.cs` | Add exception logging, remove correlation ID dependency |
| `api/src/town-crier.web/Observability/ErrorResponse.cs` | Remove CorrelationId field |
| `api/src/town-crier.web/Observability/ObservabilityJsonSerializerContext.cs` | Unchanged (still serializes ErrorResponse) |
| `api/src/town-crier.infrastructure/Cosmos/CosmosRestClient.cs` | Wrap each method with Activity spans + metrics recording |
| `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs` | Add authority-level spans + metrics recording |
| `api/src/town-crier.web/Polling/PlanItPollingService.cs` | Add root cycle span |
| `api/tests/town-crier.web.tests/DependencyInjection/ProgramDecompositionTests.cs` | Remove correlation ID assertions |
| `.github/workflows/cd-dev.yml` | Add APPLICATIONINSIGHTS_CONNECTION_STRING env var to API deploy + web build |
| `.github/workflows/cd-prod.yml` | Add APPLICATIONINSIGHTS_CONNECTION_STRING env var to API deploy + web build |
| `web/package.json` | Add @microsoft/applicationinsights-web dependency |
| `web/src/main.tsx` | Import and initialize telemetry before React render |
| `web/src/App.tsx` | Wrap app in ErrorBoundary |

### Delete
| File | Reason |
|------|--------|
| `api/src/town-crier.web/Observability/CorrelationIdMiddleware.cs` | Replaced by OTEL W3C traceparent |
| `api/src/town-crier.web/Observability/RequestLoggingMiddleware.cs` | Replaced by OTEL AspNetCore instrumentation |
| `api/tests/town-crier.web.tests/Observability/CorrelationIdMiddlewareTests.cs` | Tests for removed middleware |
| `api/tests/town-crier.web.tests/Observability/RequestLoggingMiddlewareTests.cs` | Tests for removed middleware |
| `api/tests/town-crier.web.tests/Observability/SpyLoggerProvider.cs` | Only used by removed RequestLogging tests |
| `api/tests/town-crier.web.tests/Observability/ErrorResponseCorrelationIdTests.cs` | Tests correlation ID in error responses (removed feature) |

---

### Task 1: Provision Application Insights (Pulumi)

**Files:**
- Modify: `infra/SharedStack.cs:1-168`
- Modify: `infra/EnvironmentStack.cs:262-269`

- [ ] **Step 1: Add Application Insights using statement to SharedStack.cs**

Add the Insights import at the top of the file:

```csharp
using Pulumi.AzureNative.Insights;
```

Add after the existing `using` statements (line 10).

- [ ] **Step 2: Add Application Insights resource to SharedStack.cs**

Add after the `logAnalytics` workspace block (after line 81) and before `logAnalyticsSharedKeys`:

```csharp
// Application Insights (shared, backed by Log Analytics)
var appInsights = new Component("appi-town-crier-shared", new ComponentArgs
{
    ResourceName = "appi-town-crier-shared",
    ResourceGroupName = resourceGroup.Name,
    WorkspaceResourceId = logAnalytics.Id,
    ApplicationType = "web",
    Kind = "web",
    IngestionMode = IngestionMode.LogAnalytics,
    Tags = tags,
});
```

- [ ] **Step 3: Export the connection string from SharedStack.cs**

Add to the return dictionary (before the closing `};` on line 166):

```csharp
["appInsightsConnectionString"] = appInsights.ConnectionString,
```

- [ ] **Step 4: Pass connection string to Container App in EnvironmentStack.cs**

Read the new shared stack output. Add after the existing shared stack output reads (around line 45):

```csharp
var appInsightsConnectionString = shared.GetOutput("appInsightsConnectionString").Apply(o => o?.ToString() ?? "");
```

Add to the Container App's `Env` array (after line 268, the last `EnvironmentVarArgs`):

```csharp
new EnvironmentVarArgs { Name = "APPLICATIONINSIGHTS_CONNECTION_STRING", Value = appInsightsConnectionString },
```

- [ ] **Step 5: Verify Pulumi builds**

Run: `cd /Users/christy/Dev/town-crier/infra && dotnet build`
Expected: Build succeeded.

- [ ] **Step 6: Commit**

```bash
git add infra/SharedStack.cs infra/EnvironmentStack.cs
git commit -m "infra: provision Application Insights resource and pass connection string to Container App"
```

---

### Task 2: Add OTEL NuGet Packages

**Files:**
- Modify: `api/src/town-crier.web/town-crier.web.csproj`

- [ ] **Step 1: Add the four OTEL packages**

Add to the `<ItemGroup>` containing PackageReferences (after line 22):

```xml
<PackageReference Include="Azure.Monitor.OpenTelemetry.Exporter" Version="1.7.0" />
<PackageReference Include="OpenTelemetry.Extensions.Hosting" Version="1.15.1" />
<PackageReference Include="OpenTelemetry.Instrumentation.AspNetCore" Version="1.15.1" />
<PackageReference Include="OpenTelemetry.Instrumentation.Http" Version="1.15.0" />
```

- [ ] **Step 2: Restore and verify build**

Run: `cd /Users/christy/Dev/town-crier/api && dotnet restore && dotnet build`
Expected: Build succeeded, no AOT warnings from OTEL packages.

- [ ] **Step 3: Commit**

```bash
git add api/src/town-crier.web/town-crier.web.csproj
git commit -m "build: add OpenTelemetry and Azure Monitor Exporter packages"
```

---

### Task 3: Cosmos Repository-Layer Instrumentation

**Files:**
- Create: `api/src/town-crier.infrastructure/Observability/CosmosInstrumentation.cs`
- Modify: `api/src/town-crier.infrastructure/Cosmos/CosmosRestClient.cs`

- [ ] **Step 1: Create CosmosInstrumentation.cs**

Create `api/src/town-crier.infrastructure/Observability/CosmosInstrumentation.cs`:

```csharp
using System.Diagnostics;
using System.Diagnostics.Metrics;

namespace TownCrier.Infrastructure.Observability;

internal static class CosmosInstrumentation
{
    public const string ActivitySourceName = "TownCrier.Cosmos";
    public const string MeterName = "TownCrier.Cosmos";

    public static readonly ActivitySource Source = new(ActivitySourceName);

    private static readonly Meter Meter = new(MeterName);

    public static readonly Histogram<double> RequestCharge =
        Meter.CreateHistogram<double>("towncrier.cosmos.request_charge_ru",
            unit: "RU", description: "Cosmos RU consumption per operation");

    public static readonly Counter<long> Throttles =
        Meter.CreateCounter<long>("towncrier.cosmos.throttles",
            description: "429 responses from Cosmos");
}
```

- [ ] **Step 2: Add instrumentation to CosmosRestClient.ReadDocumentAsync**

Add `using System.Diagnostics;` and `using TownCrier.Infrastructure.Observability;` to the top of `CosmosRestClient.cs`.

Replace the body of `ReadDocumentAsync` (lines 31-57) with:

```csharp
    public async Task<T?> ReadDocumentAsync<T>(
        string collection,
        string id,
        string partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        using var activity = CosmosInstrumentation.Source.StartActivity("Cosmos ReadItem");
        activity?.SetTag("db.system", "cosmosdb");
        activity?.SetTag("db.cosmosdb.container", collection);
        activity?.SetTag("db.operation.name", "ReadItem");

        var encodedId = Uri.EscapeDataString(id);
        var resourceLink = $"dbs/{this.databaseName}/colls/{collection}/docs/{encodedId}";
        using var request = new HttpRequestMessage(HttpMethod.Get, $"/{resourceLink}");
        await this.AddHeadersAsync(request, partitionKey, ct).ConfigureAwait(false);

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

        RecordResponseMetrics(activity, response);

        if (response.StatusCode == HttpStatusCode.NotFound)
        {
            return default;
        }

        response.EnsureSuccessStatusCode();

        var stream = await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false);
        await using (stream.ConfigureAwait(false))
        {
            return await JsonSerializer.DeserializeAsync(stream, typeInfo, ct).ConfigureAwait(false);
        }
    }
```

- [ ] **Step 3: Add instrumentation to CosmosRestClient.UpsertDocumentAsync**

Replace the body of `UpsertDocumentAsync` (lines 59-78) with:

```csharp
    public async Task UpsertDocumentAsync<T>(
        string collection,
        T document,
        string partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        using var activity = CosmosInstrumentation.Source.StartActivity("Cosmos Upsert");
        activity?.SetTag("db.system", "cosmosdb");
        activity?.SetTag("db.cosmosdb.container", collection);
        activity?.SetTag("db.operation.name", "Upsert");

        var resourceLink = $"dbs/{this.databaseName}/colls/{collection}";
        using var request = new HttpRequestMessage(HttpMethod.Post, $"/{resourceLink}/docs");
        await this.AddHeadersAsync(request, partitionKey, ct).ConfigureAwait(false);
        request.Headers.TryAddWithoutValidation("x-ms-documentdb-is-upsert", "True");

        request.Content = new StringContent(
            JsonSerializer.Serialize(document, typeInfo),
            Encoding.UTF8,
            "application/json");

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

        RecordResponseMetrics(activity, response);

        response.EnsureSuccessStatusCode();
    }
```

- [ ] **Step 4: Add instrumentation to CosmosRestClient.DeleteDocumentAsync**

Replace the body of `DeleteDocumentAsync` (lines 80-99) with:

```csharp
    public async Task DeleteDocumentAsync(
        string collection,
        string id,
        string partitionKey,
        CancellationToken ct)
    {
        using var activity = CosmosInstrumentation.Source.StartActivity("Cosmos Delete");
        activity?.SetTag("db.system", "cosmosdb");
        activity?.SetTag("db.cosmosdb.container", collection);
        activity?.SetTag("db.operation.name", "Delete");

        var encodedId = Uri.EscapeDataString(id);
        var resourceLink = $"dbs/{this.databaseName}/colls/{collection}/docs/{encodedId}";
        using var request = new HttpRequestMessage(HttpMethod.Delete, $"/{resourceLink}");
        await this.AddHeadersAsync(request, partitionKey, ct).ConfigureAwait(false);

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

        RecordResponseMetrics(activity, response);

        if (response.StatusCode == HttpStatusCode.NotFound)
        {
            return; // Idempotent delete
        }

        response.EnsureSuccessStatusCode();
    }
```

- [ ] **Step 5: Add instrumentation to CosmosRestClient.QueryAsync**

Add span creation at the start of `QueryAsync` (after line 108, before `var results`):

```csharp
        using var activity = CosmosInstrumentation.Source.StartActivity("Cosmos Query");
        activity?.SetTag("db.system", "cosmosdb");
        activity?.SetTag("db.cosmosdb.container", collection);
        activity?.SetTag("db.operation.name", "Query");
```

Add response metrics recording inside the `do` loop, after `await ThrowOnCosmosErrorAsync(response, sql, ct)` (after line 124):

```csharp
            RecordResponseMetrics(activity, response);
```

- [ ] **Step 6: Add the shared RecordResponseMetrics helper**

Add this private static method at the end of the `CosmosRestClient` class (before the closing `}`):

```csharp
    private static void RecordResponseMetrics(Activity? activity, HttpResponseMessage response)
    {
        var statusCode = (int)response.StatusCode;
        activity?.SetTag("db.cosmosdb.status_code", statusCode);

        if (response.Headers.TryGetValues("x-ms-request-charge", out var ruValues))
        {
            var ruString = ruValues.FirstOrDefault();
            if (ruString is not null && double.TryParse(ruString, out var ru))
            {
                activity?.SetTag("db.cosmosdb.request_charge", ru);
                CosmosInstrumentation.RequestCharge.Record(ru);
            }
        }

        if (statusCode == 429)
        {
            CosmosInstrumentation.Throttles.Add(1);
        }
    }
```

- [ ] **Step 7: Verify build**

Run: `cd /Users/christy/Dev/town-crier/api && dotnet build`
Expected: Build succeeded.

- [ ] **Step 8: Run existing tests**

Run: `cd /Users/christy/Dev/town-crier/api && dotnet test`
Expected: All existing tests pass (instrumentation is additive — spans are no-ops without a listener).

- [ ] **Step 9: Commit**

```bash
git add api/src/town-crier.infrastructure/Observability/CosmosInstrumentation.cs api/src/town-crier.infrastructure/Cosmos/CosmosRestClient.cs
git commit -m "feat(api): add Cosmos repository-layer tracing and RU metrics"
```

---

### Task 4: Polling Pipeline Instrumentation

**Files:**
- Create: `api/src/town-crier.application/Observability/PollingInstrumentation.cs`
- Create: `api/src/town-crier.application/Observability/PollingMetrics.cs`
- Modify: `api/src/town-crier.web/Polling/PlanItPollingService.cs`
- Modify: `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs`

- [ ] **Step 1: Create PollingInstrumentation.cs**

Create `api/src/town-crier.application/Observability/PollingInstrumentation.cs`:

```csharp
using System.Diagnostics;

namespace TownCrier.Application.Observability;

public static class PollingInstrumentation
{
    public const string ActivitySourceName = "TownCrier.Polling";

    public static readonly ActivitySource Source = new(ActivitySourceName);
}
```

- [ ] **Step 2: Create PollingMetrics.cs**

Create `api/src/town-crier.application/Observability/PollingMetrics.cs`:

```csharp
using System.Diagnostics.Metrics;

namespace TownCrier.Application.Observability;

public static class PollingMetrics
{
    public const string MeterName = "TownCrier.Polling";

    private static readonly Meter Meter = new(MeterName);

    public static readonly Counter<long> AuthoritiesPolled =
        Meter.CreateCounter<long>("towncrier.polling.authorities_polled");

    public static readonly Counter<long> AuthoritiesSkipped =
        Meter.CreateCounter<long>("towncrier.polling.authorities_skipped");

    public static readonly Counter<long> ApplicationsIngested =
        Meter.CreateCounter<long>("towncrier.polling.applications_ingested");

    public static readonly Counter<long> PollFailures =
        Meter.CreateCounter<long>("towncrier.polling.failures");

    public static readonly Histogram<double> PlanItLatency =
        Meter.CreateHistogram<double>("towncrier.polling.planit_latency_ms",
            unit: "ms", description: "PlanIt API response time");

    public static readonly Histogram<double> CycleDuration =
        Meter.CreateHistogram<double>("towncrier.polling.cycle_duration_ms",
            unit: "ms", description: "Full polling cycle duration");
}
```

- [ ] **Step 3: Add root cycle span to PlanItPollingService.cs**

Add `using System.Diagnostics;` and `using TownCrier.Application.Observability;` to the top of `PlanItPollingService.cs`.

Replace the `try` block inside the `while` loop (lines 33-45) with:

```csharp
            try
            {
                using var activity = PollingInstrumentation.Source.StartActivity("Polling Cycle");
                activity?.SetTag("polling.cycle_number", this.cycleNumber);
                var cycleStart = Stopwatch.GetTimestamp();

                var scope = this.scopeFactory.CreateAsyncScope();
                await using (scope.ConfigureAwait(false))
                {
                    var handler = scope.ServiceProvider.GetRequiredService<PollPlanItCommandHandler>();

                    var result = await handler.HandleAsync(new PollPlanItCommand(this.cycleNumber), stoppingToken).ConfigureAwait(false);

                    activity?.SetTag("polling.authorities_total", result.TotalActiveAuthorities);
                    activity?.SetTag("polling.authorities_polled", result.AuthoritiesPolled);
                    activity?.SetTag("polling.authorities_skipped", result.AuthoritiesSkipped);
                    activity?.SetTag("polling.applications_ingested", result.ApplicationCount);

                    PollingMetrics.CycleDuration.Record(Stopwatch.GetElapsedTime(cycleStart).TotalMilliseconds);

                    LogPollCycleCompleted(this.logger, result.ApplicationCount, this.cycleNumber, result.AuthoritiesPolled, result.AuthoritiesSkipped);
                }

                this.cycleNumber++;
            }
```

- [ ] **Step 4: Add authority-level spans and metrics to PollPlanItCommandHandler.cs**

Add `using System.Diagnostics;` and `using TownCrier.Application.Observability;` to the top of `PollPlanItCommandHandler.cs`.

Replace the `try` block in `HandleAsync` (lines 66-94) with:

```csharp
        try
        {
            var count = 0;
            foreach (var authorityId in authoritiesToPoll)
            {
                using var authorityActivity = PollingInstrumentation.Source.StartActivity("Poll Authority");
                authorityActivity?.SetTag("polling.authority_code", authorityId);
                var authorityStart = Stopwatch.GetTimestamp();

                var authorityAppCount = 0;
                await foreach (var application in this.planItClient.FetchApplicationsAsync(authorityId, lastPollTime, ct).ConfigureAwait(false))
                {
                    await this.applicationRepository.UpsertAsync(application, ct).ConfigureAwait(false);

                    if (application.Latitude.HasValue && application.Longitude.HasValue)
                    {
                        var matchingZones = await this.watchZoneRepository.FindZonesContainingAsync(
                            application.Latitude.Value, application.Longitude.Value, ct).ConfigureAwait(false);

                        foreach (var zone in matchingZones)
                        {
                            await this.notificationEnqueuer.EnqueueAsync(application, zone, ct).ConfigureAwait(false);
                        }
                    }

                    authorityAppCount++;
                    count++;
                }

                PollingMetrics.PlanItLatency.Record(Stopwatch.GetElapsedTime(authorityStart).TotalMilliseconds);
                PollingMetrics.ApplicationsIngested.Add(authorityAppCount);
                authorityActivity?.SetTag("polling.applications_found", authorityAppCount);
            }

            PollingMetrics.AuthoritiesPolled.Add(polled);
            PollingMetrics.AuthoritiesSkipped.Add(skipped);

            health.RecordSuccess(now);
            await this.pollingHealthStore.SaveAsync(health, ct).ConfigureAwait(false);
            await this.pollStateStore.SaveLastPollTimeAsync(now, ct).ConfigureAwait(false);

            return new PollPlanItResult(count, polled, skipped, totalActive);
        }
```

Replace the `catch` block (lines 95-117) with:

```csharp
        catch
        {
            PollingMetrics.PollFailures.Add(1);

            health.RecordFailure();
            await this.pollingHealthStore.SaveAsync(health, ct).ConfigureAwait(false);

            if (health.HasExceededFailureThreshold(this.healthConfig.MaxConsecutiveFailures))
            {
                await this.pollingHealthAlerter.AlertConsecutiveFailuresAsync(
                    health.ConsecutiveFailureCount, ct).ConfigureAwait(false);
            }

            if (health.IsStale(now, this.healthConfig.StalenessThreshold))
            {
                await this.pollingHealthAlerter.AlertStalenessAsync(
                    health.LastSuccessfulPollTime!.Value,
                    now - health.LastSuccessfulPollTime.Value,
                    ct).ConfigureAwait(false);
            }

            throw;
        }
```

- [ ] **Step 5: Verify build**

Run: `cd /Users/christy/Dev/town-crier/api && dotnet build`
Expected: Build succeeded.

- [ ] **Step 6: Run existing tests**

Run: `cd /Users/christy/Dev/town-crier/api && dotnet test`
Expected: All existing tests pass.

- [ ] **Step 7: Commit**

```bash
git add api/src/town-crier.application/Observability/PollingInstrumentation.cs \
       api/src/town-crier.application/Observability/PollingMetrics.cs \
       api/src/town-crier.web/Polling/PlanItPollingService.cs \
       api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs
git commit -m "feat(api): add polling pipeline tracing and metrics"
```

---

### Task 5: API Business Metrics

**Files:**
- Create: `api/src/town-crier.application/Observability/ApiMetrics.cs`

- [ ] **Step 1: Create ApiMetrics.cs**

Create `api/src/town-crier.application/Observability/ApiMetrics.cs`:

```csharp
using System.Diagnostics.Metrics;

namespace TownCrier.Application.Observability;

public static class ApiMetrics
{
    public const string MeterName = "TownCrier.Api";

    private static readonly Meter Meter = new(MeterName);

    public static readonly Counter<long> WatchZonesCreated =
        Meter.CreateCounter<long>("towncrier.watchzones.created");

    public static readonly Counter<long> NotificationsSent =
        Meter.CreateCounter<long>("towncrier.notifications.sent");

    public static readonly UpDownCounter<long> ActiveSubscriptions =
        Meter.CreateUpDownCounter<long>("towncrier.subscriptions.active");

    public static readonly Counter<long> EndpointErrors =
        Meter.CreateCounter<long>("towncrier.api.errors",
            description: "Unhandled exceptions by endpoint");
}
```

Note: These counters will be wired into command handlers incrementally as those features are exercised. The definitions alone don't change runtime behavior — OTEL ignores unrecorded metrics.

- [ ] **Step 2: Verify build**

Run: `cd /Users/christy/Dev/town-crier/api && dotnet build`
Expected: Build succeeded.

- [ ] **Step 3: Commit**

```bash
git add api/src/town-crier.application/Observability/ApiMetrics.cs
git commit -m "feat(api): define API business metrics (watch zones, notifications, subscriptions)"
```

---

### Task 6: Middleware Cleanup

**Files:**
- Delete: `api/src/town-crier.web/Observability/CorrelationIdMiddleware.cs`
- Delete: `api/src/town-crier.web/Observability/RequestLoggingMiddleware.cs`
- Delete: `api/tests/town-crier.web.tests/Observability/CorrelationIdMiddlewareTests.cs`
- Delete: `api/tests/town-crier.web.tests/Observability/RequestLoggingMiddlewareTests.cs`
- Delete: `api/tests/town-crier.web.tests/Observability/SpyLoggerProvider.cs`
- Delete: `api/tests/town-crier.web.tests/Observability/ErrorResponseCorrelationIdTests.cs`
- Modify: `api/src/town-crier.web/Extensions/WebApplicationExtensions.cs`
- Modify: `api/src/town-crier.web/Observability/ErrorResponseMiddleware.cs`
- Modify: `api/src/town-crier.web/Observability/ErrorResponse.cs`
- Modify: `api/tests/town-crier.web.tests/DependencyInjection/ProgramDecompositionTests.cs`

- [ ] **Step 1: Delete CorrelationIdMiddleware and RequestLoggingMiddleware**

```bash
rm -f api/src/town-crier.web/Observability/CorrelationIdMiddleware.cs
rm -f api/src/town-crier.web/Observability/RequestLoggingMiddleware.cs
```

- [ ] **Step 2: Delete the tests for removed middleware**

```bash
rm -f api/tests/town-crier.web.tests/Observability/CorrelationIdMiddlewareTests.cs
rm -f api/tests/town-crier.web.tests/Observability/RequestLoggingMiddlewareTests.cs
rm -f api/tests/town-crier.web.tests/Observability/SpyLoggerProvider.cs
rm -f api/tests/town-crier.web.tests/Observability/ErrorResponseCorrelationIdTests.cs
```

- [ ] **Step 3: Remove middleware from pipeline in WebApplicationExtensions.cs**

Replace the `UseMiddlewarePipeline` method body (lines 11-19) with:

```csharp
    public static void UseMiddlewarePipeline(this WebApplication app)
    {
        app.UseCors();
        app.UseMiddleware<ErrorResponseMiddleware>();
        app.UseAuthentication();
        app.UseAuthorization();
        app.UseMiddleware<RateLimitMiddleware>();
    }
```

Remove the `using TownCrier.Web.Observability;` import if it's no longer needed — but `ErrorResponseMiddleware` is in that namespace, so keep it.

- [ ] **Step 4: Update ErrorResponse.cs — remove CorrelationId field**

Replace the file content with:

```csharp
namespace TownCrier.Web.Observability;

internal sealed record ErrorResponse(int Status, string Title, string? Detail = null);
```

- [ ] **Step 5: Update ErrorResponseMiddleware.cs — add exception logging, remove correlation ID**

Replace the entire file with:

```csharp
using System.Text.Json;

namespace TownCrier.Web.Observability;

internal sealed class ErrorResponseMiddleware(RequestDelegate next, ILogger<ErrorResponseMiddleware> logger)
{
    public async Task InvokeAsync(HttpContext context)
    {
        try
        {
            await next(context).ConfigureAwait(false);
        }
#pragma warning disable CA1031 // Global error handler must catch all exceptions
        catch (Exception ex)
#pragma warning restore CA1031
        {
            LogUnhandledException(logger, ex, context.Request.Method, context.Request.Path.Value ?? "/");
            context.Items["ErrorDetail"] = ex.Message;
            if (!context.Response.HasStarted)
            {
                context.Response.StatusCode = 500;
            }
        }

        if (context.Response.StatusCode >= 400
            && !context.Response.HasStarted
            && context.Response.ContentLength is null or 0)
        {
            var detail = context.Items.TryGetValue("ErrorDetail", out var detailObj)
                ? detailObj as string
                : null;

            var errorBody = new ErrorResponse(
                context.Response.StatusCode,
                GetReasonPhrase(context.Response.StatusCode),
                detail);

            context.Response.ContentType = "application/json";
            await JsonSerializer.SerializeAsync(
                context.Response.Body,
                errorBody,
                ObservabilityJsonSerializerContext.Default.ErrorResponse,
                context.RequestAborted).ConfigureAwait(false);
        }
    }

    private static string GetReasonPhrase(int statusCode)
    {
        return statusCode switch
        {
            400 => "Bad Request",
            401 => "Unauthorized",
            403 => "Forbidden",
            404 => "Not Found",
            405 => "Method Not Allowed",
            500 => "Internal Server Error",
            _ => "Error",
        };
    }

    [LoggerMessage(Level = LogLevel.Error, Message = "Unhandled exception on {Method} {Path}")]
    private static partial void LogUnhandledException(ILogger logger, Exception exception, string method, string path);
}
```

Note: The class must also become `partial` for `LoggerMessage` to work. Update the class declaration to `internal sealed partial class ErrorResponseMiddleware`.

- [ ] **Step 6: Update ProgramDecompositionTests.cs — remove correlation ID assertions**

In `Should_ConfigureMiddlewarePipeline_When_UseMiddlewarePipelineCalled` (lines 11-25), remove the correlation ID assertion. Replace with:

```csharp
    [Test]
    public async Task Should_ConfigureMiddlewarePipeline_When_UseMiddlewarePipelineCalled()
    {
        // This test verifies UseMiddlewarePipeline exists and correctly wires
        // CORS, error response, auth, and rate limiting.
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        // Act
        using var response = await client.GetAsync(new Uri("/health", UriKind.Relative));

        // Assert — middleware pipeline must be active
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
    }
```

Remove `Should_IncludeCorrelationId_When_AnyRequestProcessed` (lines 111-119) entirely.

- [ ] **Step 7: Verify build**

Run: `cd /Users/christy/Dev/town-crier/api && dotnet build`
Expected: Build succeeded.

- [ ] **Step 8: Run tests**

Run: `cd /Users/christy/Dev/town-crier/api && dotnet test`
Expected: All remaining tests pass. Deleted test files are gone. No references to CorrelationIdMiddleware or RequestLoggingMiddleware remain.

- [ ] **Step 9: Commit**

```bash
git add -A api/src/town-crier.web/Observability/ \
         api/src/town-crier.web/Extensions/WebApplicationExtensions.cs \
         api/tests/town-crier.web.tests/Observability/ \
         api/tests/town-crier.web.tests/DependencyInjection/ProgramDecompositionTests.cs
git commit -m "refactor(api): remove CorrelationId and RequestLogging middleware, add exception logging to ErrorResponseMiddleware"
```

---

### Task 7: Wire OTEL in Program.cs

**Files:**
- Modify: `api/src/town-crier.web/Program.cs`

- [ ] **Step 1: Add OTEL using statements**

Add to the top of `Program.cs`:

```csharp
using Azure.Monitor.OpenTelemetry.Exporter;
using OpenTelemetry;
using OpenTelemetry.Logs;
using OpenTelemetry.Metrics;
using OpenTelemetry.Trace;
using TownCrier.Application.Observability;
using TownCrier.Infrastructure.Observability;
```

- [ ] **Step 2: Replace logging setup and add OTEL configuration**

Remove `builder.Logging.AddJsonConsole();` (line 7).

Add after line 6 (after `var builder = ...`):

```csharp
builder.Services.AddOpenTelemetry()
    .WithTracing(tracing => tracing
        .AddAspNetCoreInstrumentation()
        .AddHttpClientInstrumentation()
        .AddSource(PollingInstrumentation.ActivitySourceName)
        .AddSource(CosmosInstrumentation.ActivitySourceName)
        .AddAzureMonitorTraceExporter())
    .WithMetrics(metrics => metrics
        .AddAspNetCoreInstrumentation()
        .AddHttpClientInstrumentation()
        .AddMeter(ApiMetrics.MeterName)
        .AddMeter(PollingMetrics.MeterName)
        .AddMeter(CosmosInstrumentation.MeterName)
        .AddAzureMonitorMetricExporter());

builder.Logging.AddOpenTelemetry(logging =>
{
    logging.AddAzureMonitorLogExporter();
});
```

- [ ] **Step 3: Verify build**

Run: `cd /Users/christy/Dev/town-crier/api && dotnet build`
Expected: Build succeeded.

- [ ] **Step 4: Run tests**

Run: `cd /Users/christy/Dev/town-crier/api && dotnet test`
Expected: All tests pass. The OTEL SDK initializes but does not export (no connection string in test environment — it silently no-ops).

- [ ] **Step 5: Commit**

```bash
git add api/src/town-crier.web/Program.cs
git commit -m "feat(api): wire OpenTelemetry traces, metrics, and logs with Azure Monitor exporter"
```

---

### Task 8: Error Response Middleware Tests

**Files:**
- Create: `api/tests/town-crier.web.tests/Observability/ErrorResponseMiddlewareTests.cs`

- [ ] **Step 1: Write tests for updated ErrorResponseMiddleware**

Create `api/tests/town-crier.web.tests/Observability/ErrorResponseMiddlewareTests.cs`:

```csharp
using System.Net;
using Microsoft.AspNetCore.Mvc.Testing;

namespace TownCrier.Web.Tests.Observability;

public sealed class ErrorResponseMiddlewareTests
{
    [Test]
    public async Task Should_ReturnStructuredError_When_UnauthorizedRequest()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient(new WebApplicationFactoryClientOptions
        {
            AllowAutoRedirect = false,
        });

        // Act — /v1/me requires auth, no token = 401
        using var response = await client.GetAsync(new Uri("/v1/me", UriKind.Relative));

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Unauthorized);
        var body = await response.Content.ReadAsStringAsync();
        await Assert.That(body).Contains("Unauthorized");
    }

    [Test]
    public async Task Should_ReturnStructuredError_When_RouteNotFound()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        // Act
        using var response = await client.GetAsync(new Uri("/nonexistent", UriKind.Relative));

        // Assert
        await Assert.That((int)response.StatusCode).IsGreaterThanOrEqualTo(400);
        var body = await response.Content.ReadAsStringAsync();
        await Assert.That(body).Contains("Status");
    }
}
```

- [ ] **Step 2: Run the new tests**

Run: `cd /Users/christy/Dev/town-crier/api && dotnet test --filter "ErrorResponseMiddlewareTests"`
Expected: Both tests pass.

- [ ] **Step 3: Run all tests**

Run: `cd /Users/christy/Dev/town-crier/api && dotnet test`
Expected: All tests pass.

- [ ] **Step 4: Commit**

```bash
git add api/tests/town-crier.web.tests/Observability/ErrorResponseMiddlewareTests.cs
git commit -m "test(api): add ErrorResponseMiddleware tests for structured error responses"
```

---

### Task 9: Frontend App Insights SDK

**Files:**
- Create: `web/src/telemetry.ts`
- Create: `web/src/components/ErrorBoundary.tsx`
- Create: `web/src/components/ErrorBoundary.module.css`
- Modify: `web/src/main.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Install @microsoft/applicationinsights-web**

Run: `cd /Users/christy/Dev/town-crier/web && npm install @microsoft/applicationinsights-web`

- [ ] **Step 2: Create telemetry.ts**

Create `web/src/telemetry.ts`:

```typescript
import { ApplicationInsights } from '@microsoft/applicationinsights-web';

const connectionString = import.meta.env.VITE_APPLICATIONINSIGHTS_CONNECTION_STRING;

const appInsights = connectionString
  ? new ApplicationInsights({
      config: {
        connectionString,
        enableAutoRouteTracking: true,
        enableCorsCorrelation: true,
        correlationHeaderDomains: [
          'api-dev.towncrierapp.uk',
          'api.towncrierapp.uk',
        ],
      },
    })
  : null;

appInsights?.loadAppInsights();

export { appInsights };
```

The guard (`connectionString ? ... : null`) ensures local dev without a connection string doesn't crash.

- [ ] **Step 3: Create ErrorBoundary.module.css**

Create `web/src/components/ErrorBoundary.module.css`:

```css
.container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 50vh;
  padding: var(--spacing-xl);
  text-align: center;
}

.title {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  color: var(--color-text-primary);
  margin-bottom: var(--spacing-md);
}

.message {
  font-size: var(--font-size-md);
  color: var(--color-text-secondary);
  margin-bottom: var(--spacing-lg);
}

.button {
  padding: var(--spacing-sm) var(--spacing-lg);
  background: var(--color-primary);
  color: var(--color-text-on-primary);
  border: none;
  border-radius: var(--radius-md);
  font-size: var(--font-size-md);
  cursor: pointer;
}
```

- [ ] **Step 4: Create ErrorBoundary.tsx**

Create `web/src/components/ErrorBoundary.tsx`:

```tsx
import { Component, type ErrorInfo, type ReactNode } from 'react';
import { appInsights } from '../telemetry';
import styles from './ErrorBoundary.module.css';

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false };

  static getDerivedStateFromError(): State {
    return { hasError: true };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    appInsights?.trackException({
      exception: error,
      properties: { componentStack: errorInfo.componentStack ?? '' },
    });
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className={styles.container}>
          <h1 className={styles.title}>Something went wrong</h1>
          <p className={styles.message}>
            The application encountered an unexpected error. Please try
            refreshing the page.
          </p>
          <button
            className={styles.button}
            onClick={() => window.location.reload()}
          >
            Refresh
          </button>
        </div>
      );
    }

    return this.props.children;
  }
}
```

- [ ] **Step 5: Initialize telemetry in main.tsx**

Update `web/src/main.tsx` — add the telemetry import before anything else:

```tsx
import './telemetry';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import './styles/tokens.css';
import './styles/global.css';
import { App } from './App';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
```

The bare `import './telemetry'` ensures the SDK loads before React mounts.

- [ ] **Step 6: Wrap App in ErrorBoundary**

Update `web/src/App.tsx` — add the ErrorBoundary import and wrap the provider tree:

```tsx
import { BrowserRouter } from 'react-router-dom';
import { Auth0Provider } from '@auth0/auth0-react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { loadAuthConfig } from './auth/auth-config';
import { Auth0AuthAdapter } from './auth/Auth0AuthAdapter';
import { ApiClientProvider } from './api/ApiClientProvider';
import { ProfileRepositoryProvider } from './auth/ProfileRepositoryProvider';
import { AppRoutes } from './AppRoutes';
import { ErrorBoundary } from './components/ErrorBoundary';

const authConfig = loadAuthConfig();
const queryClient = new QueryClient();

export function App() {
  return (
    <ErrorBoundary>
      <Auth0Provider
        domain={authConfig.domain}
        clientId={authConfig.clientId}
        authorizationParams={{
          redirect_uri: `${window.location.origin}/callback`,
          audience: authConfig.audience,
        }}
        useRefreshTokens
        cacheLocation="localstorage"
      >
        <QueryClientProvider client={queryClient}>
          <Auth0AuthAdapter>
            <ApiClientProvider>
              <ProfileRepositoryProvider>
                <BrowserRouter>
                  <AppRoutes />
                </BrowserRouter>
              </ProfileRepositoryProvider>
            </ApiClientProvider>
          </Auth0AuthAdapter>
        </QueryClientProvider>
      </Auth0Provider>
    </ErrorBoundary>
  );
}
```

- [ ] **Step 7: Verify TypeScript compilation**

Run: `cd /Users/christy/Dev/town-crier/web && npx tsc --noEmit`
Expected: No type errors.

- [ ] **Step 8: Verify build**

Run: `cd /Users/christy/Dev/town-crier/web && npm run build`
Expected: Build succeeds. App Insights SDK tree-shakes cleanly.

- [ ] **Step 9: Commit**

```bash
git add web/src/telemetry.ts web/src/components/ErrorBoundary.tsx web/src/components/ErrorBoundary.module.css web/src/main.tsx web/src/App.tsx web/package.json web/package-lock.json
git commit -m "feat(web): add Application Insights SDK with error boundary and auto route tracking"
```

---

### Task 10: CI/CD Pipeline Updates

**Files:**
- Modify: `.github/workflows/cd-dev.yml`
- Modify: `.github/workflows/cd-prod.yml`

- [ ] **Step 1: Add App Insights connection string to dev API deploy**

In `cd-dev.yml`, in the `api-deploy` job's "Deploy to Container App" step (around line 150), add `APPLICATIONINSIGHTS_CONNECTION_STRING` to the `--set-env-vars` flag. The connection string comes from Pulumi. Add a step before the deploy to extract it:

After the "Set Container App secrets" step and before "Deploy to Container App", add:

```yaml
      - name: Get App Insights connection string
        id: appinsights
        working-directory: infra
        run: |
          CONNECTION_STRING=$(pulumi stack output appInsightsConnectionString --stack shared)
          echo "::add-mask::$CONNECTION_STRING"
          echo "connection-string=$CONNECTION_STRING" >> "$GITHUB_OUTPUT"
        env:
          PULUMI_ACCESS_TOKEN: ${{ secrets.PULUMI_ACCESS_TOKEN }}
```

Update the "Deploy to Container App" step to include the env var:

```yaml
      - name: Deploy to Container App
        run: |
          az containerapp update \
            --name "ca-town-crier-api-dev" \
            --resource-group "rg-town-crier-dev" \
            --image "${{ secrets.ACR_LOGIN_SERVER }}/town-crier-api:${{ github.sha }}" \
            --set-env-vars \
              "Admin__ApiKey=secretref:admin-api-key" \
              "Subscription__AutoGrant__ProDomains=secretref:auto-grant-pro-domains" \
              "APPLICATIONINSIGHTS_CONNECTION_STRING=${{ steps.appinsights.outputs.connection-string }}"
```

The `api-deploy` job needs `setup-dotnet-infra` for Pulumi CLI. Add the setup step:

```yaml
      - uses: ./.github/actions/setup-dotnet-infra
```

- [ ] **Step 2: Add App Insights connection string to dev web build**

In `cd-dev.yml`, in the `web-deploy` job's "Build" step (around line 179), add the Vite env var:

```yaml
          VITE_APPLICATIONINSIGHTS_CONNECTION_STRING: ${{ vars.VITE_APPLICATIONINSIGHTS_CONNECTION_STRING }}
```

This reads from a GitHub environment variable. The value is set once in the GitHub repo settings — it's not a secret (it only enables ingestion to your own App Insights).

- [ ] **Step 3: Add App Insights connection string to prod API deploy**

In `cd-prod.yml`, similar pattern. In the `api` job, after the "Set Container App secrets" step, add the Pulumi output extraction:

```yaml
      - uses: ./.github/actions/setup-dotnet-infra

      - name: Get App Insights connection string
        id: appinsights
        working-directory: infra
        run: |
          CONNECTION_STRING=$(pulumi stack output appInsightsConnectionString --stack shared)
          echo "::add-mask::$CONNECTION_STRING"
          echo "connection-string=$CONNECTION_STRING" >> "$GITHUB_OUTPUT"
        env:
          PULUMI_ACCESS_TOKEN: ${{ secrets.PULUMI_ACCESS_TOKEN }}
```

Update the "Deploy API image to prod" step:

```yaml
      - name: Deploy API image to prod
        run: |
          az containerapp update \
            --name "ca-town-crier-api-prod" \
            --resource-group "$RESOURCE_GROUP" \
            --image "${{ secrets.ACR_LOGIN_SERVER }}/town-crier-api:${{ steps.check-image.outputs.tag }}" \
            --set-env-vars \
              "Admin__ApiKey=secretref:admin-api-key" \
              "Subscription__AutoGrant__ProDomains=secretref:auto-grant-pro-domains" \
              "APPLICATIONINSIGHTS_CONNECTION_STRING=${{ steps.appinsights.outputs.connection-string }}"
        env:
          RESOURCE_GROUP: ${{ needs.infra.outputs.resource-group }}
```

- [ ] **Step 4: Add App Insights connection string to prod web build**

In `cd-prod.yml`, in the `web` job's "Build" step, add:

```yaml
          VITE_APPLICATIONINSIGHTS_CONNECTION_STRING: ${{ vars.VITE_APPLICATIONINSIGHTS_CONNECTION_STRING }}
```

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/cd-dev.yml .github/workflows/cd-prod.yml
git commit -m "ci: pass Application Insights connection string to API and web deployments"
```

---

### Task 11: Final Verification

- [ ] **Step 1: Full API build**

Run: `cd /Users/christy/Dev/town-crier/api && dotnet build`
Expected: Build succeeded, no warnings from OTEL packages.

- [ ] **Step 2: Full API test suite**

Run: `cd /Users/christy/Dev/town-crier/api && dotnet test`
Expected: All tests pass.

- [ ] **Step 3: Full web build**

Run: `cd /Users/christy/Dev/town-crier/web && npm run build`
Expected: Build succeeds.

- [ ] **Step 4: Web type check**

Run: `cd /Users/christy/Dev/town-crier/web && npx tsc --noEmit`
Expected: No type errors.

- [ ] **Step 5: Infra build**

Run: `cd /Users/christy/Dev/town-crier/infra && dotnet build`
Expected: Build succeeded.

- [ ] **Step 6: Verify no stale references**

Run: `grep -r "CorrelationIdMiddleware\|RequestLoggingMiddleware\|X-Correlation-Id" api/src/`
Expected: No matches (all references removed).

- [ ] **Step 7: Commit any remaining changes**

```bash
git status
# If any uncommitted changes remain, stage and commit them
```

---

## Post-Deployment Checklist

After the first deployment with this code:

1. **Set GitHub environment variable** `VITE_APPLICATIONINSIGHTS_CONNECTION_STRING` for the `development` and `production` environments
2. **Verify in Azure Portal** → Application Insights → Live Metrics → make a request → confirm traces appear
3. **Check Azure Portal** → Application Insights → Performance → verify request latency and dependency tracking
4. **Check Azure Portal** → Application Insights → Failures → verify exception tracking (trigger a 500 to test)
5. **Check Azure Portal** → Application Insights → Metrics → verify custom metrics (`towncrier.polling.*`, `towncrier.cosmos.*`) appear
