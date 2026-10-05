---
description: Verify Town Crier prod polling pipeline health (ADR 0049 day-window model)
---

Verify the Town Crier prod polling pipeline is healthy. Report concisely (✅ / ⚠️ / ❌ per check, evidence inline). Do NOT make code changes. If anything is broken, point at the specific symptom and code location, and don't guess at root cause. Read `POLLING.md` first; it holds the goal, the PlanIt facts and the hard call limits.

## Read this first: the model (ADR 0049)

Polling is one paced hourly job (`WORKER_MODE=poll`, cron `0 * * * *`, `api-go/internal/polling/runner.go`). There are no lanes, no Service Bus queue, no `poll_state` watermarks and no `dev-seed`. Anything checking those is stale.

- **Day windows.** Each night the runner re-reads every `start_date` day and every `decided_date` day in the alert band (ages 0 to 14) from `poll_window`. A window whose PlanIt `total` did not change costs one request (a count probe), and every window gets a full read at least every 7 days. A coverage band (ages 15 to 90) is read when its `last_complete_at` is null or older than 7 days, and stores records without alerting.
- **Day delta passes** run in the daytime at `POLLING_DELTA_SLOTS` for latency only.
- **Pacing.** The `planit_call` table is the only pacer state: at least 60 s between requests, a cap of 240 calls a budget day in prod (60 in dev), where the budget day runs 18:00 to 18:00 Europe/London. Backoff comes from the latest call: 429 uses `Retry-After` (max 3h, or 15m if absent), 403 is 24h, timeout or 5xx is 30m.
- **Events.** Ingest writes `application_event` rows (`new_application`, `decision`). The dispatcher inside the run applies the 14-day limit, quiet hours (22:00 to 07:00 Europe/London) and the surge check, then fans out. `poll_event` holds durable 24h health facts.
- **Health** is computed in Go each run and set on the `PlanIt poll run` span as `poll.health` (`ok`, `degraded`, `critical`) with `poll.health_reasons` and `poll.stop_reason` (`no_work`, `run_budget`, `daily_cap`, `backoff`, `rate_limited`, `forbidden`, `timeout`, `error`).
- **Dev** runs the same job on one authority (`POLLING_AREA_ID`) with an oracle that compares the day windows against a wide read and writes `poll_oracle_diff`.

Compute the current London time first. Daytime (06:00 to 18:00) runs only deltas, so a quiet daytime run with `stop_reason=no_work` is healthy. Windows are read at night (18:00 to 06:00).

### The most important rule: judge by health and window state, not by call counts

PlanIt is often slow or quiet, and 429s are normal self-limiting behaviour. Low call counts, a run that stops on `run_budget` or `daily_cap`, and a few 429s are not problems on their own. The job of this check is to tell these states apart:

| State | Signature | Verdict |
|---|---|---|
| **Healthy** | `poll.health=ok`, every alert-band window has `last_complete_at` since 18:00 of the last night | ✅ |
| **Healthy-backed-off** | Recent 429 or timeout, backoff active, `poll.health` still `ok` and the band verified | ✅ |
| **Degraded** | `poll.health=degraded`, with reasons such as `alert_band_unverified`, `window_short`, `window_violation`, `window_missed`, `events_low` | ⚠️ read the reason |
| **Critical** | `forbidden` (a 403) or `surge` | ❌ a person must act |
| **Not running** | No `PlanIt poll run` span for 2h | ❌ see Check 2 |

## Fixed environment

- **Two telemetry query paths — use the right one or get false gaps:**
  - **Short windows (≤1h), recent:** `az monitor app-insights query --app appi-town-crier-shared -g rg-town-crier-shared --analytics-query "..."` (classic schema: `traces`, `customMetrics`, `dependencies`, `exceptions`).
  - **Historical (≥6h) and ALL deploy-anchored queries:** `az monitor log-analytics query --workspace 842645cf-1439-4a2b-80e8-54bd02e326f9 --analytics-query "..."` (workspace schema: `AppMetrics`, `AppTraces`, `AppDependencies`, `AppExceptions`). Column map: `timestamp`→`TimeGenerated`, `cloud_RoleName`→`AppRoleName`, `name`→`Name`, `value`→`Sum`, `customDimensions['k']`→`Properties['k']`, `message`→`Message`, `resultCode`→`ResultCode`, `success`→`Success`, `duration`→`DurationMs`.
  - ⚠️ Do NOT use `az monitor app-insights query` for windows > ~1h — it surfaces only a partial recent slice and looks like a gap that isn't there.
