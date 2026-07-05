import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Tap-to-read wiring on `ApplicationListViewModel` (tc-0sfx.3, ADR 0035):
/// opening a row that shows an unread badge marks that application read via
/// the composite `(applicationUid, authorityId)` and optimistically clears the
/// badge locally; opening an already-read row issues no request. Navigation is
/// unaffected.
@Suite("ApplicationListViewModel — tap-to-read (tc-0sfx.3)")
@MainActor
struct ApplicationListViewModelTapToReadTests {

  private func makeSUT(
    applications: [PlanningApplication],
    badgeSetter: SpyBadgeSetter? = nil
  ) throws -> (ApplicationListViewModel, SpyNotificationStateRepository) {
    let appSpy = SpyPlanningApplicationRepository()
    let stateSpy = SpyNotificationStateRepository()
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    let sut = ApplicationListViewModel(
      repository: appSpy,
      zone: .cambridge,
      notificationStateRepository: stateSpy,
      badgeSetter: badgeSetter,
      userDefaults: defaults,
      sortKey: "tap.applicationsListSort"
    )
    sut.applications = applications
    return (sut, stateSpy)
  }

  private func app(authority: String, name: String, unread: Bool) -> PlanningApplication {
    PlanningApplication(
      id: PlanningApplicationId(authority: authority, name: name),
      reference: ApplicationReference(name),
      authority: LocalAuthority(code: authority, name: "Test"),
      status: .undecided,
      receivedDate: Date(timeIntervalSince1970: 1_700_000_000),
      description: "desc",
      address: "addr",
      latestUnreadEvent: unread
        ? LatestUnreadEvent(
          type: "NewApplication",
          decision: nil,
          createdAt: Date(timeIntervalSince1970: 1_700_500_000)
        )
        : nil
    )
  }

  @Test("opening an unread application marks it read with the composite (uid, authorityId)")
  func selectApplication_unread_marksReadWithComposite() async throws {
    let unread = app(authority: "42", name: "22/1234/FUL", unread: true)
    let (sut, stateSpy) = try makeSUT(applications: [unread])

    sut.selectApplication(unread.id)
    await sut.waitForPendingMarkRead()

    #expect(
      stateSpy.markApplicationReadCalls == [
        MarkApplicationReadCall(applicationUid: "22/1234/FUL", authorityId: 42)
      ]
    )
  }

  @Test("opening an unread application optimistically clears its badge (unreadCount drops)")
  func selectApplication_unread_clearsBadgeOptimistically() async throws {
    let unread = app(authority: "42", name: "22/1234/FUL", unread: true)
    let other = app(authority: "42", name: "99/0001/FUL", unread: true)
    let (sut, _) = try makeSUT(applications: [unread, other])
    #expect(sut.unreadCount == 2)

    sut.selectApplication(unread.id)

    // The optimistic clear is synchronous — no await required.
    #expect(sut.unreadCount == 1)
    let openedRow = sut.applications.first { $0.id == unread.id }
    let otherRow = sut.applications.first { $0.id == other.id }
    #expect(openedRow?.latestUnreadEvent == nil)
    #expect(otherRow?.latestUnreadEvent != nil)
  }

  @Test("opening an already-read application issues no mark-read")
  func selectApplication_read_doesNotMarkRead() async throws {
    let read = app(authority: "42", name: "22/1234/FUL", unread: false)
    let (sut, stateSpy) = try makeSUT(applications: [read])

    sut.selectApplication(read.id)
    await sut.waitForPendingMarkRead()

    #expect(stateSpy.markApplicationReadCalls.isEmpty)
  }

  @Test(
    "opening an unread application decrements the global unread count and pushes it to the badge setter"
  )
  func selectApplication_unread_decrementsGlobalUnreadCountAndPushesToBadgeSetter() async throws {
    let unread = app(authority: "42", name: "22/1234/FUL", unread: true)
    let badgeSetter = SpyBadgeSetter()
    let (sut, _) = try makeSUT(applications: [unread], badgeSetter: badgeSetter)
    sut.globalUnreadCount = 5

    sut.selectApplication(unread.id)

    // Synchronous, same as the per-row optimistic clear above — no relaunch
    // or foreground transition required to move the OS badge (tc-4x8e0).
    #expect(sut.globalUnreadCount == 4)
    #expect(badgeSetter.setBadgeCalls == [4])
  }

  @Test("selectApplication still notifies the coordinator for navigation")
  func selectApplication_notifiesCoordinator() async throws {
    let unread = app(authority: "42", name: "22/1234/FUL", unread: true)
    let (sut, _) = try makeSUT(applications: [unread])
    var selected: PlanningApplicationId?
    sut.onApplicationSelected = { selected = $0 }

    sut.selectApplication(unread.id)

    #expect(selected == unread.id)
  }
}
