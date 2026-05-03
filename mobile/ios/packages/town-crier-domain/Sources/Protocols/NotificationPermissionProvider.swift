/// Abstracts the system's notification permission request (e.g. UNUserNotificationCenter)
/// so that code depending on it can be tested without a live notification centre.
public protocol NotificationPermissionProvider: Sendable {
  func requestPermission() async throws -> Bool

  /// Reports the current notification authorization state, mapped to the
  /// domain-level `NotificationAuthorizationStatus` (collapses
  /// `provisional`/`ephemeral` into `.authorized`).
  func authorizationStatus() async -> NotificationAuthorizationStatus
}
