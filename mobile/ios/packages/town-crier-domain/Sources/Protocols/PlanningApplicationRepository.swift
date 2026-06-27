/// Port for accessing planning application data.
public protocol PlanningApplicationRepository: Sendable {
  func fetchApplications(for zone: WatchZone) async throws -> [PlanningApplication]
  func fetchApplication(by id: PlanningApplicationId) async throws -> PlanningApplication

  /// Fetches a single server-sorted, server-filtered, server-paged page of a
  /// zone's applications, returning the decoded rows plus the continuation
  /// cursor for the next page (`nil` on the last page). Used by the list's
  /// infinite scroll to page the full set in the selected server sort order and
  /// filter (GH#682 slices 1 and 4). The map keeps using
  /// ``fetchApplications(for:)`` (param-less, first page).
  func fetchApplicationsPage(
    for zone: WatchZone,
    sort: ApplicationSortOrder,
    filter: ApplicationFilter,
    cursor: String?,
    limit: Int
  ) async throws -> ApplicationPage
}
