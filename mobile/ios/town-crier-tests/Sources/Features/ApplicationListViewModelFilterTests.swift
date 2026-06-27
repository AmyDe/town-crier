import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Server-driven status/unread filtering for the watch-zone list (GH#682
/// slice 4). The status chips and the Unread toggle now drive the `?status=`/
/// `?unread=` query params rather than a client-side post-filter, so changing a
/// filter resets the cursor and reloads from page 1, the server's returned set
/// is rendered verbatim (no local re-filtering), and status and unread are never
/// sent together. The per-row `latestUnreadEvent` badge/count stays client-side.
@Suite("ApplicationListViewModel — server-side filters (GH#682 slice 4)")
@MainActor
struct ApplicationListViewModelFilterTests {

  private func makeSUT(
    pages: [ApplicationPage] = []
  ) throws -> (ApplicationListViewModel, SpyPlanningApplicationRepository) {
    let spy = SpyPlanningApplicationRepository()
    spy.pagedResponses = pages
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    defaults.set(ApplicationsSort.newest.rawValue, forKey: "test.filterSort")
    let vm = ApplicationListViewModel(
      repository: spy,
      zone: .cambridge,
      userDefaults: defaults,
      sortKey: "test.filterSort"
    )
    return (vm, spy)
  }

  private func event(at seconds: TimeInterval) -> LatestUnreadEvent {
    LatestUnreadEvent(type: "NewApplication", decision: nil, createdAt: Date(timeIntervalSince1970: seconds))
  }

  // MARK: - Filter drives the query param

  @Test("selecting a status chip issues ?status= and reloads from page 1")
  func selectingStatus_issuesStatusParam_andReloadsPageOne() async throws {
    let firstPage = ApplicationPage(applications: [.pendingReview], nextCursor: "page-2")
    let (sut, spy) = try makeSUT(pages: [firstPage])

    await sut.loadApplications()
    #expect(spy.fetchApplicationsPageCalls.last?.filter == .all)

    spy.pagedResponses = [ApplicationPage(applications: [.permitted], nextCursor: nil)]
    sut.selectedStatusFilter = .permitted
    await sut.handleFilterChanged()

    let last = try #require(spy.fetchApplicationsPageCalls.last)
    #expect(last.filter == .status(.permitted))
    #expect(last.cursor == nil)
  }

  @Test("toggling Unread issues ?unread=true and reloads from page 1")
  func togglingUnread_issuesUnreadParam_andReloadsPageOne() async throws {
    let firstPage = ApplicationPage(applications: [.pendingReview], nextCursor: "page-2")
    let (sut, spy) = try makeSUT(pages: [firstPage])

    await sut.loadApplications()
    #expect(spy.fetchApplicationsPageCalls.last?.filter == .all)

    spy.pagedResponses = [ApplicationPage(applications: [.permitted], nextCursor: nil)]
    sut.unreadOnly = true
    await sut.handleFilterChanged()

    let last = try #require(spy.fetchApplicationsPageCalls.last)
    #expect(last.filter == .unread)
    #expect(last.cursor == nil)
  }

  @Test("clearing back to All issues no status/unread and reloads from page 1")
  func clearingToAll_issuesNoFilter_andReloadsPageOne() async throws {
    let statusPage = ApplicationPage(applications: [.permitted], nextCursor: "page-2")
    let (sut, spy) = try makeSUT(pages: [statusPage])

    sut.selectedStatusFilter = .permitted
    await sut.loadApplications()
    #expect(spy.fetchApplicationsPageCalls.last?.filter == .status(.permitted))

    spy.pagedResponses = [ApplicationPage(applications: [.pendingReview], nextCursor: nil)]
    sut.selectedStatusFilter = nil
    await sut.handleFilterChanged()

    let last = try #require(spy.fetchApplicationsPageCalls.last)
    #expect(last.filter == .all)
    #expect(last.cursor == nil)
  }

  @Test("an unchanged filter does not refetch")
  func unchangedFilter_doesNotRefetch() async throws {
    let (sut, spy) = try makeSUT(pages: [ApplicationPage(applications: [.pendingReview], nextCursor: nil)])

    await sut.loadApplications()
    let callsBefore = spy.fetchApplicationsPageCalls.count

    // activeFilter is still `.all` — no real change, so no reload.
    await sut.handleFilterChanged()

    #expect(spy.fetchApplicationsPageCalls.count == callsBefore)
  }

  // MARK: - No client-side re-filtering

  @Test("the server's returned set is rendered verbatim — no local re-filter")
  func serverFilteredPage_renderedVerbatim() async throws {
    // Under the Unread filter the server returns the authoritative set. It is
    // rendered as-is: a row whose latestUnreadEvent is nil (would be dropped by
    // the old client-side unread filter) must survive, proving no local
    // re-filtering remains.
    let unreadRow = PlanningApplication.pendingReview.withLatestUnreadEvent(event(at: 1_700_000_000))
    let readRow = PlanningApplication.permitted.withLatestUnreadEvent(nil)
    let allPage = ApplicationPage(applications: [.rejected], nextCursor: nil)
    let serverFilteredPage = ApplicationPage(applications: [unreadRow, readRow], nextCursor: nil)
    let (sut, _) = try makeSUT(pages: [allPage, serverFilteredPage])

    await sut.loadApplications()
    sut.unreadOnly = true
    await sut.handleFilterChanged()

    #expect(sut.filteredApplications.map(\.id) == [unreadRow, readRow].map(\.id))
  }

  @Test("a status that no loaded row matches is still rendered verbatim")
  func statusFilteredPage_renderedVerbatim() async throws {
    // The server owns which rows match the status; the client does not re-check
    // `app_state`. The returned page contains a row whose status differs from
    // the selected chip and it must still render (server is the source of truth).
    let allPage = ApplicationPage(applications: [.pendingReview], nextCursor: nil)
    let serverFilteredPage = ApplicationPage(applications: [.rejected, .withdrawn], nextCursor: nil)
    let (sut, _) = try makeSUT(pages: [allPage, serverFilteredPage])

    await sut.loadApplications()
    sut.selectedStatusFilter = .permitted
    await sut.handleFilterChanged()

    #expect(sut.filteredApplications.map(\.id) == [PlanningApplication.rejected, .withdrawn].map(\.id))
  }

  // MARK: - Mutual exclusivity

  @Test("selecting a status while Unread is on clears Unread — never both")
  func selectingStatus_whileUnreadOn_clearsUnread() async throws {
    let (sut, _) = try makeSUT(pages: [ApplicationPage(applications: [.pendingReview], nextCursor: nil)])

    await sut.loadApplications()
    sut.unreadOnly = true
    sut.selectedStatusFilter = .permitted

    #expect(!sut.unreadOnly)
    #expect(sut.activeFilter == .status(.permitted))
  }

  @Test("toggling Unread while a status is selected clears the status — never both")
  func togglingUnread_whileStatusSelected_clearsStatus() async throws {
    let (sut, _) = try makeSUT(pages: [ApplicationPage(applications: [.pendingReview], nextCursor: nil)])

    await sut.loadApplications()
    sut.selectedStatusFilter = .permitted
    sut.unreadOnly = true

    #expect(sut.selectedStatusFilter == nil)
    #expect(sut.activeFilter == .unread)
  }

  // MARK: - Badge / unread count regression (stays client-side)

  @Test("unreadCount still derives from latestUnreadEvent after a server filter")
  func unreadCount_stillDerivesFromLatestUnreadEvent() async throws {
    let unreadRow = PlanningApplication.pendingReview.withLatestUnreadEvent(event(at: 1_700_000_000))
    let readRow = PlanningApplication.permitted.withLatestUnreadEvent(nil)
    let allPage = ApplicationPage(applications: [.rejected], nextCursor: nil)
    let serverFilteredPage = ApplicationPage(applications: [unreadRow, readRow], nextCursor: nil)
    let (sut, _) = try makeSUT(pages: [allPage, serverFilteredPage])

    await sut.loadApplications()
    sut.selectedStatusFilter = .permitted
    await sut.handleFilterChanged()

    // One of the two rendered rows carries a latestUnreadEvent.
    #expect(sut.unreadCount == 1)
    #expect(sut.hasUnread)
  }
}
