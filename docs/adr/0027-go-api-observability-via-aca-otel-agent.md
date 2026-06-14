# 0027. Go API observability via the ACA managed-environment OpenTelemetry agent

Date: 2026-06-14

## Status

Accepted

## Context

The Go API migration (GH #418, epic tc-7g3i) replaces the .NET API with a
contract-identical Go service running alongside it until a Cloudflare DNS flip
cuts `api.towncrierapp.uk` over to the Go app. Iteration 10 (tc-7g3i.11) is the
final iteration: it must give the Go app production-grade observability before
cutover.

The current telemetry picture:

- The **.NET API** emits traces, metrics, and logs **in-process** via the Azure
  Monitor OpenTelemetry exporter — `UseAzureMonitorExporter` configured with
  `APPLICATIONINSIGHTS_CONNECTION_STRING` (see `api/src/town-crier.web/Program.cs`).
  It does not use an OTLP exporter and does not read `OTEL_EXPORTER_OTLP_ENDPOINT`.
- The **Go API** had **no telemetry at all** — only an `OTEL_SERVICE_NAME` env
  var was set in infra, with nothing consuming it.
- Both apps run in the **same** Container Apps managed environment,
  `cae-town-crier-shared` (defined in `infra/SharedStack.cs`), which already
  routes console logs to the shared Log Analytics workspace and feeds the shared
  Application Insights component `appi-town-crier-shared`.

We need the Go app's request traces to land in the same Application Insights as
the .NET app — so the existing dashboards, alerts, and the `sre-observatory`
skill keep working across the migration — without:

1. **Double-counting** request telemetry while both apps are deployed, and
2. **Disturbing the live .NET prod app**, whose telemetry path must stay exactly
   as it is.

## Decision

Enable the **Azure Container Apps managed-environment OpenTelemetry agent** on
`cae-town-crier-shared` with **Application Insights as the traces destination**,
and have the Go app **export OTLP traces to that agent**.

Concretely:

- **Infra (`infra/SharedStack.cs`)** — the `cae-town-crier-shared`
  `ManagedEnvironment` gains:
  - `AppInsightsConfiguration { ConnectionString = appInsights.ConnectionString }`
  - `OpenTelemetryConfiguration { TracesConfiguration { Destinations = ["appInsights"] } }`

  Once the agent is enabled, ACA **auto-injects `OTEL_EXPORTER_OTLP_ENDPOINT`**
  (pointing at the agent's local OTLP receiver) into **every** container in the
  environment. Traces only are configured: the Go app emits traces; its `slog`
  logs already reach the `ContainerAppConsoleLogs` table via the existing
  `AppLogsConfiguration`, and it emits no OTLP metrics yet.

- **Go app (`api-go/internal/platform/telemetry.go`)** — `SetupTelemetry`
  builds an OTLP/gRPC trace exporter (`otlptracegrpc`, which reads
  `OTEL_EXPORTER_OTLP_ENDPOINT` itself), wraps it in an SDK `TracerProvider`
  with `service.name` from `OTEL_SERVICE_NAME` (default `town-crier-api-go`),
  installs it as the global provider, and sets a W3C TraceContext propagator.
  `otelhttp` wraps the outermost handler so every request is a span.

- **Self-disable when the endpoint is unset.** With no
  `OTEL_EXPORTER_OTLP_ENDPOINT` (local dev, `go test`, the contract-diff suite,
  a Cosmos-less boot), `SetupTelemetry` creates no exporter, leaves the SDK's
  no-op global provider in place, and returns a no-op shutdown. `otelhttp` then
  produces no-op spans at negligible cost. This is why **no infra-then-test
  split was needed** for the Go telemetry code (contrast the deploy-ordering
  lesson from #439 → #440): the code is safe to merge before — or without — the
  agent ever being enabled.

### Why not the obvious alternatives

- **In-process Azure Monitor exporter in Go (mirror .NET).** The Azure Monitor
  OpenTelemetry story for Go is OTLP-first; there is no first-class in-process
  Azure Monitor trace exporter for Go equivalent to .NET's
  `UseAzureMonitorExporter`. OTLP → managed agent is the idiomatic path and
  keeps the App Insights connection string **out of the app** entirely — only
  the environment/agent holds it.
- **Direct OTLP from Go straight to Application Insights.** App Insights does not
  expose a stable, generic OTLP ingestion endpoint for apps to target directly;
  the managed agent is the supported bridge from OTLP to App Insights.
- The managed agent is **environment-level** config — declared once on the
  shared environment, it serves both the dev and prod Go apps without per-app
  exporter wiring.

### No double-count, verified

The agent injects `OTEL_EXPORTER_OTLP_ENDPOINT` into the .NET container too, but
the .NET app **never reads it** — it wires only `UseAzureMonitorExporter` and
never calls `AddOtlpExporter` (`api/src/town-crier.web/Program.cs`). So the .NET
app keeps exporting in-process to App Insights exactly as before, the Go app
exports via the agent, and the two telemetry streams are distinguished in App
Insights by `AppRoleName` (`town-crier-api` vs `town-crier-api-go`). No request
is counted twice.

## Consequences

- **Easier:** Go request traces, dependencies, and durations land in the shared
  Application Insights with `service.name = town-crier-api-go`, so the existing
  dashboards and `sre-observatory` queries cover the Go app immediately —
  filter by `AppRoleName` to compare Go vs .NET during the parallel-run window.
- **Safe for prod .NET:** the SharedStack change is additive. It deploys to the
  shared managed environment via cd-dev's `infra-shared` job on merge to main
  (the shared stack is not gated behind a prod tag), but it does not alter the
  .NET app's container, exporter, or connection string.
- **Idle until cutover:** the new prod Go app (`ca-town-crier-api-go-prod`,
  added in the same iteration) runs scale-to-zero with no traffic until the DNS
  flip, so it emits effectively no telemetry and incurs no idle cost until then.
- **New dependencies:** the Go module gains `go.opentelemetry.io/otel`,
  `.../otel/sdk`, `.../otlp/otlptrace/otlptracegrpc`, and the `otelhttp`
  contrib instrumentation. The `go-coding-standards` skill favours minimal
  dependencies; this issue mandates OpenTelemetry, and the justification is
  recorded in the header of `telemetry.go`.
- **Deferred:** OTLP metrics and OTLP logs from the Go app are not configured
  (App Insights destination is traces-only here; logs already flow via console
  capture). Add them later if the Go app needs first-class custom metrics at
  parity with the .NET `customMetrics`.
- **Post-cutover follow-up:** when the api domain DNS flips to the Go app, the
  Go traces become the primary request telemetry, and a follow-up bead bumps the
  prod Go app's `MinReplicas` from 0 to 1 (matching the warm-instance policy the
  .NET prod app uses to avoid scale-from-zero cold starts).
