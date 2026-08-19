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

  private func filteredZone(
    filterKey: String = "kitchen_extension"
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
    #expect(vm.selectedFilterKey == "kitchen_extension")
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
    #expect(vm.selectedFilterKey == "kitchen_extension")
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

  // MARK: - Both label call sites resolve via the catalog lookup (GH#1104, tc-m8j90.2)

  /// The open-picker `NavigationLink` row's label
  /// (`WatchZoneEditorView+FilterSection.swift`'s unlocked branch) reads
  /// `viewModel.filterDisplayName(for:)`, not `.displayName` off a deleted
  /// enum -- exercised with the catalog fetched, so the label resolves to
  /// the real display name.
  @Test func openPickerRow_filterDisplayName_resolvesFromFetchedCatalog() async throws {
    let vm = makeViewModel(tier: .pro, editing: try filteredZone(filterKey: "loft_extension"))
    spyRepository.filterCatalogResult = .success(FilterCatalogEntry.allCatalog)
    await vm.loadFilterCatalogIfNeeded()

    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body

    #expect(vm.filterDisplayName(for: vm.selectedFilterKey) == "Loft extension")
  }

  /// The locked static label (`lockedFilterSection`'s branch) also reads
  /// `viewModel.filterDisplayName(for:)`. Exercised with the catalog never
  /// fetched, so the fallback label resolves -- the label must still read
  /// as "filter set", never "None" (tc-8z9ri).
  @Test func lockedRow_filterDisplayName_fallsBackWhenCatalogUnfetched() throws {
    let vm = makeViewModel(tier: .free, editing: try filteredZone(filterKey: "loft_extension"))
    #expect(vm.filterCatalog.isEmpty)

    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body

    let displayName = vm.filterDisplayName(for: vm.selectedFilterKey)
    #expect(displayName != nil)
    #expect(displayName != "None")
  }
}
