# 0033. Server-authoritative watch-zone application querying

Date: 2026-06-28

## Status

Accepted

See also [ADR 0032](0032-consolidate-datastore-on-postgres-postgis.md) (the Postgres + PostGIS move that made this possible) and [ADR 0014](0014-ios-offline-first-architecture.md) (the iOS offline cache this querying model interacts with).

## Context

A watch zone is a circle (centre plus radius), and the application list behind it is the app's busiest read surface — browsed as a list, sorted, filtered by status and unread state, and rendered on a map. The original design pushed all of this to the client: the iOS app fetched a zone's applications in bulk and did the sort, the status/unread filtering, the unread count, and the map clustering on-device. That worked only while the client could plausibly hold the whole zone.

Two things broke that assumption at the same time:

- **Zones are large.** A 2 km London zone already returns ~1,364 applications and a 10 km zone tens of thousands. A browse page cap had to be introduced (#641/#642), after which the client no longer holds the whole zone, so any client-side sort, filter, or count is computed over a partial set and is silently wrong.
- **Cosmos could not help.** Under the `/authorityCode` partition key the Cosmos Gateway refused cross-partition `ORDER BY` and aggregates (`BadRequest` substatus 1004 on a `COUNT`), so the server could not take over sorting or counting either. The data model fought the query shape (see [memo 0010](../memo/0010-cosmos-partition-strategy-and-postgres-postgis.md)).

Consolidating on Postgres + PostGIS ([ADR 0032](0032-consolidate-datastore-on-postgres-postgis.md)) removed the database constraint: indexed `ORDER BY`, accurate `COUNT(*)`, `ST_DWithin` radius filtering, and spatial aggregation are all ordinary SQL. That made it both possible and necessary to move the query authority from the client to the server, so every client (iOS today, web next — #701) sees the same correct, paged, filtered, clustered view. The work landed as epic [#682](https://github.com/AmyDe/town-crier/issues/682) (server-side sort/filter and keyset paging, slices 1–4) and [#698](https://github.com/AmyDe/town-crier/issues/698) (server-side map clustering, which superseded the slice-5 client-drain approach).

## Decision

**Watch-zone application reads are server-authoritative.** The API owns sort, filter, paging, and clustering; clients send intent and render what they receive. Three mechanisms implement this:

1. **Keyset (cursor) pagination.** `GET /v1/me/watch-zones/{zoneId}/applications` returns a bounded page plus an opaque base64url `X-Next-Cursor` header that encodes the sort position. Paging is keyset, not offset, so ordering stays deterministic while rows change underneath it. The cursor embeds the sort and filter it was issued under; a request whose sort or filter disagrees with its cursor is rejected with `400` (`ErrCursorSortMismatch` / `ErrCursorFilterMismatch`) rather than silently resetting to a different result set.

2. **Server-side sort and filter.** Sort modes (`app_state` ASC, `start_date` DESC) and filters (status by `app_state`, unread via the notification-state read watermark) are applied in SQL. The server is the single authority for ordering and for the unread count, so every client agrees regardless of how much of the zone it has fetched.

3. **Server-side map clustering.** `GET /v1/me/watch-zones/{zoneId}/applications/clusters` aggregates with PostGIS `ST_SnapToGrid` at a grid size derived from the requested zoom level, clipped to the visible viewport (`ST_MakeEnvelope`) and bounded to the zone circle (`ST_DWithin`). Each cell returns its centroid, member count, and per-status breakdown; single-member cells additionally carry `{authority, name}` so a tap goes straight to the summary. Output is capped at 1000 densest-first cells to bound the payload.

iOS consumes all three: filter chips drive the server filter, infinite scroll follows the cursor, and `ClusteredMapView` renders the `/clusters` response (replacing client-side `MKClusterAnnotation`). The web map will consume the same endpoints (#701), so there is one querying contract across platforms.

## Consequences

### Easier

- **Correctness is independent of zone size.** Sort, status/unread filtering, and counts are computed over the true result set in the database, so they are right for a 10-application village zone and a 20,000-application city zone alike.
- **The map scales to dense zones.** Grid aggregation returns at most ~1000 cells instead of tens of thousands of pins, so a city-centre zone renders without flooding the client.
- **Thin, interchangeable clients.** A client needs only to send sort/filter/viewport intent and render the response, which is why the same endpoints serve iOS and the forthcoming web map without re-implementing the logic twice.
- **One source of truth.** The unread watermark, the ordering, and the cluster counts live in one place, so platforms cannot disagree.

### Harder

- **The cursor contract is strict by design.** A client that changes sort or filter must start a fresh page; reusing an old cursor returns `400`. This prevents silent, confusing result-set resets but means clients must handle the mismatch explicitly.
- **Clustering is coupled to viewport and zoom.** The cluster set is only valid for the bounds and zoom it was requested at, so the client must refetch on pan/zoom (debounced ~250 ms on iOS) rather than reusing a single download.
- **Paging bypasses the offline cache.** Paginated pages always hit the network, so the iOS offline cache ([ADR 0014](0014-ios-offline-first-architecture.md)) covers the first view of a zone but not deep scrolling — an accepted trade-off for correct, server-ordered paging.
