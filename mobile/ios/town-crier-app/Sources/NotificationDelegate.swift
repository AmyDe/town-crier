import TownCrierPresentation
import UserNotifications

/// Handles notification tap responses for both cold launch and background scenarios.
@MainActor
final class NotificationDelegate: NSObject, UNUserNotificationCenterDelegate {
  private let coordinator: AppCoordinator

  init(coordinator: AppCoordinator) {
    self.coordinator = coordinator
  }

  /// Called when the user taps a notification (cold launch or background).
  nonisolated func userNotificationCenter(
    _ center: UNUserNotificationCenter,
    didReceive response: UNNotificationResponse
  ) async {
    let userInfo = response.notification.request.content.userInfo
    guard let deepLink = NotificationPayloadParser.parseDeepLink(from: userInfo) else {
      return
    }
    await coordinator.handleDeepLink(deepLink)
  }

  /// Show notification banners when the app is in the foreground.
  nonisolated func userNotificationCenter(
    _ center: UNUserNotificationCenter,
    willPresent notification: UNNotification
  ) async -> UNNotificationPresentationOptions {
    [.banner, .sound]
  }
}
