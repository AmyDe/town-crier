import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("ApplicationListViewModel — offline caching")
@MainActor
struct OfflineApplicationListViewModelTests {
    private func makeSUT(
        remoteApplications: [PlanningApplication] = [],
        cachedEntry: CacheEntry<[PlanningApplication]>? = nil,
        isConnected: Bool = true
    ) -> (ApplicationListViewModel, SpyPlanningApplicationRepository, SpyApplicationCacheStore) {
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
        let vm = ApplicationListViewModel(
            offlineRepository: offlineRepo,
            authority: .cambridge
        )
        return (vm, remote, cache)
    }

    @Test func loadApplications_online_setsFresh() async {
        let apps = [PlanningApplication.pendingReview]
        let (sut, _, _) = makeSUT(remoteApplications: apps)

        await sut.loadApplications()

        #expect(sut.dataFreshness == .fresh)
        #expect(sut.filteredApplications.count == 1)
    }

    @Test func loadApplications_offline_withCache_setsStale() async {
        let staleEntry = CacheEntry(
            data: [PlanningApplication.approved],
            fetchedAt: Date().addingTimeInterval(-2000),
            ttlSeconds: 900
        )
        let (sut, _, _) = makeSUT(cachedEntry: staleEntry, isConnected: false)

        await sut.loadApplications()

        #expect(sut.dataFreshness == .stale)
        #expect(sut.filteredApplications.count == 1)
    }

    @Test func loadApplications_offline_noCache_showsError() async {
        let (sut, _, _) = makeSUT(isConnected: false)

        await sut.loadApplications()

        #expect(sut.error == .networkUnavailable)
    }
}
