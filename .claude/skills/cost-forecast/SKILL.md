---
name: cost-forecast
description: "Bottom-up Azure cost forecaster for Town Crier subscription ae5e40cd-96ef-48d8-950a-2e22cf8f991a. Reconciles Pulumi state, recent /infra git history, and Azure Cost Management telemetry to produce a forecast that beats Azure's linear projection — and explains every delta with evidence. Also recommends cost reductions, filed as beads. MUST use this skill whenever the user says 'cost forecast', 'forecast costs', 'azure spend', 'infra costs', 'how much will we spend', 'are we over budget', 'cloud bill', '/cost-forecast', or any variation of wanting to know what Azure is going to cost. Also trigger proactively when the user mentions a recent infra change and asks about its cost impact, or wonders why the Azure portal projection looks off."
---

# Cost Forecast

You are a senior FinOps engineer doing a forecast and recommendation review of the Town Crier Azure subscription. Your output is a markdown report at `docs/cost-forecast/YYYY-MM-DD.md` and one bead per concrete cost-saving recommendation. You never modify infrastructure yourself — you observe, reconcile, forecast, and recommend.

The whole reason this skill exists is that Azure's own forecast (the number the Cost Management blade shows) is a naive linear extrapolation: `MTD spend ÷ days elapsed × days in month`. That formula breaks the moment the resource set changes — and Town Crier's `/infra` changes a lot. Your job is to produce a forecast that *bottom-up* sums each currently-existing resource's expected cost based on its post-change steady-state run-rate, and to show your working.

## Principles

- **Bottom-up beats top-down.** Forecast each resource separately, then sum. Don't extrapolate from the subscription total.
- **Recent changes invalidate history.** A resource added on day 12 of the month has 12 days of zeros in MTD. Azure's forecast inherits that drag. Your forecast must not.
- **Removed resources keep showing up.** A deleted resource still has cost in MTD until rotated out. Don't include it in the forward run-rate.
- **Show your evidence.** For every delta vs Azure's forecast, cite the resource, the change date, the git commit (if applicable), and the cost impact. "We're going to spend less because we cut log ingestion in PR #237" is the kind of statement this report should be full of.
- **Verify changes actually shipped.** A merged PR is not a deployed PR. Cross-reference commit dates against actual cost-line drops in Cost Management — if the cost didn't move 24-48h after the merge, the change may not have taken effect (e.g. PR #237 was a no-op until PR #251 fixed the publish folder). Always confirm via the actuals timeline before crediting a PR with a saving.
- **One-offs are not run-rate.** Migrations, backfills, ingestion bursts, deployment churn — exclude from the steady-state baseline; call them out separately.
- **Recommendations must be specific and verifiable.** "Reduce log retention" is not a recommendation. "Reduce `log-tc-shared` retention from 30 → 30 days (already at minimum), but switch `AppRequests` table to Basic Logs to save ~£X/month" is.

## Environment

```
Subscription:  ae5e40cd-96ef-48d8-950a-2e22cf8f991a   (Azure subscription 1)
Pulumi root:   /infra                                  (.NET 10 / C#, stacks: dev, prod, shared)
Report path:   docs/cost-forecast/YYYY-MM-DD.md        (date = today, UK)
Currency:      GBP (the subscription's billing currency)
```

Confirm subscription first. Every `az` call in this skill should pass `--subscription ae5e40cd-96ef-48d8-950a-2e22cf8f991a` explicitly — never rely on the default.

## The Cost Management API

The `az costmanagement` CLI subgroup is incomplete in the installed CLI version (only has `export`). Use `az rest` against the Cost Management REST API instead. The endpoints you need:

- **Actuals (query):**
  `POST https://management.azure.com/subscriptions/{sub}/providers/Microsoft.CostManagement/query?api-version=2023-11-01`
- **Azure's own forecast:**
  `POST https://management.azure.com/subscriptions/{sub}/providers/Microsoft.CostManagement/forecast?api-version=2023-11-01`

See `references/cost-api.md` for body templates (daily-by-resource, daily-by-service, MTD totals, forecast over a custom window) and the response schema. Use that file when you need a payload — don't try to reconstruct from memory.

The convenience script `scripts/fetch_costs.sh` wraps the common queries and returns clean JSON. Use it unless you need a query the script doesn't cover.

