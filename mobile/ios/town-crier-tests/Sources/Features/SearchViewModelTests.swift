import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("SearchViewModel")
@MainActor
struct SearchViewModelTests {

  // MARK: - Helpers

  private func makeSUT(
    tier: SubscriptionTier = .pro,
    applications: [PlanningApplication] = [],
    total: Int? = nil
  ) -> (SearchViewModel, SpySearchRepository) {
    let spy = SpySearchRepository()
    spy.searchResult = .success(
      SearchResult(
        applications: applications,
        total: total ?? applications.count,
        page: 1
      )
    )
    let vm = SearchViewModel(
      repository: spy,
      featureGate: FeatureGate(tier: tier)
    )
    return (vm, spy)
  }

  // MARK: - Basic Search

  @Test("search populates applications on success")
  func search_populatesApplicationsOnSuccess() async {
    let expected = [PlanningApplication.pendingReview, .permitted]
    let (sut, _) = makeSUT(applications: expected)
    sut.selectedAuthorityId = 123
    sut.query = "extension"

    await sut.search()

    #expect(sut.applications == expected)
    #expect(!sut.isLoading)
    #expect(sut.error == nil)
  }

  @Test("search sends correct parameters to repository")
  func search_sendsCorrectParameters() async {
    let (sut, spy) = makeSUT()
    sut.selectedAuthorityId = 456
    sut.query = "rear extension"

    await sut.search()

    #expect(spy.searchCalls.count == 1)
    #expect(spy.searchCalls[0].query == "rear extension")
    #expect(spy.searchCalls[0].authorityId == 456)
    #expect(spy.searchCalls[0].page == 1)
  }

  @Test("search sets isLoading false after completion")
  func search_setsIsLoadingFalseAfterCompletion() async {
    let (sut, _) = makeSUT()
    sut.selectedAuthorityId = 123
    sut.query = "test"

    await sut.search()

    #expect(!sut.isLoading)
  }

  @Test("search sets error on failure")
  func search_setsErrorOnFailure() async {
    let (sut, spy) = makeSUT()
    spy.searchResult = .failure(DomainError.networkUnavailable)
    sut.selectedAuthorityId = 123
    sut.query = "test"

    await sut.search()

    #expect(sut.error == .networkUnavailable)
    #expect(sut.applications.isEmpty)
  }

  @Test("search clears error on retry")
  func search_clearsErrorOnRetry() async {
    let (sut, spy) = makeSUT()
    spy.searchResult = .failure(DomainError.networkUnavailable)
    sut.selectedAuthorityId = 123
    sut.query = "test"
    await sut.search()

    spy.searchResult = .success(
      SearchResult(applications: [.pendingReview], total: 1, page: 1)
    )
    await sut.search()

    #expect(sut.error == nil)
    #expect(sut.applications.count == 1)
  }

  @Test("search does nothing when query is empty")
  func search_emptyQuery_doesNothing() async {
    let (sut, spy) = makeSUT()
    sut.selectedAuthorityId = 123
    sut.query = ""

    await sut.search()

    #expect(spy.searchCalls.isEmpty)
  }

  @Test("search does nothing when no authority selected")
  func search_noAuthority_doesNothing() async {
    let (sut, spy) = makeSUT()
    sut.selectedAuthorityId = nil
    sut.query = "extension"

    await sut.search()

    #expect(spy.searchCalls.isEmpty)
  }

  @Test("search trims whitespace from query")
  func search_trimsWhitespace() async {
    let (sut, spy) = makeSUT()
    sut.selectedAuthorityId = 123
    sut.query = "  extension  "

    await sut.search()

    #expect(spy.searchCalls.count == 1)
    #expect(spy.searchCalls[0].query == "extension")
  }

  // MARK: - Pagination

  @Test("search sets total and hasMore from result")
  func search_setsTotalAndHasMore() async {
    let (sut, spy) = makeSUT()
    spy.searchResult = .success(
      SearchResult(
        applications: [.pendingReview, .permitted],
        total: 10,
        page: 1
      )
    )
    sut.selectedAuthorityId = 123
    sut.query = "test"

    await sut.search()

    #expect(sut.total == 10)
    #expect(sut.hasMore)
  }

