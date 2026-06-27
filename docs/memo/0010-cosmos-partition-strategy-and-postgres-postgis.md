# 0010. Cosmos `/authorityCode` partition strategy and the Postgres + PostGIS question

Date: 2026-06-26

## Status

Superseded by ADR [0032](../adr/0032-consolidate-datastore-on-postgres-postgis.md). This memo records the analysis. **Owner direction (2026-06-26): the end state must be a single datastore, with Cosmos fully retired. This is NOT a hybrid and NOT a standing Cosmos+Postgres straddle — every container moves to Postgres, then Cosmos is deleted.** **Sequencing update (2026-06-26, night): resolved to a single full cutover.** Applications + WatchZones were ported first only because they hold the spatial pain, the #641 Phase-2 list, and the iOS sort release-gate — never because the other nine were staying on Cosmos. The other nine follow into Postgres in the same push; the only moment both databases coexist is the brief backfill→flip→delete cutover window, which is a deployment step, not the destination. The full-cutover spec is [#669](https://github.com/AmyDe/town-crier/issues/669) (bead tc-hpd2.5); Apps+WatchZones shipped in #657/#661/#664. Read the "Option B: Hybrid" section below as analysis-of-record from when the path was being chosen, **not** as the plan. It graduates to an ADR once Cosmos is deleted. Tracked by bead tc-zgfu / [GitHub #643](https://github.com/AmyDe/town-crier/issues/643). It is the deeper companion to the immediate fix shipped in [#642](https://github.com/AmyDe/town-crier/pull/642) (bounded watch-zone browse) and to [memo 0009](0009-watch-zone-cross-authority-matching.md) (the boundary-geometry problem the recent fixes deliberately sidestep).

## Question

The Applications container is partitioned by `/authorityCode`, but almost every user-facing read is geographic and cross-cutting: "near this point", "nearest N", "how many in this circle", sorted paging, and zone matching that ignores authority boundaries. The Cosmos Gateway refuses cross-partition `ORDER BY` and aggregates (confirmed live: `BadRequest` substatus 1004 on a `COUNT`). So the partition model fights the product's core query shape, and every workaround (bounding-box prune, partition-prune-and-merge, client-side sort and count) is friction.

Two live problems this week trace to this one root, not to two separate bugs:

