import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("SubscriptionView")
@MainActor
struct SubscriptionViewTests {
  private func makeViewModel() -> SubscriptionViewModel {
    SubscriptionViewModel(
      subscriptionService: SpySubscriptionService(),
      authenticationService: SpyAuthenticationService()
    )
  }

  @Test func body_rendersWithoutCrashing() {
    let sut = SubscriptionView(viewModel: makeViewModel())
    _ = sut.body
  }
}
