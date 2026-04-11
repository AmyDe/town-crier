import Foundation

/// The result of a planning application search, including pagination metadata.
public struct SearchResult: Equatable, Sendable {
  public let applications: [PlanningApplication]
  public let total: Int
  public let page: Int

  public init(applications: [PlanningApplication], total: Int, page: Int) {
    self.applications = applications
    self.total = total
    self.page = page
  }

  /// Whether more pages of results are available beyond the current page.
  public var hasMore: Bool {
    let pageSize = applications.count
    guard pageSize > 0 else { return false }
    return page * pageSize < total
  }
}
