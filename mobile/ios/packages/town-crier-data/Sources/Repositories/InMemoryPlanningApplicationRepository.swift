import TownCrierDomain

/// An in-memory repository for development and testing.
public final class InMemoryPlanningApplicationRepository: PlanningApplicationRepository,
  @unchecked Sendable {
  private var applications: [PlanningApplication]

  public init(applications: [PlanningApplication] = []) {
    self.applications = applications
  }

  public func fetchApplications(for zone: WatchZone) async throws -> [PlanningApplication] {
    return applications.filter { app in
      guard let location = app.location else { return false }
      return zone.contains(location)
    }
  }

  public func fetchApplication(by id: PlanningApplicationId) async throws -> PlanningApplication {
    guard let application = applications.first(where: { $0.id == id }) else {
      throw DomainError.applicationNotFound(id)
    }
    return application
  }
}
