import TownCrierDomain
import UserNotifications

/// Adapts `UNUserNotificationCenter` to the `NotificationPermissionProvider` protocol,
/// bridging the system notification permission request into the domain layer.
struct UNNotificationPermissionProvider: NotificationPermissionProvider {
  func requestPermission() async throws -> Bool {
    try await UNUserNotificationCenter.current().requestAuthorization(options: [
      .alert, .sound, .badge,
    ])
  }

  func authorizationStatus() async -> NotificationAuthorizationStatus {
    let settings = await UNUserNotificationCenter.current().notificationSettings()
    switch settings.authorizationStatus {
    case .notDetermined:
      return .notDetermined
    case .denied:
      return .denied
    case .authorized, .provisional, .ephemeral:
      return .authorized
    @unknown default:
      return .denied  // fail closed
    }
  }
}
