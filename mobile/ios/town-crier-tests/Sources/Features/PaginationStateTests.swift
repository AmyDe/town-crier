import Testing

@testable import TownCrierPresentation

@Suite("PaginationState")
struct PaginationStateTests {

  // MARK: - Initial state

  @Test("initial state starts at page 1 with no more pages")
  func initialState() {
    let sut = PaginationState()

    #expect(sut.currentPage == 1)
    #expect(!sut.hasMore)
  }

  // MARK: - Reset

  @Test("reset returns to page 1 with no more pages")
  func reset_returnsToInitial() {
    var sut = PaginationState()
    sut.advance(hasMore: true)
    sut.advance(hasMore: true)

    sut.reset()

    #expect(sut.currentPage == 1)
    #expect(!sut.hasMore)
  }

  // MARK: - Advance

  @Test("advance increments page and records hasMore")
  func advance_incrementsPage() {
    var sut = PaginationState()
    sut.startInitialLoad(hasMore: true)

    sut.advance(hasMore: true)

    #expect(sut.currentPage == 2)
    #expect(sut.hasMore)
  }

  @Test("advance sets hasMore false when no more pages")
  func advance_setsHasMoreFalse() {
    var sut = PaginationState()
    sut.startInitialLoad(hasMore: true)

    sut.advance(hasMore: false)

    #expect(sut.currentPage == 2)
    #expect(!sut.hasMore)
  }

  @Test("multiple advances track correct page number")
  func multipleAdvances_trackCorrectPage() {
    var sut = PaginationState()
    sut.startInitialLoad(hasMore: true)
    sut.advance(hasMore: true)
    sut.advance(hasMore: true)
    sut.advance(hasMore: false)

    #expect(sut.currentPage == 4)
    #expect(!sut.hasMore)
  }

  // MARK: - Start initial load

  @Test("startInitialLoad resets to page 1 and records hasMore")
  func startInitialLoad_resetsAndRecordsHasMore() {
    var sut = PaginationState()
    sut.startInitialLoad(hasMore: true)
    sut.advance(hasMore: true)

    sut.startInitialLoad(hasMore: false)

    #expect(sut.currentPage == 1)
    #expect(!sut.hasMore)
  }

  @Test("startInitialLoad with hasMore true enables loading more")
  func startInitialLoad_withHasMore_enablesLoadMore() {
    var sut = PaginationState()

    sut.startInitialLoad(hasMore: true)

    #expect(sut.currentPage == 1)
    #expect(sut.hasMore)
  }

  // MARK: - Next page

  @Test("nextPage returns current page plus one")
  func nextPage_returnsCurrentPlusOne() {
    var sut = PaginationState()
    sut.startInitialLoad(hasMore: true)

    #expect(sut.nextPage == 2)

    sut.advance(hasMore: true)

    #expect(sut.nextPage == 3)
  }
}
