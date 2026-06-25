extension AppCoordinator {
  /// Deep-link URL string for the "Rate the App" row in Settings (GH #629).
  /// The `?action=write-review` query opens the App Store straight on Town
  /// Crier's review composer, rather than the listing page.
  ///
  /// Apple ID `6764095657` is Town Crier's stable, published App Store
  /// identifier (also used by the web download CTAs). A manual tap must always
  /// do something, so this row uses the App Store URL rather than
  /// `SKStoreReviewController.requestReview`, which Apple throttles and may
  /// silently suppress (that path belongs to the automatic prompt, GH #628).
  public static let appStoreWriteReviewURLString =
    "https://apps.apple.com/app/id6764095657?action=write-review"
}
