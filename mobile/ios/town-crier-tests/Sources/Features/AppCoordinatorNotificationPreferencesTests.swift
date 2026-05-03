import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Tests covering the in-app notification preferences wiring on
/// `AppCoordinator` — the navigation flag, the view-model factory, and the
/// selected-tab binding the View layer writes to when the user taps the
/// "Watch Zones" navigation row from the preferences screen.
@Suite("AppCoordinator — Notification Preferences")
@MainActor
struct AppCoordinatorNotificationPreferencesTests {
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

  @Test func isNotificationPreferencesPresented_isFalseByDefault() {
    let sut = makeSUT()

    #expect(!sut.isNotificationPreferencesPresented)
  }

  @Test func showNotificationPreferences_setsFlagToTrue() {
    let sut = makeSUT()

    sut.showNotificationPreferences()

    #expect(sut.isNotificationPreferencesPresented)
  }

  @Test func makeNotificationPreferencesViewModel_returnsViewModelWithDefaults() {
    let sut = makeSUT()

    let vm = sut.makeNotificationPreferencesViewModel()

    #expect(vm.savedDecisionPush == true)
    #expect(vm.savedDecisionEmail == true)
    #expect(vm.emailDigestEnabled == true)
    #expect(vm.digestDay == .monday)
    #expect(vm.watchZoneCount == 0)
  }

  // MARK: - Selected Tab

  @Test func selectedTab_defaultsToApplications() {
    let sut = makeSUT()

    #expect(sut.selectedTab == .applications)
  }

  @Test func selectedTab_canBeSetToZones() {
    let sut = makeSUT()

    sut.selectedTab = .zones

    #expect(sut.selectedTab == .zones)
  }
}
