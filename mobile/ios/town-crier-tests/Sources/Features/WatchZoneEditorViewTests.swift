import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@MainActor
@Suite("WatchZoneEditorView")
struct WatchZoneEditorViewTests {

  // The View is a passive renderer of ViewModel state. These tests verify
  // the View can be constructed and body evaluated for the configurations
  // the radius slider must support (each tier, with and without an existing
  // non-100-m-multiple radius).

  private func makeViewModel(
    tier: SubscriptionTier = .personal,
    editing zone: WatchZone? = nil
  ) -> WatchZoneEditorViewModel {
    WatchZoneEditorViewModel(
      geocoder: SpyPostcodeGeocoder(),
      repository: SpyWatchZoneRepository(),
      tier: tier,
      editing: zone
    )
  }

  @Test func body_renders_inCreateMode_withoutCoordinate() {
    let vm = makeViewModel()
    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_inEditMode_freeTier() {
    let vm = makeViewModel(tier: .free, editing: .cambridge)
    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_inEditMode_personalTier() {
    let vm = makeViewModel(tier: .personal, editing: .cambridge)
    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_inEditMode_proTier() {
    let vm = makeViewModel(tier: .pro, editing: .cambridge)
    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_whenExistingRadiusIsNot100mMultiple() throws {
    // Snap-on-edit safety: zones saved before the slider change may have
    // arbitrary radii (e.g. 1234 m). The slider must initialise without
    // crashing and the View must render.
    let zone = try WatchZone(
      id: WatchZoneId("zone-legacy"),
      name: "Legacy",
      centre: .cambridge,
      radiusMetres: 1234
    )
    let vm = makeViewModel(tier: .personal, editing: zone)
    #expect(vm.selectedRadiusMetres == 1234)

    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  @Test func sliderBoundsMatchTier_forFreeTier() {
    let vm = makeViewModel(tier: .free)
    // Slider range bound: 100...maxRadiusMetres.
    #expect(vm.maxRadiusMetres == 2000)
    #expect(vm.maxRadiusMetres > 100)
  }

  @Test func sliderBoundsMatchTier_forPersonalTier() {
    let vm = makeViewModel(tier: .personal)
    #expect(vm.maxRadiusMetres == 5000)
  }

  @Test func sliderBoundsMatchTier_forProTier() {
    let vm = makeViewModel(tier: .pro)
    #expect(vm.maxRadiusMetres == 10000)
  }

  @Test func liveLabelFormatsCurrentRadius() {
    let vm = makeViewModel()
    vm.selectedRadiusMetres = 1500
    // The live label above the slider renders formatRadius(selectedRadiusMetres).
    #expect(formatRadius(vm.selectedRadiusMetres) == "1.5 km")

    vm.selectedRadiusMetres = 750
    #expect(formatRadius(vm.selectedRadiusMetres) == "750 m")
  }

  @Test func endLabelsFormatTierRange() {
    let vm = makeViewModel(tier: .pro)
    // Min label is fixed at the UI floor (100 m); max label tracks tier.
    #expect(formatRadius(100) == "100 m")
    #expect(formatRadius(vm.maxRadiusMetres) == "10 km")
  }

  // MARK: - Per-zone notification toggles (tc-kh1s)

  /// Smoke test: editing an existing zone with toggles flipped off must render without crashing.
  @Test func body_renders_inEditMode_withNotificationFlagsToggled() throws {
    let zone = try WatchZone(
      id: WatchZoneId("zone-toggles"),
      name: "Notify",
      centre: .cambridge,
      radiusMetres: 1500,
      pushEnabled: false,
      emailInstantEnabled: false
    )
    let vm = makeViewModel(tier: .personal, editing: zone)
    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }
}
