import Foundation

/// A paginated page of notification items from the API.
public struct NotificationPage: Equatable, Sendable {
  public let notifications: [NotificationItem]
  public let total: Int
  public let page: Int

  public init(notifications: [NotificationItem], total: Int, page: Int) {
    self.notifications = notifications
    self.total = total
    self.page = page
  }

  /// Whether more pages of results are available beyond the current page.
  public var hasMore: Bool {
    let pageSize = notifications.count
    guard pageSize > 0 else { return false }
    return page * pageSize < total
  }
}
