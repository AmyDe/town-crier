import Foundation
import Testing
import TownCrierDomain

@Suite("OfflineAwareRepository")
struct OfflineAwareRepositoryTests {
  private func makeSUT(
    remoteApplications: [PlanningApplication] = [],
    cachedEntry: CacheEntry<[PlanningApplication]>? = nil,
    isConnected: Bool = true
  ) -> (
    OfflineAwareRepository, SpyPlanningApplicationRepository, SpyApplicationCacheStore,
    StubConnectivityMonitor
  ) {
    let remote = SpyPlanningApplicationRepository()
    remote.fetchApplicationsResult = .success(remoteApplications)
    let cache = SpyApplicationCacheStore()
    cache.storedEntry = cachedEntry
    let connectivity = StubConnectivityMonitor(isConnected: isConnected)
    let sut = OfflineAwareRepository(
      remote: remote,
      cache: cache,
      connectivity: connectivity
    )
    return (sut, remote, cache, connectivity)
  }

  // MARK: - Online with no cache

  @Test func fetchApplications_online_noCached_fetchesFromRemoteAndCaches() async throws {
    let apps = [PlanningApplication.pendingReview, .permitted]
    let (sut, remote, cache, _) = makeSUT(remoteApplications: apps)

    let result = try await sut.fetchApplications(for: WatchZone.cambridge)

    #expect(result.data.count == 2)
    #expect(result.isFresh())
    #expect(remote.fetchApplicationsCalls.count == 1)
    #expect(cache.storeCalls.count == 1)
  }

  // MARK: - Online with stale cache

  @Test func fetchApplications_online_staleCache_fetchesRemoteAndUpdatesCache() async throws {
    let staleEntry = CacheEntry(
      data: [PlanningApplication.withdrawn],
      fetchedAt: Date().addingTimeInterval(-2000),
      ttlSeconds: 900
    )
    let freshApps = [PlanningApplication.pendingReview]
    let (sut, remote, cache, _) = makeSUT(
      remoteApplications: freshApps,
      cachedEntry: staleEntry
    )

    let result = try await sut.fetchApplications(for: WatchZone.cambridge)

    #expect(result.data.count == 1)
    #expect(result.data.first?.id == PlanningApplication.pendingReview.id)
    #expect(remote.fetchApplicationsCalls.count == 1)
    #expect(cache.storeCalls.count == 1)
  }

  // MARK: - Online with fresh cache

  @Test func fetchApplications_online_freshCache_returnsCachedWithoutRemoteCall() async throws {
    let freshEntry = CacheEntry(
      data: [PlanningApplication.pendingReview],
      fetchedAt: Date().addingTimeInterval(-60),
      ttlSeconds: 900
    )
    let (sut, remote, _, _) = makeSUT(cachedEntry: freshEntry)

    let result = try await sut.fetchApplications(for: WatchZone.cambridge)

    #expect(result.data.count == 1)
    #expect(result.isFresh())
    #expect(remote.fetchApplicationsCalls.isEmpty)
  }

  // MARK: - Offline with cache

  @Test func fetchApplications_offline_withCache_returnsCachedData() async throws {
    let cachedEntry = CacheEntry(
      data: [PlanningApplication.permitted],
      fetchedAt: Date().addingTimeInterval(-2000),
      ttlSeconds: 900
    )
    let (sut, remote, _, _) = makeSUT(cachedEntry: cachedEntry, isConnected: false)

    let result = try await sut.fetchApplications(for: WatchZone.cambridge)

    #expect(result.data.count == 1)
    #expect(result.data.first?.id == PlanningApplication.permitted.id)
    #expect(remote.fetchApplicationsCalls.isEmpty)
  }

  // MARK: - Offline with no cache

  @Test func fetchApplications_offline_noCache_throwsNetworkUnavailable() async {
    let (sut, _, _, _) = makeSUT(isConnected: false)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await sut.fetchApplications(for: WatchZone.cambridge)
    }
  }

  // MARK: - Online remote failure falls back to cache

  @Test func fetchApplications_online_remoteFails_withCache_returnsCached() async throws {
    let cachedEntry = CacheEntry(
      data: [PlanningApplication.rejected],
      fetchedAt: Date().addingTimeInterval(-500),
      ttlSeconds: 900
    )
    let (sut, remote, _, _) = makeSUT(cachedEntry: cachedEntry)
    remote.fetchApplicationsResult = .failure(DomainError.unexpected("server error"))

    let result = try await sut.fetchApplications(for: WatchZone.cambridge)

    #expect(result.data.count == 1)
    #expect(result.data.first?.id == PlanningApplication.rejected.id)
  }

  // MARK: - Online remote failure no cache propagates error

  @Test func fetchApplications_online_remoteFails_noCache_throwsError() async {
    let (sut, remote, _, _) = makeSUT()
    remote.fetchApplicationsResult = .failure(DomainError.unexpected("server error"))

    await #expect(throws: DomainError.self) {
      _ = try await sut.fetchApplications(for: WatchZone.cambridge)
    }
  }

}
