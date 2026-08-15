import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// The pre-canned filter section's three-way triad on `WatchZoneEditorView`
/// (GH#1098, bead tc-hogkn): open picker (Pro), locked static label
/// (downgraded Pro holding a filtered zone), or upsell placeholder
/// (Free/Personal). Split out of `WatchZoneEditorViewTests` to keep that
/// file under the `file_length` SwiftLint threshold, mirroring the existing
/// split into `APIWatchZoneRepositoryBoundaryTests`. The View is not
/// introspectable in this codebase (no ViewInspector), so these are
/// construction/render smoke tests plus assertions against the ViewModel
/// state the View reads.
@MainActor
@Suite("WatchZoneEditorView — pre-canned filter")
struct WatchZoneEditorViewFilterTests {

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

  private func filteredZone(
    filterKey: WatchZoneFilterKey = .kitchenExtension
  ) throws -> WatchZone {
    try WatchZone(
      id: WatchZoneId("zone-filtered"),
      name: "Filtered Area",
      centre: .cambridge,
      radiusMetres: 1500,
      filterKey: filterKey
    )
  }

  // MARK: - Pro tier: open picker

  @Test func body_renders_proTier_createMode() {
    let vm = makeViewModel(tier: .pro)
    #expect(vm.canSetWatchZoneFilter)
    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_proTier_editMode_noFilterSelected() {
    let vm = makeViewModel(tier: .pro, editing: .cambridge)
    #expect(vm.canSetWatchZoneFilter)
    #expect(vm.selectedFilterKey == nil)
    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_proTier_editMode_withFilterSelected() throws {
    let vm = makeViewModel(tier: .pro, editing: try filteredZone())
    #expect(vm.canSetWatchZoneFilter)
    #expect(vm.selectedFilterKey == .kitchenExtension)
    #expect(!vm.isFilterLocked)
    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  // MARK: - Free/Personal tier: locked upgrade affordance, cannot open picker

  @Test func body_renders_freeTier_createMode_showsUpsellPlaceholder() {
    let vm = makeViewModel(tier: .free)
    #expect(!vm.canSetWatchZoneFilter)
    #expect(!vm.isFilterLocked)
    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_personalTier_createMode_showsUpsellPlaceholder() {
    let vm = makeViewModel(tier: .personal)
    #expect(!vm.canSetWatchZoneFilter)
    #expect(!vm.isFilterLocked)
    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  /// Tapping the upsell placeholder routes to `viewPlans()`, which invokes
  /// `onUpgradeRequired` -- the same wiring the shape-mode upsell placeholder
  /// already exercises.
  @Test func upsellPlaceholder_tap_routesToUpgradeRequired() {
    let vm = makeViewModel(tier: .free)
    var upgradeRequested = false
    vm.onUpgradeRequired = { upgradeRequested = true }

    vm.viewPlans()

    #expect(upgradeRequested)
  }

  // MARK: - Downgraded Pro tier holding a filtered zone: locked static label

  @Test func body_renders_freeTier_editingFilteredZone_locked() throws {
    let vm = makeViewModel(tier: .free, editing: try filteredZone())
    #expect(vm.selectedFilterKey == .kitchenExtension)
    #expect(!vm.canSetWatchZoneFilter)
    #expect(vm.isFilterLocked)

    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_personalTier_editingFilteredZone_locked() throws {
    let vm = makeViewModel(tier: .personal, editing: try filteredZone())
    #expect(vm.isFilterLocked)

    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  /// Save must stay reachable for a locked filtered zone (renaming or
  /// toggling notifications is still allowed -- only the filter itself is
  /// locked, not the whole zone).
  @Test func saveDisabledCondition_lockedFilter_notBlockedByFilterState() throws {
    let vm = makeViewModel(tier: .free, editing: try filteredZone())

    #expect(vm.geocodedCoordinate != nil)
    #expect(!vm.nameInput.trimmingCharacters(in: .whitespaces).isEmpty)
  }
}
