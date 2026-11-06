import TownCrierPresentation
import UIKit

/// Hosts the UIKit lifecycle hooks SwiftUI does not surface natively: the APNs
/// registration callbacks and a fallback for inbound Universal Links.
final class AppDelegate: NSObject, UIApplicationDelegate {
  /// Set by `TownCrierApp.init()` so the delegate can forward APNs callbacks
  /// to the registrar. Optional because `@UIApplicationDelegateAdaptor`
  /// requires a no-arg init and the registrar is built in the composition root.
  var registrar: PushNotificationRegistrar?

  /// Set by `TownCrierApp.init()` so the Universal Link fallback below can
  /// forward to `AppCoordinator.handleDeepLink`.
  var coordinator: AppCoordinator?

  func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {
    // APNs may rotate tokens between launches, so re-register every launch.
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

  /// UIKit-level Universal Link fallback: SwiftUI's `.onContinueUserActivity`
  /// was observed on a device to sometimes not fire. The system calls this for
  /// both warm continuation and cold launch. Deprecated in iOS 26 in favour of
  /// `scene(_:continue:)`, but kept because SwiftUI owns the scene lifecycle.
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
