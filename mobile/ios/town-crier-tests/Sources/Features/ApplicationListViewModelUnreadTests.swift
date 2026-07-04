import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Tests for the unread-watermark wiring on `ApplicationListViewModel`:
/// per-zone unread count (derived client-side from the loaded apps' own
/// `latestUnreadEvent`), the four sort modes, the Mark-All-Read action,
/// and the Unread filter. Mirrors the web `useApplications` hook
/// (tc-1nsa.11) so the platforms stay behaviourally identical.
@Suite("ApplicationListViewModel — unread UI (tc-1nsa.8)")
@MainActor
struct ApplicationListViewModelUnreadTests {

  // MARK: - Helpers

  private func makeSUT(
    applications: [PlanningApplication] = [],
    state: NotificationState = NotificationState(
      lastReadAt: Date(timeIntervalSince1970: 0),
      version: 1,
      totalUnreadCount: 0
    ),
    sortKey: String = "test.applicationsListSort"
  ) throws -> (
    ApplicationListViewModel,
    SpyPlanningApplicationRepository,
    SpyNotificationStateRepository,
    UserDefaults
  ) {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(applications)
    let stateSpy = SpyNotificationStateRepository()
    stateSpy.fetchStateResult = .success(state)
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    let sut = ApplicationListViewModel(
      repository: appSpy,
      zone: .cambridge,
      notificationStateRepository: stateSpy,
      userDefaults: defaults,
      sortKey: sortKey
    )
    return (sut, appSpy, stateSpy, defaults)
  }

  private func event(at seconds: TimeInterval, type: String = "NewApplication") -> LatestUnreadEvent {
    LatestUnreadEvent(
      type: type,
      decision: nil,
      createdAt: Date(timeIntervalSince1970: seconds)
    )
  }

  // MARK: - Unread count (per-zone, client-side)

  @Test("unreadCount counts loaded applications with non-nil latestUnreadEvent")
  func unreadCount_countsRowsWithLatestUnreadEvent() async throws {
    let unreadA = PlanningApplication.pendingReview
      .withLatestUnreadEvent(event(at: 1_700_500_000))
    let unreadB = PlanningApplication.permitted
      .withLatestUnreadEvent(event(at: 1_700_600_000))
    let read = PlanningApplication.rejected
      .withLatestUnreadEvent(nil)
    let (sut, _, _, _) = try makeSUT(applications: [unreadA, unreadB, read])

    await sut.loadApplications()

    #expect(sut.unreadCount == 2)
  }

  @Test("unreadCount is zero when no loaded application has a latestUnreadEvent")
  func unreadCount_isZero_whenNoApplicationsHaveLatestUnreadEvent() async throws {
    let (sut, _, _, _) = try makeSUT(
      applications: [
        PlanningApplication.pendingReview.withLatestUnreadEvent(nil),
        PlanningApplication.permitted.withLatestUnreadEvent(nil),
      ]
    )

    await sut.loadApplications()

    #expect(sut.unreadCount == 0)
  }

  @Test("hasUnread is true when at least one loaded row has an unread event")
  func hasUnread_trueWhenAnyRowUnread() async throws {
    let (sut, _, _, _) = try makeSUT(
      applications: [
        PlanningApplication.pendingReview
          .withLatestUnreadEvent(event(at: 1_700_500_000))
      ]
    )

    await sut.loadApplications()

    #expect(sut.hasUnread)
  }

  @Test("hasUnread is false when no loaded row has an unread event")
  func hasUnread_falseWhenAllRead() async throws {
    let (sut, _, _, _) = try makeSUT()

    await sut.loadApplications()

    #expect(!sut.hasUnread)
  }

  @Test("loadApplications calls fetchState for the global count, but the chip stays client-side")
  func loadApplications_callsFetchStateForGlobalCountOnly() async throws {
    let (sut, _, stateSpy, _) = try makeSUT(
      applications: [
        PlanningApplication.pendingReview
          .withLatestUnreadEvent(event(at: 1_700_500_000))
      ],
      state: NotificationState(
        lastReadAt: Date(timeIntervalSince1970: 0),
        version: 1,
        totalUnreadCount: 99
      )
    )

    await sut.loadApplications()

    // `loadApplications` now refreshes the GLOBAL unread count (tc-c5m1,
    // GH#793) via `fetchState`, but the per-zone `unreadCount` chip stays
    // derived from each row's `latestUnreadEvent` and is untouched by the
    // server's global figure.
    #expect(stateSpy.fetchStateCallCount == 1)
    #expect(sut.unreadCount == 1)
  }

