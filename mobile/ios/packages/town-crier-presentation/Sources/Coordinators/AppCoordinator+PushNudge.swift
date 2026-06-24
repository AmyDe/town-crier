import Foundation
import TownCrierDomain

extension AppCoordinator {
  /// Factory for the home-tab push-permission nudge ViewModel (issue #624,
  /// Prong 2). Injects the current resolved tier and the shared notification
  /// service, and wires the `.denied` action to the existing iOS-Settings deep
  /// link (``showSystemNotificationSettings()``) rather than duplicating it.
  ///
  /// The Applications-tab `NavigationStack` is keyed on `subscriptionTier`, so
  /// the banner (and this freshly built ViewModel) rebuilds with the current
  /// tier whenever a purchase resolves.
  public func makePushNudgeViewModel() -> PushNudgeViewModel {
    PushNudgeViewModel(
      tier: subscriptionTier,
      notificationService: notificationService,
      onOpenSettings: { [weak self] in
        self?.showSystemNotificationSettings()
      }
    )
  }
}
