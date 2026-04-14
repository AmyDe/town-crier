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
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    let sut = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy,
      userDefaults: defaults,
      zoneSelectionKey: "test.zone"
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
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    let sut = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy,
      userDefaults: defaults,
      zoneSelectionKey: "test.zone"
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
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    let sut = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy,
      userDefaults: defaults,
      zoneSelectionKey: "test.zone"
    )

    await sut.loadApplications()

    #expect(sut.error == .networkUnavailable)
    #expect(appSpy.fetchApplicationsCalls.isEmpty)
  }

  @Test func loadApplications_withWatchZoneRepository_refreshesZonesOnEveryCall() async throws {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success([.pendingReview])
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([.cambridge])
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    let sut = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy,
      userDefaults: defaults,
      zoneSelectionKey: "test.zone"
    )

    await sut.loadApplications()
    await sut.loadApplications()

    #expect(zoneSpy.loadAllCallCount == 2)
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

  // MARK: - Zone Selection

  private func makeSUTWithZones(
    zones: [WatchZone] = [.cambridge, .london],
    applications: [PlanningApplication] = [.pendingReview],
    tier: SubscriptionTier = .free,
    persistedZoneId: String? = nil
  ) throws -> (ApplicationListViewModel, SpyPlanningApplicationRepository, SpyWatchZoneRepository, UserDefaults) {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(applications)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success(zones)
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    if let persistedZoneId {
      defaults.set(persistedZoneId, forKey: "test.zone")
    }
    let vm = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy,
      tier: tier,
      userDefaults: defaults,
      zoneSelectionKey: "test.zone"
    )
    return (vm, appSpy, zoneSpy, defaults)
  }

  @Test func loadApplications_populatesZonesFromRepository() async throws {
    let (sut, _, _, _) = try makeSUTWithZones()
    await sut.loadApplications()
    #expect(sut.zones.count == 2)
    #expect(sut.zones[0].id == WatchZone.cambridge.id)
    #expect(sut.zones[1].id == WatchZone.london.id)
  }

  @Test func loadApplications_selectsFirstZoneByDefault() async throws {
    let (sut, appSpy, _, _) = try makeSUTWithZones()
    await sut.loadApplications()
    #expect(sut.selectedZone?.id == WatchZone.cambridge.id)
    #expect(appSpy.fetchApplicationsCalls.first?.id == WatchZone.cambridge.id)
  }

  @Test func loadApplications_restoresPersistedZoneSelection() async throws {
    let (sut, appSpy, _, _) = try makeSUTWithZones(persistedZoneId: "zone-002")
    await sut.loadApplications()
    #expect(sut.selectedZone?.id == WatchZone.london.id)
    #expect(appSpy.fetchApplicationsCalls.first?.id == WatchZone.london.id)
  }

  @Test func loadApplications_fallsBackToFirstZone_whenPersistedZoneDeleted() async throws {
    let (sut, appSpy, _, _) = try makeSUTWithZones(persistedZoneId: "zone-deleted")
    await sut.loadApplications()
    #expect(sut.selectedZone?.id == WatchZone.cambridge.id)
    #expect(appSpy.fetchApplicationsCalls.first?.id == WatchZone.cambridge.id)
  }

  @Test func selectZone_fetchesApplicationsForNewZone() async throws {
    let (sut, appSpy, _, _) = try makeSUTWithZones()
    await sut.loadApplications()
    let callsBefore = appSpy.fetchApplicationsCalls.count
    await sut.selectZone(.london)
    #expect(sut.selectedZone?.id == WatchZone.london.id)
    #expect(appSpy.fetchApplicationsCalls.count == callsBefore + 1)
    #expect(appSpy.fetchApplicationsCalls.last?.id == WatchZone.london.id)
  }

  @Test func selectZone_persistsSelectionToUserDefaults() async throws {
    let (sut, _, _, defaults) = try makeSUTWithZones()
    await sut.loadApplications()
    await sut.selectZone(.london)
    #expect(defaults.string(forKey: "test.zone") == "zone-002")
  }

  @Test func selectZone_resetsStatusFilter() async throws {
    let (sut, _, _, _) = try makeSUTWithZones(tier: .personal)
    await sut.loadApplications()
    sut.selectedStatusFilter = .approved
    await sut.selectZone(.london)
    #expect(sut.selectedStatusFilter == nil)
  }

  @Test func showZonePicker_trueWhenMultipleZones() async throws {
    let (sut, _, _, _) = try makeSUTWithZones()
    await sut.loadApplications()
    #expect(sut.showZonePicker)
  }

  @Test func showZonePicker_falseWhenSingleZone() async throws {
    let (sut, _, _, _) = try makeSUTWithZones(zones: [.cambridge])
    await sut.loadApplications()
    #expect(!sut.showZonePicker)
  }

  @Test func showZonePicker_falseWhenNoZones() async throws {
    let (sut, _, _, _) = try makeSUTWithZones(zones: [])
    await sut.loadApplications()
    #expect(!sut.showZonePicker)
  }

  // MARK: - Saved Filter

  @Test func savedFilter_isNotActiveByDefault() async throws {
    let (sut, _) = try makeSUT(applications: Self.allApps)
    await sut.loadApplications()

    #expect(!sut.isSavedFilterActive)
  }

  @Test func savedFilter_canSaveAlwaysTrue() throws {
    let savedSpy = SpySavedApplicationRepository()
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success([])
    let sut = ApplicationListViewModel(
      repository: appSpy,
      zone: .cambridge,
      tier: .free,
      savedApplicationRepository: savedSpy
    )

    #expect(sut.canSave)
  }

  @Test func savedFilter_canSaveFalseWithoutRepository() throws {
    let (sut, _) = try makeSUT()

    #expect(!sut.canSave)
  }

  @Test func savedFilter_activatingFilter_showsOnlySavedApps() async throws {
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([
      SavedApplication(applicationUid: "APP-001", savedAt: Date())
    ])
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(Self.allApps)
    let sut = ApplicationListViewModel(
      repository: appSpy,
      zone: .cambridge,
      tier: .free,
      savedApplicationRepository: savedSpy
    )

    await sut.loadApplications()
    await sut.activateSavedFilter()

    #expect(sut.isSavedFilterActive)
    #expect(sut.filteredApplications.count == 1)
    #expect(sut.filteredApplications.first?.id == PlanningApplicationId("APP-001"))
  }

  @Test func savedFilter_deactivatingFilter_showsAll() async throws {
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([
      SavedApplication(applicationUid: "APP-001", savedAt: Date())
    ])
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(Self.allApps)
    let sut = ApplicationListViewModel(
      repository: appSpy,
      zone: .cambridge,
      tier: .free,
      savedApplicationRepository: savedSpy
    )

    await sut.loadApplications()
    await sut.activateSavedFilter()
    sut.deactivateSavedFilter()

    #expect(!sut.isSavedFilterActive)
    #expect(sut.filteredApplications.count == 4)
  }

  @Test func savedFilter_activating_clearsStatusFilter() async throws {
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([])
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(Self.allApps)
    let sut = ApplicationListViewModel(
      repository: appSpy,
      zone: .cambridge,
      tier: .personal,
      savedApplicationRepository: savedSpy
    )

    await sut.loadApplications()
    sut.selectedStatusFilter = .approved
    await sut.activateSavedFilter()

    #expect(sut.selectedStatusFilter == nil)
    #expect(sut.isSavedFilterActive)
  }

  @Test func savedFilter_settingStatusFilter_deactivatesSavedFilter() async throws {
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([
      SavedApplication(applicationUid: "APP-001", savedAt: Date())
    ])
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(Self.allApps)
    let sut = ApplicationListViewModel(
      repository: appSpy,
      zone: .cambridge,
      tier: .personal,
      savedApplicationRepository: savedSpy
    )

    await sut.loadApplications()
    await sut.activateSavedFilter()
    sut.selectedStatusFilter = .approved

    #expect(!sut.isSavedFilterActive)
    #expect(sut.filteredApplications.count == 1)
    #expect(sut.filteredApplications.first?.status == .approved)
  }

  @Test func savedFilter_updatesSavedUids() async throws {
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([
      SavedApplication(applicationUid: "APP-001", savedAt: Date()),
      SavedApplication(applicationUid: "APP-002", savedAt: Date()),
    ])
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(Self.allApps)
    let sut = ApplicationListViewModel(
      repository: appSpy,
      zone: .cambridge,
      tier: .free,
      savedApplicationRepository: savedSpy
    )

    await sut.loadApplications()
    await sut.activateSavedFilter()

    #expect(sut.savedApplicationUids.count == 2)
    #expect(sut.savedApplicationUids.contains("APP-001"))
    #expect(sut.savedApplicationUids.contains("APP-002"))
  }

  @Test func savedFilter_emptyState_whenNoSavedApps() async throws {
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([])
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(Self.allApps)
    let sut = ApplicationListViewModel(
      repository: appSpy,
      zone: .cambridge,
      tier: .free,
      savedApplicationRepository: savedSpy
    )

    await sut.loadApplications()
    await sut.activateSavedFilter()

    #expect(sut.isSavedFilterActive)
    #expect(sut.filteredApplications.isEmpty)
    #expect(sut.isEmpty)
  }

  @Test func savedFilter_selectZone_deactivatesSavedFilter() async throws {
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([
      SavedApplication(applicationUid: "APP-001", savedAt: Date())
    ])
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(Self.allApps)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([.cambridge, .london])
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    let sut = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy,
      tier: .free,
      userDefaults: defaults,
      zoneSelectionKey: "test.zone",
      savedApplicationRepository: savedSpy
    )

    await sut.loadApplications()
    await sut.activateSavedFilter()
    #expect(sut.isSavedFilterActive)

    await sut.selectZone(.london)

    #expect(!sut.isSavedFilterActive)
  }
}
