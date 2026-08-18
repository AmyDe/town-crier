import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// tc-edyvn: a zero-cluster viewport (or a zone with genuinely zero
/// applications) used to unmount the map behind a full-screen
/// `EmptyStateView`, trapping the user with no way to zoom back out.
/// `showMap` is what `MapView.mapBody` now branches on instead of `isEmpty`,
/// so these cover it across load/error/empty/populated states plus the
/// region-change-after-empty-viewport regression.
@Suite("MapViewModel — map mounting (tc-edyvn)")
@MainActor
struct MapViewModelMapMountingTests {
  private func makeSUT(
    clusters: [MapCluster] = [],
    watchZones: [WatchZone] = [.cambridge]
  ) -> (MapViewModel, SpyPlanningApplicationRepository, SpyWatchZoneRepository) {
    let spy = SpyPlanningApplicationRepository()
    spy.fetchClustersResult = .success(clusters)
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success(watchZones)
    let vm = MapViewModel(repository: spy, watchZoneRepository: watchZoneSpy)
    return (vm, spy, watchZoneSpy)
  }

  @Test func showMap_trueWhenNoClustersAfterLoad() async {
    let (sut, _, _) = makeSUT(clusters: [])

    await sut.loadApplications()

    #expect(sut.isEmpty)
    #expect(sut.showMap)
  }

  @Test func showMap_trueWhenClustersExist() async {
    let (sut, _, _) = makeSUT(clusters: [.bubble(count: 3)])

    await sut.loadApplications()

    #expect(sut.showMap)
  }

  @Test func showMap_falseDuringInitialLoad() async {
    let (sut, spy, _) = makeSUT()
    spy.enableGate()

    let load = Task { await sut.loadApplications() }
    // Two hops before the gated fetch (loadApplications awaits
    // watchZoneRepository.loadAll(), then loadClusters awaits the gated
    // fetchClusters) — one yield per await point, mirroring the
    // multi-yield precedent in ApplicationListViewModelConcurrentLoadTests.
    await Task.yield()
    await Task.yield()
    #expect(sut.isLoading)
    #expect(!sut.hasLoaded)
    #expect(!sut.showMap)

    spy.releaseGate()
    await load.value
  }

  @Test func showMap_falseWhenErrorOccurred() async {
    let (sut, spy, _) = makeSUT()
    spy.fetchClustersResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(!sut.showMap)
  }

  /// The concrete "trapped" scenario from the bead: the user zooms into an
  /// empty patch (zero clusters, `showMap` stays true) and then zooms back
  /// out. Proves the region-change path (`loadClusters`, what
  /// `ClusteredMapView.Coordinator`'s debounced region-change handler calls)
  /// keeps working from an empty state rather than being gated by `isEmpty` —
  /// i.e. the map the user is looking at is both mounted and still
  /// interactive.
  @Test func loadClusters_recoversClustersAfterAZeroResultViewport() async {
    let (sut, spy, _) = makeSUT(clusters: [])
    await sut.loadApplications()
    #expect(sut.isEmpty)
    #expect(sut.showMap)

    spy.fetchClustersResult = .success([.bubble(count: 7)])
    await sut.loadClusters(viewport: .test2, zoom: 12)

    #expect(sut.clusters.count == 1)
    #expect(!sut.isEmpty)
    #expect(sut.showMap)
    #expect(spy.fetchClustersCalls.count == 2)
  }
}