  // MARK: - Unread filter
  //
  // The Unread filter moved server-side in GH#682 slice 4: toggling it issues
  // `?unread=true`, resets the cursor, and reloads page 1, and the ViewModel
  // renders the server's returned set verbatim (no client-side
  // `latestUnreadEvent != nil` post-filter). That behaviour is covered in
  // `ApplicationListViewModelFilterTests`. The mutual-exclusivity rule below
  // (the chip + Unread single-select group) is still enforced in the ViewModel's
  // `didSet`s and stays here.

  @Test("toggling unreadOnly clears the status filter (single-select group)")
  func unreadOnly_clearsStatusFilter() async throws {
    let (sut, _, _, _) = try makeSUT(applications: [.pendingReview, .permitted])

    await sut.loadApplications()
    sut.selectedStatusFilter = .permitted
    sut.unreadOnly = true

    #expect(sut.selectedStatusFilter == nil)
  }

  @Test("setting status filter clears unreadOnly (single-select group)")
  func statusFilter_clearsUnreadOnly() async throws {
    let (sut, _, _, _) = try makeSUT(applications: [.pendingReview, .permitted])

    await sut.loadApplications()
    sut.unreadOnly = true
    sut.selectedStatusFilter = .permitted

    #expect(!sut.unreadOnly)
  }

  // MARK: - Sort

  @Test("default sort is recent-activity")
  func sort_defaultsToRecentActivity() throws {
    let (sut, _, _, _) = try makeSUT()
    #expect(sut.sort == .recentActivity)
  }

  @Test("setSort persists the choice to UserDefaults")
  func setSort_persistsToDefaults() throws {
    let (sut, _, _, defaults) = try makeSUT(sortKey: "persist.sort")

    sut.sort = .oldest

    #expect(defaults.string(forKey: "persist.sort") == "oldest")
  }

  @Test("ViewModel restores persisted sort on init")
  func sort_restoredFromDefaults() throws {
    let appSpy = SpyPlanningApplicationRepository()
    let stateSpy = SpyNotificationStateRepository()
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    defaults.set("status", forKey: "restore.sort")

    let sut = ApplicationListViewModel(
      repository: appSpy,
      zone: .cambridge,
      notificationStateRepository: stateSpy,
      userDefaults: defaults,
      sortKey: "restore.sort"
    )

    #expect(sut.sort == .status)
  }

  @Test("recent-activity is server-driven — the list preserves the server's order")
  func sort_recentActivity_preservesServerOrder() async throws {
    // recent-activity moved server-side in GH#682 slice 3 (#692): the API orders
    // by GREATEST(start_date, unread.created_at) DESC via the notification join
    // and the client renders that order as-is. The set below is deliberately
    // NOT in the order a local max(receivedDate, latestUnreadEvent.createdAt)
    // sort would produce (that would be [appA, appC, appB]), so a leftover
    // client-side re-sort would reorder it and fail this assertion.
    let appA = PlanningApplication.pendingReview  // 1_700_000_000
      .withLatestUnreadEvent(event(at: 1_700_900_000))
    let appB = PlanningApplication.permitted  // 1_700_100_000
      .withLatestUnreadEvent(nil)
    let appC = PlanningApplication.rejected  // 1_700_200_000
      .withLatestUnreadEvent(nil)
    let serverOrdered = [appB, appA, appC]
    let (sut, _, _, _) = try makeSUT(applications: serverOrdered)

    await sut.loadApplications()
    sut.sort = .recentActivity

    #expect(sut.filteredApplications.map(\.id) == serverOrdered.map(\.id))
  }

  // newest/oldest (GH#682 slice 1), status (slice 2) and recent-activity
  // (slice 3) are all server-driven: the API returns rows already in the
  // requested order (proven by the Go pgtest suite, #688/#690/#692) and the
  // client pages them via infinite scroll. The client must not re-sort locally —
  // that would only order the pages already loaded — so it preserves the
  // server's order verbatim. (The full reset-cursor-and-reload-on-sort-change
  // flow is covered by ApplicationListViewModelPaginationTests.)

