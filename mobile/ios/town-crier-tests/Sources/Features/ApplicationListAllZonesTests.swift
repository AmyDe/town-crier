import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Tests for the synthetic 'All' option on the list-view zone selector.
///
/// 'All' is an escape hatch that lets users see saved applications that
/// fall outside any active watch zone (e.g. saves made from Search, or
/// zones whose radius later shrank). Rules:
/// - 'All' + Saved active → all saved applications across all zones,
///   sourced via the SavedApplicationRepository's denormalised payloads.
/// - 'All' + Saved inactive → empty list with a discoverability prompt.
/// - The map view does NOT expose 'All' (geographic context required).
@Suite("ApplicationListViewModel -- All Zones")
@MainActor
struct ApplicationListAllZonesTests {
  // MARK: - Activation

  @Test func selectAllZones_clearsSelectedZone() async throws {
    let sut = try makeSUT(zones: [.cambridge, .london])
    await sut.loadApplications()
    #expect(sut.selectedZone?.id == WatchZone.cambridge.id)

    await sut.selectAllZones()

    #expect(sut.isAllZonesSelected)
    #expect(sut.selectedZone == nil)
  }

  @Test func selectZone_clearsAllZonesFlag() async throws {
    let sut = try makeSUT(zones: [.cambridge, .london])
    await sut.loadApplications()
    await sut.selectAllZones()
    #expect(sut.isAllZonesSelected)

    await sut.selectZone(.london)

    #expect(!sut.isAllZonesSelected)
    #expect(sut.selectedZone?.id == WatchZone.london.id)
  }

  // MARK: - 'All' + Saved active

  @Test func allZones_savedActive_showsSavedAppsAcrossAllZones() async throws {
    let cambridgeSave = PlanningApplication.pendingReview
    let londonSave = PlanningApplication(
      id: PlanningApplicationId("APP-LDN-1"),
      reference: ApplicationReference("2026/0500"),
      authority: LocalAuthority(code: "LDN", name: "London"),
      status: .undecided,
      receivedDate: Date(timeIntervalSince1970: 1_700_400_000),
      description: "New shopfront",
      address: "1 Oxford Street, London, W1D 1AN"
    )
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([
      SavedApplication(
        applicationUid: cambridgeSave.id.value, savedAt: Date(), application: cambridgeSave),
      SavedApplication(
        applicationUid: londonSave.id.value, savedAt: Date(), application: londonSave),
    ])
    let sut = try makeSUT(
      zones: [.cambridge, .london],
      currentZoneApps: [cambridgeSave],
      savedRepository: savedSpy
    )

    await sut.loadApplications()
    await sut.selectAllZones()
    await sut.activateSavedFilter()

    #expect(sut.isAllZonesSelected)
    #expect(sut.isSavedFilterActive)
    let ids = Set(sut.filteredApplications.map(\.id.value))
    #expect(ids.contains(cambridgeSave.id.value))
    #expect(ids.contains(londonSave.id.value))
    #expect(sut.filteredApplications.count == 2)
  }

  @Test func allZones_savedActivatedFirst_thenSelectAll_showsAllSaves() async throws {
    // Simulates the order: user is on a zone with Saved active, then taps 'All'.
    let cambridgeSave = PlanningApplication.pendingReview
    let londonSave = PlanningApplication(
      id: PlanningApplicationId("APP-LDN-1"),
      reference: ApplicationReference("2026/0500"),
      authority: LocalAuthority(code: "LDN", name: "London"),
      status: .undecided,
      receivedDate: Date(timeIntervalSince1970: 1_700_400_000),
      description: "Shopfront",
      address: "1 Oxford Street"
    )
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([
      SavedApplication(
        applicationUid: cambridgeSave.id.value, savedAt: Date(), application: cambridgeSave),
      SavedApplication(
        applicationUid: londonSave.id.value, savedAt: Date(), application: londonSave),
    ])
    let sut = try makeSUT(
      zones: [.cambridge, .london],
      currentZoneApps: [cambridgeSave],
      savedRepository: savedSpy
    )

    await sut.loadApplications()
    await sut.activateSavedFilter()
    // While zone-scoped to Cambridge, only the Cambridge save shows.
    #expect(sut.filteredApplications.count == 1)

    await sut.selectAllZones()

    // After switching to 'All', Saved stays active and now spans both zones.
    #expect(sut.isAllZonesSelected)
    #expect(sut.isSavedFilterActive)
    #expect(sut.filteredApplications.count == 2)
  }

  // MARK: - 'All' + Saved inactive

  @Test func allZones_savedInactive_showsEmptyList() async throws {
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([])
    let sut = try makeSUT(
      zones: [.cambridge, .london],
      currentZoneApps: [.pendingReview, .permitted],
      savedRepository: savedSpy
    )

    await sut.loadApplications()
    await sut.selectAllZones()

    #expect(sut.isAllZonesSelected)
    #expect(!sut.isSavedFilterActive)
    #expect(sut.filteredApplications.isEmpty)
    #expect(sut.isEmpty)
  }

