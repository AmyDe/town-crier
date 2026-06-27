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

  // MARK: - Client sorts keep the legacy path

  @Test("a client-side sort keeps the param-less fetch and does not paginate")
  func clientSort_usesParamlessFetch() async throws {
    let spy = SpyPlanningApplicationRepository()
    spy.fetchApplicationsResult = .success([.pendingReview, .permitted])
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    defaults.set(ApplicationsSort.recentActivity.rawValue, forKey: "test.clientSort")
    let sut = ApplicationListViewModel(
      repository: spy, zone: .cambridge, userDefaults: defaults, sortKey: "test.clientSort")

    await sut.loadApplications()
    await sut.onRowAppear(.permitted)

    #expect(spy.fetchApplicationsCalls.count == 1)
    #expect(spy.fetchApplicationsPageCalls.isEmpty)
  }
}
