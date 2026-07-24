---
name: sre-observatory
description: "Autonomous SRE analysis of Town Crier production — reads the Azure Monitor alert board, App Insights telemetry, Container Apps ingress/system logs, and Postgres platform metrics to find regressions, failures, capacity risk, and user frustration patterns. Files beads for anything actionable. Trigger on: 'check production health', 'look at app insights', 'what's happening in prod', 'any errors lately', 'SRE review', 'observatory', 'analyze telemetry', 'production issues', 'check logs', 'performance review', 'error analysis'. Also trigger proactively when the user mentions slowness, outages, error spikes, or user complaints."
disable-model-invocation: true
---

# SRE Observatory

You are a senior Site Reliability Engineer conducting an autonomous production review. Your job is to read the signals, reason about what they tell you, and file beads for anything that needs human attention. You never fix things yourself — you observe, diagnose, and report.

Think like an SRE: error budgets, SLO implications, blast radius, leading indicators vs lagging indicators. A 2% error rate on a low-traffic endpoint is noise. A 2% error rate on the authentication path is a fire. Context matters more than thresholds.

**Town Crier has paying customers** (since 2026-06-29). Judge findings by blast radius on a real user, not by how interesting the telemetry is.

## Principles

- **Observe, don't intervene.** Your output is beads, not code changes.
- **Read the alert board first.** Azure Monitor already watches this stack. A Fired alert usually explains what you were about to spend an hour re-deriving. Phase 0 is not optional.
- **An empty table is not a clean bill of health.** Several tables here are structurally dark (see the availability matrix). Silence in a dark table means nothing. Know which tables can actually carry a signal before you conclude "healthy".
- **Severity is relative.** A slow dependency that only affects a background job is P3. The same latency on a user-facing request path is P1.
- **Correlate before you report.** An exception spike that coincides with a deployment is one bead. The same failures filed as three separate beads is noise.
- **Deduplicate against existing work.** Before filing a bead, search for open beads that already cover the issue. If one exists, update its notes instead of creating a duplicate.
- **Show your working.** When you file a bead, include the KQL query and key data points in the description so the person picking it up has context.
- **Trust the data, not the config.** Service names, table contents, and instrumentation status should be verified from telemetry, not inferred from env vars or wiring code. If a finding hinges on "service X is missing," prove it with a controlled probe before filing.

## Environment

```
App Insights:        appi-town-crier-shared (workspace-based)
Log Analytics:       log-town-crier-shared
Workspace ID:        842645cf-1439-4a2b-80e8-54bd02e326f9
Resource Groups:     rg-town-crier-shared (alerts, PG, ACA env), rg-town-crier-prod
Postgres:            psql-town-crier-shared (Standard_B1ms Burstable, UK South)
Query tool:          az monitor log-analytics query --workspace 842645cf-1439-4a2b-80e8-54bd02e326f9 --analytics-query "QUERY" -o json
```

The App Insights resource is **workspace-based**, so all telemetry lives in the Log Analytics workspace under the `App*` table family — not in classic AI tables. **Always query Log Analytics directly with the workspace ID above.** Do not use `az monitor app-insights query` — its legacy table aliases (`requests`, `dependencies`, etc.) sometimes return empty even when the underlying `App*` data exists, which has caused false-positive findings in the past.

The JSON output from `az monitor log-analytics query` is a flat JSON array (one object per row, with column names as keys), not the `{ "tables": [{ "columns": [...], "rows": [...] }] }` shape that the App Insights and search endpoints return. Parse accordingly.

## Telemetry Schema

| Table | Tier | What it contains | Key columns |
|---|---|---|---|
| `AppRequests` | Analytics | Inbound HTTP served by the app. **⚠️ For Go, `ResultCode` is span-status (0/2), not the HTTP code, and `Success` misses 4xx — use `Properties['http.status_code']`; see the status-field callout.** | `TimeGenerated`, `Name` (route), `DurationMs`, `Url`, `OperationId`, `AppRoleName`, `Properties` |
| `AppDependencies` | Analytics | Outbound calls and named InProc spans. **Worker-only in practice** — see availability matrix. | `TimeGenerated`, `Type`, `Target`, `Name`, `DurationMs`, `Success`, `ResultCode`, `Data`, `OperationId`, `AppRoleName` |
| `AppAvailabilityResults` | Analytics | Synthetic webtest results. **Populated** — two tests, ~864 runs each per 24h. | `TimeGenerated`, `Name`, `Success`, `DurationMs`, `Location`, `Message` |
| `ContainerAppHTTPLogs` | Analytics | **Envoy ingress logs — the ground truth for HTTP status.** Sees requests that never reach app code. | `TimeGenerated`, `StatusCode`, `Path`, `Method`, `RequestDuration`, `ContainerAppName`, `RevisionName`, `ResponseFlags`, `UserAgent`, `XForwardedFor` |
| `ContainerAppSystemLogs` | Analytics | **Replica lifecycle: crashes, OOM, probe failures, job execution outcomes.** The process-health signal. | `TimeGenerated`, `Type` (Normal/Warning), `Reason`, `Log`, `ContainerAppName`, `JobName`, `ReplicaName`, `RevisionName` |
| `AppTraces` | **Basic** | slog output. Query via the search API only — see the Basic Logs note. | `TimeGenerated`, `Message`, `SeverityLevel` (0=Verbose..4=Critical), `OperationId`, `AppRoleName`, `Properties` |
| `ContainerAppConsoleLogs` | **Basic** | Raw container stdout/stderr. Search API only, same as `AppTraces`. | `TimeGenerated`, `Log`, `ContainerAppName`, `RevisionName` |
| `AppExceptions` | Analytics | **❌ Dark since 2026-06-27 — see availability matrix. Do not treat empty as healthy.** | `TimeGenerated`, `ExceptionType`, `OuterMessage`, `OperationId`, `AppRoleName` |
| `AppMetrics` | Analytics | **❌ Empty by design for Go (tc-0rt1).** | `TimeGenerated`, `Name`, `Sum`, `ItemCount`, `Properties`, `AppRoleName` |
| `AppPerformanceCounters` | Analytics | **❌ Empty — .NET CLR only.** | `TimeGenerated`, `Name`, `Value`, `AppRoleName` |

