import TownCrierDomain
import UIKit
import UserNotifications

/// Concrete `BadgeSetting` that drives the app icon badge via the iOS 17+
/// `UNUserNotificationCenter.setBadgeCount(_:withCompletionHandler:)` API.
///
/// The legacy `UIApplication.applicationIconBadgeNumber` setter is deprecated
/// in iOS 17 and emits a runtime warning; the notification-center API is the
/// supported replacement. Errors are swallowed because the foreground sync
/// path is best-effort — a transient OS failure should not crash the app.
struct UIApplicationBadgeSetter: BadgeSetting {
  func setBadge(_ count: Int) {
    let clamped = max(0, count)
    UNUserNotificationCenter.current().setBadgeCount(clamped) { _ in }
  }
}
