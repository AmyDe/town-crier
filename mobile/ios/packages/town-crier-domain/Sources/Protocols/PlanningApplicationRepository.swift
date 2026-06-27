/// Port for accessing planning application data.
public protocol PlanningApplicationRepository: Sendable {
  func fetchApplications(for zone: WatchZone) async throws -> [PlanningApplication]
  func fetchApplication(by id: PlanningApplicationId) async throws -> PlanningApplication

  /// Fetches a single server-sorted, server-filtered, server-paged page of a
  /// zone's applications, returning the decoded rows plus the continuation
  /// cursor for the next page (`nil` on the last page). The list's infinite
  /// scroll pages the full set lazily in the selected server sort order and
  /// filter (GH#682 slices 1 and 4); the map eager-drains every page to
  /// exhaustion in `distance` order for clustering (GH#682 slice 5).
  func fetchApplicationsPage(
    for zone: WatchZone,
    sort: ApplicationSortOrder,
    filter: ApplicationFilter,
    cursor: String?,
    limit: Int
  ) async throws -> ApplicationPage
}
