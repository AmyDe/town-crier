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

  // Free tier at their one-zone cap: the richer inline upsell card fills the
  // space beneath the single zone. The card itself is driven by
  // `showsFreeTierUpsell`, asserted exhaustively in the ViewModel tests.
  @Test func body_renders_whenFreeTierUpsellShown() async {
    let (vm, spy) = makeViewModel(tier: .free)
    spy.loadAllResult = .success([.cambridge])
    await vm.load()

    #expect(vm.showsFreeTierUpsell)

    let sut = WatchZoneListView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_whenPersonalAtCap_noFreeTierUpsell() async {
    let (vm, spy) = makeViewModel(tier: .personal)
    spy.loadAllResult = .success([.cambridge, .london, .cambridge])
    await vm.load()

    #expect(!vm.showsFreeTierUpsell)

    let sut = WatchZoneListView(viewModel: vm)
    _ = sut.body
  }

  // MARK: - Paused zones (GH#889 P2)

  @Test func body_renders_whenZonePaused() async {
    let (vm, spy) = makeViewModel()
    spy.loadAllResult = .success([.cambridgePaused])
    await vm.load()

    #expect(vm.zones[0].paused)

    let sut = WatchZoneListView(viewModel: vm)
    _ = sut.body
  }

  // MARK: - Unconverted device-local zones row (GH#879 Phase 5)

  @Test func body_renders_whenLocalZoneRowShown() async throws {
    let spy = SpyWatchZoneRepository()
    let localRepo = SpyDeviceLocalZoneRepository()
    localRepo.loadAllResult = [
      try DeviceLocalZone(name: "Home", centre: .cambridge, radiusMetres: 1000)
    ]
    let vm = WatchZoneListViewModel(
      repository: spy,
      featureGate: FeatureGate(tier: .free),
      deviceLocalZoneRepository: localRepo
    )
    await vm.load()

    #expect(vm.showsLocalZoneRow)

    let sut = WatchZoneListView(viewModel: vm)
    _ = sut.body
  }
}
