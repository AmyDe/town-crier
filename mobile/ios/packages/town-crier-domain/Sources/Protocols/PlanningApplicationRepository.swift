/// Port for accessing planning application data.
public protocol PlanningApplicationRepository: Sendable {
  func fetchApplications(for zone: WatchZone) async throws -> [PlanningApplication]
  func fetchApplication(by id: PlanningApplicationId) async throws -> PlanningApplication

  /// Fetches a single application by its public share identity — the API-emitted
  /// authority slug plus the full area-prefixed PlanIt ref (slashes preserved).
  /// Backs inbound Universal Link resolution for `/a/{authoritySlug}/{ref...}`
  /// via the anonymous by-slug read. The slug comes from the
  /// URL/API; iOS never computes it.
  func fetchApplication(bySlug authoritySlug: String, ref: String) async throws
    -> PlanningApplication

  /// Fetches a single server-sorted, server-filtered, server-paged page of a
  /// zone's applications, returning the decoded rows plus the continuation
  /// cursor for the next page (`nil` on the last page).
  func fetchApplicationsPage(
    for zone: WatchZone,
    sort: ApplicationSortOrder,
    filter: ApplicationFilter,
    cursor: String?,
    limit: Int
  ) async throws -> ApplicationPage

  /// Fetches the server-computed cluster aggregates for the visible map rect of a
  /// zone at a given slippy zoom. The map renders these tens of
  /// aggregates instead of eager-draining every application, refetching on
  /// debounced region change. `filter` restricts the aggregation server-side —
  /// only `.status(_)` is meaningful here (it sends `status=`); `.all` and
  /// `.unread` send no status param, as the map has no unread filter.
  func fetchClusters(
    for zone: WatchZone,
    viewport: MapViewport,
    zoom: Int,
    filter: ApplicationFilter
  ) async throws -> [MapCluster]
}
