import TownCrierDomain

final class SpyPlanningApplicationRepository: PlanningApplicationRepository, @unchecked Sendable {
  private(set) var fetchApplicationsCalls: [WatchZone] = []
  var fetchApplicationsResult: Result<[PlanningApplication], Error> = .success([])

  /// Per-zone results. When set, takes precedence over `fetchApplicationsResult`.
  var fetchApplicationsByZone: [String: [PlanningApplication]] = [:]

  /// Zone IDs that should throw an error when fetched.
  var fetchApplicationsFailureZones: Set<String> = []

  func fetchApplications(for zone: WatchZone) async throws -> [PlanningApplication] {
    fetchApplicationsCalls.append(zone)
    if fetchApplicationsFailureZones.contains(zone.id.value) {
      throw DomainError.unexpected("Simulated failure for \(zone.id.value)")
    }
    if let perZone = fetchApplicationsByZone[zone.id.value] {
      return perZone
    }
    return try fetchApplicationsResult.get()
  }

  private(set) var fetchApplicationCalls: [PlanningApplicationId] = []
  var fetchApplicationResult: Result<PlanningApplication, Error> = .success(.pendingReview)

  func fetchApplication(by id: PlanningApplicationId) async throws -> PlanningApplication {
    fetchApplicationCalls.append(id)
    return try fetchApplicationResult.get()
  }
}