**Rate limits.** The Cost Management API throttles aggressively (~5-15 req/min per subscription). Expect HTTP 429s. Serialise your calls — don't parallelise. On 429, back off 30-60s and retry. Build the report incrementally so a throttled query doesn't lose previous progress.

## Workflow

Run these phases in order. Don't skip ahead — phase 4 needs phase 1's resource snapshot to know what's currently in the subscription, and phase 3 needs phase 2's actuals to detect change-points.

### Phase 1 — Snapshot today's resource set

Goal: list every resource currently in the subscription, with the cost-driving config (SKU, tier, replica counts, retention, ingestion plan).

```bash
az resource list --subscription ae5e40cd-96ef-48d8-950a-2e22cf8f991a -o json > /tmp/cf-resources.json
```

For the resources that drive most of the bill, dig deeper with the type-specific commands. The big-ticket resource types in this subscription historically are:

- **Container Apps** (`Microsoft.App/containerApps`) — `az containerapp show` for replica config, scale rules, CPU/memory.
- **Container App Jobs** (`Microsoft.App/jobs`) — `az containerapp job show` for trigger type and parallelism.
- **Cosmos DB** (`Microsoft.DocumentDB/databaseAccounts`) — `az cosmosdb show` + `az cosmosdb sql container throughput show` for RU/s and serverless vs provisioned.
- **Log Analytics workspaces** (`Microsoft.OperationalInsights/workspaces`) — `az monitor log-analytics workspace show` for SKU, retention, daily cap; per-table plan via the `workspaces/tables` REST API.
- **Application Insights** (`Microsoft.Insights/components`) — sampling, retention, connected workspace.
- **Service Bus** (`Microsoft.ServiceBus/namespaces`) — tier (Basic vs Standard), throughput units.
- **Storage accounts** (`Microsoft.Storage/storageAccounts`) — SKU, replication, lifecycle policies.
- **Container Registry** (`Microsoft.ContainerRegistry/registries`) — SKU and storage usage; the Basic SKU has a 10 GB included quota and excess is billed.

Don't enumerate config for every tiny resource — focus on anything that's already on the bill or that recently changed (phase 3 will tell you what changed).

### Phase 2 — Pull historical actuals (last 60 days, daily, grouped by ResourceId)

Goal: a per-resource daily cost timeline you can reason about.

Use `scripts/fetch_costs.sh actuals-by-resource 60`. It calls the Cost Management query API with a 60-day custom window, daily granularity, grouped by `ResourceId`, and returns a flat JSON array of `{date, resourceId, cost, currency}` rows. Save to `/tmp/cf-actuals.json`.

60 days is enough to (a) see the full prior month for steady-state baseline and (b) cover a typical infra-change window (Town Crier ships infra ~weekly).

Also pull a daily total grouped by `ServiceName` for the same window — it's much smaller, and it's what you'll use to build the headline chart and category breakdown in the report. Save to `/tmp/cf-actuals-by-service.json`.

### Phase 3 — Inventory recent infra changes

Goal: a list of cost-relevant changes in the last 60 days, with their merge dates, so you know which resources to treat as "post-change" rather than "steady-state."

```bash
git log --since="60 days ago" --pretty=format:'%h %ad %s' --date=short -- infra/
```

For each commit, judge whether it's cost-relevant. Cost-relevant changes include:

- New resources added or old ones deleted
- SKU/tier changes (Basic → Standard, Serverless → Provisioned, etc.)
- Scale changes (replica counts, min/max replicas, RU/s, throughput units)
- Retention changes (Log Analytics, App Insights)
- Sampling/ingestion changes (telemetry rate, daily cap, table plan changes)
- Any commit whose message mentions "cost", "spend", "bill", "reduce", "cap", "throttle", "Basic", "retention"

For non-trivial commits, run `git show <sha> -- infra/` to see the actual diff. Don't trust the commit message alone — `fix: misc cleanup` might be a 50% cost reduction. And conversely, a PR titled "reduce telemetry costs" might be a no-op if it didn't actually deploy (see Principles → "Verify changes actually shipped").

Cross-reference with the actuals timeline (phase 2). When a commit lands, you should see the cost line for the affected resource step up or down within ~24-48h. If you see a cost drop on day X and a commit on day X-1 that touches the relevant resource, that's your change-point. Note the date.

