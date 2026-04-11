import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("NotificationsViewModel")
@MainActor
struct NotificationsViewModelTests {

  // MARK: - Helpers

  private func makeSUT(
    notifications: [NotificationItem] = [],
    total: Int? = nil
  ) -> (NotificationsViewModel, SpyNotificationRepository) {
    let spy = SpyNotificationRepository()
    spy.fetchResult = .success(
      NotificationPage(
        notifications: notifications,
        total: total ?? notifications.count,
        page: 1
      )
    )
    let vm = NotificationsViewModel(repository: spy)
    return (vm, spy)
  }

  // MARK: - Initial Load

  @Test("loadNotifications populates notifications on success")
  func loadNotifications_populatesOnSuccess() async {
    let expected = [NotificationItem.rearExtension, .changeOfUse]
    let (sut, _) = makeSUT(notifications: expected)

    await sut.loadNotifications()

    #expect(sut.notifications == expected)
    #expect(!sut.isLoading)
    #expect(sut.error == nil)
  }

  @Test("loadNotifications sends page 1 and default pageSize to repository")
  func loadNotifications_sendsCorrectParameters() async {
    let (sut, spy) = makeSUT()

    await sut.loadNotifications()

    #expect(spy.fetchCalls.count == 1)
    #expect(spy.fetchCalls[0].page == 1)
    #expect(spy.fetchCalls[0].pageSize == 20)
  }

  @Test("loadNotifications sets isLoading false after completion")
  func loadNotifications_setsIsLoadingFalse() async {
    let (sut, _) = makeSUT()

    await sut.loadNotifications()

    #expect(!sut.isLoading)
  }

  @Test("loadNotifications sets error on failure")
  func loadNotifications_setsErrorOnFailure() async {
    let (sut, spy) = makeSUT()
    spy.fetchResult = .failure(DomainError.networkUnavailable)

    await sut.loadNotifications()

    #expect(sut.error == .networkUnavailable)
    #expect(sut.notifications.isEmpty)
  }

  @Test("loadNotifications clears error on retry")
  func loadNotifications_clearsErrorOnRetry() async {
    let (sut, spy) = makeSUT()
    spy.fetchResult = .failure(DomainError.networkUnavailable)
    await sut.loadNotifications()

    spy.fetchResult = .success(
      NotificationPage(notifications: [.rearExtension], total: 1, page: 1)
    )
    await sut.loadNotifications()

    #expect(sut.error == nil)
    #expect(sut.notifications.count == 1)
  }

  @Test("loadNotifications resets results on subsequent call")
  func loadNotifications_resetsOnReload() async {
    let (sut, spy) = makeSUT(notifications: [.rearExtension, .changeOfUse])
    await sut.loadNotifications()

    spy.fetchResult = .success(
      NotificationPage(notifications: [.solarPanels], total: 1, page: 1)
    )
    await sut.loadNotifications()

    #expect(sut.notifications.count == 1)
    #expect(sut.notifications[0] == .solarPanels)
  }

  // MARK: - Pagination

  @Test("loadNotifications sets total and hasMore from result")
  func loadNotifications_setsTotalAndHasMore() async {
    let (sut, spy) = makeSUT()
    spy.fetchResult = .success(
      NotificationPage(
        notifications: [.rearExtension, .changeOfUse],
        total: 10,
        page: 1
      )
    )

    await sut.loadNotifications()

    #expect(sut.total == 10)
    #expect(sut.hasMore)
  }

  @Test("loadMore appends next page results")
  func loadMore_appendsNextPage() async {
    let (sut, spy) = makeSUT()
    spy.fetchResult = .success(
      NotificationPage(notifications: [.rearExtension], total: 3, page: 1)
    )
    await sut.loadNotifications()

    spy.fetchResult = .success(
      NotificationPage(notifications: [.changeOfUse], total: 3, page: 2)
    )
    await sut.loadMore()

    #expect(sut.notifications.count == 2)
    #expect(sut.notifications[0] == .rearExtension)
    #expect(sut.notifications[1] == .changeOfUse)
    #expect(spy.fetchCalls.last?.page == 2)
  }

  @Test("loadMore does nothing when no more pages")
  func loadMore_noMorePages_doesNothing() async {
    let (sut, spy) = makeSUT()
    spy.fetchResult = .success(
      NotificationPage(notifications: [.rearExtension], total: 1, page: 1)
    )
    await sut.loadNotifications()

    await sut.loadMore()

    // Only the initial fetch should have been called
    #expect(spy.fetchCalls.count == 1)
  }

  @Test("loadMore sets error on failure")
  func loadMore_setsErrorOnFailure() async {
    let (sut, spy) = makeSUT()
    spy.fetchResult = .success(
      NotificationPage(notifications: [.rearExtension], total: 3, page: 1)
    )
    await sut.loadNotifications()

    spy.fetchResult = .failure(DomainError.networkUnavailable)
    await sut.loadMore()

    #expect(sut.error == .networkUnavailable)
    // Existing items should be preserved
    #expect(sut.notifications.count == 1)
  }

  @Test("loadMore preserves existing items on failure")
  func loadMore_preservesExistingOnFailure() async {
    let (sut, spy) = makeSUT()
    spy.fetchResult = .success(
      NotificationPage(notifications: [.rearExtension], total: 3, page: 1)
    )
    await sut.loadNotifications()

    spy.fetchResult = .failure(DomainError.networkUnavailable)
    await sut.loadMore()

    #expect(sut.notifications == [.rearExtension])
  }

  // MARK: - Empty State

  @Test("isEmpty is true when load returns no results and not loading")
  func isEmpty_noResults_returnsTrue() async {
    let (sut, _) = makeSUT()

    await sut.loadNotifications()

    #expect(sut.isEmpty)
  }

  @Test("isEmpty is false before loading")
  func isEmpty_beforeLoad_returnsFalse() {
    let (sut, _) = makeSUT()

    #expect(!sut.isEmpty)
  }

  @Test("isEmpty is false when notifications exist")
  func isEmpty_withNotifications_returnsFalse() async {
    let (sut, _) = makeSUT(notifications: [.rearExtension])

    await sut.loadNotifications()

    #expect(!sut.isEmpty)
  }

  @Test("isEmpty is false when error exists")
  func isEmpty_withError_returnsFalse() async {
    let (sut, spy) = makeSUT()
    spy.fetchResult = .failure(DomainError.networkUnavailable)

    await sut.loadNotifications()

    #expect(!sut.isEmpty)
  }
}
