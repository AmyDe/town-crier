import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// `WatchZoneFilterPickerView` (GH#1098/tc-hogkn, GH#1104/tc-m8j90.2). The
/// View is not introspectable in this codebase (no ViewInspector), so these
/// are construction/render smoke tests plus assertions against the
/// ViewModel state the View reads and mutates -- the same pattern as
/// `WatchZoneEditorViewTests`. Renders from the ViewModel's fetched
/// ``WatchZoneEditorViewModel/filterCatalog`` rather than a hardcoded
/// `allCases` list.
@MainActor
@Suite("WatchZoneFilterPickerView")
struct WatchZoneFilterPickerViewTests {
  private var spyRepository: SpyWatchZoneRepository!

  init() {
    spyRepository = SpyWatchZoneRepository()
  }

  private func makeViewModel(
    tier: SubscriptionTier = .pro,
    editing zone: WatchZone? = nil
  ) -> WatchZoneEditorViewModel {
    WatchZoneEditorViewModel(
      geocoder: SpyPostcodeGeocoder(),
      repository: spyRepository,
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
      filterKey: "kitchen_extension"
    )
    let vm = makeViewModel(editing: zone)
    #expect(vm.selectedFilterKey == "kitchen_extension")

    let sut = WatchZoneFilterPickerView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_whileCatalogLoading() {
    let vm = makeViewModel()
    // Never resolved, so the fetch stays in flight for the duration of this
    // render assertion.
    let sut = WatchZoneFilterPickerView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_whenCatalogLoadFailed() async {
    let vm = makeViewModel()
    spyRepository.filterCatalogResult = .failure(DomainError.networkUnavailable)
    await vm.loadFilterCatalogIfNeeded()
    #expect(vm.filterCatalogLoadFailed)

    let sut = WatchZoneFilterPickerView(viewModel: vm)
    _ = sut.body
  }

  /// Sanity check that the picker has a row for every fetched catalog entry
  /// plus "None" available to select from -- exercised via the ViewModel
  /// the View drives, since the View itself isn't introspectable.
  @Test func selectingEachFetchedCatalogEntry_updatesSelectedFilterKey() async {
    let vm = makeViewModel()
    spyRepository.filterCatalogResult = .success(FilterCatalogEntry.allCatalog)
    await vm.loadFilterCatalogIfNeeded()
    #expect(vm.filterCatalog == FilterCatalogEntry.allCatalog)

    for entry in vm.filterCatalog {
      vm.selectFilterKey(entry.key)
      #expect(vm.selectedFilterKey == entry.key)
    }

    vm.selectFilterKey(nil)
    #expect(vm.selectedFilterKey == nil)
  }
}
