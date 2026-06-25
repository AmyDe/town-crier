# 0009. Watch-zone cross-authority matching and the boundary-geometry problem

Date: 2026-06-25

## Status

Open. The immediate fix (boundary-agnostic geographic matching) is specified in [GitHub #637](https://github.com/AmyDe/town-crier/issues/637) and tracked by bead tc-w11n. This memo records the boundary-geometry question that fix deliberately sidesteps, and the geometry-free optimisation held in reserve.

## Question

Watch zones are circles (centre lat/lon + radius) pinned to one UK local authority. Applications are stored in Cosmos partitioned by `/authorityCode`, tagged with their authority verbatim from PlanIt. A zone whose radius crosses an authority boundary silently misses every in-circle application belonging to the neighbouring authority.

How should a border-spanning zone match applications on both sides, given we have no authority boundary geometry and do not want to take on the cost of owning it?

## Analysis

### Where the miss comes from

Two scoping points restrict matching to a single authority:

1. **Notify / ingest path.** `notifydispatch.EnqueueForApplication` calls `watchzones.FindZonesContaining(app.AreaID, lat, lon)`, whose query is `WHERE c.authorityId = @authorityId AND ST_DISTANCE(c.location, @point) <= c.radiusMetres`. WatchZones is partitioned by `/userId`, so this is already a cross-partition query; the `c.authorityId` equality is a secondary, index-served prune that thins the candidate zones before the spatial test. A zone pinned to authority B is therefore never tested against an app tagged authority A.

2. **Browse path.** `watchzones.nearby` calls `applications.FindNearby(authorityCode, lat, lon, radiusMetres)`, a single-partition query scoped to the zone's one authority. It backs the create-response nearby list and the zone applications list, so both omit the neighbour side.

### The reframe that makes this cheap

Two facts change the shape of the problem:

- **Polling coverage is not the cause.** The worker's "seed" cycles poll every pollable UK authority every ~30 minutes (`polling/authorities.go`), so the neighbour's applications are already ingested. Nothing is missing from the store.
- **Authority is irrelevant to correctness.** The matching question is purely "does this point fall inside this circle." Authority was only ever a prune to keep the candidate set small. It is the prune, not a correctness axis, and it is the prune that causes the bug.

So the fix is to prune by geography instead of by authority, and the cost concern (a naive cross-partition fan-out) is avoidable.

### Why the obvious fix needs geometry we do not have

The intuitive fix is to store, per zone, the list of authorities whose territory the circle intersects, then query each of those partitions. This requires answering "which authorities does this circle intersect," which requires each authority's **boundary polygon**. We do not have them. The repo holds only a name to ID mapping (`geocoding/authority-mapping.json`) and per-authority metadata (`authorities/resources/authorities.json`), neither carrying geometry. Sourcing boundary polygons for 300+ LPAs from ONS/OS, keeping them current, and handling combined planning authorities (already a live source of bugs, see tc-zuxq) is disproportionate to the problem and fragile. It would also keep authority as the matching axis, which is the brittle thing we are trying to move away from.

### The cost objection, examined

Dropping the authority prune naively does risk ballooning RU, but only on one path, and the bounding-box prune avoids it:

- **Notify path.** Already cross-partition. The radius is a per-row field (`c.radiusMetres`), which a spatial index cannot range-prune, so deleting the authority clause without a replacement would force `ST_DISTANCE` over every zone. Replacing it with a stored bounding box (`minLat/maxLat/minLon/maxLon`) plus an exact `ST_DISTANCE` residual restores an index-served prune. The box is range-indexed for free under WatchZones' existing default `/*` indexing policy. Candidate-set size is unchanged, so there is no RU regression.
- **Browse path.** Here authority is the partition key, so going cross-partition does fan out. But the radius is the query's constant, so the Applications `/location` spatial index serves it. The query is user-initiated and low-frequency (zone create, zone open), strictly colder than the notify path that already cross-partitions. At current scale the fan-out touches few physical partitions (Cosmos co-locates many logical authority keys per physical partition until data forces a split), so the cost is small.

## Options Considered

### Option A — Authority list per zone, per-partition fan-out

Store the intersecting authorities on each zone; query each partition. **Rejected for now.** Needs boundary polygons we do not have and do not want to maintain; fragile to combined authorities; provides a targeting benefit only at scale. The "add/remove on resize" worry dissolves regardless (any geometry change recomputes the set wholesale), but the data dependency is the disqualifier.

### Option B — Pure geographic, boundary-agnostic matching (chosen for the immediate fix)

Drop authority from both match queries. Notify path: stored bounding box prune plus `ST_DISTANCE` residual. Browse path: cross-partition constant-radius `ST_DISTANCE`. No geometry, correct across borders, demotes authority to metadata, no RU regression on the hot path. Specified in GitHub #637.

### Option C — Geocoder-sampled candidate-authority set (geometry-free version of A, held in reserve)

Get the targeting benefit of Option A without owning polygons: derive a zone's candidate authorities by sampling the geocoder already used at zone-create time. Reverse-geocode the circle centre plus its bounding-box corners (or eight compass points on the circle) via postcodes.io, take the distinct `admin_district` values, map to authority IDs, dedupe. Store that set on the zone and use it to target a `WHERE c.authorityCode IN (...)` fan-out instead of a blind cross-partition query.

Over-inclusion is harmless (a couple of extra partitions). The only failure mode is a very large radius spanning a thin authority no sample point landed in, bounded by sampling more points. This keeps the optimisation geometry-free and the schema is forward-compatible with Option B (the authority list is purely additive).

## Recommendation

- **Ship Option B now** (GitHub #637): correct, cheap, no new data dependency.
- **Keep Option C in reserve** as the browse-path cost optimisation. Switch it on only when the browse cross-partition fan-out shows up materially in a cost forecast. It is additive over Option B, so there is no schema regret in deferring it.
- **Do not pursue Option A** (owning authority boundary geometry) unless a product need genuinely requires true authority boundaries, for example rendering an authority's outline on a map. Nothing currently does.

### Revisit triggers

- Browse-path cross-partition RU becomes material in the cost forecast → adopt Option C.
- A product feature needs actual authority boundary rendering → reconsider sourcing geometry (Option A territory).
- Growth in zone count or application volume changes the cross-partition economics enough to warrant targeting.

### Related notes

- Non-pinned neighbour authorities only ingest on the ~30-minute seed cycle, so a border app can notify up to ~15 minutes later than a home-authority app. Closing that gap would need adjacency or boundary data, so it is accepted for the same geometry-avoidance reason.
- Combined-authority mapping (tc-zuxq) is a separate, partition-mapping bug and is not addressed here.

## References

- [GitHub #637](https://github.com/AmyDe/town-crier/issues/637) — implementation spec for the boundary-agnostic fix.
- Beads: tc-w11n (the border-miss bug), tc-zuxq (combined-authority mapping), tc-8dud (Cosmos WatchZones audit), tc-x8w9 / tc-quqe / tc-qbq4 / tc-xj48 (spatial-index groundwork already shipped).
- [ADR 0006](../adr/0006-planit-primary-data-provider.md) — PlanIt as primary data provider / polling-based ingestion model.
- [Memo 0001](0001-cosmos-db-scaling-path.md) — Cosmos scaling path, which first noted the spatial limitation.
</content>
</invoke>
