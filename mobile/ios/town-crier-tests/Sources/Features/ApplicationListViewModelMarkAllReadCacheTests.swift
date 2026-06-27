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
    // The list path is paged for every sort now (GH#682 slice 3) and paging
    // deliberately bypasses the offline cache, so the first load and the
    // post-mark-all-read refetch are driven as successive server pages: the
    // first carries the still-unread rows, the second the read rows the server
    // returns once the watermark has advanced.
    remote.pagedResponses = [
      ApplicationPage(applications: cachedApplications, nextCursor: nil),
      ApplicationPage(applications: refetchedApplications, nextCursor: nil),
    ]
    let cache = SpyApplicationCacheStore()
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

  /// mark-all-read is a global mutation, so the view-model must clear every
  /// cached zone before refetching. The paged list path bypasses the cache,
  /// but the param-less map path still reads it, so the invalidation keeps that
  /// cache from serving stale unread flags after the watermark advances.
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

  /// End-to-end behavioural assertion (tc-e3bu contract, preserved through the
  /// GH#682 slice 3 paged migration): after mark-all-read the paged refetch goes
  /// to the network, returns rows with `latestUnreadEvent == nil`, and
  /// `unreadCount` drops to 0 so the `Unread (N)` chip clears.
  @Test func markAllRead_zeroesUnreadCount_viaPagedRefetch() async {
    let cachedUnread = PlanningApplication.pendingReview
      .withLatestUnreadEvent(event(at: 1_700_500_000))
    let refetchedRead = PlanningApplication.pendingReview
      .withLatestUnreadEvent(nil)
    let (sut, _, remote, _) = makeSUT(
      cachedApplications: [cachedUnread],
      refetchedApplications: [refetchedRead]
    )

    await sut.loadApplications()
    let remoteCallsBeforeMar = remote.fetchApplicationsPageCalls.count
    #expect(sut.unreadCount == 1)

    await sut.markAllRead()

    #expect(sut.unreadCount == 0)
    #expect(!sut.hasUnread)
    #expect(remote.fetchApplicationsPageCalls.count > remoteCallsBeforeMar)
  }
}
