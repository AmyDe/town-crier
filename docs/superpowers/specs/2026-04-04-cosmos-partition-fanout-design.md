# Cosmos DB Partition Key Range Fan-Out Design

Date: 2026-04-04

## Problem

The `CosmosRestClient.QueryAsync` method fails with a 400 BadRequest when the Cosmos DB REST API gateway cannot serve a cross-partition query directly. This affects queries using `DISTINCT`, `GROUP BY`, and certain aggregations. The gateway returns a response with `partitionedQueryExecutionInfoVersion` in the body, which tells SDK clients to fan out to individual partition key ranges and merge results client-side. Our REST client treats this as a fatal error.

**Impact:** The polling service (`PlanItPollingService`) crashes on every 15-minute cycle because `WatchZoneActiveAuthorityProvider.GetActiveAuthorityIdsAsync()` issues `SELECT DISTINCT VALUE c.authorityId FROM c` — a cross-partition DISTINCT query. No planning applications are ingested. Dashboard metrics are empty.

## Design

### Fan-out detection in QueryAsync

When the first request in `QueryAsync` returns HTTP 400 with `partitionedQueryExecutionInfoVersion` in the body (and the query is cross-partition, i.e. `partitionKey` is null):

1. Fetch partition key ranges via `GET /dbs/{db}/colls/{coll}/pkranges`
2. Re-execute the same query for each range, setting `x-ms-documentdb-partitionkeyrangeid` header
3. Each per-range query handles its own continuation pagination
4. Concatenate all per-range results into a single list

If the 400 does NOT contain the fan-out marker, throw as usual.

### Repository post-processing

The REST client returns concatenated results. Callers handle query-specific semantics:

- `GetDistinctAuthorityIdsAsync`: add `.Distinct().ToList()` — per-range DISTINCT results may overlap across ranges
- `GetZoneCountsByAuthorityAsync`: group by authorityId and sum zone counts — per-range GROUP BY returns partial aggregates

### What doesn't change

- `ICosmosRestClient` interface — no signature changes
- Single-partition queries and simple cross-partition queries (SELECT *, WHERE) — already work
- Dashboard definition — metrics will flow once polling succeeds