  @Test("loadMore appends next page results")
  func loadMore_appendsNextPageResults() async {
    let (sut, spy) = makeSUT()
    spy.searchResult = .success(
      SearchResult(
        applications: [.pendingReview],
        total: 3,
        page: 1
      )
    )
    sut.selectedAuthorityId = 123
    sut.query = "test"
    await sut.search()

    spy.searchResult = .success(
      SearchResult(
        applications: [.permitted],
        total: 3,
        page: 2
      )
    )
    await sut.loadMore()

    #expect(sut.applications.count == 2)
    #expect(sut.applications[0].id == PlanningApplication.pendingReview.id)
    #expect(sut.applications[1].id == PlanningApplication.permitted.id)
    #expect(spy.searchCalls.last?.page == 2)
  }

  @Test("loadMore does nothing when no more pages")
  func loadMore_noMorePages_doesNothing() async {
    let (sut, spy) = makeSUT()
    spy.searchResult = .success(
      SearchResult(
        applications: [.pendingReview],
        total: 1,
        page: 1
      )
    )
    sut.selectedAuthorityId = 123
    sut.query = "test"
    await sut.search()

    await sut.loadMore()

    #expect(spy.searchCalls.count == 1)
  }

  // MARK: - New search resets pagination

  @Test("new search resets results and page")
  func newSearch_resetsResultsAndPage() async {
    let (sut, spy) = makeSUT()
    spy.searchResult = .success(
      SearchResult(applications: [.pendingReview], total: 3, page: 1)
    )
    sut.selectedAuthorityId = 123
    sut.query = "test"
    await sut.search()

    spy.searchResult = .success(
      SearchResult(applications: [.permitted], total: 1, page: 1)
    )
    sut.query = "different"
    await sut.search()

    #expect(sut.applications.count == 1)
    #expect(sut.applications[0].id == PlanningApplication.permitted.id)
    #expect(spy.searchCalls.last?.page == 1)
  }

  // MARK: - Proactive Gating

  @Test("isSearchEnabled returns true for pro tier")
  func isSearchEnabled_proTier_returnsTrue() {
    let (sut, _) = makeSUT(tier: .pro)

    #expect(sut.isSearchEnabled)
  }

  @Test("isSearchEnabled returns false for free tier")
  func isSearchEnabled_freeTier_returnsFalse() {
    let (sut, _) = makeSUT(tier: .free)

    #expect(!sut.isSearchEnabled)
  }

  @Test("isSearchEnabled returns false for personal tier")
  func isSearchEnabled_personalTier_returnsFalse() {
    let (sut, _) = makeSUT(tier: .personal)

    #expect(!sut.isSearchEnabled)
  }

  @Test("search for free user triggers entitlement gate instead of searching")
  func search_freeTier_triggersEntitlementGate() async {
    let (sut, spy) = makeSUT(tier: .free)
    sut.selectedAuthorityId = 123
    sut.query = "extension"

    await sut.search()

    #expect(spy.searchCalls.isEmpty)
    #expect(sut.entitlementGate == .searchApplications)
  }

  // MARK: - Reactive 403 Fallback

  @Test("search with insufficientEntitlement error triggers entitlement gate")
  func search_insufficientEntitlement_triggersGate() async {
    let (sut, spy) = makeSUT(tier: .pro)
    spy.searchResult = .failure(
      DomainError.insufficientEntitlement(required: "searchApplications")
    )
    sut.selectedAuthorityId = 123
    sut.query = "test"

    await sut.search()

    #expect(sut.entitlementGate == .searchApplications)
    #expect(sut.error == nil)
  }

  // MARK: - Empty State

  @Test("isEmpty is true when search returns no results and not loading")
  func isEmpty_noResults_returnsTrue() async {
    let (sut, _) = makeSUT()
    sut.selectedAuthorityId = 123
    sut.query = "nonexistent"
    await sut.search()

    #expect(sut.isEmpty)
  }

  @Test("isEmpty is false before searching")
  func isEmpty_beforeSearch_returnsFalse() {
    let (sut, _) = makeSUT()

    #expect(!sut.isEmpty)
  }

  @Test("isEmpty is false when results exist")
  func isEmpty_withResults_returnsFalse() async {
    let (sut, _) = makeSUT(applications: [.pendingReview])
    sut.selectedAuthorityId = 123
    sut.query = "test"
    await sut.search()

    #expect(!sut.isEmpty)
  }

  // MARK: - Application selection

  @Test("selectApplication notifies callback")
  func selectApplication_notifiesCallback() {
    var selectedId: PlanningApplicationId?
    let (sut, _) = makeSUT()
    sut.onApplicationSelected = { selectedId = $0 }

    sut.selectApplication(PlanningApplicationId("APP-001"))

    #expect(selectedId == PlanningApplicationId("APP-001"))
  }
}
