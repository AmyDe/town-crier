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
  /// query params (sort + cursor + limit) the ViewModel drove for each page.
  struct RecordedPageRequest: Sendable {
    let zone: WatchZone
    let sort: ApplicationSortOrder
    let cursor: String?
    let limit: Int
  }

  private(set) var fetchApplicationsPageCalls: [RecordedPageRequest] = []

  /// Queue of pages returned by successive `fetchApplicationsPage` calls. When
  /// non-empty, each call dequeues the next page (driving multi-page tests).
  /// When exhausted or never set, the call falls back to a single page built
  /// from `fetchApplicationsByZone`/`fetchApplicationsResult` with no cursor.
  var pagedResponses: [ApplicationPage] = []

  /// When set, every paged fetch throws this error after recording the call —
  /// drives the "fetch error surfaces to the ViewModel" path.
  var fetchApplicationsPageError: Error?

  func fetchApplicationsPage(
    for zone: WatchZone,
    sort: ApplicationSortOrder,
    cursor: String?,
    limit: Int
  ) async throws -> ApplicationPage {
    fetchApplicationsPageCalls.append(
      RecordedPageRequest(zone: zone, sort: sort, cursor: cursor, limit: limit))
    await waitForGateIfNeeded()
    if let fetchApplicationsPageError {
      throw fetchApplicationsPageError
    }
    if fetchApplicationsFailureZones.contains(zone.id.value) {
      throw DomainError.unexpected("Simulated failure for \(zone.id.value)")
    }
    if !pagedResponses.isEmpty {
      return pagedResponses.removeFirst()
    }
    let apps = fetchApplicationsByZone[zone.id.value] ?? ((try? fetchApplicationsResult.get()) ?? [])
    return ApplicationPage(applications: apps, nextCursor: nil)
  }

  private(set) var fetchApplicationCalls: [PlanningApplicationId] = []
  var fetchApplicationResult: Result<PlanningApplication, Error> = .success(.pendingReview)

  func fetchApplication(by id: PlanningApplicationId) async throws -> PlanningApplication {
    fetchApplicationCalls.append(id)
    await waitForGateIfNeeded()
    return try fetchApplicationResult.get()
  }
}
