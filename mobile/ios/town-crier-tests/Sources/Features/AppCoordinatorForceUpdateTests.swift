import Testing

@testable import TownCrierPresentation

@Suite("AppCoordinator — force update")
@MainActor
struct AppCoordinatorForceUpdateTests {
  @Test func appStoreForceUpdateURLString_usesRealAppStoreID() {
    #expect(
      AppCoordinator.appStoreForceUpdateURLString
        == "https://apps.apple.com/app/town-crier/id6764095657"
    )
  }
}
