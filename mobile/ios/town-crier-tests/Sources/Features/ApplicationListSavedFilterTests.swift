import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("ApplicationListViewModel -- Saved Filter")
@MainActor
struct ApplicationListSavedFilterTests {
  private static let allApps: [PlanningApplication] = [
    .pendingReview, .approved, .refused, .withdrawn,
  ]

  // MARK: - Initial State

  @Test func savedFilter_isNotActiveByDefault() async throws {
    let spy = SpyPlanningApplicationRepository()
    spy.fetchApplicationsResult = .success(Self.allApps)
    let sut = ApplicationListViewModel(
      repository: spy,
      zone: .cambridge,
      tier: .free
    )
    await sut.loadApplications()

    #expect(!sut.isSavedFilterActive)
  }

  @Test func savedFilter_canSaveTrue_whenRepositoryProvided() {
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

  @Test func savedFilter_canSaveFalse_withoutRepository() {
    let spy = SpyPlanningApplicationRepository()
    let sut = ApplicationListViewModel(
      repository: spy,
      zone: .cambridge,
      tier: .free
    )

    #expect(!sut.canSave)
  }

  // MARK: - Activating / Deactivating

  @Test func savedFilter_activating_showsOnlySavedApps() async {
    let sut = makeSUT(
      savedUids: ["APP-001"],
      applications: Self.allApps
    )

    await sut.loadApplications()
    await sut.activateSavedFilter()

    #expect(sut.isSavedFilterActive)
    #expect(sut.filteredApplications.count == 1)
    #expect(sut.filteredApplications.first?.id == PlanningApplicationId("APP-001"))
  }

  @Test func savedFilter_deactivating_showsAll() async {
    let sut = makeSUT(
      savedUids: ["APP-001"],
      applications: Self.allApps
    )

    await sut.loadApplications()
    await sut.activateSavedFilter()
    sut.deactivateSavedFilter()

    #expect(!sut.isSavedFilterActive)
    #expect(sut.filteredApplications.count == 4)
  }

  // MARK: - Mutual Exclusion

  @Test func savedFilter_activating_clearsStatusFilter() async {
    let sut = makeSUT(
      savedUids: [],
      applications: Self.allApps,
      tier: .personal
    )

    await sut.loadApplications()
    sut.selectedStatusFilter = .approved
    await sut.activateSavedFilter()

    #expect(sut.selectedStatusFilter == nil)
    #expect(sut.isSavedFilterActive)
  }

  @Test func savedFilter_settingStatusFilter_deactivatesSavedFilter() async {
    let sut = makeSUT(
      savedUids: ["APP-001"],
      applications: Self.allApps,
      tier: .personal
    )

    await sut.loadApplications()
    await sut.activateSavedFilter()
    sut.selectedStatusFilter = .approved

    #expect(!sut.isSavedFilterActive)
    #expect(sut.filteredApplications.count == 1)
    #expect(sut.filteredApplications.first?.status == .approved)
  }

  // MARK: - Saved UIDs

  @Test func savedFilter_updatesSavedUids() async {
    let sut = makeSUT(
      savedUids: ["APP-001", "APP-002"],
      applications: Self.allApps
    )

    await sut.loadApplications()
    await sut.activateSavedFilter()

    #expect(sut.savedApplicationUids.count == 2)
    #expect(sut.savedApplicationUids.contains("APP-001"))
    #expect(sut.savedApplicationUids.contains("APP-002"))
  }

  // MARK: - Empty State

  @Test func savedFilter_emptyState_whenNoSavedApps() async {
    let sut = makeSUT(
      savedUids: [],
      applications: Self.allApps
    )

    await sut.loadApplications()
    await sut.activateSavedFilter()

    #expect(sut.isSavedFilterActive)
    #expect(sut.filteredApplications.isEmpty)
    #expect(sut.isEmpty)
  }

  // MARK: - Zone Change

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

  // MARK: - Helpers

  private func makeSUT(
    savedUids: [String],
    applications: [PlanningApplication],
    tier: SubscriptionTier = .free
  ) -> ApplicationListViewModel {
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success(
      savedUids.map { SavedApplication(applicationUid: $0, savedAt: Date()) }
    )
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(applications)
    return ApplicationListViewModel(
      repository: appSpy,
      zone: .cambridge,
      tier: tier,
      savedApplicationRepository: savedSpy
    )
  }
}
