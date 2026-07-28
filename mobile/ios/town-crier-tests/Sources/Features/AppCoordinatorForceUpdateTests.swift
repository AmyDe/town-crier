import Testing

@testable import TownCrierPresentation

@Suite("AppCoordinator — force update")
@MainActor
struct AppCoordinatorForceUpdateTests {
  /// The force-update blocking screen must deep-link to Town Crier's real,
  /// published App Store listing (Apple ID `6764095657`), not a placeholder
  /// ID (tc-moku).
  @Test func appStoreForceUpdateURLString_usesRealAppStoreID() {
    #expect(
      AppCoordinator.appStoreForceUpdateURLString
        == "https://apps.apple.com/app/town-crier/id6764095657"
    )
  }
}
