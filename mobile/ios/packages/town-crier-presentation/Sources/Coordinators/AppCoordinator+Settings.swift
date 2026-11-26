extension AppCoordinator {
  public func showPrivacyPolicy() {
    presentedLegalDocument = .privacyPolicy
  }

  public func showTermsOfService() {
    presentedLegalDocument = .termsOfService
  }

  public func showManageSubscription() {
    isManageSubscriptionPresented = true
  }

  /// Presents the subscription paywall nested inside the Settings sheet.
  public func showPlansFromSettings() {
    isSettingsPaywallPresented = true
  }

  /// Presents the Settings view as a sheet from any tab. Bound to the gear
  /// icon installed via the `.settingsToolbar` ViewModifier.
  public func showSettings() {
    isSettingsPresented = true
  }

  /// Deep-links to iOS system Settings (push permissions). Coordinator stays
  /// UIKit-free; `TownCrierApp` observes the flag and opens the settings URL.
  public func showSystemNotificationSettings() {
    isOpeningSystemNotificationSettings = true
  }
}
