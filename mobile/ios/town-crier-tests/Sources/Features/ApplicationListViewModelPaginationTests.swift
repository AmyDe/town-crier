import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Infinite-scroll pagination for the watch-zone list (GH#682 slice 1). For the
/// three server-supported sorts (distance/newest/oldest) the ViewModel drives
/// ordering from the server, follows `X-Next-Cursor` until it is absent, appends
/// pages as the user nears the end, and resets the cursor when the sort changes.
/// The client-side sorts (recent-activity/status) keep their param-less,
/// single-page, client-sorted path unchanged.
@Suite("ApplicationListViewModel — pagination (GH#682)")
@MainActor
struct ApplicationListViewModelPaginationTests {

  private func makeSUT(
    sort: ApplicationsSort,
    pages: [ApplicationPage] = []
  ) throws -> (ApplicationListViewModel, SpyPlanningApplicationRepository) {
    let spy = SpyPlanningApplicationRepository()
    spy.pagedResponses = pages
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    defaults.set(sort.rawValue, forKey: "test.paginationSort")
    let vm = ApplicationListViewModel(
      repository: spy,
      zone: .cambridge,
      userDefaults: defaults,
      sortKey: "test.paginationSort"
    )
    return (vm, spy)
  }

  // MARK: - status classification (GH#682 slice 2)

  @Test("status is a server-driven sort mapping to the '?sort=status' raw value")
  func status_isServerDriven() {
    #expect(ApplicationsSort.status.serverOrder == .status)
    #expect(ApplicationsSort.status.isServerSorted)
    #expect(ApplicationSortOrder.status.rawValue == "status")
  }

  // MARK: - Page append

  @Test("scrolling near the end fetches the next page via the cursor and appends")
  func loadNextPage_appendsUsingCursor() async throws {
    let page1 = ApplicationPage(applications: [.pendingReview, .permitted], nextCursor: "cursor-2")
    let page2 = ApplicationPage(applications: [.rejected, .withdrawn], nextCursor: nil)
    let (sut, spy) = try makeSUT(sort: .newest, pages: [page1, page2])

    await sut.loadApplications()
    #expect(
      sut.filteredApplications.map(\.id) == [PlanningApplication.pendingReview, .permitted].map(\.id)
    )

    await sut.onRowAppear(.permitted)

    #expect(
      sut.filteredApplications.map(\.id) == [
        PlanningApplication.pendingReview, .permitted, .rejected, .withdrawn,
      ].map(\.id)
    )
    #expect(spy.fetchApplicationsPageCalls.count == 2)
    #expect(spy.fetchApplicationsPageCalls[0].cursor == nil)
    #expect(spy.fetchApplicationsPageCalls[1].cursor == "cursor-2")
    #expect(spy.fetchApplicationsPageCalls[1].sort == .newest)
  }

  @Test("an overlapping row at a page boundary is not duplicated")
  func loadNextPage_dedupesOverlap() async throws {
    let page1 = ApplicationPage(applications: [.pendingReview, .permitted], nextCursor: "c2")
    // `.permitted` repeats across the boundary (keyset overlap).
    let page2 = ApplicationPage(applications: [.permitted, .rejected], nextCursor: nil)
    let (sut, _) = try makeSUT(sort: .newest, pages: [page1, page2])

    await sut.loadApplications()
    await sut.onRowAppear(.permitted)

    #expect(
      sut.filteredApplications.map(\.id) == [
        PlanningApplication.pendingReview, .permitted, .rejected,
      ].map(\.id)
    )
  }

  // MARK: - Sort change resets the cursor

  @Test("changing the server sort clears the cursor and reloads from page 1")
  func changingServerSort_resetsCursorAndReloadsPageOne() async throws {
    let newestPage = ApplicationPage(applications: [.pendingReview], nextCursor: "newest-cursor")
    let (sut, spy) = try makeSUT(sort: .newest, pages: [newestPage])

    await sut.loadApplications()
    #expect(spy.fetchApplicationsPageCalls.last?.sort == .newest)

    spy.pagedResponses = [ApplicationPage(applications: [.rejected], nextCursor: nil)]
    sut.sort = .oldest
    await sut.handleSortChanged()

    let last = try #require(spy.fetchApplicationsPageCalls.last)
    #expect(last.sort == .oldest)
    #expect(last.cursor == nil)
  }

  // MARK: - Last page stops the loop

  @Test("a page with no cursor ends pagination — no further fetch")
  func lastPageWithoutCursor_stopsPagination() async throws {
    let onlyPage = ApplicationPage(applications: [.pendingReview, .permitted], nextCursor: nil)
    let (sut, spy) = try makeSUT(sort: .newest, pages: [onlyPage])

    await sut.loadApplications()
    #expect(spy.fetchApplicationsPageCalls.count == 1)

    await sut.onRowAppear(.permitted)
    await sut.loadNextPage()

    #expect(spy.fetchApplicationsPageCalls.count == 1)
  }

  // MARK: - Error path

  @Test("a next-page fetch error surfaces to the ViewModel error state")
  func pageFetchError_surfacesToErrorState() async throws {
    let page1 = ApplicationPage(applications: [.pendingReview, .permitted], nextCursor: "c2")
    let (sut, spy) = try makeSUT(sort: .newest, pages: [page1])

    await sut.loadApplications()
    #expect(sut.error == nil)

    spy.fetchApplicationsPageError = DomainError.serverError(statusCode: 400, message: "bad cursor")
    await sut.onRowAppear(.permitted)

    #expect(sut.error == .serverError(statusCode: 400, message: "bad cursor"))
  }

  // MARK: - Server owns the ordering

  @Test("a server sort preserves the API-returned order (no local re-sort)")
  func serverSort_preservesServerOrder() async throws {
    // Deliberately out of date order — the server is the source of truth.
    let serverOrdered = ApplicationPage(
      applications: [.rejected, .pendingReview, .permitted], nextCursor: nil)
    let (sut, _) = try makeSUT(sort: .newest, pages: [serverOrdered])

    await sut.loadApplications()

    #expect(
      sut.filteredApplications.map(\.id) == [
        PlanningApplication.rejected, .pendingReview, .permitted,
      ].map(\.id)
    )
  }

  @Test("a server sort drives the paged endpoint, not the param-less fetch")
  func serverSort_usesPagedFetch() async throws {
    let (sut, spy) = try makeSUT(
      sort: .newest, pages: [ApplicationPage(applications: [.pendingReview], nextCursor: nil)])

    await sut.loadApplications()

    #expect(spy.fetchApplicationsPageCalls.count == 1)
    #expect(spy.fetchApplicationsCalls.isEmpty)
  }

  // MARK: - status is server-driven (GH#682 slice 2)

  @Test("selecting status issues ?sort=status and paginates via the cursor")
  func statusSort_paginatesViaCursor() async throws {
    // Pages are in the server's status order (app_state ASC, start_date DESC),
    // deliberately NOT in `status.rawValue` order, so a stray client re-sort
    // would reorder them and fail this assertion.
    let page1 = ApplicationPage(applications: [.rejected, .permitted], nextCursor: "status-cursor-2")
    let page2 = ApplicationPage(applications: [.withdrawn, .pendingReview], nextCursor: nil)
    let (sut, spy) = try makeSUT(sort: .status, pages: [page1, page2])

    await sut.loadApplications()
    #expect(spy.fetchApplicationsPageCalls.first?.sort == .status)
    #expect(spy.fetchApplicationsPageCalls.first?.cursor == nil)

    await sut.onRowAppear(.permitted)

    #expect(
      sut.filteredApplications.map(\.id) == [
        PlanningApplication.rejected, .permitted, .withdrawn, .pendingReview,
      ].map(\.id)
    )
    #expect(spy.fetchApplicationsPageCalls.count == 2)
    #expect(spy.fetchApplicationsPageCalls[1].sort == .status)
    #expect(spy.fetchApplicationsPageCalls[1].cursor == "status-cursor-2")
    #expect(spy.fetchApplicationsCalls.isEmpty)
  }

  @Test("status preserves the server order — no local app_state re-sort")
  func statusSort_preservesServerOrder() async throws {
    // Rejected, Undecided, Permitted — not in `status.rawValue` order. The
    // server owns the ordering; a local re-sort would reorder this set.
    let serverOrdered = ApplicationPage(
      applications: [.rejected, .pendingReview, .permitted], nextCursor: nil)
    let (sut, _) = try makeSUT(sort: .status, pages: [serverOrdered])

    await sut.loadApplications()

    #expect(
      sut.filteredApplications.map(\.id) == [
        PlanningApplication.rejected, .pendingReview, .permitted,
      ].map(\.id)
    )
  }

  @Test("switching to status clears the cursor and reloads from page 1")
  func switchingToStatus_resetsCursorAndReloadsPageOne() async throws {
    let newestPage = ApplicationPage(applications: [.pendingReview], nextCursor: "newest-cursor")
    let (sut, spy) = try makeSUT(sort: .newest, pages: [newestPage])

    await sut.loadApplications()
    #expect(spy.fetchApplicationsPageCalls.last?.sort == .newest)

    spy.pagedResponses = [ApplicationPage(applications: [.permitted], nextCursor: nil)]
    sut.sort = .status
    await sut.handleSortChanged()

    let last = try #require(spy.fetchApplicationsPageCalls.last)
    #expect(last.sort == .status)
    #expect(last.cursor == nil)
  }

  @Test("switching away from status clears the cursor and reloads from page 1")
  func switchingFromStatus_resetsCursorAndReloadsPageOne() async throws {
    let statusPage = ApplicationPage(applications: [.permitted], nextCursor: "status-cursor")
    let (sut, spy) = try makeSUT(sort: .status, pages: [statusPage])

    await sut.loadApplications()
    #expect(spy.fetchApplicationsPageCalls.last?.sort == .status)

    spy.pagedResponses = [ApplicationPage(applications: [.rejected], nextCursor: nil)]
    sut.sort = .oldest
    await sut.handleSortChanged()

    let last = try #require(spy.fetchApplicationsPageCalls.last)
    #expect(last.sort == .oldest)
    #expect(last.cursor == nil)
  }

  @Test("switching from client-side recent-activity to status drives the paged endpoint")
  func switchingRecentActivityToStatus_usesPagedFetch() async throws {
    // recent-activity stays client-side (param-less); status is now server-driven.
    let spy = SpyPlanningApplicationRepository()
    spy.fetchApplicationsResult = .success([.pendingReview, .permitted])
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    defaults.set(ApplicationsSort.recentActivity.rawValue, forKey: "test.raToStatus")
    let sut = ApplicationListViewModel(
      repository: spy, zone: .cambridge, userDefaults: defaults, sortKey: "test.raToStatus")

    await sut.loadApplications()
    #expect(spy.fetchApplicationsCalls.count == 1)
    #expect(spy.fetchApplicationsPageCalls.isEmpty)

    spy.pagedResponses = [ApplicationPage(applications: [.rejected, .pendingReview], nextCursor: nil)]
    sut.sort = .status
    await sut.handleSortChanged()

    #expect(spy.fetchApplicationsPageCalls.count == 1)
    #expect(spy.fetchApplicationsPageCalls.first?.sort == .status)
  }

  // MARK: - recent-activity is server-driven (GH#682 slice 3)

  @Test("selecting recent-activity issues ?sort=recent-activity and paginates via the cursor")
  func recentActivitySort_paginatesViaCursor() async throws {
    // Pages are in the server's recent-activity order
    // (GREATEST(start_date, unread.created_at) DESC), deliberately NOT in any
    // local order, so a stray client re-sort would reorder them and fail this
    // assertion. `sort.rawValue` is asserted rather than the enum case so the
    // test compiles before the `ApplicationSortOrder.recentActivity` case lands.
    let page1 = ApplicationPage(applications: [.rejected, .permitted], nextCursor: "ra-cursor-2")
    let page2 = ApplicationPage(applications: [.withdrawn, .pendingReview], nextCursor: nil)
    let (sut, spy) = try makeSUT(sort: .recentActivity, pages: [page1, page2])

    await sut.loadApplications()
    #expect(spy.fetchApplicationsPageCalls.first?.sort.rawValue == "recent-activity")
    #expect(spy.fetchApplicationsPageCalls.first?.cursor == nil)

    await sut.onRowAppear(.permitted)

    #expect(
      sut.filteredApplications.map(\.id) == [
        PlanningApplication.rejected, .permitted, .withdrawn, .pendingReview,
      ].map(\.id)
    )
    #expect(spy.fetchApplicationsPageCalls.count == 2)
    #expect(spy.fetchApplicationsPageCalls[1].sort.rawValue == "recent-activity")
    #expect(spy.fetchApplicationsPageCalls[1].cursor == "ra-cursor-2")
    #expect(spy.fetchApplicationsCalls.isEmpty)
  }

  @Test("recent-activity preserves the server order — no local max(startDate, unread) re-sort")
  func recentActivitySort_preservesServerOrder() async throws {
    // The server owns recent-activity ordering now; the client must render the
    // API order verbatim. This set is deliberately not in any client-derivable
    // order so a leftover `max(startDate, unread.createdAt)` sort would reorder it.
    let serverOrdered = ApplicationPage(
      applications: [.rejected, .pendingReview, .permitted], nextCursor: nil)
    let (sut, _) = try makeSUT(sort: .recentActivity, pages: [serverOrdered])

    await sut.loadApplications()

    #expect(
      sut.filteredApplications.map(\.id) == [
        PlanningApplication.rejected, .pendingReview, .permitted,
      ].map(\.id)
    )
  }

  @Test("switching to recent-activity clears the cursor and reloads from page 1")
  func switchingToRecentActivity_resetsCursorAndReloadsPageOne() async throws {
    let newestPage = ApplicationPage(applications: [.pendingReview], nextCursor: "newest-cursor")
    let (sut, spy) = try makeSUT(sort: .newest, pages: [newestPage])

    await sut.loadApplications()
    #expect(spy.fetchApplicationsPageCalls.last?.sort == .newest)

    spy.pagedResponses = [ApplicationPage(applications: [.permitted], nextCursor: nil)]
    sut.sort = .recentActivity
    await sut.handleSortChanged()

    let last = try #require(spy.fetchApplicationsPageCalls.last)
    #expect(last.sort.rawValue == "recent-activity")
    #expect(last.cursor == nil)
  }

  @Test("switching away from recent-activity clears the cursor and reloads from page 1")
  func switchingFromRecentActivity_resetsCursorAndReloadsPageOne() async throws {
    let raPage = ApplicationPage(applications: [.permitted], nextCursor: "ra-cursor")
    let (sut, spy) = try makeSUT(sort: .recentActivity, pages: [raPage])

    await sut.loadApplications()
    #expect(spy.fetchApplicationsPageCalls.last?.sort.rawValue == "recent-activity")

    spy.pagedResponses = [ApplicationPage(applications: [.rejected], nextCursor: nil)]
    sut.sort = .oldest
    await sut.handleSortChanged()

    let last = try #require(spy.fetchApplicationsPageCalls.last)
    #expect(last.sort == .oldest)
    #expect(last.cursor == nil)
  }
}
