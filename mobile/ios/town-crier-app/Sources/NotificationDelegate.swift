import TownCrierPresentation
import UserNotifications

/// Handles notification tap responses for both cold launch and background scenarios.
///
/// Both `userNotificationCenter` overloads are declared `nonisolated async`
/// because the conforming protocol API is not isolated to MainActor. Swift's
/// compiler-synthesized `@objc` thunk calls the original ObjC
/// `withCompletionHandler:` selector when the function returns; UIKit asserts
/// that completion runs on the main thread. We therefore wrap each body in
/// `await MainActor.run { ... }` so the function always returns on MainActor
/// regardless of which path is taken (tc-fcwv).
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
    // Parse the (non-Sendable) userInfo dict on the calling actor and hand
    // only a Sendable `DeepLink?` across the MainActor hop. The hop must run
    // unconditionally — even when no deep link is parsed — because Swift's
    // synthesized @objc thunk fires the original ObjC completion handler
    // when this function returns, and UIKit asserts that completion runs
    // on the main thread (tc-fcwv).
    let userInfo = response.notification.request.content.userInfo
    let deepLink = NotificationPayloadParser.parseDeepLink(from: userInfo)
    await MainActor.run {
      guard let deepLink else { return }
      coordinator.handleDeepLink(deepLink)
    }
  }

  /// Show notification banners when the app is in the foreground.
  nonisolated func userNotificationCenter(
    _ center: UNUserNotificationCenter,
    willPresent notification: UNNotification
  ) async -> UNNotificationPresentationOptions {
    await MainActor.run {
      [.banner, .sound]
    }
  }
}
