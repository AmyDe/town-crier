/// Tracks page number and availability of further pages for paginated API calls.
///
/// Used by ``NotificationsViewModel`` (and any future paginated list): start at
/// page 1, record whether the last response had more pages, and advance on
/// `loadMore()`. This value type centralises that state so ViewModels delegate
/// to it instead of duplicating the bookkeeping.
///
/// Usage:
/// ```swift
/// private var pagination = PaginationState()
///
/// func load() async {
///     pagination.reset()
///     let result = try await repository.fetch(page: 1, ...)
///     pagination.startInitialLoad(hasMore: result.hasMore)
/// }
///
/// func loadMore() async {
///     guard pagination.hasMore else { return }
///     let result = try await repository.fetch(page: pagination.nextPage, ...)
///     pagination.advance(hasMore: result.hasMore)
/// }
/// ```
public struct PaginationState {

  /// The page number of the most recently loaded page.
  public private(set) var currentPage: Int

  /// Whether the most recent response indicated more pages are available.
  public private(set) var hasMore: Bool

  /// The page number that should be requested next.
  public var nextPage: Int {
    currentPage + 1
  }

  public init() {
    currentPage = 1
    hasMore = false
  }

  /// Resets pagination to its initial state (page 1, no more pages).
  public mutating func reset() {
    currentPage = 1
    hasMore = false
  }

  /// Records the result of the initial (page 1) load.
  ///
  /// This resets the current page to 1 and records whether additional pages
  /// are available.
  public mutating func startInitialLoad(hasMore: Bool) {
    currentPage = 1
    self.hasMore = hasMore
  }

  /// Records a successful load of the next page.
  ///
  /// Increments the current page and records whether additional pages
  /// are available.
  public mutating func advance(hasMore: Bool) {
    currentPage += 1
    self.hasMore = hasMore
  }
}
