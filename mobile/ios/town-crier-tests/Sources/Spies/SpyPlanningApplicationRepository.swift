import TownCrierDomain

final class SpyPlanningApplicationRepository: PlanningApplicationRepository, @unchecked Sendable {
  private(set) var fetchApplicationsCalls: [LocalAuthority] = []
  var fetchApplicationsResult: Result<[PlanningApplication], Error> = .success([])

  /// Per-authority results. When set, takes precedence over `fetchApplicationsResult`.
  var fetchApplicationsByAuthority: [String: [PlanningApplication]] = [:]

  /// Authority codes that should throw an error when fetched.
  var fetchApplicationsFailureAuthorities: Set<String> = []

  func fetchApplications(for authority: LocalAuthority) async throws -> [PlanningApplication] {
    fetchApplicationsCalls.append(authority)
    if fetchApplicationsFailureAuthorities.contains(authority.code) {
      throw DomainError.unexpected("Simulated failure for \(authority.code)")
    }
    if let perAuthority = fetchApplicationsByAuthority[authority.code] {
      return perAuthority
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
