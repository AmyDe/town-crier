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

  // MARK: - Soft Paywall

  @Test(
    "free user submitting empty/no-authority does not trigger gate (matches enabled-tier guards)"
  )
  func freeUser_submitWithoutAuthority_doesNotTriggerGate() async {
    let (vm, spy) = makeViewModel(tier: .free)
    vm.query = "extension"
    vm.selectedAuthorityId = nil

    await vm.search()

    #expect(vm.entitlementGate == nil)
    #expect(spy.searchCalls.isEmpty)
  }

  @Test("free user submitting valid query triggers paywall (soft paywall)")
  func freeUser_submitValidQuery_triggersPaywall() async {
    let (vm, spy) = makeViewModel(tier: .free)
    vm.query = "extension"
    vm.selectedAuthorityId = 123

    await vm.search()

    #expect(vm.entitlementGate == .searchApplications)
    #expect(spy.searchCalls.isEmpty)
  }
}