- The watch-zone map browse meltdown (#641, fixed in #642): a 10 km London zone pulled **22,220 docs / 24.3 MB / 1,640 RU / ~7.4 s** because the spatial query fans out across every authority partition the circle touches.
- The iOS sort buttons cannot be made correct cheaply: five client-side sorts plus status chips plus an unread count all assume the client holds the whole zone, which the new 500-app cap breaks (a 2 km London zone already returns 1,364 apps).

Is the `/authorityCode` data model fit for purpose, and is now the moment to move the geographic data to Postgres + PostGIS?

## Analysis

### The root constraint: one cause, many symptoms

Cosmos requires you to pick a single partition key per container, and that key determines which reads are cheap. `/authorityCode` makes exactly two things fast, and they are both genuinely fast:

1. **The polling worker's hot path.** Point upsert and point read keyed by `(authorityCode, planitName)` cost ~1 RU. This is the highest-frequency write in the system (`Upsert`, `applications/store_cosmos.go:45`; `GetByAuthorityAndName`, `:59`).
2. **Per-authority SEO reads.** `RecentByAuthority` (`:109`) and `BreakdownByAuthority` (`:148`) are single-partition, served by the composite index `(/authorityCode ASC, /lastDifferent DESC)` and the spatial index on `/location` (`infra/environment.go:198-227`). These are bounded, index-seeking, and cheap.

Everything a *user* does is the other shape. A watch zone is a circle (centre plus radius); it is not an authority. Matching applications to a circle is a spatial question, and the partition key is the wrong axis for it. The result is that the spatial and aggregate reads are all cross-partition Gateway queries, and the Gateway will not order or aggregate across partitions. That is not a tuning problem; it is structural to the chosen key.

### Query-shape inventory

Every Applications and WatchZones operation, its current partition behaviour, and its Postgres + PostGIS equivalent. (The remaining containers are surveyed in the next section.)

| Operation | File | Cosmos behaviour today | Postgres + PostGIS |
|---|---|---|---|
| `Upsert` (poll hot path) | `applications/store_cosmos.go:45` | Point upsert, ~1 RU | `INSERT ... ON CONFLICT (authority_code, planit_name) DO UPDATE` |
| `GetByAuthorityAndName` | `:59` | Point read, ~1 RU | `WHERE authority_code=$1 AND planit_name=$2` (unique index) |
| `RecentByAuthority` (SEO) | `:109` | Single-partition, composite-index TOP-N | `WHERE authority_code=$1 ORDER BY last_different DESC LIMIT n` |
| `BreakdownByAuthority` (SEO) | `:148` | Single-partition `GROUP BY appState` | `... GROUP BY app_state` |
| `FindNearby` / `FindNearbyPage` (browse) | `:212` + `#642` page helper | **Cross-partition** `ST_DISTANCE(...) <= r`, no order; bounded to one 500-doc page because the Gateway cannot order or rank | `ORDER BY location <-> point LIMIT n` (true nearest-N, paged via keyset) |
| `RecentNearby` (SEO) | `:329` | Single-partition spatial + order by `lastDifferent` | `WHERE ST_DWithin(...) ORDER BY last_different DESC LIMIT n` |
| `NearestNearby` (SEO) | `:378` | Single-partition spatial, order by `ST_DISTANCE` | `ORDER BY location <-> point LIMIT n` |
| `BreakdownNearby` (SEO) | `:262` | Single-partition spatial `GROUP BY` | `WHERE ST_DWithin(...) GROUP BY app_state` |
| `FindZonesContaining` (notify hot path) | `watchzones/store_cosmos.go:233` | **Cross-partition** bbox prune + `ST_DISTANCE <= c.radiusMetres` residual over every user's zones | `WHERE ST_DWithin(location, point, radius_metres)` (single GiST index) |

The two cross-partition rows are where the model fights the product. `FindNearby` backs the map browse (cold, user-initiated) and `FindZonesContaining` backs the notify fan-out (hot, runs per polled application inside the worker). Both are deliberate cross-partition scans because matching is geographic, not authority-scoped (see the function comments and memo 0009). Cosmos serves the spatial filter from its `/location` index, but it cannot then order the result by distance or recency, nor count it, across partitions. That single limitation is the whole of the #641 Phase-2 problem and the whole of the iOS sort gate.

### The other nine containers

The non-geographic containers are partitioned per user or per natural key, which is the right Cosmos shape for them, and they would gain little from a move on their own merits:

| Container | Partition key | Shape | Verdict |
|---|---|---|---|
| Users | `/id` | Point CRUD; admin cross-partition scans (`GetByEmail`, `Dormant`, `LapsedPaid`, `List`) | Per-user point reads are fine on Cosmos; admin scans are cold and rare |
| WatchZones | `/userId` | Per-user list (good) + the cross-partition spatial `FindZonesContaining` (bad) | **Geographic; moves with Applications** |
| Notifications | `/userId` | Per-user reads; `GetLatestUnreadByApplications` uses `ARRAY_CONTAINS` (`notifications/...:47`); 90-day TTL | Cosmos TTL is a real convenience here |
| NotificationState | `/userId` | Point read/write + scalar `UnreadCount` (single-partition, fine) | Fine on Cosmos |
| SavedApplications | `/userId` | Point ops + `UserIDsForApplication` cross-partition by `applicationUid` (hot, decision fan-out) | Mildly geographic-adjacent; candidate for a later phase |
| DeviceRegistrations | `/userId` | Point CRUD | Fine on Cosmos |
| PollState | `/id` | Per-authority point cursor (hot) + LRU scan (cold) | Fine on Cosmos |
| Leases | `/id` | ETag CAS for the poll lease (hot) | Maps to a row lock, but works well as-is |
| OfferCodes | `/code` | Point redeem via ETag CAS | Maps to `SELECT ... FOR UPDATE`, but works well as-is |
| AppleNotifications | `/id` | Idempotency point read/write | Fine on Cosmos |

So the spatial pain is concentrated entirely in **Applications + WatchZones**. The rest is comfortable where it is *on its own merits* — but it still all migrates under the single-store decision (see Status). This table only explains the nine have no *spatial* reason to move, not that they stay on Cosmos.

### PostGIS fit for the #641 Phase-2 list and the iOS sort gate

Every open item collapses into ordinary indexed SQL with one GiST index on a `geography(Point, 4326)` column:

- **Nearest-N, paged:** `ORDER BY location <-> ST_Point(lon, lat) LIMIT n` uses the KNN GiST operator for true nearest-first, and keyset pagination gives stable "load more".
- **Radius filter:** `ST_DWithin(location, point, radius_metres)` is index-served and exact.
- **Accurate in-zone count:** `SELECT count(*) ... WHERE ST_DWithin(...)`. No substatus 1004, no client-side counting over a truncated page.
- **Viewport (bbox) loading as the user pans:** `location && ST_MakeEnvelope(...)`.
- **Cross-authority by default:** there is no partition boundary to cross, so memo 0009's border-miss problem stops existing rather than being worked around.
- **The iOS sort buttons:** the five sorts, the status chips, and the unread count all become a server-side `ORDER BY <column> LIMIT/keyset` plus a `GROUP BY app_state`, computed over the true result set rather than over whatever 500 docs the cap happened to return in partition order. The release gate evaporates.
- **A genuine product unlock:** polygonal watch zones (draw around specific streets) become a `ST_Contains(polygon, location)` query. Cosmos circle-only matching cannot do this at all.

### Three things that changed since the Cosmos decision

The Cosmos choice was correct when it was made. Three of its load-bearing assumptions no longer hold.

1. **The original rationale is obsolete.** Cosmos-over-EF-Core was chosen for .NET Native AOT compatibility (memory `project_data_access`; the AOT analysis in [memo 0007](0007-backend-language-choice-revisit.md)). The stack is Go now (.NET removed 2026-06-15, [ADR 0028](../adr/0028-migrate-backend-from-dotnet-to-go.md)), with first-class official SDKs for whatever database we pick. That reason is simply gone.
2. **Memo 0001's own revisit trigger has fired.** [Memo 0001](0001-cosmos-db-scaling-path.md) said to stay on Cosmos serverless until "RU costs exceeding ~$15/month consistently". Cosmos is now **~£37/month (~$47) and drifting up**, ~67% of the entire Azure bill (cost-forecast 2026-06-22). The trigger has been met. The cause differs slightly from what was predicted (read RU from spatial fan-out, plus continuous polling upserts, rather than the anticipated own-scraper write volume), but the threshold is crossed either way.
3. **The biggest stated blocker no longer exists.** Memo 0001 named "losing the Cosmos change feed, forcing the notification pipeline to be re-architected to an outbox or `pg_notify`" as the hardest part of any Postgres move. **There is no change feed in the shipped Go system** (confirmed: zero references in `api-go`). The notify fan-out is already a synchronous, in-process call inside the polling worker: `polling/handler.go:449` calls `notifydispatch.EnqueueForApplication`, which calls `FindZonesContaining` against the just-upserted application. The Service Bus poll chain is the trigger ([ADR 0024](../adr/0024-service-bus-only-polling.md)), not a change feed. Migrating Applications + WatchZones changes `FindZonesContaining` from a cross-partition Cosmos query into a PostGIS `ST_DWithin`; the trigger mechanism is untouched. The hardest part of the migration, as feared in 2026, is not part of the migration at all.

### Cost and ops comparison

Pricing pulled live from the Azure Retail Prices API on 2026-06-26 (Consumption, uksouth; USD converted at ~0.79).

| | Cosmos serverless (today) | Postgres Flexible Server B1ms | Postgres Flexible Server B2s |
|---|---|---|---|
| Compute | metered RU | $0.019/hr ≈ **£11/mo** flat | $0.076/hr ≈ £44/mo flat |
| Storage | bundled in RU/GB | $0.133/GB/mo (data is ~0.7 GB; 32 GB min ≈ £3.4/mo) | same |
| Per-query cost | **yes** (every spatial fan-out is metered; a 10 km browse = 1,640 RU) | none | none |
| Effective monthly | **~£37 and rising** | **~£14-15 flat** | ~£47 flat |
| Scaling behaviour | floor rises with usage | flat until you resize the instance | flat |

Two points matter more than the headline numbers:

- **Cosmos serverless is not actually "near-zero cost" here.** It is at a ~£37/month floor that *rises* with both write volume (continuous polling upserts) and read volume (each cross-partition spatial fan-out is metered RU). A B1ms Postgres is ~£14-15/month *flat*, with no per-query charge, and the spatial queries that are expensive on Cosmos become free index lookups. The cost case for migrating is neutral-to-favourable even before counting the engineering benefit.
- **The honest counter is operations, not money.** Cosmos serverless is genuinely zero-ops: no instance to size, autoscale, automatic everything. Postgres Flexible Server is low-ops but not zero: you size the instance (and may need to step B1ms up to B2s under load), you accept brief restarts on managed minor-version upgrades (a maintenance window), and you own connection pooling (Flexible Server ships PgBouncer) and backup-retention config. Autovacuum and patching are managed. For a solo developer this is a modest, well-trodden burden, but it is a real change from "nothing to operate".

**One instance can host both dev and prod, and should.** A single Flexible Server holds multiple databases, so `town_crier_dev` and `town_crier_prod` live on one instance with separate owning roles (so a dev credential cannot read or write prod data). That roughly halves the compute line versus two instances, and it mirrors the posture already in place: `cosmos-town-crier-shared` is one account serving both environment databases today (uksouth; the prod app runs in ukwest and already crosses to uksouth for Cosmos, so a shared uksouth Postgres is no latency regression). The usual objection to sharing a small Burstable box is noisy-neighbour contention (a heavy dev load test or backfill burning CPU credits or connections that prod also needs), but the local-Docker layer below neutralises most of it: the heavy and experimental dev work runs locally, leaving the shared Azure instance to carry only gentle integration traffic plus prod. Start on B1ms with the built-in PgBouncer on; resize to B2s, or promote prod onto its own instance, if connection or CPU pressure ever shows. Net effect: the single-store destination runs on roughly **£14-15/month of database for both environments**, versus today's rising ~£37/month Cosmos line that already serves both.

A single-AZ instance with automated backups is the right pre-revenue choice; zone-redundant HA doubles the compute line and is not warranted yet (it matches the fix-forward, no-rollback posture already adopted, memory `feedback_fix_forward_until_paying_users`).

### Local Postgres in Docker: the test-loop benefit Cosmos cannot give

There is a benefit the cost tables miss entirely, and it is the one the owner most wants. Postgres runs as a throwaway Docker container locally; Cosmos effectively cannot. The Cosmos emulator is heavy, flaky on Apple Silicon, and has never been in the test loop, so today there is no way to run a real-database test against pre-seeded data. That gap is why the spatial and partition behaviour has only ever been verifiable against live dev and prod, never in CI.

Postgres closes it. The TDD loop gains real-database integration tests with pre-seeded spatial scenarios (place a zone here, applications there, assert the fan-out matches exactly), running in milliseconds against a local container in CI and on the developer machine. Hand-written fakes still cover the unit layer; the new capability is a thin layer of real-Postgres tests for precisely the spatial and sorted-query behaviour that fakes cannot honestly model.

This yields a clean **Local → Dev → Prod** path, and it directly de-risks the scariest part of the cutover. Only prod runs the polling worker (dev has no poll job, memory `project_dev_has_no_poll_worker`), so `FindZonesContaining`, the notify fan-out, never executes in dev and cannot be validated there. A seeded local Postgres lets us prove that query, and the whole notify path, against known points and circles before it ever touches dev or prod. The thing we currently cannot test becomes the thing we test first.

## Options Considered

### Option A: Status quo on Cosmos, add partition-prune-and-merge

Keep Cosmos. Make nearest-N, accurate counts, and sorted paging work by querying each authority partition single-partition (where `ORDER BY` and aggregates *are* allowed), then merging and re-sorting client-side.

**Rejected as the destination, though it is the only non-migration path.** It treats the symptom and keeps every structural problem. To prune to "the partitions this circle touches" you need to know which authorities the circle intersects, which is exactly the boundary-geometry data memo 0009 deliberately refused to own; the alternative is a blind fan-out to all physical partitions, merging N sorted streams in application code. It is fragile, it is real engineering effort, every read still costs metered RU that scales with usage, and it delivers none of the product upside (polygonal zones, true cross-authority matching, server-side sort). It is throwaway work if we ever migrate. Its only virtue is zero migration risk.

### Option B: Hybrid — move Applications + WatchZones to Postgres, leave the rest on Cosmos

Move only the two geographic containers. `FindNearby`, `FindNearbyPage`, `FindZonesContaining`, the four SEO spatial reads, and the poll upsert become Postgres + PostGIS. The nine per-user/per-key containers stay on Cosmos, where their partitioning is already optimal and Cosmos TTL (Notifications' 90-day expiry) and the lease/offer-code CAS patterns keep working unchanged.

**(Analysis-of-record — superseded by the Status. This describes the option as it was weighed, NOT the plan: the nine non-geographic containers are not staying on Cosmos, they move too.)** **The right first phase, but not an acceptable end state.** The owner has ruled out operating two databases long-term (see Status and Recommendation), so this is retained only as the opening slice of the all-in path, not a destination. As a phase it targets 100% of the spatial pain with the smallest first surface and directly clears the iOS release gate. During the phase the notify fan-out reads zones from Postgres and writes notifications to Cosmos, which is fine because that write is already a separate idempotent operation; the straddle lasts only as long as the cutover window, until the remaining containers follow. The seam to watch is `SavedApplications.UserIDsForApplication` (a hot cross-partition query by `applicationUid`), which moves in a later slice.

### Option C: All-in — every container to Postgres, retire Cosmos

Move everything. One database, one mental model, real foreign keys, joins, and transactions. Cosmos and its £37/month disappear entirely.

**The chosen destination (owner decision, 2026-06-26).** It is the largest total surface, but is executed in phases (Option B first), so it is taken a slice at a time rather than big-bang. It additionally has to map: ETag CAS (Leases, OfferCodes, Users zone-count) to `SELECT ... FOR UPDATE` or optimistic `xmin` checks; Cosmos per-document TTL (Notifications) to a partitioned-table purge job or a scheduled `DELETE`; and the various cross-partition admin scans to ordinary `WHERE` clauses (which get *cheaper*, not harder). None of these is novel, but together they are more cutover risk on more surfaces at once.

## Migration mechanics (single-store, phased)

- **Test harness first.** Stand up Postgres in Docker locally and a seeded-scenario integration-test layer before moving any container. Each container then migrates test-first along Local → Dev → Prod: write the real-DB tests against seeded data locally, port the store behind its existing repository interface, prove it locally and in dev, then cut prod. This is the sequencing the owner wants, and it is what makes the prod-only notify path testable before it ships.
- **Schema.** `applications(authority_code text, planit_name text, uid text, app_state text, app_type text, decided_date date, last_different timestamptz, location geography(Point,4326), ...)` with a unique index on `(authority_code, planit_name)`, a btree on `(authority_code, last_different desc)` to replace the Cosmos composite index, and a **GiST index on `location`** to replace the spatial index. `watch_zones(id, user_id, name, location geography(Point,4326), radius_metres, authority_id, push_enabled, email_instant_enabled, ...)` with a GiST index on `location` and a unique index on `(user_id, name)`.
- **Backfill of ~622k applications.** Read the Cosmos containers via the existing data plane, transform GeoJSON points to PostGIS `geography`, and `COPY` into Postgres in batches. At ~0.7 GB this is minutes, not hours, and is fully repeatable for a dry run. WatchZones is tiny.
- **Dual-write then cutover.** The repository interfaces are already the seam (consumer-side interfaces per the Go standards). Implement a Postgres store behind the same interface, dual-write upserts during a short window, backfill, verify counts and a sample of spatial queries against both, then flip reads. Because we are pre-revenue and fix-forward, the window can be short and the rollback ceremony minimal.
- **Worker pipeline.** Unchanged in shape. The poll handler still upserts the application and still calls `EnqueueForApplication` per app; only the store implementation behind `Upsert` and `FindZonesContaining` changes. The Service Bus chain, the lease guard, and ADR 0024's at-most-once semantics are untouched. (If Leases stay on Cosmos under Option B, even the lease path is unchanged.)
- **SEO reads.** `RecentByAuthority`, `BreakdownByAuthority`, `RecentNearby`, `NearestNearby`, `BreakdownNearby` all become short indexed SQL statements, removing the 9-second long-read RU budget those queries currently need. The SEO blob-snapshot pipeline (ADR 0031) is unaffected; it just calls a faster store.
- **ADR 0006 polling model.** Unaffected. PlanIt remains the provider, polling remains the ingestion model, the change-detection cursor logic is database-agnostic.

## Recommendation

**Direction set by the owner (2026-06-26): consolidate onto a single Postgres + PostGIS datastore and retire Cosmos entirely.** A Cosmos-plus-Postgres straddle is explicitly not an acceptable end state, so Option C is the target. The migration still phases through the geographic containers first (Applications + WatchZones), because that is where the value, the iOS release-gate fix, and the new local test harness all prove out; the remaining low-volume containers follow into Postgres until Cosmos is gone — not a standing straddle. The only Cosmos/Postgres overlap is the brief backfill→flip→delete cutover window, not where we stop. The reasoning:

1. The mismatch is **structural, not tunable.** The product's dominant user-facing read is spatial and cross-cutting; a single partition key cannot serve both that and the worker's point-write hot path, and the Gateway's refusal to order or aggregate across partitions caps what we can do server-side. PostGIS serves both shapes from one table with two indexes.
2. The three assumptions that justified Cosmos have all lapsed: the AOT rationale is gone, the cost trigger from memo 0001 has fired, and the change-feed blocker never materialised in the Go system.
3. It is **cost-neutral to cheaper**, and it removes the per-query RU cost that makes the spatial fan-outs expensive.
4. It **directly clears the iOS sort release gate and the entire #641 Phase-2 list** in one move, and unlocks polygonal zones as a future product feature.
5. **Now is the cheapest window.** Pre-revenue, ~0.7 GB of data, a handful of users, a fix-forward culture with no rollback ceremony. Every one of those makes the cutover cheaper today than it will ever be again.
6. **It buys a real local test loop.** Postgres in Docker gives pre-seeded, real-database spatial tests in CI and on the machine, which Cosmos never could, plus a clean Local → Dev → Prod path that lets us validate the prod-only notify fan-out before it ships. This is both a standing engineering win and the main lever that reduces cutover risk.

The honest counter-case, which is why this is still a memo and not yet an ADR: it is a few weeks of database work at a monetisation-approaching moment; it trades Cosmos's zero-ops for Postgres's low-but-nonzero ops; and there is genuine cutover risk on the poll and notify hot paths (most of it reducible by the local seeded-test harness). None of these is disqualifying. The owner has settled the destination (a single datastore, Cosmos retired); the open questions are timing and sequencing, which is what the ADR will pin down.

### The two open release threads, decoupled from this decision

Both can be resolved now without waiting for the migration:

- **Ship #642 to prod (v0.15.46).** The bounded browse fix is a strict improvement and is independent of the database decision. Recommend cutting the tag. The migration, if it happens, replaces the bounded-arbitrary-order page with a true nearest-first paged query later.
- **iOS sort buttons.** Do **not** build partition-prune-and-merge sort on Cosmos; it is fragile and throwaway if we migrate. Either ship a clearly-scoped interim (sort and count *within* the returned page, labelled as such) to clear the gate now, or hold the correct sort and fold it into the Postgres outcome, where it is a trivial server-side `ORDER BY`. The correct, full-zone sort should be delivered by the migration, not by Cosmos gymnastics.

### Revisit triggers / what would change the answer

- If the migration is judged too risky to take on before first revenue, Option A (prune-and-merge) plus an interim within-page iOS sort is the holding pattern, accepting the structural friction and rising RU until the window reopens.
- The single-store decision would only reopen if a near-term product direction made Cosmos's per-user partitioning newly valuable in a way Postgres cannot match (none is currently in view). Absent that, consolidation stands.
- If Postgres B1ms proves undersized in dev load testing, the step to B2s (~£44/month) is still comparable to today's Cosmos spend with far more headroom; this changes the cost framing but not the recommendation.

## References

- [GitHub #643](https://github.com/AmyDe/town-crier/issues/643) (bead tc-zgfu) — this investigation.
- [GitHub #641](https://github.com/AmyDe/town-crier/issues/641) / [#642](https://github.com/AmyDe/town-crier/pull/642) — the browse meltdown and its bounded-pagination fix (bead tc-fm8f); the Phase-2 list this memo would collapse into SQL.
- [Memo 0001](0001-cosmos-db-scaling-path.md) — the original Cosmos scaling analysis; its revisit trigger and its (now-moot) change-feed concern.
- [Memo 0009](0009-watch-zone-cross-authority-matching.md) — the boundary-geometry problem that Postgres makes disappear rather than work around.
- [ADR 0006](../adr/0006-planit-primary-data-provider.md) — PlanIt provider and the polling ingestion model (unaffected by a database move).
- [ADR 0024](../adr/0024-service-bus-only-polling.md) — Service Bus poll chain; the notify trigger that replaced any change-feed design.
- [ADR 0028](../adr/0028-migrate-backend-from-dotnet-to-go.md) / [memo 0007](0007-backend-language-choice-revisit.md) — the Go migration that removed the AOT rationale for Cosmos.
- [ADR 0031](../adr/0031-decouple-seo-rendering-via-blob-snapshot.md) — SEO blob snapshot; consumes the SEO reads, agnostic to the store.
- Cost evidence: `docs/cost-forecast/2026-06-22.md` (Cosmos ~£37/month, ~67% of bill, drifting up). Postgres pricing: Azure Retail Prices API, uksouth, 2026-06-26.
- Code: container and index definitions `infra/environment.go:196-277`; cross-partition helpers `api-go/internal/platform/cosmos.go:356-410`; Applications store `api-go/internal/applications/store_cosmos.go`; WatchZones `FindZonesContaining` `api-go/internal/watchzones/store_cosmos.go:233`; notify wiring `api-go/internal/polling/handler.go:449` and `api-go/internal/notifydispatch/enqueuer.go:138`.
</content>
</invoke>
