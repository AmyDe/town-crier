import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("AppCoordinator — subscription paywall")
@MainActor
struct AppCoordinatorSubscriptionTests {
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

  @Test("isSubscriptionPresented starts false")
  func isSubscriptionPresented_startsFalse() {
    let sut = makeSUT()

    #expect(!sut.isSubscriptionPresented)
  }

  @Test("tapping View Plans in the watch-zone list presents the paywall")
  func onViewPlans_presentsPaywall() {
    let sut = makeSUT()
    let listViewModel = sut.makeWatchZoneListViewModel()

    listViewModel.onViewPlans?()

    #expect(sut.isSubscriptionPresented)
  }

  @Test("makeSubscriptionViewModel builds a SubscriptionViewModel for the paywall sheet")
  func makeSubscriptionViewModel_buildsViewModel() async {
    let sut = makeSUT()

    let viewModel = sut.makeSubscriptionViewModel()
    _ = SubscriptionView(viewModel: viewModel)

    #expect(!viewModel.isSubscribed)
  }
}
