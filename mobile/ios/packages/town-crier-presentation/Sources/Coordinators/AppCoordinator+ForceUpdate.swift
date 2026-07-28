extension AppCoordinator {
  /// Deep-link URL for the "Update Now" button on the blocking force-update
  /// screen. Apple ID `6764095657` is Town Crier's stable, published App
  /// Store identifier (also used by ``appStoreWriteReviewURLString`` and the
  /// web download CTAs) — not the `id000000000` placeholder (tc-moku).
  public static let appStoreForceUpdateURLString =
    "https://apps.apple.com/app/town-crier/id6764095657"
}
