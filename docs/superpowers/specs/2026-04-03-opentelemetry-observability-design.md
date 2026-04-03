# OpenTelemetry Observability Design

Date: 2026-04-03

## Goal

Add comprehensive observability to Town Crier — traces, metrics, and logs across the .NET API and React frontend, exported to Azure Monitor / Application Insights. Dashboards first, alerting channels later.

## Constraints

- All .NET packages must be Native AOT-compatible (.NET 10, `CreateSlimBuilder`, `PublishAot=true`)
- No reflection-based config binding — connection strings via environment variables
- No new abstractions beyond what OTEL provides

## Architecture

```
React App ──▶ App Insights JS SDK ──▶ Application Insights ──▶ Log Analytics Workspace (existing)
.NET API  ──▶ OTEL SDK + Azure Monitor Exporter ──────────────▶ (same Application Insights instance)
```

Three telemetry signals from the API: traces, metrics, logs. Frontend sends page views, exceptions, and RUM data. Both land in a single Application Insights resource backed by the existing Log Analytics Workspace.

## AOT Compatibility Verification

| Package | Version | AOT Status | Evidence |
|---------|---------|------------|----------|
| `Azure.Monitor.OpenTelemetry.AspNetCore` (distro) | 1.4.0 | **INCOMPATIBLE** | Uses reflection config binding, reflection JSON, `MakeGenericMethod`. [Azure SDK #43856](https://github.com/Azure/azure-sdk-for-net/issues/43856) |
| `Azure.Monitor.OpenTelemetry.Exporter` | 1.7.0 | Compatible | AOT documented in README, source generators in 1.7.0 |
| `OpenTelemetry.Extensions.Hosting` | 1.15.1 | Compatible | Validated in OTEL repo AOT CI pipeline |
| `OpenTelemetry.Instrumentation.AspNetCore` | 1.15.1 | Compatible | `IsAotCompatible=true` in csproj |
| `OpenTelemetry.Instrumentation.Http` | 1.15.0 | Compatible | `IsAotCompatible=true` in csproj |
| `@microsoft/applicationinsights-web` | latest | N/A | Browser JS, AOT not applicable |

The distro package is excluded. Manual wiring with the individual packages is the approach.

## 1. Infrastructure (Pulumi)

Provision an Application Insights resource in the shared stack:

- Resource: `appi-town-crier-shared`
- Type: `web`
- Ingestion mode: `LogAnalytics`
- Connected to existing `log-town-crier-shared` Log Analytics Workspace
- Single shared instance across environments (separate per-environment later when needed)

Connection string distribution:
- **API**: Environment variable `APPLICATIONINSIGHTS_CONNECTION_STRING` on the Container App
- **Web**: `VITE_APPLICATIONINSIGHTS_CONNECTION_STRING` injected at build time in CI/CD

Cost: Pay-per-GB ingestion. First 5 GB/month free. At current scale (~50-200 MB/month), effectively zero cost.

## 2. API Packages

Added to `town-crier.web.csproj`:

```xml
<PackageReference Include="Azure.Monitor.OpenTelemetry.Exporter" Version="1.7.0" />
<PackageReference Include="OpenTelemetry.Extensions.Hosting" Version="1.15.1" />
<PackageReference Include="OpenTelemetry.Instrumentation.AspNetCore" Version="1.15.1" />
<PackageReference Include="OpenTelemetry.Instrumentation.Http" Version="1.15.0" />
```

## 3. API OTEL Setup (Program.cs)

```csharp
builder.Services.AddOpenTelemetry()
    .WithTracing(tracing => tracing
        .AddAspNetCoreInstrumentation()
        .AddHttpClientInstrumentation()
        .AddSource("TownCrier.Polling")
        .AddSource("TownCrier.Cosmos")
        .AddAzureMonitorTraceExporter())
    .WithMetrics(metrics => metrics
        .AddAspNetCoreInstrumentation()
        .AddHttpClientInstrumentation()
        .AddMeter("TownCrier.Api")
        .AddMeter("TownCrier.Polling")
        .AddMeter("TownCrier.Cosmos")
        .AddAzureMonitorMetricExporter())
    .WithLogging(logging => logging
        .AddAzureMonitorLogExporter());
```

Connection string read from `APPLICATIONINSIGHTS_CONNECTION_STRING` environment variable (automatic — the Azure Monitor exporter reads this by convention). No `IConfiguration` binding.

## 4. Cosmos Repository-Layer Tracing

`ActivitySource` in the infrastructure layer for Cosmos operations:

```csharp
internal static class CosmosInstrumentation
{
    public static readonly ActivitySource Source = new("TownCrier.Cosmos");
}
```

Each repository method wraps its HTTP call:

- Span name: operation description (e.g., `"Cosmos ReadItem"`)
- Tags: `db.system=cosmosdb`, `db.cosmosdb.container`, `db.operation.name`, `db.cosmosdb.status_code`, `db.cosmosdb.request_charge`
- RU charge extracted from `x-ms-request-charge` response header

The `HttpClientInstrumentation` captures the raw HTTP span underneath. Repository spans provide the logical operation context on top.

Cosmos metrics recorded from the same call site:

```csharp
internal static class CosmosMetrics
{
    private static readonly Meter Meter = new("TownCrier.Cosmos");

    public static readonly Histogram<double> RequestCharge =
        Meter.CreateHistogram<double>("towncrier.cosmos.request_charge_ru",
            unit: "RU", description: "Cosmos RU consumption per operation");
    public static readonly Counter<long> Throttles =
        Meter.CreateCounter<long>("towncrier.cosmos.throttles",
            description: "429 responses from Cosmos");
}
```

## 5. Polling Pipeline Tracing

`ActivitySource` for the polling pipeline:

```csharp
internal static class PollingInstrumentation
{
    public static readonly ActivitySource Source = new("TownCrier.Polling");
}
```

Span hierarchy:

```
Polling Cycle (root span)
  tags: polling.cycle_id, polling.authorities_total, polling.authorities_skipped
├── Poll Authority: {name} (child span)
│   tags: polling.authority_code, polling.authority_name, polling.applications_found, polling.applications_new
│   ├── PlanIt API Call (auto: HttpClientInstrumentation)
│   ├── Parse Applications (child span)
│   │   tags: polling.parse_count, polling.parse_errors
│   └── Cosmos Upsert x N (child spans from repository layer)
├── Poll Authority: {name}
│   └── ...
└── Cycle Summary (span event with totals)
```

Child spans nest automatically via `Activity.Current` context propagation.

Polling metrics:

```csharp
internal static class PollingMetrics
{
    private static readonly Meter Meter = new("TownCrier.Polling");

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

## 6. API Business Metrics

```csharp
internal static class ApiMetrics
{
    private static readonly Meter Meter = new("TownCrier.Api");

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

Counters incremented in command/query handlers. All `System.Diagnostics.Metrics` types — BCL, fully AOT-safe.

## 7. Frontend Instrumentation

### Package

```bash
npm install @microsoft/applicationinsights-web
```

### Initialization

```typescript
import { ApplicationInsights } from '@microsoft/applicationinsights-web';

const appInsights = new ApplicationInsights({
  config: {
    connectionString: import.meta.env.VITE_APPLICATIONINSIGHTS_CONNECTION_STRING,
    enableAutoRouteTracking: true,
    enableCorsCorrelation: true,
    correlationHeaderDomains: ['api-dev.towncrierapp.uk', 'api.towncrierapp.uk'],
  },
});

appInsights.loadAppInsights();
export { appInsights };
```

### Auto-Collected

- Page views (route changes)
- Unhandled JS exceptions and promise rejections
- AJAX/fetch dependency tracking (API calls with duration and status)
- Page load performance (Core Web Vitals: LCP, FID, CLS)
- Anonymous user sessions

### Trace Correlation

`enableCorsCorrelation` + `correlationHeaderDomains` sends W3C `traceparent` headers on API calls. End-to-end traces from browser click to Cosmos query visible in Azure Portal transaction view.

### React Error Boundary

Wrap the app in an error boundary that reports component errors:

```typescript
appInsights.trackException({ exception: error });
```

## 8. Middleware Cleanup

### Remove
- **`CorrelationIdMiddleware`** — replaced by OTEL W3C `traceparent` propagation

### Modify
- **`ErrorResponseMiddleware`** — add exception logging (`logger.LogError(ex, ...)`) before returning the error response. OTEL picks up the logged exception as a trace event.
- **`RequestLoggingMiddleware`** — remove entirely. OTEL `AspNetCoreInstrumentation` captures the same data (method, path, status, duration) as trace spans.

### Update
- **`Program.cs`** — remove `CorrelationIdMiddleware` and `RequestLoggingMiddleware` registration, remove `AddJsonConsole()`, add OTEL setup.

## 9. CI/CD Changes

### Container App (cd-dev.yml, cd-prod.yml)
- Pass `APPLICATIONINSIGHTS_CONNECTION_STRING` as an environment variable to the Container App deployment

### Web Build (cd-dev.yml, cd-prod.yml)
- Set `VITE_APPLICATIONINSIGHTS_CONNECTION_STRING` during the `npm run build` step

### Connection String Source
- Pulumi exports the connection string as a stack output from the shared stack
- CD workflows read it via `pulumi stack output` and pass it to the Container App and web build steps

## Cost Controls (Not Needed Now)

Available when scale increases:
- **Sampling**: OTEL head-based sampling — reduce trace volume to 50%/10%
- **Daily cap**: Application Insights daily ingestion cap in GB
- **Log filtering**: Export only Warning+ logs, keep Debug/Info local

## Out of Scope

- Alert rules and action groups (dashboards first, alerting later)
- Per-environment Application Insights instances (single shared for now)
- Custom Azure dashboards (use built-in App Insights views initially)
- Frontend custom events / funnels (available but not day one)
- `LogPollingHealthAlerter` replacement (polling health visible via traces/metrics, formal alerting is a follow-up)