- **⚠️ Worker role name is `cae-town-crier-shared.town-crier-worker-go`** — the environment prefix is part of the value. Filter with `AppRoleName has 'worker-go'`, never `AppRoleName == 'town-crier-worker-go'` (that matches nothing).
- **⚠️ AppMetrics can be silently empty.** This has happened in prod (metrics pipeline gap) with the rest of telemetry flowing. **Run Check 0 first**; if AppMetrics is empty, every metric-based check below is BLIND — fall back to Postgres (`planit_call`, `poll_window`) (ground truth) + `AppDependencies` + the `AppTraces` /search API, and say so in the report rather than reporting false-green.
- **AppTraces & ContainerAppConsoleLogs are Basic Logs tier** — `az monitor log-analytics query` errors on them. Query via the synchronous `/search` endpoint:
  ```bash
  az rest --method post \
    --url "https://api.loganalytics.io/v1/workspaces/842645cf-1439-4a2b-80e8-54bd02e326f9/search" \
    --resource "https://api.loganalytics.io" --headers "Content-Type=application/json" \
    --body '{"query":"AppTraces | where TimeGenerated > ago(2h) | where AppRoleName has '\''worker-go'\'' | where Message has '\''poll.window_short'\'' | project TimeGenerated, Message, Properties | take 20","timespan":"PT2H"}'
  ```
- **⚠️ PlanIt dependency `ResultCode` is the OTel status (0 = ok, 2 = error), NOT the HTTP status.** HTTP 429s live in `Properties['http.response.status_code']`. Any check doing `ResultCode == '429'` matches nothing.
- **Postgres (ground truth, always available even when AppMetrics is down):** server `psql-town-crier-shared.postgres.database.azure.com`, db `town_crier_prod`, in `rg-town-crier-shared`. Read-only via Entra AD token — **you must be the server's Entra admin** (or a provisioned AD role):
  ```bash
  IP=$(curl -s https://api.ipify.org)
  az postgres flexible-server firewall-rule create --server-name psql-town-crier-shared -g rg-town-crier-shared \
    --name verify-polling-tmp --start-ip-address "$IP" --end-ip-address "$IP" -o none
  export PGPASSWORD="$(az account get-access-token --resource-type oss-rdbms --query accessToken -o tsv)"
  PGUSER="$(az ad signed-in-user show --query userPrincipalName -o tsv)"
  CONN="host=psql-town-crier-shared.postgres.database.azure.com port=5432 dbname=town_crier_prod user=$PGUSER sslmode=require connect_timeout=15"
  psql "$CONN" -c "select 1;"   # ... run the queries below ...
  # cleanup when done:
  az postgres flexible-server firewall-rule delete --server-name psql-town-crier-shared -g rg-town-crier-shared --name verify-polling-tmp --yes -o none
  ```
  The Entra AD token expires ~hourly — refetch `PGPASSWORD` if a session runs long.
- **Jobs:** `job-tc-poll-prod` (hourly cron, `WORKER_MODE=poll`) in `rg-town-crier-prod`, and the dev `poll` job for the dev trial.
- **PlanIt cross-checks are a LAST resort and rate-limited.** PlanIt is a free single-operator service; hammering it is a red line, and a blocked laptop IP is unrecoverable. Prefer telemetry + Postgres. If you must confirm PlanIt's head directly: **≤5 calls total, never 2 within 60s, `pg_sz` ≤ 300, always a bounded (`different_start`/`start_date`) query, `select` mandatory.** State plainly in the report that a PlanIt call was made.