  @Test func allZones_savedInactive_doesNotFetchPerZoneApplications() async throws {
    // 'All' without Saved should not request applications for any zone — the
    // list is empty by design (the user must pick a real zone or turn on Saved).
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success([.pendingReview])
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([.cambridge, .london])
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([])
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
    let beforeCount = appSpy.fetchApplicationsCalls.count
    await sut.selectAllZones()

    // selectAllZones must not trigger a per-zone fetch.
    #expect(appSpy.fetchApplicationsCalls.count == beforeCount)
  }

  // MARK: - Deactivating Saved while in 'All'

  @Test func allZones_deactivatingSaved_clearsApplications() async throws {
    let cambridgeSave = PlanningApplication.pendingReview
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([
      SavedApplication(
        applicationUid: cambridgeSave.id.value, savedAt: Date(), application: cambridgeSave)
    ])
    let sut = try makeSUT(
      zones: [.cambridge],
      currentZoneApps: [cambridgeSave],
      savedRepository: savedSpy
    )

    await sut.loadApplications()
    await sut.selectAllZones()
    await sut.activateSavedFilter()
    #expect(!sut.filteredApplications.isEmpty)

    sut.deactivateSavedFilter()

    #expect(!sut.isSavedFilterActive)
    #expect(sut.isAllZonesSelected)
    #expect(sut.filteredApplications.isEmpty)
    #expect(sut.isEmpty)
  }

  // MARK: - Initial state

  @Test func isAllZonesSelected_defaultsFalse() async throws {
    let sut = try makeSUT(zones: [.cambridge])
    #expect(!sut.isAllZonesSelected)
  }

  // MARK: - Zone picker visibility

  @Test func showZonePicker_trueWithSingleZone_soAllOptionIsReachable() async throws {
    // 'All' is meaningful even with one zone (it surfaces orphan saves made
    // outside that zone's geometry). The picker must therefore render so the
    // user can reach the 'All' chip.
    let sut = try makeSUT(zones: [.cambridge])
    await sut.loadApplications()

    #expect(sut.showZonePicker)
  }

  @Test func showZonePicker_falseWithNoZones() async throws {
    let sut = try makeSUT(zones: [])
    await sut.loadApplications()

    #expect(!sut.showZonePicker)
  }

  // MARK: - Persistence

  @Test func selectAllZones_persistsSentinelToUserDefaults() async throws {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success([])
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([.cambridge])
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([])
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
    await sut.selectAllZones()

    #expect(defaults.string(forKey: "test.zone") == ApplicationListViewModel.allZonesSentinel)
  }

  @Test func loadApplications_restoresAllZonesFromPersistedSentinel() async throws {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success([.pendingReview])
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([.cambridge, .london])
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([])
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    defaults.set(ApplicationListViewModel.allZonesSentinel, forKey: "test.zone")
    let sut = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy,
      tier: .free,
      userDefaults: defaults,
      zoneSelectionKey: "test.zone",
      savedApplicationRepository: savedSpy
    )

    await sut.loadApplications()

    #expect(sut.isAllZonesSelected)
    #expect(sut.selectedZone == nil)
    // 'All' + Saved-inactive must not trigger a per-zone fetch.
    #expect(appSpy.fetchApplicationsCalls.isEmpty)
  }

  // MARK: - Empty state messaging

  @Test func emptyStateKind_allZonesSavedInactive_isPickAZoneOrTurnOnSaved() async throws {
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([])
    let sut = try makeSUT(
      zones: [.cambridge, .london],
      savedRepository: savedSpy
    )

    await sut.loadApplications()
    await sut.selectAllZones()

    #expect(sut.emptyStateKind == .allZonesNoSavedFilter)
  }

  @Test func emptyStateKind_savedActiveNoResults_isNoSavedApplications() async throws {
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([])
    let sut = try makeSUT(
      zones: [.cambridge],
      savedRepository: savedSpy
    )

    await sut.loadApplications()
    await sut.activateSavedFilter()

    #expect(sut.emptyStateKind == .savedFilterNoResults)
  }

  @Test func emptyStateKind_zoneSelectedNoApps_isNoApplications() async throws {
    let sut = try makeSUT(
      zones: [.cambridge],
      currentZoneApps: []
    )

    await sut.loadApplications()

    #expect(sut.emptyStateKind == .zoneNoApplications)
  }

  // MARK: - Helpers

  private func makeSUT(
    zones: [WatchZone],
    currentZoneApps: [PlanningApplication] = [],
    savedRepository: SavedApplicationRepository? = nil,
    tier: SubscriptionTier = .free
  ) throws -> ApplicationListViewModel {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(currentZoneApps)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success(zones)
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    return ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy,
      tier: tier,
      userDefaults: defaults,
      zoneSelectionKey: "test.zone",
      savedApplicationRepository: savedRepository
    )
  }
}
