import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("InMemoryPlanningApplicationRepository")
struct InMemoryPlanningApplicationRepositoryTests {
    @Test func fetchApplications_returnsMatchingAuthority() async throws {
        let expected = [PlanningApplication.pendingReview]
        let sut = InMemoryPlanningApplicationRepository(applications: expected)

        let result = try await sut.fetchApplications(for: .cambridge)

        #expect(result == expected)
    }

    @Test func fetchApplications_filtersOutOtherAuthorities() async throws {
        let other = LocalAuthority(code: "OXF", name: "Oxford")
        let sut = InMemoryPlanningApplicationRepository(
            applications: [.pendingReview]
        )

        let result = try await sut.fetchApplications(for: other)

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