## Checks

### 0. Telemetry pipeline is alive (gating, run first)
```bash
for t in AppMetrics AppDependencies AppExceptions; do
  c=$(az monitor log-analytics query --workspace 842645cf-1439-4a2b-80e8-54bd02e326f9 \
      --analytics-query "$t | where TimeGenerated > ago(2h) | where AppRoleName has 'worker-go' | summarize c=count()" \
      -o tsv --query "[0].c" 2>/dev/null); echo "$t: ${c:-0}"; done
```
`AppDependencies` should be non-zero. **If `AppMetrics` is 0**, flag it (a real, separate defect) and treat metric checks as UNKNOWN, leaning on Postgres, AppDependencies and AppTraces.

### 0.5. Recent Azure Monitor alerts (orientation, run early)
```bash
SUB=$(az account show --query id -o tsv)
for rg in rg-town-crier-prod rg-town-crier-shared; do
  echo "=== $rg ==="
  az rest --method get \
    --url "https://management.azure.com/subscriptions/$SUB/providers/Microsoft.AlertsManagement/alerts?api-version=2019-05-05-preview&targetResourceGroup=$rg&timeRange=1d" \
    --query "value[].{name:name, sev:properties.essentials.severity, cond:properties.essentials.monitorCondition, resource:properties.essentials.targetResourceName, started:properties.essentials.startDateTime}" -o table
done
```
- The polling alerts are `alert-planit-poll-critical-shared` (Sev 1, `poll.health == "critical"` in 1h), `alert-planit-poll-degraded-shared` (Sev 2, `degraded` in 2h) and `alert-planit-poll-heartbeat-shared` (Sev 2, no `PlanIt poll run` span in 2h). The spans carry `deployment.environment`, and dev alerts are Sev 3.
- `cond: Fired` is **currently open**: treat it as a lead. `alert-job-failed-*` goes to Check 2, `alert-planit-poll-*` to Checks 3 to 5, `alert-pg-*` to Postgres capacity.
- `cond: Resolved` within the window is a closed incident: a one-line footnote, not a ❌.
- If the call 403s, mark this check UNKNOWN. Don't report a clean board on the strength of an error.

### 1. Deployment is current
```bash
gh run list --workflow='CD Production' --limit 3
```
Latest `CD Production` run `completed / success` and newer than the most recent merged polling PR.

### 2. The job is running and runs finish
```bash
az containerapp job execution list --name job-tc-poll-prod -g rg-town-crier-prod \
  --query "reverse(sort_by([].{start:properties.startTime,status:properties.status},&start))[:8]" -o table
```
```kql
AppDependencies
| where TimeGenerated > ago(6h) and Name == "PlanIt poll run" and AppRoleName has "worker-go"
| extend env = tostring(Properties['deployment.environment']),
         health = tostring(Properties['poll.health']),
         reasons = tostring(Properties['poll.health_reasons']),
         stop = tostring(Properties['poll.stop_reason']),
         pages = toint(Properties['poll.pages']),
         calls_today = toint(Properties['poll.calls_today'])
| project TimeGenerated, env, health, reasons, stop, pages, calls_today, DurationMs
| order by TimeGenerated desc
```
- Executions `Succeeded`, about one per hour. Overlapping runs exit 0 on the lease, so a run with no span and a fast exit is fine occasionally.
- No `PlanIt poll run` span for 2h in a row is ❌ (job disabled, lease stuck for its 65 min TTL, or a crash before the span).
- `stop_reason` `forbidden` or `error` repeating is ❌. `no_work` at night means every window is complete, which is the good outcome.

