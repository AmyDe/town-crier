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

  // MARK: - Distance sort (tc-mso6)

  @Test("ApplicationsSort.distance has stable raw value mirroring web sibling")
  func sort_distance_rawValueMatchesWeb() {
    #expect(ApplicationsSort.distance.rawValue == "distance")
  }

  @Test("ApplicationsSort.distance has a user-facing label")
  func sort_distance_displayLabel() {
    #expect(ApplicationsSort.distance.displayLabel == "Distance")
  }

  @Test("distance sort orders by haversine distance from active zone centre, ascending")
  func sort_distance_ordersByHaversineFromZoneCentre() async throws {
    // Cambridge centre: 52.2053, 0.1218.
    // permitted    location 52.2053, 0.1218 — exactly at centre (~0m)
    // pendingReview location 52.2043, 0.1243 — close
    // rejected     location 52.2010, 0.1300 — further
    // (See PlanningApplication+Fixtures for source-of-truth coords.)
    let (sut, _, _, _) = try makeSUT(
      applications: [.rejected, .pendingReview, .permitted]
    )

    await sut.loadApplications()
    sut.sort = .distance

    #expect(
      sut.filteredApplications.map(\.id) == [
        PlanningApplication.permitted.id,
        PlanningApplication.pendingReview.id,
        PlanningApplication.rejected.id,
      ]
    )
  }

  @Test("distance sort places apps without a location last")
  func sort_distance_appsWithoutLocationLast() async throws {
    let withoutLocation = PlanningApplication(
      id: PlanningApplicationId("APP-NO-LOC"),
      reference: ApplicationReference("2026/9999"),
      authority: .cambridge,
      status: .undecided,
      receivedDate: Date(timeIntervalSince1970: 1_700_000_000),
      description: "Application missing coordinates",
      address: "Unknown",
      location: nil
    )
    let (sut, _, _, _) = try makeSUT(
      applications: [withoutLocation, .permitted, .pendingReview]
    )

    await sut.loadApplications()
    sut.sort = .distance

    #expect(sut.filteredApplications.last?.id == withoutLocation.id)
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
