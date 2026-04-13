import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("InMemoryPlanningApplicationRepository")
struct InMemoryPlanningApplicationRepositoryTests {
  @Test func fetchApplications_returnsApplicationsWithinZone() async throws {
    let expected = [PlanningApplication.pendingReview]
    let sut = InMemoryPlanningApplicationRepository(applications: expected)

    let result = try await sut.fetchApplications(for: WatchZone.cambridge)

    #expect(result == expected)
  }

  @Test func fetchApplications_filtersOutApplicationsOutsideZone() async throws {
    let farAwayZone = try WatchZone(
      id: WatchZoneId("zone-far"),
      name: "Far Away",
      centre: Coordinate(latitude: 0, longitude: 0),
      radiusMetres: 1
    )
    let sut = InMemoryPlanningApplicationRepository(
      applications: [.pendingReview]
    )

    let result = try await sut.fetchApplications(for: farAwayZone)

    #expect(result.isEmpty)
  }

  @Test func fetchApplication_returnsMatchingApplication() async throws {
    let sut = InMemoryPlanningApplicationRepository(
      applications: [.pendingReview]
    )

    let result = try await sut.fetchApplication(by: PlanningApplicationId("APP-001"))

    #expect(result == .pendingReview)
  }

  @Test func fetchApplication_throwsWhenNotFound() async {
    let sut = InMemoryPlanningApplicationRepository(applications: [])

    await #expect(throws: DomainError.applicationNotFound(PlanningApplicationId("MISSING"))) {
      try await sut.fetchApplication(by: PlanningApplicationId("MISSING"))
    }
  }
}
