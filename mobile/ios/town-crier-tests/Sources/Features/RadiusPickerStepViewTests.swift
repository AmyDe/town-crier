import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@MainActor
@Suite("RadiusPickerStepView")
struct RadiusPickerStepViewTests {

  // The step is a passive renderer of OnboardingViewModel state. These tests
  // verify the slider-based step constructs and its body evaluates across the
  // tier range it must support (tc-w3cb.2).

  private func makeViewModel(tier: SubscriptionTier) -> OnboardingViewModel {
    OnboardingViewModel(
      geocoder: SpyPostcodeGeocoder(),
      watchZoneRepository: SpyWatchZoneRepository(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      subscriptionTier: tier
    )
  }

  @Test func body_renders_forFreeTier() {
    let sut = RadiusPickerStepView(viewModel: makeViewModel(tier: .free))
    _ = sut.body
  }

  @Test func body_renders_forPersonalTier() {
    let sut = RadiusPickerStepView(viewModel: makeViewModel(tier: .personal))
    _ = sut.body
  }

  @Test func body_renders_forProTier() {
    let sut = RadiusPickerStepView(viewModel: makeViewModel(tier: .pro))
    _ = sut.body
  }

  @Test func body_renders_whenLargeRadiusWarningShown() {
    let vm = makeViewModel(tier: .pro)
    vm.selectedRadiusMetres = 3000  // above the 2.1 km large-radius threshold
    #expect(vm.showsLargeRadiusWarning)

    let sut = RadiusPickerStepView(viewModel: vm)
    _ = sut.body
  }
}
