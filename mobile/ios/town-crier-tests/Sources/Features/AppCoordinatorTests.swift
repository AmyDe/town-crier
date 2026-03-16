import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("AppCoordinator")
@MainActor
struct AppCoordinatorTests {
    private func makeSUT() -> (AppCoordinator, SpyPlanningApplicationRepository) {
        let spy = SpyPlanningApplicationRepository()
        let authSpy = SpyAuthenticationService()
        let coordinator = AppCoordinator(repository: spy, authService: authSpy)
        return (coordinator, spy)
    }

    // MARK: - Detail ViewModel Factory

    @Test func makeApplicationDetailViewModel_createsViewModelWithApplication() {
        let (sut, _) = makeSUT()
        let vm = sut.makeApplicationDetailViewModel(application: .pendingReview)

        #expect(vm.reference == "2026/0042")
        #expect(vm.address == "12 Mill Road, Cambridge, CB1 2AD")
    }

    @Test func makeApplicationDetailViewModel_dismissClearsDetailApplication() {
        let (sut, _) = makeSUT()
        sut.detailApplication = .pendingReview
        let vm = sut.makeApplicationDetailViewModel(application: .pendingReview)

        vm.dismiss()

        #expect(sut.detailApplication == nil)
    }

    // MARK: - Map selection triggers detail

    @Test func mapViewModel_onApplicationSelected_fetchesAndSetsDetail() async throws {
        let (sut, spy) = makeSUT()
        spy.fetchApplicationResult = .success(.approved)
        let mapVM = sut.makeMapViewModel(watchZone: .cambridge)

        // Simulate the map selecting an application
        mapVM.onApplicationSelected?(PlanningApplicationId("APP-002"))

        // Give the async Task time to complete
        try await Task.sleep(for: .milliseconds(50))

        #expect(sut.detailApplication == .approved)
        #expect(spy.fetchApplicationCalls == [PlanningApplicationId("APP-002")])
    }
}
