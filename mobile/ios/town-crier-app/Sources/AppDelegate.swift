import TownCrierPresentation
import UIKit

/// Hosts the UIKit lifecycle hooks SwiftUI does not surface natively —
/// the APNs registration callbacks, and (tc-28x2) a belt-and-braces
/// fallback for inbound Universal Links. Forwards captured device tokens
/// (and registration failures) to the `PushNotificationRegistrar`, and
/// forwards continued browsing activities to the `AppCoordinator`.
final class AppDelegate: NSObject, UIApplicationDelegate {
  /// Set by `TownCrierApp.init()` so the delegate can forward APNs callbacks
  /// to the registrar. Optional because `@UIApplicationDelegateAdaptor`
  /// requires a no-arg init and the registrar is built in the composition root.
  var registrar: PushNotificationRegistrar?

  /// Set by `TownCrierApp.init()`, mirroring `registrar` above, so the
  /// UIKit-level Universal Link fallback below can forward to the existing
  /// `AppCoordinator.handleDeepLink` seam (tc-28x2).
  var coordinator: AppCoordinator?

  func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {
    // Re-register on every launch so the backend always has the freshest
    // token. APNs may rotate tokens between launches; `didRegister...` will
    // be called with the (possibly new) token.
    application.registerForRemoteNotifications()
    return true
  }

  func application(
    _ application: UIApplication,
    didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data
  ) {
    guard let registrar else { return }
    Task { await registrar.didReceiveDeviceToken(deviceToken) }
  }

  func application(
    _ application: UIApplication,
    didFailToRegisterForRemoteNotificationsWithError error: Error
  ) {
    Task { await registrar?.didFailToRegister(error: error) }
  }

  /// UIKit-level Universal Link fallback (tc-28x2, GH #763 Problem 1).
  /// SwiftUI's `.onContinueUserActivity` (TownCrierApp.swift) is the
  /// primary handler, but was confirmed on a physical device to sometimes
  /// not fire at all — this is the belt-and-braces second path. Apple's
  /// docs state the system calls this method for both a warm continuation
  /// (app already running) and a cold launch (after
  /// `application(_:didFinishLaunchingWithOptions:)` returns), so one
  /// implementation covers both cases. Reuses the existing
  /// `UniversalLinkParser` / `AppCoordinator.handleDeepLink` seam — parsing
  /// is not duplicated here.
  ///
  /// Deprecated by Apple in iOS 26 in favour of `UIScene`'s
  /// `scene(_:continue:)`, but still functional and the lowest-risk
  /// fallback given this app has no custom `UISceneDelegate` (SwiftUI owns
  /// scene lifecycle here) — kept for the iOS 17+ deployment target.
  ///
  /// `@escaping` below is required even though this implementation never
  /// calls `restorationHandler`: the closure is bridged from an
  /// Objective-C block in the `UIApplicationDelegate` protocol requirement,
  /// so dropping the keyword risks silently failing to satisfy that
  /// requirement (i.e. never being called by UIKit at all).
  func application(
    _ application: UIApplication,
    continue userActivity: NSUserActivity,
    // swiftlint:disable:next unneeded_escaping
    restorationHandler: @escaping ([any UIUserActivityRestoring]?) -> Void
  ) -> Bool {
    guard let deepLink = UniversalLinkParser.parse(userActivity) else { return false }
    Task { @MainActor [weak self] in
      self?.coordinator?.handleDeepLink(deepLink)
    }
    return true
  }
}
