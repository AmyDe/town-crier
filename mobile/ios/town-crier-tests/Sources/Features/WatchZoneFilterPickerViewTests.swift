import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// `WatchZoneFilterPickerView` (GH#1098, bead tc-hogkn). The View is not
/// introspectable in this codebase (no ViewInspector), so these are
/// construction/render smoke tests plus assertions against the ViewModel
/// state the View reads and mutates -- the same pattern as
/// `WatchZoneEditorViewTests`.
@MainActor
@Suite("WatchZoneFilterPickerView")
struct WatchZoneFilterPickerViewTests {

  private func makeViewModel(
    tier: SubscriptionTier = .pro,
    editing zone: WatchZone? = nil
  ) -> WatchZoneEditorViewModel {
    WatchZoneEditorViewModel(
      geocoder: SpyPostcodeGeocoder(),
      repository: SpyWatchZoneRepository(),
      tier: tier,
      editing: zone
    )
  }

  @Test func body_renders_withNoSelection() {
    let vm = makeViewModel()
    let sut = WatchZoneFilterPickerView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_withAnExistingSelection() throws {
    let zone = try WatchZone(
      id: WatchZoneId("zone-filtered"),
      name: "Filtered Area",
      centre: .cambridge,
      radiusMetres: 1500,
      filterKey: .kitchenExtension
    )
    let vm = makeViewModel(editing: zone)
    #expect(vm.selectedFilterKey == .kitchenExtension)

    let sut = WatchZoneFilterPickerView(viewModel: vm)
    _ = sut.body
  }

  /// Sanity check that the picker has a row for every catalog entry plus
  /// "None" available to select from -- exercised via the ViewModel the View
  /// drives, since the View itself isn't introspectable.
  @Test func selectingEachCatalogEntry_updatesSelectedFilterKey() {
    let vm = makeViewModel()

    for filterKey in WatchZoneFilterKey.allCases {
      vm.selectFilterKey(filterKey)
      #expect(vm.selectedFilterKey == filterKey)
    }

    vm.selectFilterKey(nil)
    #expect(vm.selectedFilterKey == nil)
  }
}
