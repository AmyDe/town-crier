import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("SearchView")
@MainActor
struct SearchViewTests {

  // MARK: - Helpers

  private func makeViewModel(
    tier: SubscriptionTier = .pro
  ) -> (SearchViewModel, SpySearchRepository) {
    let spy = SpySearchRepository()
    let vm = SearchViewModel(
      repository: spy,
      featureGate: FeatureGate(tier: tier)
    )
    return (vm, spy)
  }

  // MARK: - View Construction

  @Test("SearchView can be constructed with pro tier")
  func construction_proTier_succeeds() {
    let (vm, _) = makeViewModel(tier: .pro)
    let authorities = [LocalAuthority(code: "123", name: "Cambridge")]

    let view = SearchView(
      viewModel: vm,
      authorities: authorities
    ) {}

    // View can be constructed without error
    _ = view
  }

  @Test("SearchView can be constructed with free tier")
  func construction_freeTier_succeeds() {
    let (vm, _) = makeViewModel(tier: .free)

    let view = SearchView(
      viewModel: vm,
      authorities: []
    ) {}

    _ = view
  }
}
