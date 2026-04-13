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
  ) throws -> (ApplicationListViewModel, SpyPlanningApplicationRepository) {
    let spy = SpyPlanningApplicationRepository()
    spy.fetchApplicationsResult = .success(applications)
    let vm = ApplicationListViewModel(
      repository: spy,
      zone: .cambridge,
      tier: tier
    )
    return (vm, spy)
  }

  private static let allApps: [PlanningApplication] = [
    .pendingReview, .approved, .refused, .withdrawn,
  ]

  // MARK: - Loading

  @Test func loadApplications_populatesApplicationsSortedByDateDescending() async throws {
    let older = PlanningApplication.pendingReview  // 1_700_000_000
    let newer = PlanningApplication.approved  // 1_700_100_000
    let newest = PlanningApplication.refused  // 1_700_200_000
    let (sut, _) = try makeSUT(applications: [older, newest, newer])

    await sut.loadApplications()

    #expect(sut.filteredApplications.count == 3)
    #expect(sut.filteredApplications[0].id == newest.id)
    #expect(sut.filteredApplications[1].id == newer.id)
    #expect(sut.filteredApplications[2].id == older.id)
  }

  @Test func loadApplications_setsIsLoadingFalseAfterFetch() async throws {
    let (sut, _) = try makeSUT()

    #expect(!sut.isLoading)
    await sut.loadApplications()
    #expect(!sut.isLoading)
  }

  @Test func loadApplications_setsErrorOnFailure() async throws {
    let (sut, spy) = try makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(sut.error == .networkUnavailable)
    #expect(sut.filteredApplications.isEmpty)
  }

  @Test func loadApplications_clearsErrorOnRetry() async throws {
    let (sut, spy) = try makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.networkUnavailable)
    await sut.loadApplications()

    spy.fetchApplicationsResult = .success([.pendingReview])
    await sut.loadApplications()

    #expect(sut.error == nil)
    #expect(sut.filteredApplications.count == 1)
  }

  @Test func loadApplications_callsRepositoryWithZone() async throws {
    let (sut, spy) = try makeSUT()

    await sut.loadApplications()

    #expect(spy.fetchApplicationsCalls.count == 1)
    #expect(spy.fetchApplicationsCalls.first?.id == WatchZone.cambridge.id)
  }

  // MARK: - Selection

  @Test func selectApplication_notifiesCallback() async throws {
    var selectedId: PlanningApplicationId?
    let (sut, _) = try makeSUT(applications: [.pendingReview])
    sut.onApplicationSelected = { selectedId = $0 }

    sut.selectApplication(PlanningApplicationId("APP-001"))

    #expect(selectedId == PlanningApplicationId("APP-001"))
  }

  // MARK: - Status Filtering

  @Test func filterByStatus_freeTier_cannotFilter() async throws {
    let (sut, _) = try makeSUT(applications: Self.allApps, tier: .free)
    await sut.loadApplications()

    #expect(!sut.canFilter)
    #expect(sut.filteredApplications.count == 4)
  }

  @Test func filterByStatus_personalTier_canFilter() async throws {
    let (sut, _) = try makeSUT(applications: Self.allApps, tier: .personal)
    await sut.loadApplications()

    #expect(sut.canFilter)

    sut.selectedStatusFilter = .approved
    #expect(sut.filteredApplications.count == 1)
    #expect(sut.filteredApplications.first?.status == .approved)
  }

  @Test func filterByStatus_proTier_canFilter() async throws {
    let (sut, _) = try makeSUT(applications: Self.allApps, tier: .pro)
    await sut.loadApplications()

    #expect(sut.canFilter)

    sut.selectedStatusFilter = .refused
    #expect(sut.filteredApplications.count == 1)
    #expect(sut.filteredApplications.first?.status == .refused)
  }

  @Test func filterByStatus_nilFilter_showsAll() async throws {
    let (sut, _) = try makeSUT(applications: Self.allApps, tier: .personal)
    await sut.loadApplications()

    sut.selectedStatusFilter = .approved
    #expect(sut.filteredApplications.count == 1)

    sut.selectedStatusFilter = nil
    #expect(sut.filteredApplications.count == 4)
  }

  @Test func filterByStatus_noMatchingResults_returnsEmpty() async throws {
    let (sut, _) = try makeSUT(
      applications: [.pendingReview],
      tier: .personal
    )
    await sut.loadApplications()

    sut.selectedStatusFilter = .approved
    #expect(sut.filteredApplications.isEmpty)
  }

  @Test func isServerError_trueForServerError() async throws {
    let (sut, spy) = try makeSUT()
    spy.fetchApplicationsResult = .failure(
      DomainError.serverError(statusCode: 500, message: nil)
    )

    await sut.loadApplications()

    #expect(sut.isServerError)
    #expect(!sut.isNetworkError)
  }

  @Test func isServerError_falseForNetworkError() async throws {
    let (sut, spy) = try makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(!sut.isServerError)
    #expect(sut.isNetworkError)
  }

  // MARK: - Error Classification

  @Test func isNetworkError_trueForNetworkUnavailable() async throws {
    let (sut, spy) = try makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(sut.isNetworkError)
  }

  @Test func isNetworkError_falseForOtherErrors() async throws {
    let (sut, spy) = try makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.unexpected("Server error"))

    await sut.loadApplications()

    #expect(!sut.isNetworkError)
  }

  @Test func isSessionExpired_trueForSessionExpired() async throws {
    let (sut, spy) = try makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.sessionExpired)

    await sut.loadApplications()

    #expect(sut.isSessionExpired)
  }

  @Test func isSessionExpired_falseForOtherErrors() async throws {
    let (sut, spy) = try makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(!sut.isSessionExpired)
  }

  // MARK: - Zone Resolution

  @Test func loadApplications_withWatchZoneRepository_resolvesFirstZoneAndFetches() async throws {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success([.pendingReview])
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([.cambridge, .london])
    let sut = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy
    )

    await sut.loadApplications()

    #expect(zoneSpy.loadAllCallCount == 1)
    #expect(appSpy.fetchApplicationsCalls.count == 1)
    #expect(appSpy.fetchApplicationsCalls.first?.id == WatchZone.cambridge.id)
    #expect(sut.filteredApplications.count == 1)
  }

  @Test func loadApplications_withWatchZoneRepository_noZones_showsEmptyNotError() async throws {
    let appSpy = SpyPlanningApplicationRepository()
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([])
    let sut = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy
    )

    await sut.loadApplications()

    #expect(sut.filteredApplications.isEmpty)
    #expect(sut.error == nil)
    #expect(!sut.isLoading)
  }

  @Test func loadApplications_withWatchZoneRepository_zoneFetchFails_setsError() async throws {
    let appSpy = SpyPlanningApplicationRepository()
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .failure(DomainError.networkUnavailable)
    let sut = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy
    )

    await sut.loadApplications()

    #expect(sut.error == .networkUnavailable)
    #expect(appSpy.fetchApplicationsCalls.isEmpty)
  }

  @Test func loadApplications_withWatchZoneRepository_cachesResolvedZoneOnRetry() async throws {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success([.pendingReview])
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([.cambridge])
    let sut = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy
    )

    await sut.loadApplications()
    await sut.loadApplications()

    #expect(zoneSpy.loadAllCallCount == 1)
    #expect(appSpy.fetchApplicationsCalls.count == 2)
  }

  // MARK: - Empty State

  @Test func isEmpty_trueWhenNoApplicationsLoaded() async throws {
    let (sut, _) = try makeSUT()
    await sut.loadApplications()

    #expect(sut.isEmpty)
  }

  @Test func isEmpty_falseWhenApplicationsExist() async throws {
    let (sut, _) = try makeSUT(applications: [.pendingReview])
    await sut.loadApplications()

    #expect(!sut.isEmpty)
  }
}
