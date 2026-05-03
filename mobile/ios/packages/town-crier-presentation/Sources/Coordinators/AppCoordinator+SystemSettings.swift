#if canImport(UIKit)
  import UIKit
#endif

extension AppCoordinator {
  /// Deep-link URL string used by the view layer to open the iOS Notifications
  /// subpage for this app (Settings -> Town Crier -> Notifications), rather
  /// than the general per-app Settings page.
  ///
  /// Resolves to `UIApplication.openNotificationSettingsURLString` on iOS 16+
  /// (the deployment target is iOS 17, so no availability guard is required).
  /// On non-UIKit platforms the literal `"app-settings:notification"` is used
  /// so the package still builds for cross-platform testing.
  public static let systemNotificationSettingsURLString: String = {
    #if canImport(UIKit)
      return UIApplication.openNotificationSettingsURLString
    #else
      return "app-settings:notification"
    #endif
  }()
}
