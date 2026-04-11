import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("DashboardView")
@MainActor
struct DashboardViewTests {

  // MARK: - Helpers

  private func makeViewModel(
    tier: SubscriptionTier = .free,
    zones: [WatchZone] = [],
    authorities: [LocalAuthority] = []
  ) -> DashboardViewModel {
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success(zones)
    let authoritySpy = SpyApplicationAuthorityRepository()
    authoritySpy.fetchAuthoritiesResult = .success(
      ApplicationAuthorityResult(authorities: authorities, count: authorities.count)
    )
    return DashboardViewModel(
      watchZoneRepository: zoneSpy,
      authorityRepository: authoritySpy,
      featureGate: FeatureGate(tier: tier)
    )
  }

  // MARK: - View Construction

  @Test("DashboardView can be constructed with empty state")
  func construction_emptyState_succeeds() {
    let vm = makeViewModel()

    let view = DashboardView(viewModel: vm)

    _ = view
  }

  @Test("DashboardView can be constructed with zones and authorities")
  func construction_withData_succeeds() {
    let bath = LocalAuthority(code: "123", name: "Bath and NE Somerset")
    let vm = makeViewModel(
      zones: [.cambridge],
      authorities: [bath]
    )

    let view = DashboardView(viewModel: vm)

    _ = view
  }

  @Test("DashboardView can be constructed with pro tier")
  func construction_proTier_succeeds() {
    let vm = makeViewModel(tier: .pro)

    let view = DashboardView(viewModel: vm)

    _ = view
  }
}
