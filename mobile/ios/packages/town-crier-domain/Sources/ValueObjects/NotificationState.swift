import Foundation

/// Snapshot of the caller's notification read state. `totalUnreadCount` is
/// precomputed server-side. `version` bumps on every mark-all-read or
/// per-application mark-read, so clients can detect mutations from other
/// devices. `lastReadAt` is vestigial (kept for DTO-shape stability) and does
/// not drive unread.
public struct NotificationState: Equatable, Sendable {
  public let lastReadAt: Date
  public let version: Int
  public let totalUnreadCount: Int

  public init(lastReadAt: Date, version: Int, totalUnreadCount: Int) {
    self.lastReadAt = lastReadAt
    self.version = version
    self.totalUnreadCount = totalUnreadCount
  }

  public var hasUnread: Bool {
    totalUnreadCount > 0
  }
}
