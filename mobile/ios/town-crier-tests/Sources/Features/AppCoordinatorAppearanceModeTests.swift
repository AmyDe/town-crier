import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// GH#878: the composition root injects one shared `AppearanceStore` into
/// `AppCoordinator`, which `makeSettingsViewModel()` must forward rather than
/// let `SettingsViewModel` fall back to a private store — otherwise the
/// Settings picker would silently diverge from the welcome screen's control.
@Suite("AppCoordinator -- Appearance Mode")
@MainActor
struct AppCoordinatorAppearanceModeTests {
  private func makeSUT(appearanceStore: AppearanceStore? = nil) -> AppCoordinator {
    AppCoordinator(
      repository: SpyPlanningApplicationRepository(),
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: SpyWatchZoneRepository(),
      geocoder: SpyPostcodeGeocoder(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService(),
      appearanceStore: appearanceStore
    )
  }

  @Test func makeSettingsViewModel_usesTheInjectedAppearanceStore() {
    let defaults = UserDefaults(suiteName: UUID().uuidString)
    // swiftlint:disable:next force_unwrapping
    let appearanceStore = AppearanceStore(defaults: defaults!)
    appearanceStore.appearanceMode = .oledDark
    let sut = makeSUT(appearanceStore: appearanceStore)

    let vm = sut.makeSettingsViewModel()

    #expect(vm.appearanceMode == .oledDark)
  }

  @Test func makeSettingsViewModel_settingAppearanceMode_updatesTheSameInjectedStore() {
    let defaults = UserDefaults(suiteName: UUID().uuidString)
    // swiftlint:disable:next force_unwrapping
    let appearanceStore = AppearanceStore(defaults: defaults!)
    let sut = makeSUT(appearanceStore: appearanceStore)
    let vm = sut.makeSettingsViewModel()

    vm.appearanceMode = .light

    #expect(appearanceStore.appearanceMode == .light)
  }
}
