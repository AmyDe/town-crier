import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("AppCoordinator — View Plans from Settings")
@MainActor
struct AppCoordinatorViewPlansTests {
  private func makeSUT() -> AppCoordinator {
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
      versionConfigService: SpyVersionConfigService()
    )
  }

  @Test func isSettingsPaywallPresented_isFalseByDefault() {
    let sut = makeSUT()

    #expect(!sut.isSettingsPaywallPresented)
  }

  @Test func showPlansFromSettings_setsIsSettingsPaywallPresentedToTrue() {
    let sut = makeSUT()

    sut.showPlansFromSettings()

    #expect(sut.isSettingsPaywallPresented)
  }
}
