import Testing

@testable import TownCrierDomain
@testable import TownCrierPresentation

/// Tests for the "Paused" badge shown on a watch zone row that currently
/// exceeds the user's tier quota (GH#889 P2). The badge is itself the
/// upgrade affordance — tapping it opens the subscription paywall via the
/// host's `onUpgrade` closure, which ``WatchZoneListView`` wires to
/// ``WatchZoneListViewModel/viewPlans()`` — the same routing method used by
/// the toolbar upgrade badge and the free-tier inline upsell card, so every
/// "upgrade" entry point in the Watch Zones screen converges on one paywall
/// presentation path.
@MainActor
@Suite("PausedZoneBadge")
struct PausedZoneBadgeTests {

  // MARK: - Copy

  @Test func label_readsPaused() {
    #expect(PausedZoneBadge.Copy.label == "Paused")
  }

  @Test func accessibilityHint_opensPlans() {
    #expect(PausedZoneBadge.Copy.accessibilityHint == "Opens subscription plans")
  }

  // MARK: - View

  @Test func body_renders() {
    let sut = PausedZoneBadge {}
    _ = sut.body
  }

  // MARK: - Callback

  @Test func onUpgrade_isCalled_whenTapped() {
    var called = false
    let sut = PausedZoneBadge { called = true }

    sut.simulateUpgradeTap()

    #expect(called)
  }

  @Test func upgradeTap_triggersViewModelViewPlans() async {
    let spy = SpyWatchZoneRepository()
    let viewModel = WatchZoneListViewModel(
      repository: spy,
      featureGate: FeatureGate(tier: .free)
    )
    var viewPlansCalled = false
    viewModel.onViewPlans = { viewPlansCalled = true }
    let sut = PausedZoneBadge { viewModel.viewPlans() }

    sut.simulateUpgradeTap()

    #expect(viewPlansCalled)
    #expect(!viewModel.isUpgradePromptPresented)
  }
}