### 3. Alert band verified (the core check)
Every alert-band window (ages 0 to 14) should have completed a read since 18:00 Europe/London of the night that just ended. Read-only, against Postgres:
```bash
psql "$CONN" -c "select axis, day, last_complete_at, last_full_read_at, last_total from poll_window where day >= (now() at time zone 'Europe/London')::date - 14 order by day desc, axis;"
psql "$CONN" -c "select count(*) filter (where last_complete_at is null or last_complete_at < date_trunc('day', now() at time zone 'Europe/London' - interval '6 hours') + interval '18 hours' - interval '1 day') as unverified, count(*) as windows from poll_window where day >= (now() at time zone 'Europe/London')::date - 14;"
```
- After 06:00 London, `unverified` should be 0 and there should be 30 windows (15 days, two axes). During the night some windows are still due, which is expected.
- `unverified > 0` after 06:00 for a whole night is the `alert_band_unverified` reason (⚠️). Two nights running is ❌: look at Check 4 for why the runs stopped (cap, backoff, timeouts).
- `last_full_read_at` should never be older than 7 days for an alert-band window.

### 4. Pacing and budget (the call log)
```bash
psql "$CONN" -c "select work, count(*) calls, count(*) filter (where status=429) c429, count(*) filter (where status=403) c403, count(*) filter (where status is null) no_response from planit_call where at > now() - interval '24 hours' group by work order by calls desc;"
psql "$CONN" -c "select at, work, window_day, page_index, status, total, retry_after from planit_call order by at desc limit 15;"
psql "$CONN" -c "select at - lag(at) over (order by at) as gap from planit_call where at > now() - interval '3 hours' order by gap asc limit 3;"
```
- **Spacing:** the smallest gap between consecutive calls must be at least 60 s. Less is ❌ (point at `api-go/internal/polling/pacer.go`).
- **Budget:** total calls in the current 18:00 to 18:00 Europe/London budget day must be at most 240 in prod. Expect 175 to 225. Over the cap is ❌. Hitting it before morning on several nights is ⚠️ (compare prod 240 plus dev 60 against 300).
- **Daytime calls** should be deltas only (`delta_start`, `delta_decided`), at about 4 slots.
- **403 in the last 24h is ❌ critical.** Check that the User-Agent is `TownCrier/<version> (+https://towncrierapp.uk; support@towncrierapp.uk)`.
- **The first call after a 429** must be after its `retry_after` (backoff honoured). A call inside the backoff is ❌.
- Dependency status split, if `planit_call` is not enough (`ResultCode` is the OTel status, HTTP is in `Properties`):
```kql
AppDependencies
| where TimeGenerated > ago(24h) and Name == "PlanIt search" and AppRoleName has "worker-go"
| extend work = tostring(Properties['planit.work']), hs = tostring(Properties['http.response.status_code'])
| extend mode = iff(hs == "", "no-response", hs)
| summarize calls = count(), p50 = percentile(DurationMs, 50) by work, mode
| order by calls desc
```
`no-response` at about 30,000 ms is a client timeout. A rising share of those is the worrying trend, because backoff does not fix it.

