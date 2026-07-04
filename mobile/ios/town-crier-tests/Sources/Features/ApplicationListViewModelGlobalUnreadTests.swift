import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Tests for gating the Mark-All-Read toolbar button on the GLOBAL unread
/// count rather than the per-zone chip (tc-c5m1, GH#793).
///
/// The app-icon badge is set from the server's global
/// `notification-state.totalUnreadCount`, but the Mark-All-Read button was
/// gated on `hasUnread` — derived purely from the currently loaded zone's
/// rows. When global unread existed but no loaded row carried it (different
/// zone, deleted zone, unfetched page), the button hid and the badge became
/// unclearable. `hasClearableUnread` decouples the button from the per-zone
/// chip so it always tracks the same count the badge does, and
/// `markAllRead()` now clears that badge immediately on success.
@Suite("ApplicationListViewModel — global unread / clearable badge (tc-c5m1)")
@MainActor
struct ApplicationListViewModelGlobalUnreadTests {

  // MARK: - Helpers

  private func makeSUT(
    applications: [PlanningApplication] = [],
    totalUnreadCount: Int = 0,
    badgeSetter: SpyBadgeSetter? = nil
  ) -> (ApplicationListViewModel, SpyNotificationStateRepository) {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(applications)
    let stateSpy = SpyNotificationStateRepository()
    stateSpy.fetchStateResult = .success(
      NotificationState(
        lastReadAt: Date(timeIntervalSince1970: 0),
        version: 1,
        totalUnreadCount: totalUnreadCount
      )
    )
    let sut = ApplicationListViewModel(
      repository: appSpy,
      zone: .cambridge,
      notificationStateRepository: stateSpy,
      badgeSetter: badgeSetter
    )
    return (sut, stateSpy)
  }

  private func event(at seconds: TimeInterval) -> LatestUnreadEvent {
    LatestUnreadEvent(type: "NewApplication", decision: nil, createdAt: Date(timeIntervalSince1970: seconds))
  }

  // MARK: - hasClearableUnread

  @Test("hasClearableUnread is true when global unread exists even with no unread rows loaded")
  func hasClearableUnread_trueWhenGlobalUnreadExists_withNoUnreadRowsLoaded() async {
    let (sut, _) = makeSUT(
      applications: [.pendingReview.withLatestUnreadEvent(nil)],
      totalUnreadCount: 5
    )

    await sut.loadApplications()

    #expect(sut.hasClearableUnread)
    #expect(!sut.hasUnread)
  }

  @Test("hasClearableUnread is false when global unread is zero")
  func hasClearableUnread_falseWhenGlobalUnreadIsZero() async {
    let (sut, _) = makeSUT(totalUnreadCount: 0)

    await sut.loadApplications()

    #expect(!sut.hasClearableUnread)
  }

  @Test("hasUnread and hasClearableUnread are independent: unread row but zero global count")
  func hasUnread_andHasClearableUnread_areIndependent() async {
    let (sut, _) = makeSUT(
      applications: [.pendingReview.withLatestUnreadEvent(event(at: 1_700_500_000))],
      totalUnreadCount: 0
    )

    await sut.loadApplications()

    #expect(sut.hasUnread)
    #expect(!sut.hasClearableUnread)
  }

  // MARK: - markAllRead badge clearing

  @Test("markAllRead clears the global count and sets the badge to zero on success")
  func markAllRead_clearsCountAndBadge_onSuccess() async {
    let badgeSetter = SpyBadgeSetter()
    let (sut, stateSpy) = makeSUT(totalUnreadCount: 5, badgeSetter: badgeSetter)

    await sut.loadApplications()
    await sut.markAllRead()

    #expect(stateSpy.markAllReadCallCount == 1)
    #expect(sut.globalUnreadCount == 0)
    #expect(badgeSetter.setBadgeCalls.last == 0)
  }

  @Test("markAllRead leaves the badge untouched when the repository call fails")
  func markAllRead_leavesBadgeUntouched_onFailure() async {
    let badgeSetter = SpyBadgeSetter()
    let (sut, stateSpy) = makeSUT(totalUnreadCount: 5, badgeSetter: badgeSetter)
    stateSpy.markAllReadResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()
    await sut.markAllRead()

    #expect(badgeSetter.setBadgeCalls.isEmpty)
    #expect(sut.globalUnreadCount == 5)
  }
}
