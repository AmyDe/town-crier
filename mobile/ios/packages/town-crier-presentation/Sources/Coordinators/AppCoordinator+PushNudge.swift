import Foundation
import TownCrierDomain

extension AppCoordinator {
  /// Builds the home-tab push-permission nudge ViewModel; `.denied` opens
  /// ``showSystemNotificationSettings()``.
  public func makePushNudgeViewModel() -> PushNudgeViewModel {
    PushNudgeViewModel(
      tier: subscriptionTier,
      notificationService: notificationService
    ) { [weak self] in
      self?.showSystemNotificationSettings()
    }
  }
}
