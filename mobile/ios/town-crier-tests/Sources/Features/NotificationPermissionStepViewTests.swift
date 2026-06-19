import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@MainActor
@Suite("NotificationPermissionStepView")
struct NotificationPermissionStepViewTests {

  // The step renders one of two tier-aware variants. These smoke tests verify
  // both construct and evaluate their body (tc-w3cb.4).

  private func makeViewModel(tier: SubscriptionTier) -> OnboardingViewModel {
    OnboardingViewModel(
      geocoder: SpyPostcodeGeocoder(),
      watchZoneRepository: SpyWatchZoneRepository(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      subscriptionTier: tier
    )
  }

  @Test func body_renders_freeTier_weeklyDigestVariant() {
    let vm = makeViewModel(tier: .free)
    #expect(!vm.deliversInstantAlerts)

    let sut = NotificationPermissionStepView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_paidTier_instantAlertsVariant() {
    let vm = makeViewModel(tier: .personal)
    #expect(vm.deliversInstantAlerts)

    let sut = NotificationPermissionStepView(viewModel: vm)
    _ = sut.body
  }
}
