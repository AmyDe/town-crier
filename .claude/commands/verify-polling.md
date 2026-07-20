---
description: Verify Town Crier prod polling pipeline health (ADR 0041/0044 national-lane model)
---

Verify the Town Crier prod polling pipeline is healthy. Report concisely (✅ / ⚠️ / ❌ per check, evidence inline). Do NOT make code changes. If anything is broken, point at the specific symptom and code location — don't guess at root cause.

## Read this first — the model changed (ADR 0041 + 0044)

Polling is no longer per-authority. There are **four national lanes**, driven by a planner/executor loop in one Service-Bus-triggered handler (`api-go/internal/polling/nationallane.go` → `NationalPollHandler.Handle`, planner in `planner.go`). There is **no per-authority state, no `never_polled` cohort, no `authorities_polled`, no `GetLeastRecentlyPolled`** — anything checking those is stale.

| Lane | Sentinel `poll_state.authority_id` | Purpose | Eligible | Notifies? |
|---|---|---|---|---|
| A | −1 | new applications (masked delta, descending) | 24/7 | yes |
| B | −2 | decisions (masked delta, descending) | 24/7 | yes |
| C | −3 | inverse-mask reconciliation (ascending epoch) | **07:00–19:00 Europe/London only** | yes (hydrations) |
| D | `backfill_state` singleton | historical backward backfill | **19:00–07:00 Europe/London only** | **never** (nil fan-out, by design) |

Europe/London is UTC+1 in summer (BST) → Lane C runs ~06:00–18:00 UTC, Lane D ~18:00–06:00 UTC; it's UTC in winter. **Compute the current London time first and judge each lane against its window.** A lane that is idle *outside* its window is healthy, not broken.

### The single most important rule: a quiet lane is usually healthy

PlanIt's cost is **rows served**, and its feed is often genuinely quiet (nights, weekends, and outright upstream outages — one ran 2026-07-18→19 for ~35h). Under ADR 0044 the correct response to "nothing new upstream" is:

- **Lane A/B `high_water_mark` stops advancing** and stays put. This is **not** a failure by itself.
- The cycle ends `TerminationNatural` and reschedules **+1h**, so PlanIt request volume drops to **~2/hour**. Low request volume is **healthy**, not a stalled poller.

Do **not** flag a frozen watermark, an hourly cadence, or ~2 req/hour as problems on their own. The job of this check is to distinguish these states:

| State | Signature | Verdict |
|---|---|---|
| **Healthy-quiet** | watermark frozen AND PlanIt's masked head == our watermark (nothing newer exists) | ✅ |
| **Healthy-active** | watermark advancing and/or backlog draining, notifications firing | ✅ |
| **Upstream-frozen** | watermark frozen because PlanIt itself serves nothing new (its change axis stopped) | ⚠️ upstream — not our bug |
| **Broken (silent skip)** | PlanIt's masked head is **newer** than our watermark but we ingest nothing | ❌ point at `nationallane.go RunOnePage` |

The discriminator is the diagnostic log line **`"lane delta page fetched"`** (see Check 3).

## Fixed environment

- **Two telemetry query paths — use the right one or get false gaps:**
  - **Short windows (≤1h), recent:** `az monitor app-insights query --app appi-town-crier-shared -g rg-town-crier-shared --analytics-query "..."` (classic schema: `traces`, `customMetrics`, `dependencies`, `exceptions`).
  - **Historical (≥6h) and ALL deploy-anchored queries:** `az monitor log-analytics query --workspace 842645cf-1439-4a2b-80e8-54bd02e326f9 --analytics-query "..."` (workspace schema: `AppMetrics`, `AppTraces`, `AppDependencies`, `AppExceptions`). Column map: `timestamp`→`TimeGenerated`, `cloud_RoleName`→`AppRoleName`, `name`→`Name`, `value`→`Sum`, `customDimensions['k']`→`Properties['k']`, `message`→`Message`, `resultCode`→`ResultCode`, `success`→`Success`, `duration`→`DurationMs`.
  - ⚠️ Do NOT use `az monitor app-insights query` for windows > ~1h — it surfaces only a partial recent slice and looks like a gap that isn't there.
