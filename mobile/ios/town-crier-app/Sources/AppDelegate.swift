import TownCrierPresentation
import UIKit

/// Hosts the UIKit lifecycle hooks SwiftUI does not surface natively —
/// specifically the APNs registration callbacks. Forwards captured device
/// tokens (and registration failures) to the `PushNotificationRegistrar`.
final class AppDelegate: NSObject, UIApplicationDelegate {
  /// Set by `TownCrierApp.init()` so the delegate can forward APNs callbacks
  /// to the registrar. Optional because `@UIApplicationDelegateAdaptor`
  /// requires a no-arg init and the registrar is built in the composition root.
  var registrar: PushNotificationRegistrar?

  func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]? = nil
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
}
