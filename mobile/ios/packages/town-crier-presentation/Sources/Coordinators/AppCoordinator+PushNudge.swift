import Foundation
import TownCrierDomain

extension AppCoordinator {
  /// Builds the home-tab push-permission nudge ViewModel; `.denied` opens
  /// ``showSystemNotificationSettings()``.
  ///
  /// The Applications-tab `NavigationStack` is keyed on `subscriptionTier`, so
  /// the banner (and this freshly built ViewModel) rebuilds with the current
  /// tier whenever a purchase resolves.
  public func makePushNudgeViewModel() -> PushNudgeViewModel {
    PushNudgeViewModel(
      tier: subscriptionTier,
      notificationService: notificationService
    ) { [weak self] in
      self?.showSystemNotificationSettings()
    }
  }
}
