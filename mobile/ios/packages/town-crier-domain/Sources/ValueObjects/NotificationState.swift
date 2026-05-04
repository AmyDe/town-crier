import Foundation

/// Snapshot of the caller's notification read-state watermark.
///
/// A notification is "unread" iff its `createdAt` is strictly after
/// `lastReadAt`. The server returns `totalUnreadCount` precomputed so the
/// client can drive the app icon badge and the Applications-screen Unread
/// chip without rescanning the notification list locally.
///
/// `version` is the aggregate's mutation counter from the server: it bumps
/// on every successful mark-all-read or advance, letting the client detect
/// out-of-band mutations across devices.
///
/// Spec: `docs/specs/notifications-unread-watermark.md#api-surface`.
public struct NotificationState: Equatable, Sendable {
  public let lastReadAt: Date
  public let version: Int
  public let totalUnreadCount: Int

  public init(lastReadAt: Date, version: Int, totalUnreadCount: Int) {
    self.lastReadAt = lastReadAt
    self.version = version
    self.totalUnreadCount = totalUnreadCount
  }

  /// Whether the user has at least one unread notification.
  public var hasUnread: Bool {
    totalUnreadCount > 0
  }
}
