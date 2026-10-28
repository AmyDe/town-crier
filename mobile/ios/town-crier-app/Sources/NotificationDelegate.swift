import TownCrierPresentation
import UserNotifications

/// Handles notification tap responses for both cold launch and background
/// scenarios.
///
/// Both delegate overloads inherit the class's `@MainActor` isolation so the
/// synthesized `@objc` completion thunk fires on the main thread, as
/// `UNUserNotificationCenter` asserts. Do not reintroduce `nonisolated` here.
///
/// `@preconcurrency` suppresses the non-`Sendable` diagnostics for
/// `UNUserNotificationCenter` / `UNNotificationResponse`; UIKit already
/// delivers them on the main thread.
@MainActor
final class NotificationDelegate: NSObject, @preconcurrency UNUserNotificationCenterDelegate {
  let coordinator: AppCoordinator

  init(coordinator: AppCoordinator) {
    self.coordinator = coordinator
  }

  /// Forwards the APNs `userInfo` payload to `AppCoordinator.handlePushTap`,
  /// which routes the application deep link and marks that application read.
  /// Digest pushes carry no application deep link and no-op.
  func userNotificationCenter(
    _ center: UNUserNotificationCenter,
    didReceive response: UNNotificationResponse
  ) async {
    coordinator.handlePushTap(
      userInfo: response.notification.request.content.userInfo
    )
  }

  func userNotificationCenter(
    _ center: UNUserNotificationCenter,
    willPresent notification: UNNotification
  ) async -> UNNotificationPresentationOptions {
    [.banner, .sound]
  }
}
