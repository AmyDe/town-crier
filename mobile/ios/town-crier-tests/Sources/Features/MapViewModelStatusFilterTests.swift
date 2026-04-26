import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("MapViewModel — Status Filtering")
@MainActor
struct MapViewModelStatusFilterTests {
  private static let allApps: [PlanningApplication] = [
    .pendingReview, .permitted, .rejected, .withdrawn,
  ]

  private func makeSUT(
    applications: [PlanningApplication] = [],
    watchZones: [WatchZone] = [.cambridge],
    tier: SubscriptionTier = .free
  ) -> (MapViewModel, SpyPlanningApplicationRepository, SpyWatchZoneRepository) {
    let spy = SpyPlanningApplicationRepository()
    spy.fetchApplicationsResult = .success(applications)
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success(watchZones)
    let vm = MapViewModel(
      repository: spy,
      watchZoneRepository: watchZoneSpy,
      tier: tier
    )
    return (vm, spy, watchZoneSpy)
  }

  // MARK: - canFilter

  @Test func canFilter_freeTier_returnsFalse() {
    let (sut, _, _) = makeSUT(tier: .free)
    #expect(!sut.canFilter)
  }

  @Test func canFilter_personalTier_returnsTrue() {
    let (sut, _, _) = makeSUT(tier: .personal)
    #expect(sut.canFilter)
  }

  @Test func canFilter_proTier_returnsTrue() {
    let (sut, _, _) = makeSUT(tier: .pro)
    #expect(sut.canFilter)
  }

  // MARK: - filteredAnnotations

  @Test func filteredAnnotations_freeTier_showsAll() async {
    let (sut, _, _) = makeSUT(applications: Self.allApps, tier: .free)
    await sut.loadApplications()

    sut.selectedStatusFilter = .permitted
    #expect(sut.filteredAnnotations.count == 4)
  }

  @Test func filteredAnnotations_personalTier_filtersApproved() async {
    let (sut, _, _) = makeSUT(applications: Self.allApps, tier: .personal)
    await sut.loadApplications()

    sut.selectedStatusFilter = .permitted
    #expect(sut.filteredAnnotations.count == 1)
    #expect(sut.filteredAnnotations.first?.status == .permitted)
  }

  @Test func filteredAnnotations_nilFilter_showsAll() async {
    let (sut, _, _) = makeSUT(applications: Self.allApps, tier: .personal)
    await sut.loadApplications()

    sut.selectedStatusFilter = nil
    #expect(sut.filteredAnnotations.count == 4)
  }

  @Test func filteredAnnotations_noMatches_returnsEmpty() async {
    let (sut, _, _) = makeSUT(
      applications: [.pendingReview],
      tier: .personal
    )
    await sut.loadApplications()

    sut.selectedStatusFilter = .permitted
    #expect(sut.filteredAnnotations.isEmpty)
  }

  // MARK: - Zone change resets filter

  @Test func selectZone_resetsStatusFilter() async throws {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(Self.allApps)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([.cambridge, .london])
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    let sut = MapViewModel(
      repository: appSpy,
      watchZoneRepository: zoneSpy,
      tier: .personal,
      userDefaults: defaults,
      zoneSelectionKey: "test.zone"
    )
    await sut.loadApplications()
    sut.selectedStatusFilter = .permitted
    #expect(sut.selectedStatusFilter == .permitted)

    await sut.selectZone(.london)

    #expect(sut.selectedStatusFilter == nil)
  }

  // MARK: - Saved Filter

  @Test func savedFilter_isNotActiveByDefault() async {
    let (sut, _, _) = makeSUT(applications: Self.allApps)
    await sut.loadApplications()

    #expect(!sut.isSavedFilterActive)
  }

  @Test func savedFilter_canSaveTrue_whenRepositoryProvided() {
    let savedSpy = SpySavedApplicationRepository()
    let appSpy = SpyPlanningApplicationRepository()
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([.cambridge])
    let sut = MapViewModel(
      repository: appSpy,
      watchZoneRepository: zoneSpy,
      savedApplicationRepository: savedSpy
    )

    #expect(sut.canSave)
  }

  @Test func savedFilter_canSaveFalse_withoutRepository() {
    let (sut, _, _) = makeSUT()

    #expect(!sut.canSave)
  }

  @Test func savedFilter_activating_showsOnlySavedAnnotations() async {
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([
      SavedApplication(applicationUid: "APP-001", savedAt: Date())
    ])
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(Self.allApps)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([.cambridge])
    let sut = MapViewModel(
      repository: appSpy,
      watchZoneRepository: zoneSpy,
      savedApplicationRepository: savedSpy
    )

    await sut.loadApplications()
    await sut.activateSavedFilter()

    #expect(sut.isSavedFilterActive)
    #expect(sut.filteredAnnotations.count == 1)
    #expect(sut.filteredAnnotations.first?.applicationId == PlanningApplicationId("APP-001"))
  }

  @Test func savedFilter_deactivating_showsAll() async {
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([
      SavedApplication(applicationUid: "APP-001", savedAt: Date())
    ])
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(Self.allApps)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([.cambridge])
    let sut = MapViewModel(
      repository: appSpy,
      watchZoneRepository: zoneSpy,
      savedApplicationRepository: savedSpy
    )

    await sut.loadApplications()
    await sut.activateSavedFilter()
    sut.deactivateSavedFilter()

    #expect(!sut.isSavedFilterActive)
    #expect(sut.filteredAnnotations.count == 4)
  }

  @Test func savedFilter_activating_clearsStatusFilter() async {
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([])
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(Self.allApps)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([.cambridge])
    let sut = MapViewModel(
      repository: appSpy,
      watchZoneRepository: zoneSpy,
      tier: .personal,
      savedApplicationRepository: savedSpy
    )

    await sut.loadApplications()
    sut.selectedStatusFilter = .permitted
    await sut.activateSavedFilter()

    #expect(sut.selectedStatusFilter == nil)
    #expect(sut.isSavedFilterActive)
  }

  @Test func savedFilter_settingStatusFilter_deactivatesSavedFilter() async {
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([
      SavedApplication(applicationUid: "APP-001", savedAt: Date())
    ])
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(Self.allApps)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([.cambridge])
    let sut = MapViewModel(
      repository: appSpy,
      watchZoneRepository: zoneSpy,
      tier: .personal,
      savedApplicationRepository: savedSpy
    )

    await sut.loadApplications()
    await sut.activateSavedFilter()
    sut.selectedStatusFilter = .permitted

    #expect(!sut.isSavedFilterActive)
    #expect(sut.filteredAnnotations.count == 1)
    #expect(sut.filteredAnnotations.first?.status == .permitted)
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
    let sut = MapViewModel(
      repository: appSpy,
      watchZoneRepository: zoneSpy,
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