- **⚠️ Worker role name is `cae-town-crier-shared.town-crier-worker-go`** — the environment prefix is part of the value. Filter with `AppRoleName has 'worker-go'`, never `AppRoleName == 'town-crier-worker-go'` (that matches nothing).
- **⚠️ AppMetrics can be silently empty.** This has happened in prod (metrics pipeline gap) with the rest of telemetry flowing. **Run Check 0 first**; if AppMetrics is empty, every metric-based check below is BLIND — fall back to Postgres `poll_state` (ground truth) + `AppDependencies` + the `AppTraces` /search API, and say so in the report rather than reporting false-green.
- **AppTraces & ContainerAppConsoleLogs are Basic Logs tier** — `az monitor log-analytics query` errors on them. Query via the synchronous `/search` endpoint:
  ```bash
  az rest --method post \
    --url "https://api.loganalytics.io/v1/workspaces/842645cf-1439-4a2b-80e8-54bd02e326f9/search" \
    --resource "https://api.loganalytics.io" --headers "Content-Type=application/json" \
    --body '{"query":"AppTraces | where TimeGenerated > ago(2h) | where AppRoleName has '\''worker-go'\'' | where Message has '\''lane delta page fetched'\'' | project TimeGenerated, Message, Properties | take 20","timespan":"PT2H"}'
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
- **Service Bus:** namespace `sb-town-crier-prod` in `rg-town-crier-prod`, queue `poll`. Use `az servicebus queue show ... --query countDetails` (`az monitor metrics list` does NOT work for queues).
- **Jobs:** `job-tc-poll-prod` (orchestrator, KEDA), `job-tc-poll-bootstrap-prod` (cron `*/30`), both in `rg-town-crier-prod`.
- **PlanIt cross-checks are a LAST resort and rate-limited.** PlanIt is a free single-operator service; hammering it is a red line, and a blocked laptop IP is unrecoverable. Prefer telemetry + Postgres. If you must confirm PlanIt's head directly: **≤5 calls total, never 2 within 60s, `pg_sz` ≤ 300, always a bounded (`different_start`/`start_date`) query, `select` mandatory.** State plainly in the report that a PlanIt call was made.

## Checks

### 0. Telemetry pipeline is alive (gating — run first)
```bash
for t in AppMetrics AppDependencies AppExceptions; do
  c=$(az monitor log-analytics query --workspace 842645cf-1439-4a2b-80e8-54bd02e326f9 \
      --analytics-query "$t | where TimeGenerated > ago(2h) | where AppRoleName has 'worker-go' | summarize c=count()" \
      -o tsv --query "[0].c" 2>/dev/null); echo "$t: ${c:-0}"; done
```
`AppDependencies` should be non-zero. **If `AppMetrics` is 0**, flag it (it's a real, separate defect) and treat Checks 4/5 and any metric total as UNKNOWN, leaning on Postgres + AppDependencies + AppTraces instead.

### 1. Deployment is current
```bash
gh run list --workflow='CD Production' --limit 3
```
Latest `CD Production` run `completed / success` and newer than the most recent merged polling PR.

### 2. Worker is running and cycles succeed
```bash
az containerapp job execution list --name job-tc-poll-prod -g rg-town-crier-prod \
  --query "reverse(sort_by([].{start:properties.startTime,status:properties.status},&start))[:8]" -o table
psql "$CONN" -c "select authority_id, last_poll_time, high_water_mark from poll_state where authority_id<=0 order by authority_id desc;"
```
- Recent executions `Succeeded`, not `Failed`.
- **`last_poll_time` for −1/−2 recent for the current state**: within ~65 min is fine when caught-up (hourly Natural rhythm); minutes apart when a backlog or Lane D is draining. Hours-stale on −1/−2 → worker not running or planner stuck.

### 3. Lane A/B freshness — healthy-quiet vs broken (the core check)
Read the diagnostic log (AppTraces, Basic → /search). Its fields (`watermarkBefore`, `firstLastDifferent`, `recordsSeen`, `planitTotal`) are nanosecond-epoch strings:
```bash
az rest --method post --url "https://api.loganalytics.io/v1/workspaces/842645cf-1439-4a2b-80e8-54bd02e326f9/search" \
  --resource "https://api.loganalytics.io" --headers "Content-Type=application/json" \
  --body '{"query":"AppTraces | where TimeGenerated > ago(3h) | where AppRoleName has '\''worker-go'\'' | where Message has '\''lane delta page fetched'\'' | extend lane=tostring(Properties['\''lane'\'']), seen=toint(Properties['\''recordsSeen'\'']), wm=tostring(Properties['\''watermarkBefore'\'']), firstLD=tostring(Properties['\''firstLastDifferent'\'']), ptotal=tostring(Properties['\''planitTotal'\'']) | project TimeGenerated, lane, seen, ptotal, wm, firstLD | order by TimeGenerated desc | take 12","timespan":"PT3H"}'
