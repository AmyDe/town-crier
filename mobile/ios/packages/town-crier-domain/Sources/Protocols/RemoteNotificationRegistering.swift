/// Abstracts the platform-specific call that asks the OS to register the
/// device with APNs (typically `UIApplication.shared.registerForRemoteNotifications()`),
/// so that callers in the domain/data layers can trigger registration without
/// importing UIKit.
public protocol RemoteNotificationRegistering: Sendable {
  /// Asks the operating system to register this device with APNs. The result
  /// is delivered asynchronously via `application(_:didRegisterForRemoteNotificationsWithDeviceToken:)`
  /// or `application(_:didFailToRegisterForRemoteNotificationsWithError:)`.
  func registerForRemoteNotifications()
}
