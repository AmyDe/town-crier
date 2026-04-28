import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("ApplicationListViewModel -- Saved Filter")
@MainActor
struct ApplicationListSavedFilterTests {
  private static let allApps: [PlanningApplication] = [
    .pendingReview, .permitted, .rejected, .withdrawn,
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
    sut.selectedStatusFilter = .permitted
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
    sut.selectedStatusFilter = .permitted

    #expect(!sut.isSavedFilterActive)
    #expect(sut.filteredApplications.count == 1)
    #expect(sut.filteredApplications.first?.status == .permitted)
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

  // MARK: - Loading State

  @Test func isLoadingSaved_defaultsFalse() {
    let savedSpy = SpySavedApplicationRepository()
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success([])
    let sut = ApplicationListViewModel(
      repository: appSpy,
      zone: .cambridge,
      tier: .free,
      savedApplicationRepository: savedSpy
    )

    #expect(!sut.isLoadingSaved)
  }

  @Test func isLoadingSaved_isTrueWhileLoadAllInFlight() async {
    // A controllable saved repository that suspends loadAll() until resume() is called.
    let controllable = ControllableSavedApplicationRepository()
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(Self.allApps)
    let sut = ApplicationListViewModel(
      repository: appSpy,
      zone: .cambridge,
      tier: .free,
      savedApplicationRepository: controllable
    )
    await sut.loadApplications()

    // Kick off the activation; the task will suspend inside loadAll().
    let task = Task { await sut.activateSavedFilter() }

    // Wait for activateSavedFilter() to enter loadAll() and register the continuation.
    await controllable.waitForCall()

    // While loadAll() is in flight, the spinner flag must be true and isEmpty must
    // be suppressed so the view does not render the misleading empty state.
    #expect(sut.isLoadingSaved)
    #expect(!sut.isEmpty)

    // Resume the in-flight loadAll() with an empty result.
    controllable.resume(with: .success([]))
    await task.value

    // Once the await completes, the flag clears regardless of the result.
    #expect(!sut.isLoadingSaved)
  }

  @Test func isLoadingSaved_clearsAfterSuccess() async {
    let sut = makeSUT(savedUids: ["APP-001"], applications: Self.allApps)

    await sut.loadApplications()
    await sut.activateSavedFilter()

    #expect(!sut.isLoadingSaved)
  }

  @Test func isLoadingSaved_clearsAfterFailure() async {
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .failure(DomainError.networkUnavailable)
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

    #expect(!sut.isLoadingSaved)
  }

  // MARK: - Cross-Zone Saved Applications

  @Test func savedFilter_showsSavedAppsNotInCurrentList() async {
    // The current zone's applications list contains only APP-001 and APP-002
    let currentZoneApps: [PlanningApplication] = [.pendingReview, .permitted]
    // A saved application from a different zone — not in the current applications list
    let otherZoneApp = PlanningApplication(
      id: PlanningApplicationId("APP-OTHER-ZONE"),
      reference: ApplicationReference("2026/0500"),
      authority: LocalAuthority(code: "LDN", name: "London"),
      status: .undecided,
      receivedDate: Date(timeIntervalSince1970: 1_700_400_000),
      description: "New shopfront",
      address: "1 Oxford Street, London, W1D 1AN"
    )
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([
      SavedApplication(applicationUid: "APP-001", savedAt: Date(), application: .pendingReview),
      SavedApplication(
        applicationUid: "APP-OTHER-ZONE", savedAt: Date(), application: otherZoneApp),
    ])
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(currentZoneApps)
    let sut = ApplicationListViewModel(
      repository: appSpy,
      zone: .cambridge,
      tier: .free,
      savedApplicationRepository: savedSpy
    )

    await sut.loadApplications()
    await sut.activateSavedFilter()

    // Both saved apps should appear — including the one from another zone
    #expect(sut.filteredApplications.count == 2)
    let ids = Set(sut.filteredApplications.map(\.id.value))
    #expect(ids.contains("APP-001"))
    #expect(ids.contains("APP-OTHER-ZONE"))
  }

  @Test func savedFilter_doesNotDuplicateAppsAlreadyInList() async {
    // APP-001 is in both the current list and the saved applications
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([
      SavedApplication(applicationUid: "APP-001", savedAt: Date(), application: .pendingReview)
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

    // APP-001 should appear exactly once — no duplicates
    let matchingApps = sut.filteredApplications.filter { $0.id.value == "APP-001" }
    #expect(matchingApps.count == 1)
    #expect(sut.filteredApplications.count == 1)
  }

  @Test func savedFilter_ignoresSavedAppsWithoutEmbeddedData() async {
    // A saved application that has no embedded application data should not appear
    // if it's also not in the current applications list
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([
      SavedApplication(applicationUid: "APP-UNKNOWN", savedAt: Date(), application: nil)
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

    // APP-UNKNOWN is not in the current list and has no embedded data, so it shouldn't appear
    #expect(sut.filteredApplications.isEmpty)
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
