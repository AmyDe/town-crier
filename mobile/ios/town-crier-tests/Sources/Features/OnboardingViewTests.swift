import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@MainActor
@Suite("OnboardingView")
struct OnboardingViewTests {

  // Smoke tests: the wizard container is a passive renderer. Verify it
  // constructs and its body evaluates, including the in-wizard radius upsell
  // sheet path (tc-w3cb.3).

  private func makeViewModel(tier: SubscriptionTier = .free) -> OnboardingViewModel {
    OnboardingViewModel(
      geocoder: SpyPostcodeGeocoder(),
      watchZoneRepository: SpyWatchZoneRepository(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      subscriptionTier: tier
    )
  }

  @Test func body_renders() {
    let sut = OnboardingView(viewModel: makeViewModel())
    _ = sut.body
  }

  @Test func body_renders_whenRadiusUpsellPresented() {
    let vm = makeViewModel(tier: .free)
    vm.makeUpsellViewModel = {
      SubscriptionViewModel(
        subscriptionService: SpySubscriptionService(),
        authenticationService: SpyAuthenticationService()
      )
    }
    vm.requestLargerRadiusUpgrade()
    #expect(vm.isRadiusUpsellPresented)

    let sut = OnboardingView(viewModel: vm)
    _ = sut.body
  }
}
