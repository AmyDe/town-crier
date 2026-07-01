import Foundation

/// Snapshot of the caller's notification read state.
///
/// The server returns `totalUnreadCount` precomputed (the `read_at IS NULL`
/// tally) so the client can drive the app icon badge and the
/// Applications-screen Unread chip without rescanning the notification list
/// locally.
///
/// `version` is the aggregate's mutation counter from the server: it bumps on
/// every successful mark-all-read or per-application mark-read, letting the
/// client detect out-of-band mutations across devices. `lastReadAt` is
/// vestigial (retained for DTO-shape stability) and no longer drives unread.
///
/// See ADR 0035 (`docs/adr/0035-per-application-notification-read-state.md`).
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
