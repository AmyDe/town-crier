# 0041. Poll PlanIt nationally on a churn-masked delta axis

Date: 2026-07-14

## Status

Accepted — supersedes the polling design in [0006](0006-planit-primary-data-provider.md) and [0021](0021-resumable-pagination-cursor-for-planit-polling.md); retires the watch-zone-density polling tiers introduced in [0012](0012-dynamic-polling-prioritisation.md)

> **Revision note.** The first version of this ADR, accepted and merged earlier on 2026-07-14 (PR #960), proposed polling on `start_date` and `decided_start` as the *primary* axes and retiring `last_different` entirely. A falsification pass against live PlanIt and prod Postgres later the same day showed that design re-fetches ~224,000 records/day (more rows than today) and still cannot deliver new applications within the hour, because `start_date` cannot express a delta. This version keeps that ADR's central diagnosis — which held up to the digit — and changes the remedy. The original numbers and the errors found in them are recorded under "What the first version got wrong".

## Context

Town Crier exists to tell residents that a planning application near them has opened for comment, while the consultation window is still open. That window is typically 21 days from validation. Everything else the product does is secondary to hitting it.

We do not hit it. Measured against prod on 2026-07-14:

- Camden holds **66** applications with a `start_date` in the last 30 days. PlanIt has **300**. **We hold roughly one in five.**
- 131 of 485 authorities carry an unfinished pagination cursor; 193 have a high-water mark more than 7 days stale, 73 more than 30 days.
- Only 316 of 485 authorities were polled at all in a 24-hour window. The median authority gets **one visit per day**.
- Camden's poll window held **18,150** records — **2.4% of our entire 749,266-row national corpus** — and the drain, at 100 records per request, had to walk all of it before the high-water mark could advance.

### The root cause is the axis, not the throughput

`different_start` / `last_different` is **scrape bookkeeping**: it moves whenever PlanIt's scraper decides a record differs. A PlanIt re-index rewrites it across an authority's entire back-catalogue.

The churn is not a minor tax. It is essentially the whole signal. Measured on Camden (`applications`, prod):

| `last_different` day | records touched | of which plausibly new |
|---|---|---|
| 2026-05-24 | 2,981 | **1** |
| 2026-05-29 | 1,785 | 3 |
| 2026-05-17 | 1,850 | 0 |
| 2026-05-15 | 1,790 | 20 |

So we notify people about planning applications by draining an axis that is **~99.9% re-scrapes of old records**. New applications sit *behind* that churn in ascending `last_different` order. PlanIt has re-indexed twice in four months, and each time it buries the product signal under tens of thousands of rows of bookkeeping.

Throughput work does not fix this. Doubling the drain rate halves the time to walk 18,150 records of junk; it does not put a single application in front of a user any sooner.

### The decisive observation

**A re-index only ever touches records that already exist — which means it only touches records with an old `start_date`.** Camden's 05-24 wave: 2,981 records touched, exactly one of them recent.

The churn and the signal are therefore *separable by a filter we are not using*. `start_date` is the council's own date. A re-index cannot move it. Applying it as a **mask** on the existing delta query removes the churn and leaves the new applications.

Measured against the live API (2026-07-14):

```
GET /api/applics/json?auth=300&different_start=2026-07-14&start_date=2026-05-30
  → total: 10 records, 1,236 bytes, secs_taken 0.084
```

Ten records. The same authority, the same axis, the same cursor — with one extra query parameter — instead of 18,150.

And it composes nationally, with no `auth` filter at all:

```
GET /api/applics/json?different_start=2026-07-14&start_date=2026-05-30
    &sort=-last_different&pg_sz=300&select=<ingest fields>&compress=on
  → total: 1,717 records   (the whole country's change set for the day)
    777.6 bytes/record, 233 KB/page, secs_taken 0.203
```

**1,717 records is the entire national daily delta.** Six pages at PlanIt's own default page size.

### Two facts that constrain the design

1. **Scraper discovery lag is large, and it rules out `start_date` as a cursor.** An application reaches PlanIt days or weeks after its `start_date`. Measured on Camden's last 30 days: **p50 lag 7 days, p90 20 days**. PlanIt holds only 15 Camden applications with a `start_date` inside 7 days, but 145 in the 7–20 day band. A watermark on `start_date` would therefore step straight over half of everything PlanIt discovers — you cannot ask "what is new since I last looked" on an axis whose values are back-dated. This is why the delta must stay on `last_different`, and `start_date` must be a mask rather than a cursor.
2. **`decided_date` is not universal.** Of decided-state rows, between 1.8% (Permitted) and 16.1% (Referred) carry no `decided_date`, and 2.0% of all rows carry no `app_state` at all. About one decision in forty is invisible to a decision poll. This is handled by a reconciliation lane, not by reverting to an unmasked churn axis.

## Decision

Poll PlanIt **nationally**, on the **`last_different` delta axis, masked by `start_date` (or `decided_date`) so that re-index churn is filtered out upstream**, with a `select` projection.

Retire per-authority high-water marks, pagination cursors, and watch-zone-density tiering. Lane state is **one global watermark per lane** — a single timestamp, not 485 cursors.

Three lanes.

### Lane A — new applications (the product)

The critical path. One national query, hourly:

```
?different_start=<watermark_date>   # coarse date-granular prefilter
&start_date=<today-90>              # churn mask: re-index of old records is excluded
&sort=-last_different               # newest-discovered first
&pg_sz=300                          # PlanIt's documented default
&select=<every field we ingest, minus other_fields>
&compress=on
```

Page **descending** until `last_different <= watermark`, then stop. `different_start` is date-granular, so it is only a coarse prefilter; the descending sort plus an in-memory timestamp watermark gives exact delta semantics with no cursor to persist and no state to corrupt. A crash mid-page is harmless — the next run re-reads from the same watermark and ingest is idempotent.

Steady state is **~72 records/hour nationally → one request per hour**.

Because the delta is on **discovery**, not on `start_date`, an application reaches a user within the hour of PlanIt finding it *whatever its `start_date`* — which is the only way to beat a p50 discovery lag of 7 days.

The mask width (90 days) is a config dial, not a correctness boundary: anything the mask misses is caught by Lane C. The 45-day mask was measured at 1,717 records/day nationally; 90 days is modestly more and still a handful of pages. Widen it freely — it is cheap.

### Lane B — decisions

Same shape, masked on the decision date instead:

```
?different_start=<watermark_date>&decided_start=<today-90>&sort=-last_different&pg_sz=300
```

This catches decisions on applications whose `start_date` is older than Lane A's mask — the case Lane A structurally cannot see. Hourly, ~1 request.

### Lane C — reconciliation (completeness)

Catches what the delta axis structurally cannot: decisions with no `decided_date`, rows with no `app_state`, applications discovered so late their `start_date` falls outside both masks, upstream corrections, and deletions.

Per authority, a light projection sweep (`select=uid,app_state,decided_date,last_different`, ~100 bytes/record) covering that authority's live set — one request each at `pg_sz=300`. **485 requests, run weekly (~70/day amortised).** Diff against Postgres in the worker; hydrate only rows whose `app_state`, `decided_date` or `last_different` actually differs.

Per-authority, not national, because an unbounded national sweep is the query we must never send (`start_date >= today-365` returned `total: null` after **45 seconds**).

**Lane C is not on the critical path for the cutover.** It is a completeness backstop, and it is deliberately *not* the verification mechanism — see "The oracle is free" below.

### The oracle is free

A cutover to a new ingest axis needs an independent check that it is not silently dropping records. That check must not cost PlanIt requests, and it must not come from a developer machine.

It doesn't have to. **PlanIt returns `total` in the very response Lane A is already fetching** — the count of records matching the filter. Stamp it on the poll span alongside what we actually saw:

| Span attribute | Meaning |
|---|---|
| `planit.total` | records PlanIt says match the window |
| `poll.records_seen` | records we fetched |
| `poll.records_ingested` | records that were new or changed |
| `poll.watermark_before` / `poll.watermark_after` | the delta boundary we moved |
| `poll.pages` | pages walked |

For a given `different_start` day, the union of `records_seen` across that day's runs must equal `planit.total`. If it does, we skipped nothing — **proven from telemetry, at zero PlanIt cost, with nothing issued from a laptop.**

Note that `records_seen` — not a Postgres row count — is the correct left-hand side. An unchanged record that PlanIt re-touches is fetched, found identical, and *not written* (`polling/ingester.go` gates on `HasSameBusinessFieldsAs`), so its stored `last_different` stays at the old value. A Postgres-side count would therefore under-report and look like a skip when nothing was skipped.

### Retired

- The **unmasked** `different_start` drain, and with it the `MaxPagesPerAuthorityPerCycle` / `PageSize` throughput levers.
- Per-authority pagination cursors and the density-tiered scheduler (ADR 0021, ADR 0012). A national poll costs the same whether one user or ten thousand watch an authority, so there is nothing to prioritise.
- The freshness probe (GH #955). It was a workaround for the churn; with the mask, every poll *is* a freshness poll.
- GH #955 PR B (`pg_sz=300`, `MaxPages=2`) is cancelled. It accelerates a lane that no longer exists.

The `poll_state` **columns are left in place and unused** at cutover — see "Migration". They are dropped only once the new design is confirmed, because they are what a rollback lands on.

### Request budget

The PlanIt free-service red line is ~1,500 requests/day and is non-negotiable. We are currently **at** it (measured: 2,999 PlanIt request spans in 48h) — and it is the *old drain* that puts us there. Every day we keep it running is a day we sit on the line.

| Lane | Cadence | Requests/day | Records/day |
|---|---|---|---|
| A — new applications | hourly, national | ~24–30 | ~1,700 |
| B — decisions | hourly, national | ~24–30 | ~1,700 |
| C — reconciliation | **daily during soak**, then weekly | ~485 → ~70 | ~19,400 |
| Hydration of Lane C deltas | as needed | ~20 | small |
| **Total (soak)** | | **~560** | **~23,000** |
| **Total (steady state)** | | **~140–150** | **~23,000** |

Against today's ~1,500 requests/day and ~150,000 records/day: **~90% fewer requests and ~85% fewer records served** in steady state, and still ~60% fewer requests during the soak. Bandwidth falls from ~345 MB/day to under ~20 MB/day.

Both axes fall. That matters: request count alone is the wrong unit for a free service, because the operator's cost is dominated by rows retrieved and serialised, not by HTTP overhead. A design that cut requests while raising rows served would not be a saving.

The budget no longer scales with the number of watched authorities, which removes the growth cliff in the current design.

**This is also why the new lanes are not run in parallel with the old drain.** A parallel soak would hold us at ~1,530 requests/day — above the red line — for its whole duration, and would keep the load on PlanIt high precisely while we are least sure of ourselves. The cutover is the polite move as well as the cheap one. The freed budget is spent on verification instead (Lane C, daily), which is a far better use of it than running a drain we have already decided is wrong.

### Guardrails

- **PlanIt is called from the deployed worker only. Never from a developer machine.** Ad-hoc exploration, backfills and verification scripts must not hit planit.org.uk from a laptop. A blocked home or office IP is unrecoverable — we would lose our only data source and could not restore it. All PlanIt traffic originates from the Container Apps worker's egress. Verification of a deploy is done from telemetry and Postgres, never by re-querying PlanIt locally.
- **Bounded windows only.** No national query without both a `different_start` prefilter and a date mask. The 45-second `total: null` response is the evidence for why, and the 11.7-second response to a masked-but-unprefiltered national query is the evidence that the prefilter is what makes it cheap.
- **`pg_sz=300`.** PlanIt's documented default, and its docs explicitly ask callers not to raise it: *"Please don't try to get all the data in one request by setting this as your default page size. Instead make multiple requests with a smaller `pg_sz`."* At 777.6 bytes/record a 300-row page is 233 KB — 4.3x headroom under the 1,000 kB cap, so no adaptive page-size logic is needed.
- **`select` is mandatory** on every poll request. `other_fields` is ~60% of a record's bytes and nothing reads it back today.
- **Sort fields must appear in `select`** — a documented PlanIt constraint.
- **Watermarks only advance on records actually ingested.** Never advance a watermark past a page that errored.

## Consequences

### Positive

- **New applications reach users within the hour, nationally** — regardless of how far PlanIt's discovery lags the council's `start_date`.
- **A PlanIt re-index becomes a non-event.** It moves `last_different` on old records, which the `start_date` mask excludes.
- **No per-authority state to starve.** No cursors, no high-water marks, no drain, no stuck authorities. Two timestamps replace 485 cursors.
- **~90% less load on PlanIt on both requests and rows served** — a free service we have a standing obligation not to hammer.
- **Coverage stops depending on backlog depth.** Birmingham gets the same freshness as Camden, watched or not.
- **The scheduler, the LRU, the density tiers and the freshness probe all delete.** This is a large net reduction in code, not a trade.

### Negative / risks

- **The mask has a tail.** An application PlanIt discovers more than 90 days after its `start_date` is invisible to Lane A. Measured p99 lag is ~28 days, so this is a thin tail, and Lane C is the backstop — but such an application waits up to a week rather than an hour.
- **The cutover does not backfill.** It fixes the flow of new applications, not the stock of missing ones. Camden stays at 66/300 until a separate historical sweep runs. Anyone reading the coverage numbers after the cutover must not mistake this for a failure of the new axis.
- **A small coverage regression against a *healthy* drain.** The unmasked drain saw every change to every record. The masked delta cannot see a change to an application whose `start_date` is outside the mask and which carries no `decided_date` (2–16% of decided rows by state). Lane C is the backstop, but between Lane C runs those changes are invisible. Against today's *actual* drain — 131 authorities stuck mid-cursor — this is still a large net gain, but it is a real loss against the design's intent.
- **`other_fields` goes stale** for records ingested through the new lanes. Nothing consumes it today, but GH #935 ingested it deliberately. Needs a lazy-hydration path (on app-detail view) or a slow background job. **Open question, tracked separately.**
- **Bulk hydration by identity is unproven.** Lane C flags changed rows by `uid`; whether `id_match` accepts a comma-separated list of uids is untested. If it is single-valued, hydration costs one request per straggler.
- **Lane B's volume is inferred, not measured.** Lane A's national delta was measured at 1,717 records/day; Lane B is assumed to be the same order. Measure it before the cutover, not after.
- **This is a cutover, and the risk is a silent skip, not a crash.** The delta relies on a descending page and an in-memory watermark because `different_start` is date-granular; a boundary bug there drops applications with no error and no alert. Lane C's coverage metric is the mitigation, and it is the reason Lane C ships in the same release rather than later. See "Migration".
- **Rollback is safe but not free.** The old drain resumes from high-water marks that have gone stale by the length of the soak, so its windows come back larger. The decision window must be short.
- **A national query is a single point of failure.** Today one authority failing does not stop the others; after the cutover, one failing national query stops everything. Mitigated by the existing retry/backoff discipline and by the fact that the query is now cheap (0.2s) and shallow, but it is a genuine concentration of risk.

### Neutral

- ADR 0024 (Service Bus-only poll triggering) is unchanged; the cycle still fires the same way, it just asks PlanIt a different question.
- Watch-zone matching and notification fan-out are unchanged; they run on ingest as before. Re-ingesting an unchanged record already costs **no write and no fan-out** (`polling/ingester.go` gates on `HasSameBusinessFieldsAs`), so idempotency is not at risk.
- The `applications` table is unchanged. `poll_state` shrinks to a handful of rows.

## Migration

**A committed cutover, not a parallel run.** The request budget does not allow both (see "Request budget"), and the old drain is what holds us on the red line. We replace it in one deploy, and we buy safety with *verification* rather than with a parallel drain.

### Scope: forward flow only. This migration does not backfill.

**Say this plainly, because it is the easiest thing to get wrong: the masked delta only ingests records PlanIt touches from now on.** It does not go back and fetch what we are already missing. Camden's 234 absent applications arrive only when PlanIt next re-touches them, which for a quiet undecided application may be weeks.

So the cutover fixes the *flow* and does not fix the *stock*. Coverage of new applications goes to ~100% immediately; the historical hole stays until something else fills it. That is accepted deliberately: filling it is a separate exercise (a paced, worker-side historical sweep backwards on `start_date`) with its own budget and its own risks, and conflating the two would make both harder to verify.

**Corollary for the rollback trigger:** do *not* gate the cutover on Camden's 30-day coverage climbing to 90%. It will not, and it is not supposed to. Gate it on forward flow.

### Ship together, in one release

1. **Lanes A and B** replace the drain in the poll cycle, with the span attributes above.
2. **Lane C** ships in the same binary but may run on its weekly cadence from the start; it is a completeness backstop, not the cutover's verification.
3. **The `poll_state` columns are not dropped.** No migration runs. Rollback is a pure image redeploy.
4. The old drain code is **left in the binary but not wired**, so reverting is a config change if we need it faster than a redeploy.

### The failure mode we are actually guarding against

Not a crash — a crash is loud and the existing alerts catch it. The risk is a **silent skip**: `different_start` is date-granular, so exact delta semantics depend on paging descending until we cross an in-memory timestamp watermark. A boundary bug there drops applications quietly. No error, no 429, no alert; a resident simply never hears about the development next door.

The `planit.total` vs `records_seen` invariant is what detects it, and it costs nothing.

### Rollback trigger — named in advance

Roll back if, in the first 24 hours:

- **The coverage invariant breaks.** For any `different_start` day, `records_seen` < `planit.total` with no error and no 429. That is a silent skip, and it is the reason we are watching.
- **Forward flow stalls.** No new applications ingested for 3 consecutive hours while `planit.total` is non-zero. (PlanIt's national delta runs ~72 records/hour; measured, its scrape batch lands roughly 02:00–05:30 UTC, so a quiet evening hour is normal and an empty *batch window* is not.)
- **Freshness regresses.** `max(last_different)` in `applications` stops tracking PlanIt. Pre-cutover baseline: **2026-07-14 05:14:58Z**, against PlanIt's newest of 05:16:14Z — i.e. we are currently within ~1 minute at the head, and must stay there.
- PlanIt request volume exceeds ~600/day, or any 429 is returned.

Rollback is a redeploy of the prior revision. **It is safe but not free:** the old drain resumes from `high_water_mark` values that have gone as stale as the soak was long, so its windows come back bigger. Keep the decision window **short — days, not weeks**.

### After the soak

Once the coverage invariant has held for a week:

5. Delete the drain, the freshness probe, the density tiers and the scheduler tiering.
6. Drop `poll_state.cursor_next_index` / `cursor_next_page` / `cursor_known_total` / `high_water_mark`. **This is the point of no return** — after it, rollback means a re-seed, not a redeploy. Do it deliberately and not before.
7. Only then take on the historical backfill, as separate work.

## What the first version got wrong

Kept deliberately, because the errors are instructive and the evidence that exposed them was cheap.

The first version's **diagnosis was sound and its measurements were accurate** — Camden's 66 held applications, the 18,150-record window at 2.4% of the corpus, the 56,537-record 45-day national window, the 0.00% null `start_date` rate, and above all the finding that `last_different` is dominated by re-index churn. All confirmed.

Its **remedy** did not survive:

- **It abandoned the delta.** Making `start_date` the primary axis discards the only field that can answer "what changed since I last looked". With no delta, it was forced to re-sweep a 45-day window (56,537 records) *and* a 90-day decisions window (~113,000 records) **every single day** — ~224,000 records/day, against today's ~150,000. It cut requests by 78% while raising rows served by ~50%, and reported this as "~80% less load on PlanIt".
- **Its headline freshness claim was false for about half of all applications.** Its hourly poll used a 7-day `start_date` window; the measured p50 discovery lag is 7 days. Half of everything PlanIt discovers is already outside that window on the day it is found, and would have fallen through to the *daily* catch-up. Of the 10 Camden applications PlanIt touched on 2026-07-14, exactly **one** had a `start_date` inside 7 days.
- **It proposed `pg_sz=1000` and `pg_sz=5000`**, against PlanIt's explicit written request to use *smaller* pages — while invoking the free-service red line as its justification. It also condemned `pg_sz=300` for leaving "only 1.45x of headroom" under the 1 MB cap, then proposed `pg_sz=1000` at ~1.3x.
- **It dismissed the freshness probe on theory, one day after the probe shipped.** Prod data showed the probe ingesting 23 recent Camden applications on 07-13 and 10 on 07-14 — roughly Camden's full new-application run-rate.
- **Its starvation evidence could not distinguish "wrong axis" from "broken cursor".** The cursor-destroying bug (GH #958) was fixed in v0.20.2 at 11:46Z the same day, and the drain was already self-healing by the time the ADR was written (Camden's window shrank 18,150 → 12,410 on the first clean pass). The axis is still wrong — the churn table above settles that — but the *urgency* was an artifact of a bug that had already been fixed.

The general lesson: **the load a data provider bears is measured in rows it must retrieve, not in requests we send**, and a design that improves one while worsening the other has not saved anything.

## References

- GH #955 — polling throughput and freshness (index pagination, probe, telemetry). Its PR A shipped and is retained; its PR B is cancelled by this ADR. The freshness probe it added is superseded.
- GH #958 / PR #959 — mid-drain fetch error destroyed the poll cursor (fixed v0.20.2). The bug that exposed all of this.
- PR #960 — the first version of this ADR.
- Bead tc-eus6 — the original 2026-06-18 starvation data.
- PlanIt API docs: https://www.planit.org.uk/api/ — `select`, `start_date`/`end_date`, `different_start`, `decided_start`, `pg_sz` (default 300), 5,000-result and 1,000 kB caps, and the fair-use guidance on page size.
- Measurements in this ADR were taken against the live PlanIt API and prod Postgres on 2026-07-14.
