import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("ApplicationListViewModel")
@MainActor
struct ApplicationListViewModelTests {
  private func makeSUT(
    applications: [PlanningApplication] = [],
    tier: SubscriptionTier = .free
  ) -> (ApplicationListViewModel, SpyPlanningApplicationRepository) {
    let spy = SpyPlanningApplicationRepository()
    spy.fetchApplicationsResult = .success(applications)
    let vm = ApplicationListViewModel(
      repository: spy,
      authority: .cambridge,
      tier: tier
    )
    return (vm, spy)
  }

  private static let allApps: [PlanningApplication] = [
    .pendingReview, .approved, .refused, .withdrawn,
  ]

  // MARK: - Loading

  @Test func loadApplications_populatesApplicationsSortedByDateDescending() async {
    let older = PlanningApplication.pendingReview  // 1_700_000_000
    let newer = PlanningApplication.approved  // 1_700_100_000
    let newest = PlanningApplication.refused  // 1_700_200_000
    let (sut, _) = makeSUT(applications: [older, newest, newer])

    await sut.loadApplications()

    #expect(sut.filteredApplications.count == 3)
    #expect(sut.filteredApplications[0].id == newest.id)
    #expect(sut.filteredApplications[1].id == newer.id)
    #expect(sut.filteredApplications[2].id == older.id)
  }

  @Test func loadApplications_setsIsLoadingFalseAfterFetch() async {
    let (sut, _) = makeSUT()

    #expect(!sut.isLoading)
    await sut.loadApplications()
    #expect(!sut.isLoading)
  }

  @Test func loadApplications_setsErrorOnFailure() async {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(sut.error == .networkUnavailable)
    #expect(sut.filteredApplications.isEmpty)
  }

  @Test func loadApplications_clearsErrorOnRetry() async {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.networkUnavailable)
    await sut.loadApplications()

    spy.fetchApplicationsResult = .success([.pendingReview])
    await sut.loadApplications()

    #expect(sut.error == nil)
    #expect(sut.filteredApplications.count == 1)
  }

  @Test func loadApplications_callsRepositoryWithAuthority() async {
    let (sut, spy) = makeSUT()

    await sut.loadApplications()

    #expect(spy.fetchApplicationsCalls.count == 1)
    #expect(spy.fetchApplicationsCalls.first?.code == "CAM")
  }

  // MARK: - Selection

  @Test func selectApplication_notifiesCallback() async {
    var selectedId: PlanningApplicationId?
    let (sut, _) = makeSUT(applications: [.pendingReview])
    sut.onApplicationSelected = { selectedId = $0 }

    sut.selectApplication(PlanningApplicationId("APP-001"))

    #expect(selectedId == PlanningApplicationId("APP-001"))
  }

  // MARK: - Status Filtering

  @Test func filterByStatus_freeTier_cannotFilter() async {
    let (sut, _) = makeSUT(applications: Self.allApps, tier: .free)
    await sut.loadApplications()

    #expect(!sut.canFilter)
    #expect(sut.filteredApplications.count == 4)
  }

  @Test func filterByStatus_personalTier_canFilter() async {
    let (sut, _) = makeSUT(applications: Self.allApps, tier: .personal)
    await sut.loadApplications()

    #expect(sut.canFilter)

    sut.selectedStatusFilter = .approved
    #expect(sut.filteredApplications.count == 1)
    #expect(sut.filteredApplications.first?.status == .approved)
  }

  @Test func filterByStatus_proTier_canFilter() async {
    let (sut, _) = makeSUT(applications: Self.allApps, tier: .pro)
    await sut.loadApplications()

    #expect(sut.canFilter)

    sut.selectedStatusFilter = .refused
    #expect(sut.filteredApplications.count == 1)
    #expect(sut.filteredApplications.first?.status == .refused)
  }

  @Test func filterByStatus_nilFilter_showsAll() async {
    let (sut, _) = makeSUT(applications: Self.allApps, tier: .personal)
    await sut.loadApplications()

    sut.selectedStatusFilter = .approved
    #expect(sut.filteredApplications.count == 1)

    sut.selectedStatusFilter = nil
    #expect(sut.filteredApplications.count == 4)
  }

  @Test func filterByStatus_noMatchingResults_returnsEmpty() async {
    let (sut, _) = makeSUT(
      applications: [.pendingReview],
      tier: .personal
    )
    await sut.loadApplications()

    sut.selectedStatusFilter = .approved
    #expect(sut.filteredApplications.isEmpty)
  }

  // MARK: - Error Classification

  @Test func isNetworkError_trueForNetworkUnavailable() async {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(sut.isNetworkError)
  }

  @Test func isNetworkError_falseForOtherErrors() async {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.unexpected("Server error"))

    await sut.loadApplications()

    #expect(!sut.isNetworkError)
  }

  @Test func isSessionExpired_trueForSessionExpired() async {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.sessionExpired)

    await sut.loadApplications()

    #expect(sut.isSessionExpired)
  }

  @Test func isSessionExpired_falseForOtherErrors() async {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(!sut.isSessionExpired)
  }

  // MARK: - Empty State

  @Test func isEmpty_trueWhenNoApplicationsLoaded() async {
    let (sut, _) = makeSUT()
    await sut.loadApplications()

    #expect(sut.isEmpty)
  }

  @Test func isEmpty_falseWhenApplicationsExist() async {
    let (sut, _) = makeSUT(applications: [.pendingReview])
    await sut.loadApplications()

    #expect(!sut.isEmpty)
  }
}