  @Test("newest is server-driven — the list preserves the server's order")
  func sort_newest_preservesServerOrder() async throws {
    let serverOrdered: [PlanningApplication] = [.rejected, .permitted, .pendingReview]
    let (sut, _, _, _) = try makeSUT(applications: serverOrdered)

    await sut.loadApplications()
    sut.sort = .newest

    #expect(sut.filteredApplications.map(\.id) == serverOrdered.map(\.id))
  }

  @Test("oldest is server-driven — the list preserves the server's order")
  func sort_oldest_preservesServerOrder() async throws {
    let serverOrdered: [PlanningApplication] = [.pendingReview, .permitted, .rejected]
    let (sut, _, _, _) = try makeSUT(applications: serverOrdered)

    await sut.loadApplications()
    sut.sort = .oldest

    #expect(sut.filteredApplications.map(\.id) == serverOrdered.map(\.id))
  }

  @Test("status is server-driven — the list preserves the server's order")
  func sort_status_preservesServerOrder() async throws {
    // Rejected, Undecided, Permitted — deliberately not in `status.rawValue`
    // order. Status moved server-side in GH#682 slice 2, so the client must
    // render the API order as-is rather than re-sorting by `app_state` locally.
    let serverOrdered: [PlanningApplication] = [.rejected, .pendingReview, .permitted]
    let (sut, _, _, _) = try makeSUT(applications: serverOrdered)

    await sut.loadApplications()
    sut.sort = .status

    #expect(sut.filteredApplications.map(\.id) == serverOrdered.map(\.id))
  }

  // Distance-sort tests live in `ApplicationListViewModelDistanceSortTests`
  // (tc-mso6) — split out to keep this file under SwiftLint's 400-line
  // ceiling.

  // MARK: - Mark all read

  @Test("markAllRead calls the notification-state repository")
  func markAllRead_callsRepository() async throws {
    let (sut, _, stateSpy, _) = try makeSUT(
      applications: [
        PlanningApplication.pendingReview.withLatestUnreadEvent(
          event(at: 1_700_500_000)
        )
      ],
      state: NotificationState(
        lastReadAt: Date(timeIntervalSince1970: 0),
        version: 1,
        totalUnreadCount: 5
      )
    )

    await sut.loadApplications()
    await sut.markAllRead()

    #expect(stateSpy.markAllReadCallCount == 1)
  }

  @Test("markAllRead drops the chip count to zero after refetch")
  func markAllRead_zeroesCountAfterRefetch() async throws {
    let unread = PlanningApplication.pendingReview
      .withLatestUnreadEvent(event(at: 1_700_500_000))
    let (sut, appSpy, _, _) = try makeSUT(applications: [unread])

    await sut.loadApplications()
    #expect(sut.unreadCount == 1)

    // Server-side mark-all-read flips every row's latestUnreadEvent to nil.
    appSpy.fetchApplicationsResult = .success([
      PlanningApplication.pendingReview.withLatestUnreadEvent(nil)
    ])

    await sut.markAllRead()

    #expect(sut.unreadCount == 0)
    #expect(!sut.hasUnread)
  }

  @Test("markAllRead refetches applications so latestUnreadEvent drops to nil")
  func markAllRead_refetchesApplications() async throws {
    let unread = PlanningApplication.pendingReview
      .withLatestUnreadEvent(event(at: 1_700_500_000))
    let (sut, appSpy, _, _) = try makeSUT(applications: [unread])

    await sut.loadApplications()
    let callsBefore = appSpy.fetchApplicationsPageCalls.count

    // Server-side mark-all-read flips every row's latestUnreadEvent to nil. The
    // refetch is paged now (GH#682 slice 3), so it drives the paged endpoint.
    appSpy.fetchApplicationsResult = .success([
      PlanningApplication.pendingReview.withLatestUnreadEvent(nil)
    ])

    await sut.markAllRead()

    #expect(appSpy.fetchApplicationsPageCalls.count > callsBefore)
    #expect(sut.applications.first?.latestUnreadEvent == nil)
  }

  @Test("markAllRead silently swallows repository failure (no error surfaced)")
  func markAllRead_swallowsFailure() async throws {
    let (sut, _, stateSpy, _) = try makeSUT()
    stateSpy.markAllReadResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()
    await sut.markAllRead()

    // Repository failure is swallowed per spec decision #8.
    #expect(sut.error == nil)
  }
}
