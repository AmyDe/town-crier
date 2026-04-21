# 0018. OpenTelemetry Observability with Azure Monitor

Date: 2026-04-03

## Status

Accepted

## Context

Town Crier had no centralised observability. The API used bespoke middleware (`CorrelationIdMiddleware`, `RequestLoggingMiddleware`) that produced unstructured console logs with no trace correlation, metrics, or dashboards. The React frontend had no error reporting or performance tracking. When something went wrong in production — a Cosmos throttle, a PlanIt timeout, a polling cycle failure — there was no way to see it without SSH-ing into logs.

We needed traces, metrics, and logs across the .NET API and React frontend, exported to a single backend, with end-to-end correlation from browser click to Cosmos query.

Key constraints:
- All .NET packages must be **Native AOT-compatible** (see [ADR 0001](0001-initial-tech-stack.md)) — no reflection-based config binding
- Minimal new abstractions — use what the OpenTelemetry SDK provides
- Cost must be near-zero at current scale (~50–200 MB/month ingestion)

## Decision

We adopted **OpenTelemetry** as the instrumentation standard and **Azure Monitor / Application Insights** as the telemetry backend.

### Why OpenTelemetry over proprietary SDKs

The Azure Monitor "distro" package (`Azure.Monitor.OpenTelemetry.AspNetCore`) was evaluated and **rejected** — it uses reflection-based configuration binding and `MakeGenericMethod`, which are incompatible with Native AOT ([azure-sdk-for-net#43856](https://github.com/Azure/azure-sdk-for-net/issues/43856)). Instead, we wire the individual OTel SDK packages manually with the Azure Monitor **Exporter** (which is AOT-compatible):

- `Azure.Monitor.OpenTelemetry.Exporter` 1.7.0
- `OpenTelemetry.Extensions.Hosting` 1.15.1
- `OpenTelemetry.Instrumentation.AspNetCore` 1.15.1
- `OpenTelemetry.Instrumentation.Http` 1.15.0

### API instrumentation

Three telemetry signals are exported:

1. **Traces** — auto-instrumentation for ASP.NET Core requests and HttpClient calls, plus two custom `ActivitySource`s:
   - `TownCrier.Polling` — spans for each polling cycle and per-authority processing, forming a span hierarchy from cycle root to individual Cosmos upserts
   - `TownCrier.Cosmos` — spans for each repository operation with RU charge and status code tags

2. **Metrics** — three custom `Meter`s (11 instruments total):
   - `TownCrier.Api` — watch zone creation, notifications sent, active subscriptions, endpoint errors
   - `TownCrier.Polling` — authorities polled/skipped, applications ingested, failures, PlanIt latency, cycle duration
   - `TownCrier.Cosmos` — RU consumption (histogram), throttle count

3. **Logs** — `ILogger` output exported to Azure Monitor via `AddAzureMonitorLogExporter()`, replacing console JSON logging

### Frontend instrumentation

The React frontend uses `@microsoft/applicationinsights-web` (v3.3.11) for:
- Automatic page view tracking on route changes
- Unhandled exception and promise rejection capture
- AJAX/fetch dependency tracking with duration and status
- Core Web Vitals (LCP, FID, CLS)
- **W3C trace correlation** via `enableCorsCorrelation` — the browser sends `traceparent` headers to the API, enabling end-to-end transaction views in Azure Portal

A React error boundary catches component-level errors and reports them via `trackException()`.

### Infrastructure

A single Application Insights resource (`appi-town-crier-shared`) backed by a Log Analytics Workspace (`log-town-crier-shared`, 30-day retention, PerGB2018 SKU) is provisioned via Pulumi in the shared stack. The connection string is:
- Injected as `APPLICATIONINSIGHTS_CONNECTION_STRING` env var on the Container App (API)
- Injected as `VITE_APPLICATIONINSIGHTS_CONNECTION_STRING` at build time in CI/CD (web)

Single shared instance across environments for now; per-environment separation is a future option.

### Middleware cleanup

The bespoke observability middleware was removed:
- `CorrelationIdMiddleware` — replaced by OTel W3C `traceparent` propagation
- `RequestLoggingMiddleware` — replaced by `AspNetCoreInstrumentation` (captures method, path, status, duration as trace spans)
- `ErrorResponseMiddleware` — retained but updated to log exceptions via `ILogger` (which OTel picks up as trace events)

## Consequences

**Easier:**
- Production visibility — traces, metrics, and logs in a single Azure Portal view
- End-to-end debugging — click a browser request, see the full trace through the API to Cosmos
- Capacity planning — Cosmos RU consumption and polling cycle duration are now metered
- Incident detection — endpoint error counters and polling failure counters are queryable
- Cost tracking — RU histograms show per-operation cost distribution

**Harder:**
- Package upgrades require AOT re-verification (the distro package may become AOT-compatible in future, which would simplify wiring)
- Single shared Application Insights instance means dev noise mixes with prod data (acceptable at current scale; split when it becomes a problem)

**Not yet done:**
- Alert rules and action groups (dashboards first, alerting later)
- iOS telemetry (MetricKit crash reporting exists per [ADR 0014](0014-ios-offline-first-architecture.md), but no centralised export)
- Custom Azure dashboards (using built-in App Insights views initially)
- Sampling configuration (full collection at current ingestion volume)

## Amendments

### 2026-04-21
- Corrected: **the React frontend instrumentation section above is aspirational and has not shipped.** `@microsoft/applicationinsights-web` is not a dependency in `web/package.json`, no SDK initialisation is present, and the `ErrorBoundary` component does not call `trackException`. `VITE_APPLICATIONINSIGHTS_CONNECTION_STRING` is wired in CI/CD but unused at runtime. End-to-end W3C trace correlation from browser to API is therefore not yet achievable. Revisit once a real frontend telemetry need is confirmed.
- Retained: API-side instrumentation remains accurate. `Azure.Monitor.OpenTelemetry.Exporter` 1.7.0 is wired in both `town-crier.web` and `town-crier.worker`, with ActivitySources `TownCrier.Polling` and `TownCrier.Cosmos` and Meters `TownCrier.Api`, `TownCrier.Polling`, `TownCrier.Cosmos`, `TownCrier.PlanIt`. A `SuccessfulCosmosDependencyFilter` trims noisy successful dependency spans before export.
- Added: an operational Azure dashboard (`dash-towncrier-operational`) is provisioned in Pulumi with KQL tiles over the shared Application Insights / Log Analytics workspace. The "Not yet done: Custom Azure dashboards" item is now partially addressed.
