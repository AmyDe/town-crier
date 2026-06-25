import TownCrierDomain

extension AppCoordinator {
  /// Records that the app was foregrounded, feeding the review-prompt loyalty
  /// signal (GH #628). `isReactivation` is `true` only for a background→active
  /// re-entry, never the cold-launch render.
  public func recordAppForegrounded(isReactivation: Bool) {
    reviewPromptTracker?.recordAppForegrounded(isReactivation: isReactivation)
  }
}