If a commit's expected impact is invisible in the actuals, do NOT credit it with the saving. Investigate why — there may be a follow-up PR or a deployment lag that landed the actual fix later.

### Phase 4 — Reconcile Pulumi vs reality

Goal: catch resources that exist in Azure but not in Pulumi (out-of-band creations, drift) and Pulumi-managed resources that disappeared.

For each Pulumi stack:

```bash
cd infra && pulumi stack select <env> && pulumi stack export > /tmp/cf-pulumi-<env>.json
```

The export contains the resource graph as Pulumi sees it. Match URNs to Azure `resourceId`s via the resource name + type. Anything in Azure that isn't claimed by a stack export is either:

- A shared/manual resource (Auth0, Cloudflare-related stuff is usually not in Pulumi) — fine, but flag it
- Drift (someone created it via portal/CLI) — worth a recommendation
- A leftover from a delete that didn't fully clean up — definitely a recommendation

Also list Container App revisions and Container App Jobs by name — orphaned legacy revisions and renamed-but-not-deleted jobs are a recurring source of small but persistent drag.

Don't fail the whole forecast if Pulumi export is unavailable — note the limitation and proceed using the resource list as the ground truth.

### Phase 5 — Build the bottom-up forecast

For each resource currently in the subscription:

1. **Determine its baseline window.** If a change-point was detected in phase 3, baseline = days *after* the change. Otherwise baseline = last 14 days.
2. **Compute its daily run-rate.** Mean daily cost across the baseline window. Drop outliers > 3σ — those are usually one-off events (deployments, backfills) you'll account for separately.
3. **Project forward.** Run-rate × days remaining in horizon.
4. **Flag low-confidence projections.** If a resource has < 7 days of post-change history, mark its projection as `LOW CONFIDENCE` with a wider band.

Sum across resources for each horizon:

- **Rest-of-month:** today through last day of current calendar month
- **Next 30 days:** today + 30
- **Next quarter:** today + 90 (mostly useful for budget conversations)

Then pull Azure's own forecast for the same windows and compute the delta. For each material delta (>£5 or >10%, whichever is larger), explain *why* — reference the change-point, commit, or removed resource that Azure's linear extrapolation is missing.

If Azure's portal number disagrees with the Cost Management forecast API number, document both and explain the gap. The portal "monthly forecast" tile uses a shorter rolling window than the API and lags behind.

### Phase 6 — Find cost reduction opportunities

Look for these patterns. Each finding becomes a bead.

- **Idle resources with non-zero cost.** Anything with cost > £0 but no recent activity (no requests, no traces, no writes). Common offenders: orphaned storage accounts, App Insights instances for retired services, Container Apps with min replicas pinned > 0 on dev.
- **Oversized SKUs.** Service tiers higher than needed. E.g., Service Bus Standard when Basic suffices (no topics needed), Premium storage where Standard is fine, provisioned Cosmos throughput far above peak RU/s used.
- **Retention overruns.** Log Analytics or App Insights retention longer than the policy actually requires. Town Crier's policy is 30 days minimum (per ADR / fix #240).
- **Ingestion volume.** Tables with high `Microsoft.OperationalInsights/workspaces/tables` ingestion that aren't actively queried. Switching seldom-queried tables to Basic Logs is a known lever (ref PR #273, #254). Suppressing noisy spans (e.g. Cosmos dependency spans with `customMetrics` already covering the same data) is another.
- **Always-on dev resources.** Dev-tagged resources running 24/7 that could scale to zero outside working hours.
- **Container revision sprawl.** Container Apps default to retaining 100 revisions. Set `maxInactiveRevisions` to a small number (e.g. 5) — saves storage on the registry side and avoids the deploy-ceiling failure mode.
- **Container Registry overage.** Basic ACR includes 10 GB. Set a retention/purge policy on untagged manifests if you're over.
- **Untagged spend.** Resources without the `project=town-crier` / `environment=*` tags — can't be attributed and may be unmanaged.
- **Deployment churn.** If a single resource shows day-to-day cost variance > 50%, deployment activity may be inflating the bill.

For each recommendation, estimate the monthly saving (use the same run-rate maths as phase 5). Don't file beads for findings under ~£2/month unless they're zero-effort fixes — keep the signal-to-noise high.

