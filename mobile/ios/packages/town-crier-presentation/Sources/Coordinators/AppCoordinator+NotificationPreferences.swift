import Foundation
import TownCrierDomain

/// The four primary tabs in the root `TabView`. Defined as an enum so the
/// coordinator can drive tab selection (e.g. tapping "Watch Zones" inside
/// the in-app notification preferences screen) without the View layer
/// plumbing tab indices around.
public enum MainTab: Hashable, Sendable {
  case applications
  case saved
  case map
  case zones
}

extension AppCoordinator {
  /// Presents the in-app `NotificationPreferencesView` from Settings. The
  /// Settings sheet observes `isNotificationPreferencesPresented` and pushes
  /// the screen via `.navigationDestination(isPresented:)`.
  public func showNotificationPreferences() {
    isNotificationPreferencesPresented = true
  }

  /// Factory for the in-app notification-preferences ViewModel. Injects the
  /// shared profile + watch-zone repositories so the screen can load the
  /// current preferences and the watch-zone count badge in one task.
  public func makeNotificationPreferencesViewModel() -> NotificationPreferencesViewModel {
    NotificationPreferencesViewModel(
      userProfileRepository: userProfileRepository,
      watchZoneRepository: watchZoneRepository
    )
  }
}
