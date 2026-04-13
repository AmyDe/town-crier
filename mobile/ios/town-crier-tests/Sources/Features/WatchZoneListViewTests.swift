import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("WatchZoneListView")
@MainActor
struct WatchZoneListViewTests {

  private func makeViewModel(
    tier: SubscriptionTier = .free
  ) -> (WatchZoneListViewModel, SpyWatchZoneRepository) {
    let spy = SpyWatchZoneRepository()
    let vm = WatchZoneListViewModel(
      repository: spy,
      featureGate: FeatureGate(tier: tier)
    )
    return (vm, spy)
  }

  @Test func body_renders_whenNoZones() {
    let (vm, _) = makeViewModel()
    let sut = WatchZoneListView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_whenUpgradePromptPresented() async {
    let (vm, spy) = makeViewModel()
    spy.loadAllResult = .success([.cambridge])
    await vm.load()
    vm.addZone()

    #expect(vm.isUpgradePromptPresented)

    let sut = WatchZoneListView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_withZones() async {
    let (vm, spy) = makeViewModel()
    spy.loadAllResult = .success([.cambridge])
    await vm.load()

    let sut = WatchZoneListView(viewModel: vm)
    _ = sut.body
  }
}