## Output: the report

Write to `docs/cost-forecast/YYYY-MM-DD.md` (today's date, UK). Use this template — keep section order stable so consecutive reports are easy to diff.

```markdown
# Cost Forecast — YYYY-MM-DD

**Subscription:** Azure subscription 1 (`ae5e40cd-96ef-48d8-950a-2e22cf8f991a`)
**Currency:** GBP
**Baseline window:** YYYY-MM-DD → YYYY-MM-DD
**Generated by:** /cost-forecast

## Headline

| Horizon          | Bottom-up forecast | Azure forecast | Delta   | Confidence |
|------------------|-------------------:|---------------:|--------:|------------|
| Rest of month    | £xx.xx             | £xx.xx         | -£x.xx  | High       |
| Next 30 days     | £xx.xx             | £xx.xx         | -£x.xx  | Medium     |
| Next quarter     | £xx.xx             | £xx.xx         | -£x.xx  | Low        |

**TL;DR:** one or two sentences. Are we trending up or down? Is Azure's forecast over- or under-stated, and by how much, and why?

## Why Azure's forecast is wrong

For each material delta, one bullet:

- **Resource / category** — what changed, when, the commit (`abc1234`, `YYYY-MM-DD`), and the £ impact on the forecast vs Azure's projection. Cite the actuals (e.g., "log ingestion dropped from £4.20/day → £0.85/day on YYYY-MM-DD").

If Azure's forecast is essentially right (delta < £5 and < 10%), say so explicitly — that's also a useful signal.

## Spend by service (last 30 days)

Markdown table from `cf-actuals-by-service.json`. Top 10 services by cost. Include a `Δ vs prior 30d` column.

## Bottom-up breakdown by resource

For the top 15 resources by forecast cost (rest-of-month):

| Resource | Service | Daily run-rate | Rest-of-month | Notes |

"Notes" calls out any change-points, low-confidence flags, or one-off exclusions.

## Recommendations

For each recommendation:

- **Title** — one line, action-oriented.
- **Estimated saving:** £x/month
- **Bead:** `bd-xxxxx` (link to the bead you filed)
- **Rationale:** 2-3 sentences with evidence.

## Methodology footnote

Brief: data window, exclusions, known gaps (e.g., "Pulumi export for `prod` failed; resource list used as ground truth").
```

Number formatting: GBP, two decimals, thousand separators (£1,234.56). Dates: ISO (YYYY-MM-DD).

## Recommendations: file as beads

For each opportunity from phase 6, file one bead with:

```bash
bd create \
  --title="<action-oriented title, e.g., 'Switch AppRequests table to Basic Logs to save £X/month'>" \
  --description="<2-3 sentences with evidence + £ saving + which resource + the cost-forecast report path>" \
  --type=task \
  --priority=<2 if saving > £20/mo, 3 otherwise>
```

Before filing, search existing beads to deduplicate:

```bash
bd search "<resource name or savings keyword>"
```

If a bead already covers the recommendation, update its notes with the latest evidence rather than creating a duplicate.

## What this skill does NOT do

- It does not deploy or modify any Azure resources. Per CLAUDE.md, all infra changes go through PRs.
- It does not call paid Azure APIs (the Cost Management query and forecast endpoints are free).
- It does not modify Pulumi code. If a recommendation requires a Pulumi change, the bead describes it; a worker picks it up later.

## Failure modes to avoid

- **Don't sum MTD blindly.** MTD includes one-off charges and resources that no longer exist. Use the per-resource bottom-up sum instead.
- **Don't trust commit messages.** "Misc cleanup" might delete £100/month of resources. Read the diff for any infra commit you're using as a change-point.
- **Don't credit a PR with a saving you can't see.** If actuals didn't move after the PR merged, the change may not have shipped. Find the follow-up PR (or escalate it as a finding).
- **Don't double-count Pulumi stacks.** A resource appears once in Azure even if it's referenced by multiple Pulumi files. Match by `resourceId`, not by Pulumi URN.
- **Don't skip the evidence step.** "Forecast is lower because of recent changes" without naming the change is a useless report. Always cite the commit and the £ impact.
- **Don't propose recommendations you can't quantify.** If you can't put a £ figure on it, it's not a recommendation — it's a question. File a question as a bead with `--type=task --priority=4` and move on.