```
Interpret (convert ns → time with `date -r $((ns/1000000000))` or Python):
- **`firstLastDifferent` ≤ `watermarkBefore`** → the newest masked record PlanIt has is already ingested. **Healthy-quiet ✅** — the watermark *should* be frozen.
- **`firstLastDifferent` > `watermarkBefore` and records are being ingested** (walk advancing across pages, watermark moving) → **Healthy-active ✅** (draining a backlog — normal after a quiet spell or outage).
- **`firstLastDifferent` > `watermarkBefore` but nothing ingested / watermark not moving over multiple cycles** → **Broken ❌**, silent skip. Point at `nationallane.go` `RunOnePage` boundary logic (`!app.LastDifferent.After(watermarkBefore)`).
- **No `"lane delta page fetched"` lines at all in-window while jobs are running** → worker not reaching the fetch, or a deploy without the diag line. Check the image version.

**Backlog-drain progress (post-quiet recovery):** if a backlog is draining, the watermark should climb toward PlanIt's masked head (`firstLastDifferent` of the newest page) across cycles. If it advances only by the *boundary* page's span each cycle and re-walks the same range, suspect the per-page `maxIngested` scope in `nationallane.go` under-advancing the watermark on multi-page walks — flag it (it delays catch-up and re-serves rows; it does not double-notify).

**Upstream-frozen confirmation (optional, sparing PlanIt call):** if the watermark has been frozen for hours and you need to know whether PlanIt itself is dead vs genuinely quiet, one bounded call settles it (mind the PlanIt budget above):
```bash
curl -s --compressed "https://www.planit.org.uk/api/applics/json?different_start=$(date -u +%Y-%m-%d)&sort=-last_different&pg_sz=5&select=uid,last_different&compress=on"
```
`total: null` / empty ⇒ PlanIt's change axis is frozen ⇒ our freeze is **⚠️ upstream, expected**, not our bug.

### 4. Retry-After / 429 honoured
429s are the **normal** cycle terminator (the loop hammers PlanIt until one 429, then breaks and reschedules on `Retry-After`, capped 3h in `scheduler.go`). ADR 0044 deleted the old per-lane breaker — the single loop breaks on the first 429 from any lane. Do **not** flag 429 counts, `rate_limited`, or `PlanItRateLimitException` volume.
- **The regression is a 429 as the *first* PlanIt response of a cycle** (previous `Retry-After` not honoured). Query dependencies with the correct HTTP-status field:
```bash
az monitor log-analytics query --workspace 842645cf-1439-4a2b-80e8-54bd02e326f9 \
  --analytics-query "AppDependencies | where TimeGenerated > ago(2h) | where AppRoleName has 'worker-go' | where Name == 'PlanIt search' | extend http=tostring(Properties['http.response.status_code']) | summarize arg_min(TimeGenerated, http) by AppRoleInstance | summarize cycles=count(), first_429=countif(http=='429')"
```
`first_429` **MUST be 0**. Non-zero → point at `api-go/internal/planit/retryafter.go` + `scheduler.go`.
- Retry-After distribution (AppMetrics; skip if Check 0 showed it empty): `towncrier.polling.retry_after_seconds`, tagged `lane`, `header_present`. `max_s < 10800` (under the 3h cap).

### 5. One handler at a time (lease CAS) — ADR 0024
```bash
az monitor log-analytics query --workspace 842645cf-1439-4a2b-80e8-54bd02e326f9 \
  --analytics-query "AppMetrics | where TimeGenerated > ago(6h) | where Name startswith 'towncrier.polling.lease' | summarize total=sum(Sum) by Name"
```
`towncrier.polling.lease.released_412` **MUST be 0** (non-zero = concurrent handlers stomping). `acquired` non-zero; `held_by_peer` ≥0 fine. If AppMetrics is empty, mark UNKNOWN.

### 6. Queue depth ≤ 1 (+ bootstrap doesn't double-post) — sample 5× over ~40s
```bash
for i in 1 2 3 4 5; do
  az servicebus queue show --namespace-name sb-town-crier-prod --resource-group rg-town-crier-prod --name poll \
    --query "{a:countDetails.activeMessageCount,s:countDetails.scheduledMessageCount,dlq:countDetails.deadLetterMessageCount}" -o json | tr -d '\n '; echo; sleep 8
done
```
Steady state: `active + scheduled ∈ {0,1}` (`{a:0,s:1}` healthy, next poll queued; `{a:1,s:0}` handler running). If `> 1` at any sample a duplicate publish-after-consume chain appeared — **escalate before touching anything** (per `bd memories poll-queue-max-one-message` a brute-force purge is pre-authorised, but confirm the symptom first). The `*/30` bootstrap should acquire-probe-publish-only-if-empty-release; the queue must not jump 1→2 at bootstrap ticks.

### 7. Lane C (reconciliation) — judge against its window and its design
Lane C is **daytime-only** and **forward-only**: it seeds a zero-width epoch on first run, then idles ~24h (`laneCIdleAnchorInterval`, `planner.go`) before its first real epoch, and it **only reconciles status changes PlanIt stamps after it started — it does NOT recover applications missed before it existed** (that's Lane D). **Finding 0 stragglers is normal.**
```bash
psql "$CONN" -c "select last_poll_time, high_water_mark epoch_upper, cursor_different_start epoch_lower, cursor_next_index from poll_state where authority_id=-3;"
# daytime only: Lane C spans + hydration counts
az monitor log-analytics query --workspace 842645cf-1439-4a2b-80e8-54bd02e326f9 \
  --analytics-query "AppDependencies | where TimeGenerated > ago(8h) | where AppRoleName has 'worker-go' | where Name contains 'Lane C' | extend seen=toint(Properties['poll.records_seen']), hydrated=toint(Properties['poll.records_ingested']) | project TimeGenerated, seen, hydrated | order by TimeGenerated desc | take 10"
