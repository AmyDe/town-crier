/// Port for the anonymous (pre-signup) map/list's data access: fetches
/// planning applications near a point with no account or session, backed by
/// the public `GET /v1/applications/near-point` endpoint.
public protocol AnonymousApplicationsRepository: Sendable {
  /// Fetches one page of planning applications within `radiusMetres` of
  /// (`latitude`, `longitude`), ordered per `sort` and capped at `limit`.
  /// Single page; no pagination.
  func fetchNearby(
    latitude: Double,
    longitude: Double,
    radiusMetres: Double,
    limit: Int,
    sort: NearbyApplicationSortOrder
  ) async throws -> [PlanningApplication]

  /// Fetches the server-computed cluster aggregates for the anonymous map's
  /// visible viewport within `radiusMetres` of (`latitude`, `longitude`) at a
  /// given slippy zoom — the anonymous mirror of
  /// `PlanningApplicationRepository.fetchClusters`, backed by the public
  /// `GET /v1/applications/clusters` endpoint. No `status` param: the
  /// anonymous map has no filter chips.
  func fetchClusters(
    latitude: Double,
    longitude: Double,
    radiusMetres: Double,
    viewport: MapViewport,
    zoom: Int
  ) async throws -> [AnonymousMapCluster]
}

extension AnonymousApplicationsRepository {
  /// Convenience overload defaulting to `.distance`, the server's default.
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
