---
description: Verify Town Crier prod polling pipeline health against the five invariants
---

Verify the Town Crier prod polling pipeline is healthy. Report concisely (green/red per check, evidence inline). Do NOT make code changes. If anything is broken, point at the specific symptom and code location — don't guess at root cause.

## Fixed environment

- **App Insights**: resource `appi-town-crier-shared` in `rg-town-crier-shared` (App ID `80cacb3f-59ff-4c3f-aa35-61d818e49dbd`). Use **classic schema** (`traces`, `exceptions`, `dependencies`, `requests`, `customMetrics`) — NOT workspace schema (`AppTraces` etc. return BadRequest). Invoke with `az monitor app-insights query --app appi-town-crier-shared -g rg-town-crier-shared --analytics-query "..."`.
- **Service Bus**: namespace `sb-town-crier-prod` in `rg-town-crier-prod`, queue `poll`. Use `az servicebus queue show ... --query countDetails` — `az monitor metrics list` does NOT work for `Microsoft.ServiceBus/namespaces/queues`.
- **Jobs**: `job-tc-poll-prod` (orchestrator, KEDA ~30s cadence), `job-tc-poll-bootstrap-prod` (cron `*/30 * * * *`). Both in `rg-town-crier-prod`.
- **Worker role name** in telemetry: `town-crier-worker`.
- **Lease counter names** (from PR #291 T17): `towncrier.polling.lease.acquired`, `towncrier.polling.lease.held_by_peer`, `towncrier.polling.lease.released_412`.

## Checks to run (in parallel where possible)

### 1. Deployment is current
```bash
gh pr list --state merged --limit 5
gh run list --workflow='CD Production' --limit 3
```
Confirm latest `CD Production` run is `completed / success` and newer than the most recent merged polling-related PR.

### 2. Poll job not aborting
```bash
az monitor app-insights query --app appi-town-crier-shared -g rg-town-crier-shared \
  --analytics-query "traces | where timestamp > ago(1h) | where cloud_RoleName == 'town-crier-worker' | where message contains 'HandlerBudget' or message contains 'Aborting' | summarize count() by bin(timestamp, 5m) | order by timestamp asc"
az containerapp job execution list --name job-tc-poll-prod -g rg-town-crier-prod \
  --query "[].{start:properties.startTime, status:properties.status, name:name}" -o table | head -20
```
Expected: 0 aborts in last 15 min. Recent executions should be `Succeeded`, not `Failed`.

### 3. Five invariants vs live telemetry

**3a. Ingesting at a good rate**
```bash
az monitor app-insights query --app appi-town-crier-shared -g rg-town-crier-shared \
  --analytics-query "customMetrics | where timestamp > ago(30m) | where name contains 'polling' or name contains 'ingest' | summarize total=sum(value), count() by name | order by name asc"
```
Expect non-zero `applications_ingested`, `authorities_polled`, `cycles_completed`.

**3b. Honouring Retry-After on 429**

**429s from PlanIt are expected and part of normal operation** — the worker deliberately hammers PlanIt serially until it gets a 429, then short-circuits the cycle and schedules the next ASB message using the `Retry-After` header. A 429 in the *middle* of a cycle is the healthy termination signal. Do NOT treat 429 counts, `rate_limited` metrics, or `PlanItRateLimitException` exceptions as watch items by themselves, and do NOT recommend "watching 429 cadence" in the report.

**The actual regression** is a 429 as the **first** PlanIt response in a cycle — that means we didn't honour the previous cycle's `Retry-After` before starting the next one. Check this per cycle:

```bash
az monitor app-insights query --app appi-town-crier-shared -g rg-town-crier-shared \
  --analytics-query "dependencies | where timestamp > ago(1h) | where target contains 'planit' and cloud_RoleName == 'town-crier-worker' | summarize arg_min(timestamp, resultCode) by cloud_RoleInstance | extend first_response_429 = (resultCode == '429') | summarize total_cycles = count(), cycles_with_first_429 = countif(first_response_429)"
```

`cycles_with_first_429` **MUST be 0**. If non-zero, that's the symptom — point at `PlanItClient.cs` / the retry-after handling in the orchestrator's next-message scheduler.

For context only (not a pass/fail signal), you can also report the 429 distribution, but state it as expected cadence not a concern:

```bash
az monitor app-insights query --app appi-town-crier-shared -g rg-town-crier-shared \
  --analytics-query "dependencies | where timestamp > ago(30m) | where target contains 'planit' | summarize total=count(), rate429=countif(resultCode == '429'), failures=countif(success == false and resultCode != '429'), avgDurMs=avg(duration) by bin(timestamp, 5m) | order by timestamp asc"
```

`failures` (excluding 429) must be near 0. `rate429` being non-zero is fine — that's the cycle termination marker.

**Retry-After value distribution** — what is PlanIt actually asking us to wait? The worker emits `towncrier.polling.retry_after_seconds` (histogram) on every 429 with the parsed `Retry-After` header value, tagged `header_present=true|false`. Use this to confirm whether the configured `RetryAfterCap` (currently 3h, [PollNextRunSchedulerOptions.cs:14](api/src/town-crier.application/Polling/PollNextRunSchedulerOptions.cs)) is large enough for the values PlanIt is sending:

```bash
az monitor app-insights query --app appi-town-crier-shared -g rg-town-crier-shared \
  --analytics-query "customMetrics | where timestamp > ago(6h) | where name == 'towncrier.polling.retry_after_seconds' | extend header_present = tostring(customDimensions['header_present']) | where header_present == 'true' | summarize samples=count(), p50_s=percentile(value, 50), p90_s=percentile(value, 90), p99_s=percentile(value, 99), max_s=max(value)"
```

Pass condition: `max_s < 10800` (3h cap, in seconds). If `max_s ≥ 10800`, PlanIt is asking for backoffs longer than the cap and we are clipping — bump `RetryAfterCap` higher or escalate to a human. If `samples == 0` and `cycles_with_first_429 > 0`, then PlanIt is sending 429s without `Retry-After` headers; check the `header_present='false'` row separately:

```bash
az monitor app-insights query --app appi-town-crier-shared -g rg-town-crier-shared \
  --analytics-query "customMetrics | where timestamp > ago(6h) | where name == 'towncrier.polling.retry_after_seconds' | extend header_present = tostring(customDimensions['header_present']), authority = tostring(customDimensions['polling.authority_code']) | summarize samples=count() by header_present, authority | order by samples desc | take 10"
```

A high `header_present='false'` count means PlanIt isn't sending `Retry-After` at all, and we're falling back to the configured default — point at `PollNextRunScheduler.cs` retry-after parsing if `cycles_with_first_429` is also non-zero.

**3c. Only 1 thread at a time (lease CAS)**
```bash
az monitor app-insights query --app appi-town-crier-shared -g rg-town-crier-shared \
  --analytics-query "customMetrics | where timestamp > ago(6h) | where name startswith 'towncrier.polling.lease' | summarize total=sum(value), count() by name | order by name asc"
```
`released_412` **MUST be 0** (non-zero = concurrent handlers stomping each other). `held_by_peer` ≥0 is fine (bootstrap/orchestrator race is expected; CAS is working). `acquired` should be non-zero.

**3d. ASB queue depth ≤ 1** — sample 5× over ~40s to catch transitions:
```bash
for i in 1 2 3 4 5; do
  echo -n "t=${i}: "
  az servicebus queue show --namespace-name sb-town-crier-prod --resource-group rg-town-crier-prod --name poll \
    --query "{a:countDetails.activeMessageCount, s:countDetails.scheduledMessageCount, dlq:countDetails.deadLetterMessageCount}" -o json | tr -d '\n '
  echo
  sleep 8
done
```
Steady state: `active + scheduled ∈ {0, 1}`. `{a:0, s:1}` is healthy (next poll queued). `{a:1, s:0}` is the transient hand-off (handler running). If `active + scheduled > 1` at any sample, a duplicate publish-after-consume chain has appeared — **escalate before touching anything**. Per `bd memories poll-queue-max-one-message` the user has pre-authorised brute-force purge, but confirm the symptom first.

**3e. Keeping up with the authority backlog (oldest LastPollTime age)**

> **Naming note:** the metric is `towncrier.polling.oldest_hwm_age_seconds` for legacy reasons (PR #298 / tc-m6fx split LastPollTime from HighWaterMark but kept the metric name). The value is the **LastPollTime age**, not the HighWaterMark age — see the description in [PollingMetrics.cs](api/src/town-crier.application/Observability/PollingMetrics.cs).

This check focuses on **polled** authorities (`never_polled='false'`). The never-polled cohort is covered by 3g — they always report `(now − UnixEpoch)` which is huge by design and would otherwise dominate this signal.

```bash
az monitor app-insights query --app appi-town-crier-shared -g rg-town-crier-shared \
  --analytics-query "customMetrics | where timestamp > ago(6h) | where name == 'towncrier.polling.oldest_hwm_age_seconds' | extend authority = tostring(customDimensions['polling.authority_code']), never_polled = tostring(customDimensions['never_polled']) | where never_polled == 'false' | summarize latest_age_s = arg_max(timestamp, value, authority) by bin(timestamp, 30m) | order by timestamp desc | take 10"
```

Read the most recent row: `value` is how far behind the stalest **polled** authority is, in seconds; `authority` identifies it. Healthy means the latest `value` is within the expected cycle cadence — roughly `authorities_polled_per_cycle × cycle_interval`. A monotonically climbing value over the 6h window means the pipeline is falling behind and needs more frequency or more throughput per cycle. Report the current lag in human units (e.g. "oldest LastPollTime is 47 min behind, authority 123").

**3f. Bootstrap doesn't double-post during active cycle**
```bash
az containerapp job execution list --name job-tc-poll-bootstrap-prod -g rg-town-crier-prod \
  --query "[].{start:properties.startTime, status:properties.status}" -o table | head -10
az monitor app-insights query --app appi-town-crier-shared -g rg-town-crier-shared \
  --analytics-query "traces | where timestamp > ago(1h) | where message contains 'Safety-net' or message contains 'queue empty' or message contains 'bootstrap' | project timestamp, cloud_RoleName, message | order by timestamp asc | take 30"
```
Bootstrap (`*/30`) should acquire lease, probe, publish only if empty, release. During an orchestrator cycle (lease TTL 4.5 min), a concurrent bootstrap tick should see Held and no-op. Queue must not jump from 1 → 2 at bootstrap tick times.

**3g. Backlog drain rate (never-polled cohort)**

Counts authorities with no PollState document at cycle start. Should monotonically drain to 0 within 24–48h after any deploy that adds new authorities or changes selection logic. **A flat-line non-zero count over 24h is the canonical tc-ews7 starvation regression** — surface it loudly.

```bash
az monitor app-insights query --app appi-town-crier-shared -g rg-town-crier-shared \
  --analytics-query "customMetrics | where timestamp > ago(24h) | where name == 'towncrier.polling.never_polled_count' | extend cycle = tostring(customDimensions['cycle.type']) | where cycle == 'seed' | summarize count_=avg(value) by bin(timestamp, 1h) | order by timestamp asc"
```

Pass conditions:
- Latest `count_` is 0, **or**
- Latest `count_` is strictly less than the value 6h earlier (drain in progress), **or**
- Latest `count_` equals the count of authorities added in the most recent CD deploy and the deploy was within the last 6h (fresh-deploy transient).

Fail conditions:
- Latest `count_` is non-zero AND has been flat or rising for ≥6h after a deploy that's been live ≥6h. Point at `CycleAlternatingAuthorityProvider`, `CosmosPollStateStore.GetLeastRecentlyPolledAsync`, and `PollPlanItCommandHandler`'s natural-end branch (lines 296-308) — that's the drain path.
- Metric is missing from telemetry. Confirm the worker image is post-tc-ifdl deploy.

The Watched cycle reports its own `never_polled_count` (count of watch-zone authorities with no state — usually 0). Filter on `cycle.type == 'seed'` for the canonical drain signal; report Watched separately only if non-zero.

### 4. Exceptions sanity check
```bash
az monitor app-insights query --app appi-town-crier-shared -g rg-town-crier-shared \
  --analytics-query "exceptions | where timestamp > ago(1h) | where cloud_RoleName == 'town-crier-worker' | summarize count() by type, outerMessage | top 10 by count_"
```
Should be empty or known-benign. `PlanItRateLimitException` is expected (see 3b) and is not a concern in any volume — do not flag it.

### 5. Runs since last deploy (for the per-run table)

Anchor the window to the most recent successful `CD Production` run, then enumerate orchestrator executions and join to per-run ingestion counts.

```bash
# Deploy anchor (ISO-8601 UTC). Use updatedAt of the latest successful CD Production run.
DEPLOY_TS=$(gh run list --workflow='CD Production' --status=success --limit 1 --json updatedAt --jq '.[0].updatedAt')
echo "Deploy anchor: $DEPLOY_TS"

# Executions since deploy (orchestrator + bootstrap)
az containerapp job execution list --name job-tc-poll-prod -g rg-town-crier-prod \
  --query "[?properties.startTime >= '$DEPLOY_TS'].{name:name, start:properties.startTime, end:properties.endTime, status:properties.status}" -o json
az containerapp job execution list --name job-tc-poll-bootstrap-prod -g rg-town-crier-prod \
  --query "[?properties.startTime >= '$DEPLOY_TS'].{name:name, start:properties.startTime, end:properties.endTime, status:properties.status}" -o json

# Per-instance counters since deploy — each cloud_RoleInstance is one job execution
az monitor app-insights query --app appi-town-crier-shared -g rg-town-crier-shared \
  --analytics-query "customMetrics | where timestamp > datetime($DEPLOY_TS) | where cloud_RoleName == 'town-crier-worker' | where name in ('towncrier.polling.applications_ingested','towncrier.polling.authorities_polled','towncrier.polling.authorities_skipped','towncrier.polling.cycles_completed','towncrier.polling.cursor_advanced','towncrier.polling.rate_limited','towncrier.polling.lease.acquired','towncrier.polling.lease.held_by_peer','towncrier.polling.lease.released_412') | summarize total=sum(value) by cloud_RoleInstance, name | order by cloud_RoleInstance asc, name asc"

# Oldest-LastPollTime age per cycle since deploy — one emission per cycle at cycle start
az monitor app-insights query --app appi-town-crier-shared -g rg-town-crier-shared \
  --analytics-query "customMetrics | where timestamp > datetime($DEPLOY_TS) | where name == 'towncrier.polling.oldest_hwm_age_seconds' | extend authority = tostring(customDimensions['polling.authority_code']), never_polled = tostring(customDimensions['never_polled']) | project timestamp, age_s = value, authority, never_polled, cloud_RoleInstance | order by timestamp asc"

# Never-polled count per cycle since deploy — drains monotonically toward 0 (see 3g)
az monitor app-insights query --app appi-town-crier-shared -g rg-town-crier-shared \
  --analytics-query "customMetrics | where timestamp > datetime($DEPLOY_TS) | where name == 'towncrier.polling.never_polled_count' | extend cycle = tostring(customDimensions['cycle.type']) | project timestamp, count_=value, cycle, cloud_RoleInstance | order by timestamp asc"

# Map cloud_RoleInstance -> execution by first-seen timestamp (matches execution startTime within a few seconds)
az monitor app-insights query --app appi-town-crier-shared -g rg-town-crier-shared \
  --analytics-query "union customMetrics, traces | where timestamp > datetime($DEPLOY_TS) | where cloud_RoleName == 'town-crier-worker' | summarize firstSeen=min(timestamp), lastSeen=max(timestamp) by cloud_RoleInstance | order by firstSeen asc"
```

Match each `cloud_RoleInstance` to an execution by taking the execution whose `startTime` is within ~30s before `firstSeen`. Short-lived instances (~6s lifetime, `lease.acquired=1` only) are bootstrap ticks — label them as such.

## Report format

Output three tables, in this order:

1. **Invariants** — seven invariants (3a–3g) + deployment status + abort check. One row each, ✅ / ⚠️ / ❌ + one line of evidence (metric name and value, or trace count, or queue sample). For 3e (oldest LastPollTime), include the authority id and the lag in a human unit (e.g. "47m behind, authority 123"). For 3g (never-polled drain), include the latest count and the trend versus 6h earlier (e.g. "0 (was 12, draining)" or "272 (flat for 18h — STARVATION)").
2. **Runs since last deploy** — columns: `#`, execution short-name (last segment after `job-tc-poll-prod-`), start UTC, duration, status, apps, auths, cycles, notes. Include bootstrap ticks as separate rows marked `— bootstrap HH:MM —` with dashes for apps/auths/cycles and a note like "lease acquired, no-op reseed" or "reseeded (queue empty)".
3. **Totals since deploy** — window length, successful cycles, failed cycles, applications ingested, authorities polled, orchestrator lease acquisitions, bootstrap lease acquisitions, `lease.released_412` (must be 0), `cycles_with_first_429` (must be 0 — see 3b), retry-after `p50/p90/max` in seconds with sample count (must have `max_s < 10800`; if no samples but 429s present, surface the `header_present='false'` count instead), oldest-LastPollTime age at start vs. end of window (and whether it grew or shrank — the single most important "are we keeping up?" signal), never-polled count at start vs. end of window (must monotonically drain or stay 0 — the canonical tc-ews7 starvation signal).

End with 1–3 watch items and one recommendation line. Keep the prose under 200 words — tables can be as long as the data demands.

**Do not include as watch items**: PlanIt 429 counts or cadence, `PlanItRateLimitException`, `rate_limited` metric totals, or cycles that polled 0 authorities because they hit a 429 on first draw. Those are expected behaviour, not concerns. Only flag a 429-related issue if `cycles_with_first_429 > 0`.
