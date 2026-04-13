# Spatial Filtering for Watch Zone Applications

Date: 2026-04-13

## Problem

Both the web and iOS apps display all planning applications for an entire local authority, rather than only those within the user's watch zone radius. The syncing architecture intentionally pulls all applications per authority (to avoid overlapping jobs at scale), but the API endpoint serving the browse/map flows returns the full authority dataset with no spatial scoping.

This is also a security concern: a user with a valid JWT can call `GET /v1/applications?authorityId=` to pull all applications for any authority, accessing more data than their watch zone entitles them to.

## Decision

Replace the authority-scoped browse endpoint with a zone-scoped endpoint that uses the existing `FindNearbyAsync` Cosmos DB spatial query (`ST_DISTANCE`). Applications without coordinates are excluded from zone results.

## API Changes

### New Endpoint

`GET /v1/me/watch-zones/{zoneId}/applications`

- Authenticated. Looks up the watch zone by `zoneId` and authenticated user ID (validates ownership).
- Extracts `centre.latitude`, `centre.longitude`, `radiusMetres`, and `authorityId` from the zone.
- Calls `FindNearbyAsync(authorityCode, lat, lng, radiusMetres)` -- single-partition `ST_DISTANCE` query against the Applications container.
- Returns the same `PlanningApplicationSummary` shape clients already consume.
- Behind the same auth middleware as other `/me/` endpoints.

### Removed Endpoint

`GET /v1/applications?authorityId={id}` -- endpoint, handler (`GetApplicationsByAuthorityQueryHandler`), query class (`GetApplicationsByAuthorityQuery`), and associated tests are all deleted.

### Unchanged Endpoints

| Endpoint | Reason |
|----------|--------|
| `GET /v1/me/watch-zones` | Listing zones -- still needed |
| `GET /v1/applications/{uid}` | Single application detail view |
| `GET /v1/search?q=&authorityId=&page=` | PlanIt search -- always requires a search term, no bulk exposure |
| `GET /v1/me/application-authorities` | Used during zone creation flow (picking which authority) |

### Repository Interface

`IPlanningApplicationRepository.GetByAuthorityIdAsync` is kept for internal use (polling ingestion) but is no longer exposed via any HTTP endpoint.

## iOS Changes

### Protocol

`PlanningApplicationRepository.fetchApplications(for authority: LocalAuthority)` changes to `fetchApplications(for zone: WatchZone)` (or a `ZoneId` value type).

### APIPlanningApplicationRepository

Calls `GET /v1/me/watch-zones/{zoneId}/applications` instead of the removed authority endpoint.

### ApplicationListViewModel

Fetches by selected zone instead of selected authority. Zone picker replaces authority picker as the primary filter control.

### MapViewModel

Fetches applications for the currently displayed zone instead of all applications across all authorities. Pins now match the zone circle overlay.

### OfflineAwareRepository

Cache key changes from authority code to zone ID.

## Web Changes

### MapPort Interface

`fetchApplicationsByAuthority(authorityId)` becomes `fetchApplicationsByZone(zoneId)`.

### ApiMapAdapter

Calls `GET /v1/me/watch-zones/{zoneId}/applications`.

### useMapData Hook

Fetches all zones then fans out per zone (replacing authority fan-out). Deduplication required since zones could overlap.

### Dashboard / Applications List

Zone selector replaces authority selector as the primary filter. Zone list already available via `GET /v1/me/watch-zones`.

## What Stays Unchanged

- **Polling flow** -- still ingests by authority from PlanIt, still does reverse spatial matching (`FindZonesContainingAsync`) for push notifications.
- **Search flow** -- still queries PlanIt with a required search term, caches results in Cosmos.
- **Zone CRUD** -- create, list, delete watch zones unchanged.

## Design Decisions

- **Applications without coordinates are excluded.** Not all PlanIt applications have lat/long. These are dropped from zone results since the whole point is geographic relevance. They remain findable via search.
- **Per-zone endpoint (not aggregated).** The UI already navigates by zone. A single zone query maps to one `FindNearbyAsync` call (one partition key, one `ST_DISTANCE`). Clients can aggregate across zones if needed.
- **Authority endpoint removed (not deprecated).** Keeping it would leave the security hole open. There is no legitimate client use case for it once zone-scoped fetching exists.
