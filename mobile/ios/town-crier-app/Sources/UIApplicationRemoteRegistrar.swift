import TownCrierDomain
import UIKit

/// Concrete `RemoteNotificationRegistering` that asks `UIApplication` to
/// register the device with APNs. Hopping to the main actor is required
/// because `UIApplication` is `@MainActor`-isolated.
struct UIApplicationRemoteRegistrar: RemoteNotificationRegistering {
  func registerForRemoteNotifications() {
    Task { @MainActor in
      UIApplication.shared.registerForRemoteNotifications()
    }
  }
}
