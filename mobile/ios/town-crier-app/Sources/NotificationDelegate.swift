import TownCrierPresentation
import UserNotifications

/// Handles notification tap responses for both cold launch and background
/// scenarios.
///
/// The class is `@MainActor`; both delegate overloads inherit that isolation
/// (no `nonisolated`). Swift's compiler-synthesized `@objc` thunk that
/// bridges the `async` return to the original `withCompletionHandler:` ObjC
/// selector then fires on the main thread deterministically — which is what
/// UIKit's `UNUserNotificationCenter` asserts (tc-cbmk).
///
/// The prior shape (tc-fcwv) used `nonisolated async` + `await MainActor.run`
/// to hop back to main inside the body. That left a gap: the @objc thunk's
/// completion still fired on whatever cooperative thread the runtime resumed
/// us on. Removing `nonisolated` and letting class-level MainActor
/// inheritance carry the body is Apple's WWDC23 guidance for
/// `UNUserNotificationCenterDelegate` async overloads on a `@MainActor`
/// class. Do not reintroduce `nonisolated` here.
@MainActor
final class NotificationDelegate: NSObject, UNUserNotificationCenterDelegate {
  let coordinator: AppCoordinator

  init(coordinator: AppCoordinator) {
    self.coordinator = coordinator
  }

  /// Called when the user taps a notification (cold launch or background).
  /// Forwards the APNs `userInfo` payload to `AppCoordinator.handlePushTap`,
  /// which parses the deep link and the watermark instant independently.
  /// Each branch is no-oppable: digest pushes carry neither field, older
  /// builds may omit `createdAt`.
  func userNotificationCenter(
    _ center: UNUserNotificationCenter,
    didReceive response: UNNotificationResponse
  ) async {
    coordinator.handlePushTap(
      userInfo: response.notification.request.content.userInfo
    )
  }

  /// Show notification banners when the app is in the foreground.
  func userNotificationCenter(
    _ center: UNUserNotificationCenter,
    willPresent notification: UNNotification
  ) async -> UNNotificationPresentationOptions {
    [.banner, .sound]
  }
}