```
- **Outside 07:00–19:00 London: idle is expected ✅.** Don't flag.
- Inside the window: expect it to run; `hydrated ≥ 0` (0 is fine). Only ❌ on errors, or a cursor that climbs for hours mid-epoch and never drains.

### 8. Lane D (historical backfill) — progress, not freshness
Lane D is **off-hours-only** and **never notifies** (nil fan-out, by design — don't expect notification metrics from it). Progress lives in `backfill_state`:
```bash
psql "$CONN" -c "select window_end, cursor_next_index, window_records_seen, consecutive_empty_windows, complete, last_run_time from backfill_state;"
az monitor log-analytics query --workspace 842645cf-1439-4a2b-80e8-54bd02e326f9 \
  --analytics-query "AppDependencies | where TimeGenerated > ago(3h) | where AppRoleName has 'worker-go' | where Name contains 'backfill' | summarize sweeps=count() by bin(TimeGenerated,15m) | order by TimeGenerated desc"
```
- **Daytime: idle is expected ✅.**
- Off-hours: `last_run_time` recent, `cursor_next_index`/`window_records_seen` climbing within a window and/or `window_end` creeping backward across nights, `complete=false`, `consecutive_empty_windows` low. It's a deliberately slow (default 2 pages/cycle), multi-week+ job — slow ≠ broken. Confirm `POLLING_BACKFILL_ENABLED=true` on the job if you expect it running.

### 9. Notifications actually delivered (the product outcome)
The point of all of the above is telling residents about applications. Confirm the pipeline reaches users:
```bash
psql "$CONN" -c "select date_trunc('day', created_at) d, count(*), count(*) filter (where push_sent) pushed, count(*) filter (where email_sent) emailed from notifications where created_at > now() - interval '7 days' group by 1 order by 1 desc;"
psql "$CONN" -c "select max(created_at) as newest_notification from notifications;"
```
- A zero-notification day lines up 1:1 with a PlanIt outage/quiet spell and is a **symptom, not a cause** — cross-reference Check 3. During a genuine quiet spell (weekend/outage) low or zero is expected.
- After recovery, `newest_notification` should track the resumed ingestion; `push_sent`/`email_sent` should be non-zero for recent rows. If apps are ingesting (Check 3) but notifications are NOT being created, that's a real fan-out break — point at the Ingester fan-out wiring and lane `WithFanOut`.

### 10. Exceptions
```bash
az monitor log-analytics query --workspace 842645cf-1439-4a2b-80e8-54bd02e326f9 \
  --analytics-query "AppExceptions | where TimeGenerated > ago(6h) | where AppRoleName has 'worker-go' | summarize c=count() by ProblemId, tostring(OuterMessage) | top 10 by c"
```
Empty or known-benign. `PlanItRateLimitException` is expected — not a concern in any volume.

## Report format

State the current **London time and which lanes are in-window** up front — every lane verdict depends on it.

1. **Lane status** — one row each for A, B, C, D: ✅ / ⚠️ / ❌ + eligibility-aware evidence. For A/B give the healthy-quiet / active / upstream-frozen / broken verdict with `watermark` vs `firstLastDifferent`. For C/D say "idle (out of window) — expected" when applicable, else progress.
2. **Cross-cutting invariants** — deploy current, worker running, lease `released_412`=0, queue ≤1, telemetry-pipeline (AppMetrics present?), notifications flowing, exceptions. One row each, ✅ / ⚠️ / ❌ + one line of evidence.
3. **Recovery / backlog** (only if a backlog is draining) — watermark at start vs now, PlanIt masked head, rough catch-up trend.

End with 1–3 watch items and one recommendation line. Keep prose under 200 words.

**Never flag as problems on their own:** a frozen A/B watermark, ~2 req/hour or hourly cadence, PlanIt 429 counts/cadence, `PlanItRateLimitException`, `rate_limited` totals, Lane C finding 0 stragglers, or Lane C/D idle outside their windows. Only escalate a 429 issue if `first_429 > 0`, and only call A/B "stalled" if PlanIt's masked head is provably newer than our watermark.