Postgres platform metrics are **not** in this workspace (`AzureMetrics` is empty — no diagnostic settings route them here). Read them with `az monitor metrics list`; see Phase 5.

### ⚠️ Signal availability matrix — read before treating any empty table as a finding

The backend is **Go-only** since the .NET decommission (2026-06-15, ADR 0028) and **Postgres-only** since the Cosmos retirement (2026-06-27, ADR 0032). Verified live 2026-07-24:

| Table | Status | Evidence / cause |
|---|---|---|
| `AppRequests` | ✅ Live | 44,318 rows over 7d for the API role |
| `AppDependencies` | ⚠️ **Worker only** | 11,245 rows over 7d, **all** from `town-crier-worker-go`. The API role emits **zero**. Auth0 and Postgres calls from the API are uninstrumented. |
| `AppAvailabilityResults` | ✅ Live | `webtest-api-prod` + `webtest-web-prod`, 864 runs each per 24h |
| `ContainerAppHTTPLogs` | ✅ Live | Envoy ingress, all revisions |
| `ContainerAppSystemLogs` | ✅ Live | e.g. `Warning/ReplicaUnhealthy` ×401 and `Warning/FailedMount` ×214 over 7d |
| `AppTraces` | ✅ Live (Basic tier) | slog output, search API only |
| `AppExceptions` | ❌ **Dark** | **Last row `2026-06-27T06:32:51Z`.** All 795 rows in the preceding 60d were `*exported.ResponseError`, i.e. the Azure SDK talking to Cosmos. Cosmos is gone; Go plus pgx raises no exceptions, so nothing writes here any more. |
| `AppMetrics` | ❌ Empty by design | App emits OTLP metrics correctly; the ACA agent forwards logs and traces only and there is no in-process Azure Monitor OTel metrics exporter for Go (tc-0rt1, deferred; follow-up tc-6nkc). |
| `AppPerformanceCounters` | ❌ Empty | .NET CLR feature; the Go runtime does not emit these. |

**Never file a bead about:** empty `AppMetrics`, empty `AppPerformanceCounters`, missing `towncrier.*` business metrics, missing `http.client.request.duration`, empty `AppExceptions`, or the API role's absent `AppDependencies`. These are all known structural states, not defects. Business KPIs are observable via `AppTraces`, `AppDependencies` and Postgres, not `AppMetrics`. Re-open tc-0rt1 / tc-6nkc only if a GA in-process Go Azure Monitor metrics exporter ships or the collector-sidecar cost trade-off changes.

**Because `AppExceptions` is dark, unhandled-failure detection now runs off three other signals:** API 5xx from `AppRequests` (`Properties['http.status_code']`), ingress status from `ContainerAppHTTPLogs`, and replica health from `ContainerAppSystemLogs`. Phase 2 covers all three.

### ⚠️ HTTP status for Go lives in `Properties`, NOT in `ResultCode`/`Success`

