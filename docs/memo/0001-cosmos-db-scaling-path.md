# 0001. Cosmos DB Scaling Path

Date: 2026-04-02

## Status

Open

## Question

If Town Crier replaces PlanIt polling with its own web scraping infrastructure, will Cosmos DB serverless remain the right database choice as data volume grows?

## Analysis

Cosmos DB serverless is ideal at current scale — near-zero cost, pay-per-request, and the change feed powers the entire notification pipeline (ADR 0009). However, several pressure points emerge with sustained scraping writes:

**Burst ceiling**: Serverless caps at 1000 RU/s. Bulk upserts across 417 LPAs from a scraper will hit this, requiring retry/backoff logic and slowing ingestion.

**Write cost at volume**: Each 1KB upsert costs ~5.7 RUs. At 50K documents/day (modest for 417 councils), that's ~285K RUs/day on writes alone — no longer near-zero.

**Storage cap**: Serverless containers max at 1TB. Building historical depth (application lifecycle over months/years) will eventually require TTL or archival strategy.

**Spatial query cost**: `ST_DISTANCE` queries consume 10-50+ RUs depending on result set. The change feed processor running `FindZonesContainingAsync` on every upsert multiplies this by write volume.

## Options Considered

### 1. Stay on Cosmos DB Serverless (current)

Best for: pre-revenue, low traffic, PlanIt polling at 15-min intervals.

- **Pro**: Zero base cost, change feed is free infrastructure, existing architecture works.
- **Con**: Burst limit, per-RU cost scales linearly with scraping volume.

### 2. Cosmos DB Provisioned Throughput (autoscale)

Switches from per-request to reserved capacity with autoscale floor.

- **Pro**: Higher throughput ceiling, more predictable costs at sustained load.
- **Con**: Minimum cost even at idle (~$24/month for 400 RU/s autoscale), same spatial limitations.

### 3. PostgreSQL + PostGIS

The industry standard for spatial data. Azure Database for PostgreSQL Flexible Server (B1ms) at ~$13/month.

- **Pro**: Full spatial indexing (R-tree/GiST), polygonal watch zones (not just circles), spatial joins, no per-query cost, PostGIS is vastly more expressive than Cosmos spatial.
- **Con**: Loses Cosmos change feed — notification pipeline (ADR 0009) needs rearchitecting to outbox pattern, pg_notify, or a message queue. Always-on base cost even at zero traffic.

### 4. Hybrid (PostgreSQL for applications, Cosmos for user data)

Applications + WatchZones in PostgreSQL (spatial-heavy, write-heavy from scraping). User profiles, notifications, device registrations stay in Cosmos (low volume, partition-per-user is ideal).

- **Pro**: Best-of-both-worlds for each workload shape.
- **Con**: Two databases to operate, more infrastructure complexity for a solo developer.

## Recommendation

**Stay on Cosmos DB serverless for now.** The economics are unbeatable pre-revenue, and the change feed is load-bearing infrastructure.

**When to revisit:** If/when own scraping infrastructure is built and write volume causes either (a) RU costs exceeding ~$15/month consistently, or (b) 429 throttling from the burst limit. At that point, PostgreSQL + PostGIS is the likely migration target — the hexagonal architecture (repository interfaces) means the domain and application layers don't change. The hardest part is replacing the change feed processor with an outbox pattern or pg_notify.

PostGIS also unlocks polygonal watch zones (drawing around specific streets/neighbourhoods rather than just circles), which is a meaningful product improvement.
