import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Status filtering on MapViewModel. Free for all subscription tiers as of
/// tc-acf0 — `canFilter` and the Saved filter on Map were removed when the
/// dedicated Saved tab landed.
@Suite("MapViewModel — Status Filtering")
@MainActor
struct MapViewModelStatusFilterTests {
  private static let allApps: [PlanningApplication] = [
    .pendingReview, .permitted, .rejected, .withdrawn,
  ]

  private func makeSUT(
    applications: [PlanningApplication] = [],
    watchZones: [WatchZone] = [.cambridge]
  ) -> (MapViewModel, SpyPlanningApplicationRepository, SpyWatchZoneRepository) {
    let spy = SpyPlanningApplicationRepository()
    spy.fetchApplicationsResult = .success(applications)
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success(watchZones)
    let vm = MapViewModel(
      repository: spy,
      watchZoneRepository: watchZoneSpy
    )
    return (vm, spy, watchZoneSpy)
  }

  // MARK: - filteredAnnotations

  @Test func filteredAnnotations_filtersByPermitted() async {
    let (sut, _, _) = makeSUT(applications: Self.allApps)
    await sut.loadApplications()

    sut.selectedStatusFilter = .permitted

    #expect(sut.filteredAnnotations.count == 1)
    #expect(sut.filteredAnnotations.first?.status == .permitted)
  }

  @Test func filteredAnnotations_nilFilter_showsAll() async {
    let (sut, _, _) = makeSUT(applications: Self.allApps)
    await sut.loadApplications()

    sut.selectedStatusFilter = nil

    #expect(sut.filteredAnnotations.count == 4)
  }

  @Test func filteredAnnotations_noMatches_returnsEmpty() async {
    let (sut, _, _) = makeSUT(applications: [.pendingReview])
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
      userDefaults: defaults,
      zoneSelectionKey: "test.zone"
    )
    await sut.loadApplications()
    sut.selectedStatusFilter = .permitted

    await sut.selectZone(.london)

    #expect(sut.selectedStatusFilter == nil)
  }
}
