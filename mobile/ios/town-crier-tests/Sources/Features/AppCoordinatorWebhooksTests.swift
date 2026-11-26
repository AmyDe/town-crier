import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("AppCoordinator webhooks")
@MainActor
struct AppCoordinatorWebhooksTests {
  private let repository = SpyWebhookRepository()
  private let clipboard = SpyClipboardWriter()

  private func makeSUT(cachedTier: SubscriptionTier? = nil) -> AppCoordinator {
    let tierCache = UserDefaults(suiteName: UUID().uuidString) ?? .standard
    tierCache.set(cachedTier?.rawValue, forKey: "cachedSubscriptionTier")
    return AppCoordinator(
      repository: SpyPlanningApplicationRepository(),
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: SpyWatchZoneRepository(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService(),
      tierCache: tierCache,
      webhookRepository: repository,
      clipboardWriter: clipboard
    )
  }

  @Test("showWebhooks presents the screen")
  func showWebhooks_setsFlag() {
    let sut = makeSUT()
    #expect(!sut.isWebhooksPresented)

    sut.showWebhooks()

    #expect(sut.isWebhooksPresented)
  }

  @Test(
    "makeWebhookViewModel passes the current tier",
    arguments: [
      (SubscriptionTier.pro, WebhookViewModel.State.empty),
      (.personal, .upsell),
      (.free, .upsell),
    ])
  func makeWebhookViewModel_passesTier(
    tier: SubscriptionTier, expected: WebhookViewModel.State
  ) async {
    let sut = makeSUT(cachedTier: tier)

    let viewModel = sut.makeWebhookViewModel()
    await viewModel.load()

    #expect(viewModel.state == expected)
    #expect(repository.fetchCallCount == 1)
  }

  @Test("the view model copies through the coordinator's clipboard")
  func makeWebhookViewModel_usesClipboard() async {
    let sut = makeSUT(cachedTier: .pro)
    let viewModel = sut.makeWebhookViewModel()
    await viewModel.rotateSecret()

    viewModel.copySecret()

    #expect(clipboard.copyCalls == ["whsec_rotated"])
  }

  @Test("View Plans opens the Settings paywall")
  func viewPlans_presentsSettingsPaywall() {
    let sut = makeSUT()

    sut.makeWebhookViewModel().viewPlans()

    #expect(sut.isSettingsPaywallPresented)
  }
}