### 5. Window integrity and self-checks
```kql
AppDependencies
| where TimeGenerated > ago(24h) and Name == "PlanIt poll run" and AppRoleName has "worker-go"
| extend violations = toint(Properties['poll.window_violations']), missed = toint(Properties['poll.window_missed']),
         reasons = tostring(Properties['poll.health_reasons'])
| summarize violations = sum(violations), missed = sum(missed), degraded_runs = countif(reasons != "")
```
- `poll.window_violations` must be 0 (a record on a window page had its axis date outside the window, so PlanIt's date filters changed behaviour). Non-zero is ❌.
- `poll.window_missed` must be 0 (a delta saw a record that a later full read of its window did not return). Non-zero is ❌ and means the window read is missing records.
- Short reads, if any, are in AppTraces (Basic Logs, use `/search`):
```bash
az rest --method post --url "https://api.loganalytics.io/v1/workspaces/842645cf-1439-4a2b-80e8-54bd02e326f9/search" \
  --resource "https://api.loganalytics.io" --headers "Content-Type=application/json" \
  --body '{"query":"AppTraces | where TimeGenerated > ago(24h) | where AppRoleName has '\''worker-go'\'' | where Message has '\''poll.window_short'\'' | project TimeGenerated, Message, Properties | take 20","timespan":"P1D"}'
```
A window short twice in one budget day is the `window_short` reason (⚠️).

### 6. Events and notifications reach users (the product outcome)
```bash
psql "$CONN" -c "select date_trunc('day', detected_at at time zone 'Europe/London') d, kind, status, count(*) from application_event where detected_at > now() - interval '7 days' group by 1,2,3 order by 1 desc, 2, 3;"
psql "$CONN" -c "select count(*) pending, min(detected_at) oldest from application_event where status = 'pending';"
psql "$CONN" -c "select date_trunc('day', created_at) d, count(*), count(*) filter (where push_sent) pushed, count(*) filter (where email_sent) emailed from notifications where created_at > now() - interval '7 days' group by 1 order by 1 desc;"
```
- A normal weekday is about 1,460 `new_application` events nationally and about 1,260 `decision` events. Weekday `new_application` events in the last 24h under 20% of the 14-day median is the `events_low` reason (⚠️). Weekends are low by nature.
- `pending` events should be empty outside quiet hours (22:00 to 07:00 London), where they wait until 07:00. Events pending for many hours in the daytime mean the dispatcher is not running (point at `api-go/internal/appevents/dispatcher.go`).
- `stale` events are those over 14 days old, or a surge. A `surge` health reason (more than 10,000 events in 24h) is ❌ critical: a PlanIt re-key or a bug. Its events were marked `stale` and nothing was sent.
- If events are being written but no notifications appear for matching watch zones, that is a real fan-out break, not a poll problem.
- `poll_event` holds the durable 24h health facts (for example a 403 or a short window). Use it to see why health was set when a run's span has rolled off.

### 7. Dev trial and oracle (dev only)
Skip for prod. The dev poller runs on one authority with `POLLING_ORACLE_ENABLED=true`. Against the dev database (`town_crier_dev`):
```bash
psql "$DEVCONN" -c "select day, axis, reason, count(*) from poll_oracle_diff where found_at > now() - interval '7 days' group by 1,2,3 order by 1 desc;"
```
- A row with reason `miss` is ❌ and blocks the prod tag. It is the `oracle_miss` health reason.
- `late`, `date_changed` and `planit_deleted` are explained, not failures. A row unclassified after two nights is ⚠️.
- The gate for the prod tag is at least 3 nights with no `miss`, no integrity or completeness failures and `poll.window_missed` at 0. The owner posts the per-night counts on GH#1178.

### 8. Exceptions
```bash
az monitor log-analytics query --workspace 842645cf-1439-4a2b-80e8-54bd02e326f9 \
  --analytics-query "AppExceptions | where TimeGenerated > ago(6h) | where AppRoleName has 'worker-go' | summarize c=count() by ProblemId, tostring(OuterMessage) | top 10 by c"
```
Empty or known-benign. A rate-limit error is expected and not a concern in any volume.

## Report format

State the current **London time and whether it is inside the night window (18:00 to 06:00)** up front, because the alert-band check depends on it.

1. **Poller status.** One row each for: job running (Check 2), alert band verified (Check 3), pacing and budget (Check 4), integrity and self-checks (Check 5), events and notifications (Check 6). ✅ / ⚠️ / ❌ with one line of evidence. Include the latest `poll.health` and reasons.
2. **Cross-cutting.** Azure Monitor alerts (any Fired?), deploy current, telemetry pipeline (AppMetrics present?), exceptions. One row each.
3. **Dev oracle** (only when asked about the trial): counts per night and any `miss`.

End with 1 to 3 watch items and one recommendation line. Keep prose under 200 words.

**Never flag as problems on their own:** a few 429s, backoff, an hourly cadence, a run stopping on `run_budget` or `no_work`, a quiet daytime, quiet-hours `pending` events, or a `PlanIt` timeout share that is flat. Escalate a 429 only when the first request after a backoff came before its `Retry-After`. Never make a live PlanIt call as part of this check: the hard limits in `POLLING.md` apply, and the call log already holds the facts.
