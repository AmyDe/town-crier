import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("MapViewModel — offline caching")
@MainActor
struct OfflineMapViewModelTests {
    private func makeSUT(
        remoteApplications: [PlanningApplication] = [],
        cachedEntry: CacheEntry<[PlanningApplication]>? = nil,
        isConnected: Bool = true,
        watchZone: WatchZone = .cambridge
    ) -> (MapViewModel, SpyPlanningApplicationRepository, SpyApplicationCacheStore, StubConnectivityMonitor) {
        let remote = SpyPlanningApplicationRepository()
        remote.fetchApplicationsResult = .success(remoteApplications)
        let cache = SpyApplicationCacheStore()
        cache.storedEntry = cachedEntry
        let connectivity = StubConnectivityMonitor(isConnected: isConnected)
        let offlineRepo = OfflineAwareRepository(
            remote: remote,
            cache: cache,
            connectivity: connectivity
        )
        let vm = MapViewModel(
            offlineRepository: offlineRepo,
            watchZone: watchZone
        )
        return (vm, remote, cache, connectivity)
    }

    @Test func loadApplications_online_setsFreshness_fresh() async {
        let apps = [PlanningApplication.pendingReview]
        let (sut, _, _, _) = makeSUT(remoteApplications: apps)

        await sut.loadApplications()

        #expect(sut.dataFreshness == .fresh)
    }

    @Test func loadApplications_offline_withCache_setsFreshness_staleOrOffline() async {
        let staleEntry = CacheEntry(
            data: [PlanningApplication.approved],
            fetchedAt: Date().addingTimeInterval(-2000),
            ttlSeconds: 900
        )
        let (sut, _, _, _) = makeSUT(cachedEntry: staleEntry, isConnected: false)

        await sut.loadApplications()

        #expect(sut.dataFreshness == .stale || sut.dataFreshness == .offline)
    }

    @Test func loadApplications_offline_noCache_setsError() async {
        let (sut, _, _, _) = makeSUT(isConnected: false)

        await sut.loadApplications()

        #expect(sut.error == .networkUnavailable)
    }

    @Test func loadApplications_populatesAnnotations_fromCache() async {
        let cachedEntry = CacheEntry(
            data: [PlanningApplication.pendingReview, .approved],
            fetchedAt: Date().addingTimeInterval(-60),
            ttlSeconds: 900
        )
        let (sut, _, _, _) = makeSUT(cachedEntry: cachedEntry)

        await sut.loadApplications()

        #expect(sut.annotations.count == 2)
    }
}
