/// Port for accessing planning application data.
public protocol PlanningApplicationRepository: Sendable {
  func fetchApplications(for zone: WatchZone) async throws -> [PlanningApplication]
  func fetchApplication(by id: PlanningApplicationId) async throws -> PlanningApplication
}
