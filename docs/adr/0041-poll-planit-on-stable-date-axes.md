# 0041. Poll PlanIt nationally on stable date axes; retire the per-authority `last_different` drain

Date: 2026-07-14

## Status

Accepted — supersedes the polling design in [0006](0006-planit-primary-data-provider.md); reverses [0012](0012-dynamic-polling-prioritisation.md) and [0021](0021-resumable-pagination-cursor-for-planit-polling.md)

## Context

Town Crier exists to tell residents that a planning application near them has opened for comment, while the consultation window is still open. That window is typically 21 days from validation. Everything else the product does is secondary to hitting it.

We do not hit it. On 2026-07-14, measured against prod:

- Camden holds **66** applications with a `start_date` in the last 30 days. PlanIt has ~350 for the same window (510 since 2026-06-01). **We hold roughly one in five.**
- Adur and Worthing (2 users watching) had **zero** new applications ingested in 7 days.
- Birmingham — the largest planning authority in the country — had **zero** new applications ingested in 34 days. Portsmouth: 88 days. St Albans: 69. Preston: 55. Forty-plus authorities were more than a week blind.
- 54 authorities were in a starving state (GH #955), Camden's high-water mark pinned 51 days stale.

### How we got here

The polling design drifted across three ADRs, each step locally reasonable:

| ADR | What it decided | Effect |
|-----|-----------------|--------|
| 0006 | One **national** delta poll: `?different_start={last_poll}&pg_sz=5000&sort=-last_different` | Stateless, whole-country, one query |
| 0012 | Tier authorities by watch-zone density and poll each individually | One national query became ~485 per-authority polls |
| 0021 | Add a resumable pagination cursor per authority | Each of the 485 polls became stateful, and could get stuck |

The result is 485 independent cursors, each able to starve on its own. A PlanIt bulk re-index starved 54 of them at once.

### The root cause is the axis, not the throughput

`last_different` is **scrape bookkeeping**: it moves whenever PlanIt's scraper decides a record differs. A PlanIt re-index rewrites it across an authority's entire corpus. Camden's re-touch window held **18,150 records — 2.5% of our whole national dataset** — and our drain, at 100 records per request, had to walk all of it before the high-water mark could advance. PlanIt has done this twice in four months.

So we notify people about planning applications by polling the one field that has nothing to do with when an application appeared. New applications sit *behind* the churn, in ascending `last_different` order. The freshness probe added in GH #955 does not save us either: it sorts by `-last_different`, so during a re-index its 100-row page is filled with re-scraped junk while genuinely new applications sit below it.

Throughput work (GH #955 PR B: `pg_sz` 300, `MaxPages` 2) accelerates the churn drain. It does not put a single new application in front of a user any sooner, and it is therefore not the fix.

### What PlanIt actually offers (measured 2026-07-14)

The API has axes we have never used. `select` is the one that changes the economics:

| Projection | Bytes/record | Records per request |
|---|---|---|
| Everything (what we send today) | ~2,300 | ~430 max; we ask for **100** |
| Every field we ingest, minus `other_fields` | **766** | ~1,000 (1 MB cap binds) |
| `uid` + `last_different` only | 68 | 5,000 (result cap binds) |

And two stable axes a re-index cannot touch, because they are the council's dates, not PlanIt's bookkeeping:

- `start_date` / `recent=N` — when the application was submitted.
- `decided_start` / `decided=N` — when the council decided it.

Measured against the live API:

- **A national query works and is fast.** `start_date >= today-2`, no `auth` filter: 187 records, 66 authorities, 143 KB, **0.11s**.
- **The whole country's new applications for 45 days is 56,537 records** — about 1,256/day nationally. At 766 bytes/record that window is **57 requests**.
- PlanIt **rejects** an over-cap page rather than truncating it: `HTTP 400 {"error": "Response content too large: 1409780 > 1000000"}`. Our client classifies 400 as permanent, so an authority whose page exceeds 1 MB errors out. GH #955 PR B's `pg_sz=300` leaves only 1.45x of headroom on Camden's fat records; this is a live stall risk.
- **Do not issue unbounded national queries.** `start_date >= today-365` returned `total: null` after **`secs_taken: 45.016`** — PlanIt ground for 45 seconds and gave up counting. Our own 30s client timeout would have killed it. Windows must be bounded and sliced.

### Two facts that constrain the design

1. **Scraper discovery lag.** An application reaches PlanIt days or weeks after its `start_date`. Only 187 applications nationally carry a `start_date` inside the last 2 days, far below the 1,256/day true rate — the rest arrive later, already back-dated. A narrow cursor on `start_date` would step straight over them. For decisions, measured lag between `decided_date` and PlanIt noticing is p50 6.3 days, p90 18.5, p99 32.9, max 86.6 (an upper bound; re-touches inflate it).
2. **`decided_date` is not universal.** Of decided-state rows: Permitted 1.8%, Rejected 2.5%, Conditions 3.4%, Withdrawn 8.9%, Referred 16.1% carry **no** `decided_date`, and 2.8% of all rows carry no `app_state` at all. About one decision in forty is invisible to a `decided_start` poll.

Both are handled by widening windows (cheap, because volume is low) and by a reconciliation lane, not by reverting to the churn axis.

## Decision

Poll PlanIt **nationally**, on the **stable date axes**, with a `select` projection. Retire per-authority high-water marks, cursors, density tiering, and the `last_different` drain.

Three lanes.

### Lane A — new applications (the product)

The critical path. Two cadences, both national, both `select`-projected (`other_fields` dropped), `compress=on`:

- **Fresh poll, hourly:** `?start_date=<today-7>&sort=-start_date&pg_sz=1000` — ~650 records, 1 request. Anything PlanIt has discovered promptly reaches a user within the hour.
- **Catch-up poll, daily:** the full 45-day window (56,537 records), **sliced by date range** (`start_date` + `end_date` per slice) rather than deep index paging, so no query goes deep or long. ~57-90 requests depending on slice width.

The 45-day window absorbs discovery lag: an application PlanIt finds a fortnight late still carries a `start_date` inside the window, so it is still ingested. Ingest is idempotent, so re-seeing a record costs a cheap no-op write and nothing else.

### Lane B — decisions

Same shape, on `decided_start`:

- **Fresh poll, hourly:** `?decided_start=<today-7>` — 1 request.
- **Catch-up poll, daily:** `decided_start=<today-90>`, date-sliced. The 90-day window covers the p99 discovery lag of 33 days with a wide margin.

### Lane C — reconciliation (completeness)

Catches what the date axes structurally cannot: decisions with no `decided_date`, rows with no `app_state`, upstream corrections, and deletions.

Per authority, a light projection sweep (`select=uid,app_state,decided_date,last_different`, `pg_sz=5000`, ~68-100 bytes/record) covering that authority's live set. At ~280 pending applications per authority this is one request each: **485 requests, run weekly (~70/day amortised)**. Diff against Postgres locally; hydrate only rows whose `app_state`, `decided_date` or `last_different` actually differs.

Per-authority, not national, because an unbounded national sweep is the 45-second query we must never send.

### Retired

- The `different_start` drain, and with it the `MaxPagesPerAuthorityPerCycle` / `PageSize` throughput levers.
- Per-authority `poll_state` high-water marks and pagination cursors (ADR 0021). Lane state is a handful of global watermarks, not 485 cursors.
- Watch-zone-density polling tiers (ADR 0012). A national poll costs the same whether one user or ten thousand watch an authority, so there is nothing to prioritise.
- GH #955 PR B (`pg_sz=300`, `MaxPages=2`) is cancelled. It accelerates a lane that no longer exists.

### Request budget

The PlanIt free-service red line is ~1,500 requests/day and is non-negotiable.

| Lane | Cadence | Requests/day |
|---|---|---|
| A — new applications, fresh | hourly, 7-day window | ~24 |
| A — new applications, catch-up | daily, 45-day window, sliced | ~57-90 |
| B — decisions, fresh | hourly, 7-day window | ~24 |
| B — decisions, catch-up | daily, 90-day window, sliced | ~90-120 |
| C — reconciliation | weekly, per authority | ~70 |
| Hydration of Lane C deltas | as needed | ~40 |
| **Total** | | **~305-370** |

Roughly **a fifth of today's ~1,600/day**, for complete national coverage and sub-hour freshness on new applications. Bandwidth falls too: ~72 MB/day against today's ~368 MB (1,600 fat pages).

The budget no longer scales with the number of watched authorities, which removes the growth cliff in the current design (at hourly per-authority polling we could not have exceeded ~62 watched authorities without breaching the red line).

### Guardrails

- **Bounded windows only.** No national query without a date range. The 45-second `total: null` response is the evidence for why.
- **Adaptive page size.** On `HTTP 400 "Response content too large"`, halve `pg_sz` and retry. The 1 MB cap is a hard rejection, not a truncation.
- **`select` is mandatory** on every poll request. `other_fields` is ~60% of a record's bytes and nothing reads it back today (GH #935 stores it against future use).

## Consequences

### Positive

- **New applications reach users within the hour, nationally.** The thing the product is for.
- **A PlanIt re-index becomes a non-event.** It moves `last_different`, which no lane on the critical path reads.
- **No per-authority state to starve.** No cursors, no high-water marks, no drain, no 54 stuck authorities.
- **~80% less load on PlanIt** — a free service we have a standing obligation not to hammer.
- **Coverage stops depending on backlog depth.** Birmingham gets the same freshness as Camden, watched or not.

### Negative / risks

- **`other_fields` goes stale** for records ingested through the new lanes. Nothing consumes it today, but GH #935 ingested it deliberately. Needs a lazy-hydration path (on app-detail view) or a slow background job. **Open question, tracked separately.**
- **Bulk hydration by identity is unproven.** Lane C flags changed rows by `uid`; whether `id_match` accepts a comma-separated list of uids is untested. If it is single-valued, hydration costs one request per straggler (~40/day, still affordable).
- **Reliance on `start_date` presence.** Rows with a null `start_date` are invisible to Lane A. Measured at 0.00% of our corpus, so this is theoretical, but Lane C is the backstop.
- **Migration is a rewrite of the poll handler**, not a tweak. It runs alongside the existing drain until verified, then the drain is deleted.

### Neutral

- ADR 0024 (Service Bus-only poll triggering) is unchanged; the cycle still fires the same way, it just asks PlanIt different questions.
- Watch-zone matching and notification fan-out are unchanged; they run on ingest as before.
- The `applications` table is unchanged. `poll_state` shrinks dramatically.

## Migration

1. **Lane A first**, shipped alongside the existing drain. It is additive: a new fetch on a new axis, ingesting through the existing idempotent path. Verify on prod telemetry that new-application coverage for Camden and Adur and Worthing goes to ~100%.
2. **Lane B**, same shape.
3. **Lane C**, plus the hydration path.
4. **Delete the drain**, the cursor, the high-water marks and the density tiers once Lanes A-C are proven. Drop `poll_state.cursor_next_index` / `cursor_next_page` / `high_water_mark`.

Rollback at every step is "stop running the new lane"; the drain remains until step 4, so there is no window where the product is worse off than today.

## References

- GH #955 — polling throughput and freshness (index pagination, probe, telemetry). Its PR A shipped and is retained; its PR B is cancelled by this ADR.
- GH #958 / PR #959 — mid-drain fetch error destroyed the poll cursor (fixed v0.20.2). The bug that exposed all of this.
- Bead tc-eus6 — the original 2026-06-18 starvation data.
- PlanIt API docs: https://www.planit.org.uk/api/ — `select`, `start_date`, `decided_start`, `pg_sz` (default 300), 5,000-result and 1,000 kB caps.
- Measurements in this ADR were taken against the live PlanIt API and prod Postgres on 2026-07-14.
