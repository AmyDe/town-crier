import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Status filtering on MapViewModel. Free for all subscription tiers as of
/// tc-acf0. With server-side clustering (GH#698) a chip change refetches the
/// clusters for the current viewport with `status=` rather than filtering an
/// in-memory set client-side.
@Suite("MapViewModel — Status Filtering")
@MainActor
struct MapViewModelStatusFilterTests {
  private func makeSUT(
    clusters: [MapCluster] = [.bubble(count: 5)],
    watchZones: [WatchZone] = [.cambridge]
  ) -> (MapViewModel, SpyPlanningApplicationRepository, SpyWatchZoneRepository) {
    let spy = SpyPlanningApplicationRepository()
    spy.fetchClustersResult = .success(clusters)
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success(watchZones)
    let vm = MapViewModel(repository: spy, watchZoneRepository: watchZoneSpy)
    return (vm, spy, watchZoneSpy)
  }

  @Test func applyStatusFilter_refetchesClustersWithStatus() async {
    let (sut, spy, _) = makeSUT()
    await sut.loadApplications()

    await sut.applyStatusFilter(.permitted)

    #expect(sut.selectedStatusFilter == .permitted)
    #expect(spy.fetchClustersCalls.last?.filter == .status(.permitted))
  }

  @Test func applyStatusFilter_nil_refetchesWithoutStatus() async {
    let (sut, spy, _) = makeSUT()
    await sut.loadApplications()
    await sut.applyStatusFilter(.permitted)

    await sut.applyStatusFilter(nil)

    #expect(sut.selectedStatusFilter == nil)
    #expect(spy.fetchClustersCalls.last?.filter == .all)
  }

  @Test func applyStatusFilter_refetchesForTheCurrentViewport() async {
    let (sut, spy, _) = makeSUT()
    await sut.loadApplications()
    await sut.loadClusters(viewport: .test2, zoom: 16)

    await sut.applyStatusFilter(.rejected)

    #expect(spy.fetchClustersCalls.last?.viewport == .test2)
    #expect(spy.fetchClustersCalls.last?.zoom == 16)
  }

  @Test func selectZone_resetsStatusFilter() async throws {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchClustersResult = .success([.bubble(count: 2)])
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
    await sut.applyStatusFilter(.permitted)

    await sut.selectZone(.london)

    #expect(sut.selectedStatusFilter == nil)
  }
}
