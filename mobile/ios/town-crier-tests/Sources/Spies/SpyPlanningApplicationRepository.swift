import TownCrierDomain

final class SpyPlanningApplicationRepository: PlanningApplicationRepository, @unchecked Sendable {
  private(set) var fetchApplicationsCalls: [WatchZone] = []
  var fetchApplicationsResult: Result<[PlanningApplication], Error> = .success([])

  /// Per-zone results. When set, takes precedence over `fetchApplicationsResult`.
  var fetchApplicationsByZone: [String: [PlanningApplication]] = [:]

  /// Zone IDs that should throw an error when fetched.
  var fetchApplicationsFailureZones: Set<String> = []

  /// Optional gate that holds every fetch open until ``releaseGate()`` is
  /// called. Lets a test start a fetch, observe the in-flight load, issue a
  /// second concurrent call, then release both — proving (or disproving) the
  /// ViewModel's re-entrancy guard (bd tc-eum5).
  ///
  /// Implemented as a cooperatively-polled flag rather than a continuation so
  /// it can never leak or double-resume a `CheckedContinuation` when a test
  /// holds multiple fetches open at once.
  private var isGateClosed = false

  func enableGate() {
    isGateClosed = true
  }

  func releaseGate() {
    isGateClosed = false
  }

  private func waitForGateIfNeeded() async {
    while isGateClosed {
      await Task.yield()
    }
  }

  func fetchApplications(for zone: WatchZone) async throws -> [PlanningApplication] {
    fetchApplicationsCalls.append(zone)
    await waitForGateIfNeeded()
    if fetchApplicationsFailureZones.contains(zone.id.value) {
      throw DomainError.unexpected("Simulated failure for \(zone.id.value)")
    }
    if let perZone = fetchApplicationsByZone[zone.id.value] {
      return perZone
    }
    return try fetchApplicationsResult.get()
  }

  // MARK: - Paged fetch (GH#682)

  /// A recorded `fetchApplicationsPage` invocation — lets tests assert the exact
  /// query params (sort + filter + cursor + limit) the ViewModel drove for each
  /// page.
  struct RecordedPageRequest: Sendable {
    let zone: WatchZone
    let sort: ApplicationSortOrder
    let filter: ApplicationFilter
    let cursor: String?
    let limit: Int
  }

  private(set) var fetchApplicationsPageCalls: [RecordedPageRequest] = []

  /// Queue of pages returned by successive `fetchApplicationsPage` calls. When
  /// non-empty, each call dequeues the next page (driving multi-page tests).
  /// When exhausted or never set, the call falls back to a single page built
  /// from `fetchApplicationsByZone`/`fetchApplicationsResult` with no cursor.
  var pagedResponses: [ApplicationPage] = []

  /// Per-call results for `fetchApplicationsPage`, dequeued one at a time. When
  /// non-empty this takes precedence over `pagedResponses` and lets a test
  /// sequence a mid-stream failure — e.g. page 1 succeeds, page 2 throws — to
  /// prove the map's eager drain discards partial results (GH#682 slice 5).
  var pagedResults: [Result<ApplicationPage, Error>] = []

  /// When set, every paged fetch throws this error after recording the call —
  /// drives the "fetch error surfaces to the ViewModel" path.
  var fetchApplicationsPageError: Error?

  func fetchApplicationsPage(
    for zone: WatchZone,
    sort: ApplicationSortOrder,
    filter: ApplicationFilter,
    cursor: String?,
    limit: Int
  ) async throws -> ApplicationPage {
    fetchApplicationsPageCalls.append(
      RecordedPageRequest(zone: zone, sort: sort, filter: filter, cursor: cursor, limit: limit))
    await waitForGateIfNeeded()
    if !pagedResults.isEmpty {
      return try pagedResults.removeFirst().get()
    }
    if let fetchApplicationsPageError {
      throw fetchApplicationsPageError
    }
    if fetchApplicationsFailureZones.contains(zone.id.value) {
      throw DomainError.unexpected("Simulated failure for \(zone.id.value)")
    }
    if !pagedResponses.isEmpty {
      return pagedResponses.removeFirst()
    }
    let apps =
      fetchApplicationsByZone[zone.id.value] ?? ((try? fetchApplicationsResult.get()) ?? [])
    return ApplicationPage(applications: apps, nextCursor: nil)
  }

  private(set) var fetchApplicationCalls: [PlanningApplicationId] = []
  var fetchApplicationResult: Result<PlanningApplication, Error> = .success(.pendingReview)

  func fetchApplication(by id: PlanningApplicationId) async throws -> PlanningApplication {
    fetchApplicationCalls.append(id)
    await waitForGateIfNeeded()
    return try fetchApplicationResult.get()
  }

  // MARK: - Cluster fetch (GH#698)

  /// A recorded `fetchClusters` invocation — lets tests assert the exact
  /// viewport, zoom, and filter the map drove for the current map rect.
  struct RecordedClusterRequest: Sendable {
    let zone: WatchZone
    let viewport: MapViewport
    let zoom: Int
    let filter: ApplicationFilter
  }

  private(set) var fetchClustersCalls: [RecordedClusterRequest] = []
  var fetchClustersResult: Result<[MapCluster], Error> = .success([])

  func fetchClusters(
    for zone: WatchZone,
    viewport: MapViewport,
    zoom: Int,
    filter: ApplicationFilter
  ) async throws -> [MapCluster] {
    fetchClustersCalls.append(
      RecordedClusterRequest(zone: zone, viewport: viewport, zoom: zoom, filter: filter))
    await waitForGateIfNeeded()
    return try fetchClustersResult.get()
  }
}