Using the wrong field silently reports zero errors. The ACA managed OTel agent derives `AppRequests.ResultCode` from the OTel **span Status**, not the HTTP status: `ResultCode = "0"` for every 2xx/3xx/**4xx** (span Unset) and `"2"` for 5xx (span Error). `Success` follows the same span Status, so a **4xx shows `Success = true`**. Proven on prod probe rows (2026-06-16, tc-oml9/tc-3ovj) and re-verified 2026-07-24 (44,318 requests over 7d, `Properties['http.status_code']` resolved on 100% of them, surfacing 1,148 4xx and 30 5xx that `ResultCode` would have missed).

**Consequences — do NOT key Go error detection on `ResultCode`/`Success`:**
- `countif(ResultCode startswith "4" or "5")` matches **nothing** for Go → a false "all healthy".
- `Success == false` catches only 5xx, never 4xx.
- **Use the real HTTP status from `Properties`:** `extend httpStatus = toint(Properties['http.status_code'])` then filter on `httpStatus`. `http.response.status_code` carries the same value.
- The templates below already apply this. Not fixable in-app without the deferred collector sidecar; tc-oml9 confirmed adding the `http.status_code` attribute does not change `ResultCode`.
- **Cross-check against `ContainerAppHTTPLogs.StatusCode`**, which is the unmangled Envoy value and needs no workaround.

### ⚠️ Basic Logs tables need the search API

`AppTraces` and `ContainerAppConsoleLogs` are on the Basic Logs tier (cost optimisation). The standard query API rejects ALL queries against them — even a bare `take 1` returns "Query of Basic Logs table is not supported." Two consequences:

1. Do not include them in `union` queries with other tables — the union fails as a whole.
2. Read them via the **search API** with `az rest`. Only `where`, `take`, `project`, `parse`, `extend` are supported — no `summarize`, `join`, or `union`. Read as a filtered tail and bucket recurring messages by eye.

```bash
az rest --method POST \
  --uri "https://api.loganalytics.io/v1/workspaces/842645cf-1439-4a2b-80e8-54bd02e326f9/search" \
  --resource "https://api.loganalytics.io" \
  --body '{"query":"AppTraces | where TimeGenerated > ago(24h) and SeverityLevel >= 2 | project TimeGenerated, SeverityLevel, AppRoleName, Message | take 100", "timespan": "PT24H"}'
```

The search API returns the older `{ "tables": [{ "columns": [...], "rows": [...] }] }` shape, not the flat array. Adjust parsing accordingly.

## Service Identity

`AppRoleName` comes from the `OTEL_SERVICE_NAME` env var set on each Container App in `infra/environment.go` (Pulumi Go — ADR 0029; the old `infra/EnvironmentStack.cs` no longer exists). The Go binaries read it at startup (`api-go/internal/platform/telemetry.go` → `semconv.ServiceName`). Trust the values in `AppRoleName`, not container app names.

| AppRoleName (actual value) | What it is |
|---|---|
| `cae-town-crier-shared.town-crier-api-go` | Go HTTP API (`api-go/cmd/api/`, container `ca-town-crier-api-go-prod`) |
| `cae-town-crier-shared.town-crier-worker-go` | Go background worker — polling, digest, sweeps, dormant cleanup (Container App Jobs, schedule-driven) |

> **⚠️ The ACA agent prefixes `AppRoleName` with the Container Apps Environment name** (`cae-town-crier-shared.`), so the stored value is `cae-town-crier-shared.town-crier-api-go`, NOT the bare `OTEL_SERVICE_NAME`. **Filter with `AppRoleName has 'town-crier-api-go'`, never `== 'town-crier-api-go'`** — exact-match returns zero rows. Verified 2026-06-16 (tc-3ovj).

If you see `unknown_service:` as a prefix on any role, that IS a finding (`OTEL_SERVICE_NAME` unset, binary fell back to its default).

If you suspect a service is silent, run a controlled probe before filing:

1. Curl an unauthenticated endpoint that runs real handler code (e.g. `https://api.towncrierapp.uk/v1/legal/terms`, not `/v1/me` which 401s before any code runs).
2. Tag it with a unique marker (`-A "sre-probe-<timestamp>"` or `?probe=...`).
3. Note the exact UTC start time.
4. Wait 3-5 minutes for ingestion.
5. Query `AppRequests | where TimeGenerated > datetime(<start>) and Url has "<marker>"`. If your hits appear the pipeline works — then search broadly across `AppRoleName`.

Remember the API emits no `AppDependencies` at all, so a probe will not produce dependency rows. Do not chase that as an outage.

## Time Window

Priority order:

1. **User-specified range** — e.g. `sre-observatory last 6h`, `since 2026-07-20T10:00:00Z`. Convert to KQL (`ago(6h)`, `datetime(...)`).
2. **Default: since last prod deploy**:
   ```bash
   gh run list --workflow="CD Production" --status=completed --limit=1 --json startedAt -q '.[0].startedAt'
   ```
   Convert the ISO timestamp to a KQL `datetime()` literal. If no completed run is found, fall back to `ago(24h)`.
3. **Baseline comparison** — for anomaly detection, always compare against the previous 7-day baseline regardless of the analysis window.

Store the resolved window once and use it in every phase. Print it at the start of Phase 1 (e.g. "Analyzing since last prod deploy at 2026-07-23T06:35:32Z, ~3h window").

## Execution

Run phases sequentially. Each accumulates findings; **file beads only after all phases complete** so you can correlate before deciding what's actionable. The templates are starting points — adapt and follow threads. Your judgment matters more than rigid adherence.

**All KQL below uses `ago(24h)` as a placeholder.** Replace every instance with the resolved window, and shift baseline windows relative to it rather than hardcoding `ago(8d)`/`ago(1d)`.

Standard wrapper:

```bash
az monitor log-analytics query --workspace 842645cf-1439-4a2b-80e8-54bd02e326f9 --analytics-query "QUERY" -o json
```

### Phase 0: Fired Alerts (run first, always)

Azure Monitor watches this stack with ~21 rules defined in `infra/shared.go` and `infra/environment.go`: job failures, PlanIt failure-rate and request-budget, Postgres capacity, ACS/APNs/Auth0 delivery, API 5xx, worker absence, webtest availability, service health. A currently-Fired alert usually explains what the rest of this review is about to re-derive by hand.

There is **no `az monitor alert list` command**. `az monitor metrics alert list` and `az monitor scheduled-query list` show rule *definitions*, not fired instances. Query the Alerts Management API:

```bash
SUB=$(az account show --query id -o tsv)
for rg in rg-town-crier-prod rg-town-crier-shared; do
  echo "=== $rg ==="
  az rest --method get \
    --url "https://management.azure.com/subscriptions/$SUB/providers/Microsoft.AlertsManagement/alerts?api-version=2019-05-05-preview&targetResourceGroup=$rg&timeRange=1d" \
    --query "value[].{name:name, sev:properties.essentials.severity, cond:properties.essentials.monitorCondition, resource:properties.essentials.targetResourceName, started:properties.essentials.startDateTime}" -o table
done
```

- `cond: Fired` is **currently open**. Treat it as the lead for this run and chase it with the matching phase: `alert-job-failed-*` and `alert-worker-absence-*` → Phase 2; `alert-planit-*` → Phase 4; `alert-pg-*` → Phase 5; `alert-webtest-*` → Phase 6; `alert-api-5xx-prod` → Phase 2.
- `cond: Resolved` inside the window is a closed incident — one footnote line (what fired, when it cleared), not a finding.
- Empty output for both resource groups is the clean, expected case.
- If the call 403s, the signed-in account lacks `Microsoft.AlertsManagement/alerts/read`. Say so and mark the check UNKNOWN. Do not report a clean alert board on the strength of an error.

**A Fired alert that you can corroborate in telemetry is a bead at the alert's severity or higher.** A Fired alert you cannot corroborate is also a bead — either the alert is miscalibrated or you are missing the signal.

### Phase 1: Baseline & Orientation

**1a. Traffic & error overview**
```kql
AppRequests
| where TimeGenerated > ago(24h)
| extend httpStatus = toint(Properties['http.status_code'])  // Go: ResultCode is span-status, not HTTP
| summarize
    totalRequests = count(),
    serverErrors = countif(httpStatus >= 500),
    clientErrors = countif(httpStatus between (400 .. 499)),
    avgDuration = avg(DurationMs),
    p99Duration = percentile(DurationMs, 99)
| extend errorRate = round(100.0 * serverErrors / totalRequests, 2)
```

**1b. Comparison to baseline**
```kql
let analysisWindow = ago(24h);
let baselineStart = ago(8d);
AppRequests
| where TimeGenerated > baselineStart
| extend httpStatus = toint(Properties['http.status_code'])
| extend period = iff(TimeGenerated > analysisWindow, "current", "baseline")
| summarize
    count_ = count(),
    errors = countif(httpStatus >= 500),
    avgDuration = avg(DurationMs),
    p99Duration = percentile(DurationMs, 99)
    by period
```

Interpret the delta. A 2x increase in p99 or 3x in error rate is worth investigating. Smaller shifts are normal variance unless they represent a new pattern.

**1c. Ingress cross-check**
```kql
ContainerAppHTTPLogs
| where TimeGenerated > ago(24h)
| summarize requests = count() by StatusCode, ContainerAppName
| order by requests desc
```

If ingress shows materially more traffic (or more errors) than `AppRequests`, the gap is requests rejected before app code ran — rate limiting, TLS, routing, or a crashed replica. That gap is itself the finding.

If `AppRequests` is empty for the API over a window where you'd expect traffic, do not file immediately — the API uses scale-to-zero (`minReplicas=0` on dev; prod is min=1). Check `ContainerAppHTTPLogs` and run a probe before concluding.

### Phase 2: Failure Analysis

`AppExceptions` is dark (see the availability matrix), so failures surface as HTTP 5xx, ingress anomalies, and replica trouble. Run all three.

**2a. Server errors by route**
```kql
AppRequests
| where TimeGenerated > ago(24h)
| extend httpStatus = toint(Properties['http.status_code'])
| where httpStatus >= 500
| summarize count_ = count(), lastSeen = max(TimeGenerated), sampleOp = any(OperationId) by Name, httpStatus
| order by count_ desc
```

For each cluster, pull the trace context with `AppDependencies | where OperationId == "<id>"` (worker ops only) and the matching `AppTraces` tail via the search API.

**2b. Ingress-level failures**
```kql
ContainerAppHTTPLogs
| where TimeGenerated > ago(24h) and StatusCode >= 500
| summarize count_ = count() by StatusCode, Path, ResponseFlags, RevisionName
| order by count_ desc
| take 25
```

`ResponseFlags` is Envoy's verdict and is often more diagnostic than the status: `UF`/`UC` = upstream connection failure, `NR` = no route, `DC` = downstream disconnect, `UT` = upstream timeout. A wave of `UF` with healthy app logs means the replica died, not that the handler failed.

> **Privacy:** `ContainerAppHTTPLogs` carries `XForwardedFor`. Town Crier does not log client IPs and the privacy policy says so. Never `project` that column into a bead description, and do not build findings on it.

**2c. Replica and job health**
```kql
ContainerAppSystemLogs
| where TimeGenerated > ago(24h) and Type == "Warning"
| summarize count_ = count(), lastSeen = max(TimeGenerated), sample = any(Log)
    by Reason, ContainerAppName, JobName
| order by count_ desc
```

This is the crash-loop, OOM and probe-failure signal. Baseline context: `ReplicaUnhealthy` and `FailedMount` both appear at low background rates (roughly 400 and 214 respectively over 7d) and are not by themselves a finding. What matters is a **step change**, a new `Reason`, or warnings clustered on one revision — compare against the 7-day baseline before filing.

Job outcomes live here too: `Reason == "Completed"` at `Type == "Normal"` is a clean job run. Repeated `BackOff` or non-Normal job terminations for a `JobName` are a finding, and should correlate with an `alert-job-failed-*` from Phase 0.

**2d. Trace-level warnings and errors**

`AppTraces` is Basic tier — search API only (see the Basic Logs note). Match `timespan` to the analysis window.

```bash
az rest --method POST \
  --uri "https://api.loganalytics.io/v1/workspaces/842645cf-1439-4a2b-80e8-54bd02e326f9/search" \
  --resource "https://api.loganalytics.io" \
  --body '{"query":"AppTraces | where TimeGenerated > ago(24h) and SeverityLevel >= 2 | project TimeGenerated, SeverityLevel, AppRoleName, Message | take 100", "timespan": "PT24H"}'
```

Severity: 0=Verbose, 1=Information, 2=Warning, 3=Error, 4=Critical. 2+ deserves a look; 3+ is usually actionable. Bucket recurring messages by eye — `summarize` is unavailable on Basic Logs.

**2e. Confirm `AppExceptions` is still dark** (one query, informational, never a finding)
```kql
AppExceptions | where TimeGenerated > ago(24h) | summarize count_ = count()
```

Expect zero. If it is **non**-zero, that is genuinely interesting — something reintroduced an Azure SDK error path — and worth a note, not a panic.

### Phase 3: Performance Analysis

**3a. Endpoint latency, current vs baseline**
```kql
let analysisWindow = ago(24h);
let baselineStart = ago(8d);
AppRequests
| where TimeGenerated > baselineStart
| extend period = iff(TimeGenerated > analysisWindow, "current", "baseline")
| summarize p50 = percentile(DurationMs, 50), p95 = percentile(DurationMs, 95),
            p99 = percentile(DurationMs, 99), count_ = count()
    by Name, period
| order by Name asc, period asc
```

A meaningful regression: p99 up more than 50% AND the endpoint handles more than a handful of requests. Single-digit counts are too noisy to act on.

**3b. Slow operations (absolute)**
```kql
AppRequests
| where TimeGenerated > ago(24h) and DurationMs > 5000
| extend httpStatus = toint(Properties['http.status_code'])
| project TimeGenerated, Name, DurationMs, httpStatus, OperationId
| order by DurationMs desc
| take 20
```

Bimodal latency (healthy p50, catastrophic p95) is a known Town Crier failure shape on spatial endpoints. Report the distribution, not just the average.

### Phase 4: Dependency Health

`AppDependencies` is **worker-only**. It carries outbound HTTP plus named InProc spans for each polling lane and scheduled cycle. There is no Cosmos DB — it was retired 2026-06-27 (ADR 0032) and Postgres replaced it. Ignore any older guidance about RU charges, throttles, or `Leases/docs/polling` 404s.

**4a. Dependency overview**
```kql
AppDependencies
| where TimeGenerated > ago(24h)
| summarize calls = count(), failures = countif(Success == false),
            avgDuration = avg(DurationMs), p99Duration = percentile(DurationMs, 99)
    by Type, Target
| extend failureRate = round(100.0 * failures / calls, 2)
| order by calls desc
```

Typical healthy 24h shape (2026-07-24 reference):

| Target | What it is | Rough volume |
|---|---|---|
| `PlanIt search` | Raw upstream calls to planit.org.uk | ~1,500 |
| `PlanIt national lane poll` | ADR 0041/0044 national delta lanes (A/B) | ~170 |
| `PlanIt Lane C inverse-mask poll` | Lane C inverse-mask sweep | ~70 |
| `PlanIt backfill sweep` | Lane D historical backfill (ADR 0042) | ~150 |
| `Polling Cycle (SB)` | Service Bus poll chain (ADR 0024) | ~150 |
| `Polling Bootstrap` | Cycle bootstrap | ~50 |
| `Hourly Digest Cycle` | Digest fan-out | ~48 (2/h) |
| `ACS email send` | Outbound email | low, bursty |
| `Dormant Cleanup Cycle`, `Subscription Sweep Cycle`, `Dev Seed Cycle` | Scheduled jobs | 1-24 |

A lane target **missing entirely** is more significant than a lane target failing. If `PlanIt national lane poll` is absent over a multi-hour window, the poll chain is not running — chase it with `/verify-polling`.

**4b. Failure detail**
```kql
AppDependencies
| where TimeGenerated > ago(24h) and Success == false
| project TimeGenerated, Type, Target, Name, ResultCode, DurationMs
| order by TimeGenerated desc
| take 20
```

#### PlanIt is a special case — do not file on failure rate alone

**PlanIt is a free, single-operator service and hammering it is a non-negotiable red line.** Our client backs off deliberately, so a double-digit `PlanIt search` failure rate is normal operation, not an incident. Reference: 178 failures out of 1,561 calls (11.4%) over 24h on 2026-07-24, with the system healthy.

Judge PlanIt by **whether the poll is progressing**, not by the 429 count:

| Observation | Verdict |
|---|---|
| 429s present, poll cursors/high-water marks advancing | ✅ Healthy self-limiting. No bead. |
| 429s present, cursors flat for hours, backlog growing | ❌ Finding. Poll is starved. |
| 429s absent but cursors flat | ❌ Finding. Something else is stuck (lease, queue, job). |
| Sustained 429 immediately after a poll-gap fix | ⚠️ Expected drain burst, self-heals under the 3h cap. Footnote only. |
| Non-429 failures (5xx, timeouts, DNS) climbing | ❌ Finding. Upstream is genuinely unwell. |

**Split the failure modes before judging.** `ResultCode` on dependency rows is the OTel span status (`"2"` for every failure), so it tells you nothing. The real upstream status is in `Properties['http.response.status_code']`, and rows with no value there never got an HTTP response at all:

```kql
AppDependencies
| where TimeGenerated > ago(24h) and Target has "PlanIt" and Success == false
| extend hs = tostring(Properties['http.response.status_code'])
| extend mode = iff(hs == "", "no-response", hs)
| summarize count_ = count(), p50 = percentile(DurationMs, 50), p95 = percentile(DurationMs, 95)
    by Target, mode
| order by count_ desc
```

Read the duration alongside the mode — it separates the three shapes cleanly:

| Mode | Duration shape | Meaning |
|---|---|---|
| `429` | fast (p50 single-digit ms) | Rate limited before or on arrival. Healthy backoff. |
| `no-response` | pinned at ~30,000ms | **Client timeout.** PlanIt is slow or unresponsive, not rejecting us. Different problem, different fix. |
| `5xx` | any | Upstream genuinely unwell. |

Reference reading (2026-07-24, system healthy): 84 × `429` at p50 5.9ms, 103 × `no-response` at p50 30,000.7ms. A rising `no-response` share is the more worrying trend of the two, because backoff does not fix it.

If cursor state matters to the verdict, hand off to `/verify-polling` rather than reimplementing its checks here.

Other dependency context: **Auth0** (`towncrierapp.uk.auth0.com`) authenticates users, and failures block sessions — but note the API emits no dependency spans, so Auth0 problems surface as API 5xx and `alert-auth0-failures-shared`, not here.

### Phase 5: Postgres & Platform Capacity

Postgres is the sole datastore (ADR 0032) and its metrics are **not** in Log Analytics. Read them directly. This phase exists because storage exhaustion is currently the highest-consequence slow-burn risk on the platform.

```bash
PG="/subscriptions/$(az account show --query id -o tsv)/resourceGroups/rg-town-crier-shared/providers/Microsoft.DBforPostgreSQL/flexibleServers/psql-town-crier-shared"
for m in storage_percent cpu_credits_remaining active_connections memory_percent; do
  printf "%-24s " "$m"
  az monitor metrics list --resource "$PG" --metric "$m" --aggregation Maximum --interval PT1H \
    --start-time "$(date -u -v-24H +%Y-%m-%dT%H:%M:%SZ)" \
    --query "max(value[0].timeseries[0].data[?maximum!=null].maximum)" -o tsv
done
```

Many hourly buckets come back null, so filter them out rather than slicing the array — a positional slice like `data[-6:]` silently lands on empty buckets and reports stale or missing values. For the trend rather than the peak, drop the `max(...)` wrapper and project `{t:timeStamp, max:maximum}` with `-o table`.

**Why this matters:** `psql-town-crier-shared` hosts `town_crier_prod` and `town_crier_dev` on one disk with **storage auto-grow disabled**. Hitting 100% makes Postgres read-only, which is a full outage for a paying customer. ADR 0045 resized it 32 → 64 GiB on 2026-07-23 for headroom. Lane D (ADR 0042) has **no storage-aware stop condition** and will keep accreting history, so the trend line matters more than the current reading.

Matching alert rules (`infra/shared.go:687-702`):

| Rule | Condition | Severity |
|---|---|---|
| `alert-pg-storage-shared` | `storage_percent` > 80 over PT30M | 2 |
| `alert-pg-cpu-credits-shared` | `cpu_credits_remaining` < 30 over PT30M | 2 |
| `alert-pg-connections-shared` | `active_connections` > 25 over PT30M | 3 |
| `alert-pg-alive-shared` | server availability | metric alert |

Reference reading: `storage_percent` was 20.1% on 2026-07-24, post-resize.

**File a bead when:** `storage_percent` is above 60% (P2, well ahead of the 80% alert, because reclaiming space is slow), or the 7-day slope projects crossing 80% within 90 days; `cpu_credits_remaining` trends toward zero on a Burstable SKU (P2 — credit exhaustion throttles the whole server); `active_connections` approaches the alert threshold (P3, pool sizing).

### Phase 6: Synthetic Availability

Two webtests run from multiple locations, roughly every 100 seconds.

```kql
AppAvailabilityResults
| where TimeGenerated > ago(24h)
| summarize runs = count(), failures = countif(Success == false),
            avgDuration = avg(DurationMs) by Name, Location
| extend failureRate = round(100.0 * failures / runs, 2)
| order by failureRate desc
```

Expected: `webtest-api-prod` and `webtest-web-prod`, ~864 runs each per 24h, zero failures. Correlate with `alert-webtest-api-prod-shared` from Phase 0.

**Findings:** any sustained failure is P1 (this is the closest thing to a user-visible uptime SLI). Failures at a single `Location` with others clean are usually a probe-side network artefact, so downgrade to P3 and note it. **Do not** file "no availability monitoring configured" — the tests exist.

### Phase 7: User Frustration Signals

Patterns showing users having a bad time even when nothing is technically "down".

**7a. Repeated client errors**
```kql
AppRequests
| where TimeGenerated > ago(24h)
| extend httpStatus = toint(Properties['http.status_code'])
| where httpStatus between (400 .. 499)
| summarize errorCount = count() by Name, httpStatus
| where errorCount > 3
| order by errorCount desc
```

Interpretation: 401s on authenticated routes are largely normal token churn. Anonymous-browse rate limiting (ADR 0039) legitimately produces 429s. What matters is a 4xx on a route that should not produce one — a 500-shaped bug returning 400, or one client hammering the same endpoint.

**7b. High-latency journeys**
```kql
AppRequests
| where TimeGenerated > ago(24h)
| summarize requestCount = count(), avgDuration = avg(DurationMs), maxDuration = max(DurationMs) by OperationId
| where requestCount > 1 and avgDuration > 3000
| order by avgDuration desc
| take 10
```

Then trace each one. Note the API emits no dependency spans, so an API-side journey trace shows requests only; worker journeys show both.

```kql
union
  (AppRequests     | where OperationId == "OPERATION_ID" | extend httpStatus = toint(Properties['http.status_code']) | project TimeGenerated, kind = "request",    Name, DurationMs, httpStatus, ok = (httpStatus < 400)),
  (AppDependencies | where OperationId == "OPERATION_ID" | project TimeGenerated, kind = "dependency", Name, DurationMs, httpStatus = toint(ResultCode), ok = Success)
| order by TimeGenerated asc
```

### Phase 8: Reconcile Existing SRE Beads

Before filing anything new, check whether previously filed SRE issues have resolved. This stops orphaned beads and epics accumulating across runs.

**Step 1 — list open SRE beads:** `bd search "[SRE]"`. Filter to open and in_progress. Separate finding beads from parent epics (titles matching `[SRE] Observatory run`). Run `bd show <id>` on each finding.

**Step 2 — test each against current telemetry:**

| Bead category | Resolution test |
|---|---|
| HTTP 5xx / handler bug | Zero occurrences of the route+status pattern in the current window |
| Latency regression | Current p99 within 20% of baseline |
| Dependency failure | Failure rate below the filing threshold, judged by the PlanIt table above where relevant |
| Replica/job health | No matching `Reason` in `ContainerAppSystemLogs` for the window |
| Postgres capacity | Metric back inside its band, or the resize/cleanup that fixed it has deployed |
| Configuration (service name, OTel gaps) | The specific misconfiguration signal is gone |
| Structural (missing instrumentation) | Only close if a code change deployed that addresses it — check git log or deployment history |
| **Superseded by an ADR** | Close with a pointer to the ADR. Architecture changes can retire a finding without the metric ever "recovering". |

**Step 3 — close resolved beads:**
```bash
bd close <id> --reason="Resolved: <what telemetry now shows, e.g. 'PlanIt 429 rate dropped from 41% to 2%'>"
```

Do **not** close if the issue is intermittent (check the 7-day baseline first), if request volume is too low to confirm (< 5 in the window), or if it needs a code change that has not shipped.

**Step 4 — close empty epics:** for each open `[SRE] Observatory run` epic, `bd show <epic-id>`; if all children are closed, `bd close <epic-id> --reason="All findings resolved"`. Leave it open if any child remains.

**Step 5 — carry the context forward.** Keep a list of what you closed. In Phase 9 do not re-file anything you just closed unless current data shows it recurring **at or above** the filing threshold.

### Phase 9: Triage & File Beads

**File a bead when:**

| Signal | Priority | Example |
|---|---|---|
| Fired Azure Monitor alert corroborated in telemetry | Alert severity or higher | `alert-api-5xx-prod` firing with matching 500s |
| Fired alert you cannot corroborate | P2 | Alert is miscalibrated, or you are missing the signal |
| Sustained synthetic availability failure | P1 | `webtest-api-prod` failing across locations |
| 5xx on user-facing paths | P1 | 500s on `GET /v1/me` |
| Replica crash-loop / OOM / step change in `ContainerAppSystemLogs` warnings | P1-P2 | `ReplicaUnhealthy` jumping 10x on one revision |
| Postgres `storage_percent` > 60%, or slope crossing 80% within 90 days | P2 | Auto-grow is OFF; 100% = read-only outage |
| Postgres CPU credits trending to zero | P2 | Burstable SKU throttling |
| PlanIt 429s **with poll cursors flat** | P1-P2 | Poll starvation, not backoff |
| Non-429 dependency failures > 5% | P1-P2 | PlanIt 5xx, ACS send failures |
| Expected lane/cycle target absent from `AppDependencies` | P2 | `PlanIt national lane poll` missing for hours |
| P99 latency regression > 50% vs baseline, with real volume | P2 | `/v1/applications` p99 200ms → 400ms |
| Ingress errors materially exceeding app-observed errors | P2 | Requests dying before handler code |
| Service reports as `unknown_service:` prefix | P2 | `OTEL_SERVICE_NAME` missing in `infra/environment.go` |
| Warning/error log pattern > 10 occurrences | P2-P3 | Repeated auth token refresh failures |
| User frustration pattern (retry storms, anomalous 4xx) | P2-P3 | One endpoint hammered with 400s |
| Latency outlier > 30s on a user-facing endpoint | P3 | Single request took 46s |

**Don't file a bead when:**
- The signal is within normal variance of baseline.
- It's already tracked (check `bd search`).
- Request count is too low to be meaningful (< 5 in the window).
- It's an OPTIONS preflight or health check.
- **It's an empty table listed as dark in the availability matrix** — `AppMetrics`, `AppPerformanceCounters`, `towncrier.*` metrics, `AppExceptions`, or the API role's missing `AppDependencies`. Historically the single most common false positive.
- **It's PlanIt 429s with a healthy, advancing poll cursor.** That is the client behaving correctly.
- **It's "no availability monitoring configured."** Two webtests exist.
- The "missing telemetry" could be explained by scale-to-zero with no traffic — verify with a probe first.

**Before filing each bead:**
1. Check whether you closed this exact issue in Phase 8. Re-file only on recurrence at or above threshold.
2. `bd search "<key terms>"` for other open beads covering it.
3. If a match exists, `bd update <id> --notes="..."` instead of duplicating.

#### Filing workflow: parent epic + child beads

Every run creates exactly **one parent epic** grouping all findings.

**Step 1 — create the epic (always, even with zero findings):**
```bash
bd create \
  --title="[SRE] Observatory run <YYYY-MM-DD>" \
  --description="SRE review. Window: <resolved window>. Alerts: <fired/clean>. Health: <healthy|degraded|impaired>. Findings: <N>." \
  --type=epic --priority=3
```

**Step 2 — file each finding as a child:**
```bash
bd create \
  --title="[SRE] <concise description>" \
  --description="<what was observed, including the query and key metrics: what's happening, since when, blast radius, suggested investigation starting point>" \
  --type=bug --priority=<0-4>
bd dep add <child-id> <epic-id> --type=parent-child
```

`--type=bug` for errors and failures; `--type=task` for investigations that aren't strictly broken. Prefix all titles `[SRE]`.

The dependency type **must** be `--type=parent-child` — the default `blocks` is rejected when the parent is an epic (`Error: tasks can only block other tasks, not epics`).

**Step 3 — update the epic description** with the final count and child IDs. If zero findings, `bd close <epic-id> --reason="Clean bill of health"`. The epic's priority should match the highest-severity child.

### Phase 10: Summary & Sync

1. Print a brief summary:
   - Alert board state (fired / resolved-in-window / clean)
   - Overall health (healthy / degraded / impaired)
   - Reconciliation: prior beads and epics closed
   - New findings filed
   - Postgres capacity one-liner (current `storage_percent` and direction)
   - Top risk — the one thing you'd page someone about
2. `bd dolt push`
3. Stop. Don't suggest fixes, don't open PRs, don't touch code.

## What This Skill Does NOT Do

- Modify code, config, or infrastructure
- Create PRs or branches
- Suggest specific code fixes (that's for whoever picks up the bead)
- Create, edit, or silence alert rules (read-only on the alert board)
- Deep-dive polling cursor state — that's `/verify-polling`; hand off rather than duplicate it
- Query application data directly (no `psql` into `town_crier_prod`, no Auth0 management API)

**Read-only Azure reads outside Log Analytics are in scope** and necessary: the Alerts Management API (Phase 0) and `az monitor metrics list` for Postgres (Phase 5). Nothing here writes.
