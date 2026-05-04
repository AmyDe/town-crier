/// Abstracts the platform-specific call that sets the app icon's badge count
/// (typically `UIApplication.shared.setBadgeCount(_:)` on iOS 17+ via
/// `UNUserNotificationCenter`), so callers in the domain/presentation layers
/// can drive the badge without importing UIKit.
///
/// Used by ``AppCoordinator`` to reconcile the badge with the server-side
/// `totalUnreadCount` whenever the app enters the foreground (spec
/// `docs/specs/notifications-unread-watermark.md#ios-badge-foreground-push`).
public protocol BadgeSetting: Sendable {
  /// Sets the application icon badge to `count`. Negative values are
  /// implementation-defined; conforming types should clamp to zero.
  func setBadge(_ count: Int)
}
