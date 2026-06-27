# 0032. Consolidate the datastore on Postgres + PostGIS (retire Cosmos DB)

Date: 2026-06-27

## Status

Accepted

Retires the Cosmos DB data model in [ADR 0008](0008-cosmos-db-data-model.md) and graduates [memo 0010](../memo/0010-cosmos-partition-strategy-and-postgres-postgis.md), which holds the full analysis. Executed as epic `tc-hpd2` ([GH #681](https://github.com/AmyDe/town-crier/issues/681)).

## Context

The Applications container was partitioned by `/authorityCode`, but almost every user-facing read is geographic and cross-cutting: "near this point", "nearest N", "how many in this circle", sorted paging, and watch-zone matching that ignores authority boundaries. Cosmos forces a single partition key per container, and `/authorityCode` made only two things cheap: the polling worker's point upsert/read and the per-authority SEO reads. Everything a user does is the other shape, so the spatial and aggregate reads all became cross-partition Gateway queries. The Gateway will not `ORDER BY` or aggregate across partitions (confirmed live: `BadRequest` substatus 1004 on a `COUNT`), so the mismatch was structural, not tunable. Two live problems traced to this one root: a 10 km London watch-zone browse pulled 22,220 docs / 1,640 RU / ~7.4 s by fanning out across every authority partition it touched (#641), and the iOS sort buttons could not be made correct because the client no longer holds the whole zone under the 500-app page cap.

Three assumptions that justified the original Cosmos choice had all lapsed:

- **The AOT rationale is gone.** Cosmos-over-EF-Core was chosen for .NET Native AOT compatibility. The backend is Go now ([ADR 0028](0028-migrate-backend-from-dotnet-to-go.md)), with first-class official SDKs for whatever database we pick, so that reason no longer applies.
- **Memo 0001's cost trigger fired.** [Memo 0001](../memo/0001-cosmos-db-scaling-path.md) set a revisit threshold of "RU costs exceeding ~$15/month consistently". Cosmos had reached ~£37/month (~67% of the Azure bill) and was drifting up, driven by spatial fan-out read RU and continuous polling upserts.
- **The change-feed blocker never materialised.** Memo 0001 named losing the Cosmos change feed as the hardest part of any Postgres move. There is no change feed in the shipped Go system: the notify fan-out is a synchronous, in-process call inside the polling worker, and the Service Bus poll chain ([ADR 0024](0024-service-bus-only-polling.md)) is the trigger. The feared hardest part of the migration was not part of the migration at all.

See [memo 0010](../memo/0010-cosmos-partition-strategy-and-postgres-postgis.md) for the full query-shape inventory, per-container analysis, cost comparison, and options weighed.

## Decision

**Consolidate all data onto a single Postgres + PostGIS datastore and retire Cosmos DB entirely.** One Azure Database for PostgreSQL Flexible Server hosts both environment databases (`town_crier_dev` and `town_crier_prod`) with separate owning roles, mirroring the single-account posture Cosmos held before it. This is a single store, not a hybrid and not a standing Cosmos+Postgres straddle.

The migration was executed in phases rather than big-bang:

1. **Applications + WatchZones first**, because they hold the spatial pain, the #641 browse meltdown, and the iOS sort release-gate. Data was migrated behind the existing repository interfaces, with a GiST index on a `geography(Point, 4326)` location column replacing the Cosmos spatial index.
2. **The remaining containers** (Users, Notifications, NotificationState, SavedApplications, DeviceRegistrations, PollState, Leases, OfferCodes, AppleNotifications) followed into Postgres behind a `STORE_BACKEND` flag across the API and worker, with Cosmos ETag-CAS patterns mapping to row locks and the Notifications TTL mapping to a scheduled purge job.
3. **Code strip.** All Cosmos SDK usage, stores, and the `STORE_BACKEND` flag were removed (v0.15.49), leaving Postgres as the sole datastore.
4. **Resource deletion.** Both Cosmos databases and the `cosmos-town-crier-shared` account were deleted (2026-06-27).

The migration is now complete.

## Consequences

### Easier

- **Spatial reads are indexed PostGIS.** Radius filters become `ST_DWithin(location, point, radius_metres)`, nearest-N becomes the KNN GiST operator `ORDER BY location <-> point LIMIT n`, and counts become an accurate `COUNT(*)` over the true result set. No cross-partition fan-out, no substatus 1004, no client-side merge-and-resort. The #641 browse meltdown and the iOS sort release-gate both dissolve into ordinary indexed SQL.
- **Polygonal watch zones become possible.** `ST_Contains(polygon, location)` lets users draw around specific streets, a product feature Cosmos circle-only matching could not serve at all.
- **A real local test loop.** Postgres runs as a throwaway Docker container, so the TDD loop gains real-database integration tests with pre-seeded spatial scenarios (behind the `//go:build integration` tag, via the `internal/platform/postgres/pgtest` harness). This gives a clean Local → Dev → Prod path and made the prod-only notify fan-out testable before it shipped, which the Cosmos emulator never delivered.
- **One database, one mental model.** Foreign keys, joins, and transactions are available, and the cross-partition admin scans that Cosmos charged for become plain `WHERE` clauses.
- **Flat, lower database cost.** Both environments now run on roughly £14-15/month of Burstable Flexible Server with no per-query charge, versus the rising ~£37/month metered Cosmos RU line that scaled with both read and write volume.

### Harder

- **Postgres is low-ops, not zero-ops.** The trade-off for retiring Cosmos's zero-ops posture is that we now size the instance (and may step it up under load), accept brief restarts on managed minor-version upgrades, and own connection-pooling and backup-retention configuration. For a solo developer this is a modest, well-trodden burden, but it is a real change from "nothing to operate".
- **Cutover risk on the poll and notify hot paths** was carried during the phased migration, most of it reduced by the local seeded-test harness that let the notify fan-out be proven against known points and circles before it touched dev or prod.

## See also

- [memo 0010](../memo/0010-cosmos-partition-strategy-and-postgres-postgis.md): the full analysis this ADR graduates, covering the query-shape inventory, per-container survey, cost/ops comparison, and options A/B/C.
- [ADR 0008](0008-cosmos-db-data-model.md): the Cosmos DB data model this retires.
- [ADR 0006](0006-planit-primary-data-provider.md): PlanIt provider and the polling ingestion model, unaffected by the database move.
- [ADR 0024](0024-service-bus-only-polling.md): the Service Bus poll chain that triggers the notify fan-out; there was never a change feed to lose.
- [ADR 0028](0028-migrate-backend-from-dotnet-to-go.md): the Go migration that removed the Native AOT rationale for Cosmos.
- [memo 0001](../memo/0001-cosmos-db-scaling-path.md): the original Cosmos scaling analysis whose cost revisit trigger fired.
</content>
</invoke>
