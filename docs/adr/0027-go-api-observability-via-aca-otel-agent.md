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

### Update 2026-06-14 (tc-8x8g) — dependencies, logs, and exceptions

Live verification after the api/api-dev DNS cutover showed the Go app emitting
**only** request spans (`AppRequests`): `AppDependencies`, `AppTraces`, and
`AppExceptions` were all empty, so outbound Cosmos calls were uninstrumented,
500s carried no error detail, and slog logs never reached App Insights. This
amendment closes that gap by extending — not replacing — the pipeline above:

- **Cosmos dependency spans (`api-go/internal/platform/cosmos.go`).** Every
  outbound `azcosmos` call is wrapped in an OTel **client** span
  (`db.system=cosmosdb`, `db.operation`, `db.cosmosdb.container`,
  `server.address`), recording the error and `Error` status on failure. These
  become `AppDependencies`, so Cosmos latency/throttling is visible and a failed
  read shows as a failed dependency on the request's trace.
- **slog → OTel logs (`telemetry.go`, `logger.go`).** `SetupTelemetry` now also
  builds an OTLP/gRPC **logs** exporter and SDK `LoggerProvider` (shared
  resource), and the production logger fans out to both stdout JSON
  (`ContainerAppConsoleLogs`, unchanged) and the `otelslog` bridge. slog records
  now land as trace-correlated `AppTraces`. The logs pipeline self-disables on an
  unset `OTEL_EXPORTER_OTLP_ENDPOINT` exactly like traces.
- **Exceptions / failed requests.** The panic-recovery middleware records the
  panic on the request span (`AppExceptions`), and `ErrorBody` marks the request
  span `Error` on any `>= 500` so failed requests are queryable in `AppRequests`.
- **Infra (`infra/SharedStack.cs`).** `OpenTelemetryConfiguration` gains
  `LogsConfiguration { Destinations = ["appInsights"] }` alongside the existing
  traces destination, so the agent forwards the Go app's OTLP logs to App
  Insights. The .NET app emits no OTLP logs (it logs in-process via Azure
  Monitor and ignores the injected OTLP endpoint), so there is still no
  double-count. Metrics OTLP remains unconfigured.

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
- **Deferred:** OTLP **metrics** from the Go app are still not configured. Add
  them later if the Go app needs first-class custom metrics at parity with the
  .NET `customMetrics`. (OTLP **logs** were initially deferred here too, but the
  tc-8x8g amendment above enabled them once it became clear `AppTraces` /
  `AppExceptions` were needed to diagnose production 500s.)
- **Post-cutover follow-up:** when the api domain DNS flips to the Go app, the
  Go traces become the primary request telemetry, and a follow-up bead bumps the
  prod Go app's `MinReplicas` from 0 to 1 (matching the warm-instance policy the
  .NET prod app uses to avoid scale-from-zero cold starts).

## Amendments

### 2026-06-15 — parallel-run window closed; .NET decommissioned

The DNS cutover (api + api-dev, 2026-06-14) and Phase 3 decommission (epic
tc-tbyp, 2026-06-15) are complete — see [ADR 0028](0028-migrate-backend-from-dotnet-to-go.md).
Consequences for this ADR:

- **The .NET API and worker are gone.** The "no double-count" reasoning above —
  the .NET app ignoring the injected `OTEL_EXPORTER_OTLP_ENDPOINT` and exporting
  in-process — is now historical. Only the Go app remains in the shared managed
  environment, so the Go OTLP→agent stream is the **sole** request telemetry
  source. `AppRoleName` is uniformly `town-crier-api-go`; there is no longer a
  `town-crier-api` role to compare against.
- **The MinReplicas follow-up is done.** The prod Go app runs the warm-instance
  `MinReplicas` policy (active via v0.14.1), so it no longer scales from zero.
- **Retained:** the agent configuration on `cae-town-crier-shared`
  (`AppInsightsConfiguration` + `OpenTelemetryConfiguration` with traces and logs
  destinations) and the Go-side `SetupTelemetry` pipeline are unchanged and remain
  the production observability path. OTLP **metrics** from the Go app are still
  not configured.
