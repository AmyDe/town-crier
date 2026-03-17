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
        let subscriptionSpy = SpySubscriptionService()
        let coordinator = AppCoordinator(
            repository: spy,
            authService: authSpy,
            subscriptionService: subscriptionSpy
        )
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

    // MARK: - Subscription ViewModel Factory

    @Test func makeSubscriptionViewModel_createsViewModel() {
        let (sut, _) = makeSUT()
        let vm = sut.makeSubscriptionViewModel()

        #expect(vm.products.isEmpty)
        #expect(!vm.isLoading)
    }

    // MARK: - Application List Factory

    @Test func makeApplicationListViewModel_createsViewModelWithAuthority() async {
        let (sut, spy) = makeSUT()
        spy.fetchApplicationsResult = .success([.pendingReview])
        let vm = sut.makeApplicationListViewModel(authority: .cambridge)

        await vm.loadApplications()

        #expect(spy.fetchApplicationsCalls.first?.code == "CAM")
    }

    @Test func applicationListViewModel_onApplicationSelected_fetchesAndSetsDetail() async throws {
        let (sut, spy) = makeSUT()
        spy.fetchApplicationResult = .success(.approved)
        let vm = sut.makeApplicationListViewModel(authority: .cambridge)

        vm.onApplicationSelected?(PlanningApplicationId("APP-002"))

        try await Task.sleep(for: .milliseconds(50))

        #expect(sut.detailApplication == .approved)
        #expect(spy.fetchApplicationCalls == [PlanningApplicationId("APP-002")])
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
