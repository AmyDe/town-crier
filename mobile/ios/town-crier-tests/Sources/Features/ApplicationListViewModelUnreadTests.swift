import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Tests for the unread-watermark wiring on `ApplicationListViewModel`:
/// unread count, the four sort modes, the Mark-All-Read action, and the
/// Unread filter. Mirrors the web `useApplications` hook (tc-1nsa.11) so
/// the platforms stay behaviourally identical.
///
/// Spec: `docs/specs/notifications-unread-watermark.md#ios-applications-unread-ui`.
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

  // MARK: - Unread count

  @Test("unreadCount comes from the notification-state repository")
  func unreadCount_fromState() async throws {
    let (sut, _, _, _) = try makeSUT(
      state: NotificationState(
        lastReadAt: Date(timeIntervalSince1970: 0),
        version: 1,
        totalUnreadCount: 7
      )
    )

    await sut.loadApplications()

    #expect(sut.unreadCount == 7)
  }

  @Test("hasUnread is true when totalUnreadCount > 0")
  func hasUnread_trueWhenPositive() async throws {
    let (sut, _, _, _) = try makeSUT(
      state: NotificationState(
        lastReadAt: Date(timeIntervalSince1970: 0),
        version: 1,
        totalUnreadCount: 1
      )
    )

    await sut.loadApplications()

    #expect(sut.hasUnread)
  }

  @Test("hasUnread is false when totalUnreadCount is zero")
  func hasUnread_falseWhenZero() async throws {
    let (sut, _, _, _) = try makeSUT()

    await sut.loadApplications()

    #expect(!sut.hasUnread)
  }

  @Test("loadApplications keeps unread count zero when fetchState fails")
  func unreadCount_silentOnFetchFailure() async throws {
    let (sut, _, stateSpy, _) = try makeSUT()
    stateSpy.fetchStateResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(sut.unreadCount == 0)
    #expect(sut.error == nil) // applications still rendered, state failure silent
  }

  // MARK: - Unread filter

  @Test("unreadOnly filter keeps only rows with non-nil latestUnreadEvent")
  func unreadOnly_filtersToUnreadRows() async throws {
    let unread = PlanningApplication.pendingReview
      .withLatestUnreadEvent(event(at: 1_700_500_000))
    let read = PlanningApplication.permitted
      .withLatestUnreadEvent(nil)
    let (sut, _, _, _) = try makeSUT(applications: [unread, read])

    await sut.loadApplications()
    sut.unreadOnly = true

    #expect(sut.filteredApplications.count == 1)
    #expect(sut.filteredApplications.first?.id == unread.id)
  }

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

  @Test("recent-activity sort orders by max(receivedDate, latestUnreadEvent.createdAt) desc")
  func sort_recentActivity_ordersByLatestActivity() async throws {
    // app A: received earliest, but has a much-later unread event → top
    // app B: received in the middle, no unread event → middle
    // app C: received latest, no unread event → just below A
    let appA = PlanningApplication.pendingReview // 1_700_000_000
      .withLatestUnreadEvent(event(at: 1_700_900_000))
    let appB = PlanningApplication.permitted // 1_700_100_000
      .withLatestUnreadEvent(nil)
    let appC = PlanningApplication.rejected // 1_700_200_000
      .withLatestUnreadEvent(nil)
    let (sut, _, _, _) = try makeSUT(applications: [appB, appA, appC])

    await sut.loadApplications()
    sut.sort = .recentActivity

    #expect(sut.filteredApplications.map(\.id) == [appA.id, appC.id, appB.id])
  }

  @Test("newest sort orders by receivedDate desc")
  func sort_newest_ordersByReceivedDateDesc() async throws {
    let older = PlanningApplication.pendingReview // 1_700_000_000
    let middle = PlanningApplication.permitted // 1_700_100_000
    let newest = PlanningApplication.rejected // 1_700_200_000
    let (sut, _, _, _) = try makeSUT(applications: [middle, older, newest])

    await sut.loadApplications()
    sut.sort = .newest

    #expect(sut.filteredApplications.map(\.id) == [newest.id, middle.id, older.id])
  }

  @Test("oldest sort orders by receivedDate asc")
  func sort_oldest_ordersByReceivedDateAsc() async throws {
    let older = PlanningApplication.pendingReview
    let middle = PlanningApplication.permitted
    let newest = PlanningApplication.rejected
    let (sut, _, _, _) = try makeSUT(applications: [middle, older, newest])

    await sut.loadApplications()
    sut.sort = .oldest

    #expect(sut.filteredApplications.map(\.id) == [older.id, middle.id, newest.id])
  }

  @Test("status sort orders by appState raw value")
  func sort_status_ordersByAppState() async throws {
    let permitted = PlanningApplication.permitted // "Permitted"
    let rejected = PlanningApplication.rejected // "Rejected"
    let undecided = PlanningApplication.pendingReview // "Undecided"
    let (sut, _, _, _) = try makeSUT(applications: [undecided, rejected, permitted])

    await sut.loadApplications()
    sut.sort = .status

    let labels = sut.filteredApplications.map(\.status.rawValue)
    #expect(labels == labels.sorted())
  }

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

  @Test("markAllRead optimistically clears unread count before refetch")
  func markAllRead_optimisticallyZerosCount() async throws {
    let (sut, _, _, _) = try makeSUT(
      state: NotificationState(
        lastReadAt: Date(timeIntervalSince1970: 0),
        version: 1,
        totalUnreadCount: 5
      )
    )

    await sut.loadApplications()
    #expect(sut.unreadCount == 5)

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
    let callsBefore = appSpy.fetchApplicationsCalls.count

    // Server-side mark-all-read flips every row's latestUnreadEvent to nil
    appSpy.fetchApplicationsResult = .success([
      PlanningApplication.pendingReview.withLatestUnreadEvent(nil)
    ])

    await sut.markAllRead()

    #expect(appSpy.fetchApplicationsCalls.count > callsBefore)
    #expect(sut.applications.first?.latestUnreadEvent == nil)
  }

  @Test("markAllRead silently swallows repository failure (optimistic UI)")
  func markAllRead_swallowsFailure() async throws {
    let (sut, _, stateSpy, _) = try makeSUT(
      state: NotificationState(
        lastReadAt: Date(timeIntervalSince1970: 0),
        version: 1,
        totalUnreadCount: 3
      )
    )
    stateSpy.markAllReadResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()
    await sut.markAllRead()

    // Optimistic UI still shows zero per spec decision #8.
    #expect(sut.unreadCount == 0)
    #expect(sut.error == nil)
  }
}
