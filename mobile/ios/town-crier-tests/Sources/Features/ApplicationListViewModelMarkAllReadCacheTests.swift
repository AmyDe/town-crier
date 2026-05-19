import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Cache-invalidation contract for `ApplicationListViewModel.markAllRead`
/// when backed by an `OfflineAwareRepository` (tc-e3bu).
///
/// Mark-all-read is a global mutation: the server advances the watermark
/// for every zone the user can see. The per-zone applications cache must
/// be cleared before the refetch, otherwise a TTL-fresh cache hit will
/// keep serving rows with the old `latestUnreadEvent` and the
/// `Unread (N)` chip will stay stuck until the cache TTL expires.
@Suite("ApplicationListViewModel.markAllRead — cache invalidation (tc-e3bu)")
@MainActor
struct ApplicationListViewModelMarkAllReadCacheTests {

  private func makeSUT(
    cachedApplications: [PlanningApplication],
    refetchedApplications: [PlanningApplication]
  ) -> (
    ApplicationListViewModel,
    SpyApplicationCacheStore,
    SpyPlanningApplicationRepository,
    SpyNotificationStateRepository
  ) {
    let remote = SpyPlanningApplicationRepository()
    remote.fetchApplicationsResult = .success(refetchedApplications)
    let cache = SpyApplicationCacheStore()
    cache.storedEntry = CacheEntry(
      data: cachedApplications,
      fetchedAt: Date().addingTimeInterval(-60),
      ttlSeconds: 900
    )
    let offlineRepository = OfflineAwareRepository(
      remote: remote,
      cache: cache,
      connectivity: StubConnectivityMonitor(isConnected: true)
    )
    let stateRepo = SpyNotificationStateRepository()
    stateRepo.markAllReadResult = .success(())
    let sut = ApplicationListViewModel(
      offlineRepository: offlineRepository,
      zone: .cambridge,
      notificationStateRepository: stateRepo
    )
    return (sut, cache, remote, stateRepo)
  }

  private func event(at seconds: TimeInterval) -> LatestUnreadEvent {
    LatestUnreadEvent(
      type: "NewApplication",
      decision: nil,
      createdAt: Date(timeIntervalSince1970: seconds)
    )
  }

  /// Without invalidation, the refetch after the mark-all-read POST would
  /// short-circuit on the TTL-fresh cache and the chip would stay stuck
  /// at the prior count. The view-model must clear every cached zone
  /// before refetching.
  @Test func markAllRead_invalidatesEveryCachedZoneBeforeRefetch() async {
    let cachedUnread = PlanningApplication.pendingReview
      .withLatestUnreadEvent(event(at: 1_700_500_000))
    let refetchedRead = PlanningApplication.pendingReview
      .withLatestUnreadEvent(nil)
    let (sut, cache, _, _) = makeSUT(
      cachedApplications: [cachedUnread],
      refetchedApplications: [refetchedRead]
    )

    await sut.loadApplications()
    #expect(sut.unreadCount == 1)

    await sut.markAllRead()

    #expect(cache.invalidateAllCallCount == 1)
  }

  /// End-to-end behavioural assertion: a TTL-fresh cache must not stop the
  /// chip from zeroing after mark-all-read. With the invalidation in place,
  /// the refetch goes to the network, returns rows with `latestUnreadEvent
  /// == nil`, and `unreadCount` drops to 0 (tc-e3bu repro path).
  @Test func markAllRead_zeroesUnreadCount_evenWhenCacheIsFresh() async {
    let cachedUnread = PlanningApplication.pendingReview
      .withLatestUnreadEvent(event(at: 1_700_500_000))
    let refetchedRead = PlanningApplication.pendingReview
      .withLatestUnreadEvent(nil)
    let (sut, _, remote, _) = makeSUT(
      cachedApplications: [cachedUnread],
      refetchedApplications: [refetchedRead]
    )

    await sut.loadApplications()
    let remoteCallsBeforeMar = remote.fetchApplicationsCalls.count
    #expect(sut.unreadCount == 1)

    await sut.markAllRead()

    #expect(sut.unreadCount == 0)
    #expect(!sut.hasUnread)
    #expect(remote.fetchApplicationsCalls.count > remoteCallsBeforeMar)
  }
}
