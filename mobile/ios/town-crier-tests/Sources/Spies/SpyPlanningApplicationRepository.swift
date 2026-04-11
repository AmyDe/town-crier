import TownCrierDomain

final class SpyPlanningApplicationRepository: PlanningApplicationRepository, @unchecked Sendable {
  private(set) var fetchApplicationsCalls: [LocalAuthority] = []
  var fetchApplicationsResult: Result<[PlanningApplication], Error> = .success([])

  func fetchApplications(for authority: LocalAuthority) async throws -> [PlanningApplication] {
    fetchApplicationsCalls.append(authority)
    return try fetchApplicationsResult.get()
  }

  private(set) var fetchApplicationCalls: [PlanningApplicationId] = []
  var fetchApplicationResult: Result<PlanningApplication, Error> = .success(.pendingReview)

  func fetchApplication(by id: PlanningApplicationId) async throws -> PlanningApplication {
    fetchApplicationCalls.append(id)
    return try fetchApplicationResult.get()
  }
}
