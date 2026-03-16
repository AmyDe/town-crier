# 0008. Cosmos DB Data Model & Partition Strategy

Date: 2026-03-16

## Status

Accepted

## Context

Town Crier uses Azure Cosmos DB (Serverless) as its primary data store. Cosmos DB requires upfront decisions on container boundaries, partition keys, and indexing policies that are difficult to change later. The application has four distinct data entities — users, watch zones, planning applications, and notifications — each with different access patterns, write volumes, and query requirements.

Key access patterns to optimise for:

1. **Application ingestion**: bulk upserts from PlanIt polling, scoped by local planning authority
2. **Spatial matching**: ST_DISTANCE queries to find applications within a user's watch zone radius
3. **Application detail**: point reads for a single application (from deep links, notification taps, list selection)
4. **User profile**: point reads by user ID after authentication
5. **Watch zone listing**: all zones belonging to a user
6. **Notification feed**: per-user, reverse-chronological

## Decision

### Containers & Partition Keys

| Container | Partition Key | Rationale |
|-----------|--------------|-----------|
| `Applications` | `/authorityCode` | Aligns with PlanIt polling granularity (per-authority). ~417 LPAs give good write distribution. Spatial queries (ST_DISTANCE) work cross-partition. Application detail reads include authority code in the API route to enable single-partition point reads |
| `Users` | `/id` | Point reads by user ID after Auth0 login. Low volume, simple access pattern |
| `WatchZones` | `/userId` | Always queried per-user ("give me this user's zones"). Spatial matching (change feed processor finding which zones an application falls within) is a background job that can tolerate cross-partition fan-out across the moderate number of active zones |
| `Notifications` | `/userId` | Feed is always per-user. Supports reverse-chronological queries within a single partition |

### API Route Design

Application detail endpoints include the authority code to enable single-partition point reads:

```
GET /v1/authorities/{authorityCode}/applications/{applicationId}
```

This avoids cross-partition point reads on the Applications container.

### Indexing Policies

**Applications container:**
- Spatial index on `/location` (Point type) for ST_DISTANCE queries in watch zone matching
- Composite index on `/authorityCode` + `/lastDifferent` (descending) for polling change detection queries
- Include `/status`, `/applicationType`, `/decisionDate` in range indexes for filtering

**Other containers:**
- Default indexing policy (all properties indexed). Low document volumes don't justify custom tuning at this stage.

### TTL Policies

| Container | TTL | Rationale |
|-----------|-----|-----------|
| `Applications` | Configurable, default 2 years past decision date | Bounds storage costs as data accumulates. Archived data available via PlanIt if needed |
| `Notifications` | 90 days | Notification history has diminishing value. Keeps per-user partitions lean |
| `Users` | None | Retained until account deletion |
| `WatchZones` | None | Retained until user deletes zone or account |

### Change Feed

The Applications container change feed is consumed by the notification processor (see ADR 0009). A `Leases` container in the same database supports change feed checkpointing.

### Unique Key Policies

| Container | Unique Key | Purpose |
|-----------|-----------|---------|
| `Applications` | `/planitName` | Idempotency key for upserts. PlanIt `name` field (`{area_name}/{uid}`) is globally unique |
| `WatchZones` | `/userId`, `/name` | Prevent duplicate zone names per user |

## Consequences

- **Application detail requires authority code in the route.** All clients (iOS app, deep links, notifications) must carry the authority code alongside the application ID. This is a minor data-passing overhead in exchange for efficient point reads.
- **Spatial queries on Applications are cross-partition.** Acceptable because spatial matching runs in a background change feed processor, not in user-facing request paths. Users browse applications via pre-matched results in their notification feed or zone-scoped list (which queries by authority code partition).
- **Separate WatchZones container** means the notification processor can query all active zones without loading user profile data. Adds one container to manage but keeps concerns cleanly separated.
- **TTL on Applications** means very old applications disappear from the local cache. This is acceptable — PlanIt retains historical data and the app is focused on recent/active applications.
