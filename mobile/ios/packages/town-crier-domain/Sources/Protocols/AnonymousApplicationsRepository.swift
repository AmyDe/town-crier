/// Port for the anonymous (pre-signup) map/list's data access (GH#868 Phase
/// 3): fetches planning applications near a point with no account or
/// session, backed by the public `GET /v1/applications/near-point` endpoint.
public protocol AnonymousApplicationsRepository: Sendable {
  /// Fetches one page of planning applications within `radiusMetres` of
  /// (`latitude`, `longitude`), ordered per `sort` and capped at `limit`. No
  /// clustering or infinite scroll — the anonymous browse surfaces are a
  /// deliberately reduced feature set, so a single bounded page per query is
  /// sufficient.
  func fetchNearby(
    latitude: Double,
    longitude: Double,
    radiusMetres: Double,
    limit: Int,
    sort: NearbyApplicationSortOrder
  ) async throws -> [PlanningApplication]

  /// Fetches the server-computed cluster aggregates for the anonymous map's
  /// visible viewport within `radiusMetres` of (`latitude`, `longitude`) at a
  /// given slippy zoom (GH#924 Phase 2) — the anonymous mirror of
  /// `PlanningApplicationRepository.fetchClusters`, backed by the public
  /// `GET /v1/applications/clusters` endpoint. The map renders these
  /// aggregates across the whole radius circle instead of a truncated
  /// nearest-N page, refetching on debounced pan/zoom. No `status` param —
  /// the anonymous map has no filter chips (GH#879 scoped anonymous as a
  /// deliberately reduced surface).
  func fetchClusters(
    latitude: Double,
    longitude: Double,
    radiusMetres: Double,
    viewport: MapViewport,
    zoom: Int
  ) async throws -> [AnonymousMapCluster]
}

extension AnonymousApplicationsRepository {
  /// Convenience overload preserving the pre-GH#912 call shape: defaults to
  /// `.distance`, the server's own default and the semantics the anonymous
  /// map relies on (GH#912 settled decision #5 — `near-point` keeps
  /// `distance` as default, so the map's existing `fetchNearby(...)` call
  /// sites need no change).
  public func fetchNearby(
    latitude: Double,
    longitude: Double,
    radiusMetres: Double,
    limit: Int
  ) async throws -> [PlanningApplication] {
    try await fetchNearby(
      latitude: latitude,
      longitude: longitude,
      radiusMetres: radiusMetres,
      limit: limit,
      sort: .distance)
  }
}
