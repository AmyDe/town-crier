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

  // MARK: - Radius upsell chip (tc-w3cb.3)

  @Test func canUnlockLargerRadius_belowTopTier() {
    #expect(makeViewModel(tier: .free).canUnlockLargerRadius)
    #expect(makeViewModel(tier: .personal).canUnlockLargerRadius)
  }

  @Test func canUnlockLargerRadius_falseAtProTier() {
    #expect(!makeViewModel(tier: .pro).canUnlockLargerRadius)
  }

  @Test func liveLabelFormatsCurrentRadius() {
    let vm = makeViewModel()
    vm.selectedRadiusMetres = 1500
    // The live label above the slider renders formatRadius(selectedRadiusMetres).
    #expect(formatRadius(vm.selectedRadiusMetres) == "1.5km")

    vm.selectedRadiusMetres = 750
    #expect(formatRadius(vm.selectedRadiusMetres) == "750m")
  }

  @Test func endLabelsFormatTierRange() {
    let vm = makeViewModel(tier: .pro)
    // Min label is fixed at the UI floor (100 m); max label tracks tier.
    #expect(formatRadius(100) == "100m")
    #expect(formatRadius(vm.maxRadiusMetres) == "10km")
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

  // MARK: - Instant-alert gating in the View (tc-bd6i)

  /// Free-tier create mode now shows the notifications section with locked
  /// ``GatedToggle`` rows + upsell instead of hiding it; the View must render.
  @Test func body_renders_inCreateMode_freeTier_withGatedToggles() {
    let vm = makeViewModel(tier: .free)
    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  /// The notifications section is now shown for every tier in create mode.
  @Test func body_renders_inCreateMode_personalTier_withInteractiveToggles() {
    let vm = makeViewModel(tier: .personal)
    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  /// Free-tier edit mode renders the gated toggles + upsell without crashing.
  @Test func body_renders_inEditMode_freeTier_withGatedToggles() {
    let vm = makeViewModel(tier: .free, editing: .cambridge)
    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  /// While the in-editor upsell gate is active, the View still renders.
  @Test func body_renders_whenEntitlementGateActive() {
    let vm = makeViewModel(tier: .free)
    vm.requestInstantAlertUpgrade()
    #expect(vm.entitlementGate == vm.instantAlertEntitlement)
    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  // MARK: - Circle/Custom shape mode (GH#1031, tc-6he3x.8)

  /// A paid tier with a geocoded coordinate must render the segmented
  /// Circle/Custom control without crashing.
  @Test func body_renders_personalTier_inCircleMode() {
    let vm = makeViewModel(tier: .personal, editing: .cambridge)
    #expect(vm.shapeMode == .circle)
    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  /// Switching to Custom swaps the radius slider/map preview for the
  /// boundary-drawing map; the View must still render.
  @Test func body_renders_personalTier_inCustomMode() {
    let vm = makeViewModel(tier: .personal, editing: .cambridge)
    vm.selectShapeMode(.custom)
    #expect(vm.shapeMode == .custom)
    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_proTier_inCustomMode() {
    let vm = makeViewModel(tier: .pro, editing: .cambridge)
    vm.selectShapeMode(.custom)
    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  /// Free tier never offers the segmented control — it must render the
  /// upsell placeholder instead, without crashing.
  @Test func body_renders_freeTier_showsUpsellPlaceholderInsteadOfShapeToggle() {
    let vm = makeViewModel(tier: .free, editing: .cambridge)
    #expect(!vm.canDrawCustomShape)
    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  /// A free-tier attempt to select Custom is ignored by the ViewModel, so
  /// the View stays on the circle layout.
  @Test func selectShapeMode_custom_onFreeTier_staysCircle() {
    let vm = makeViewModel(tier: .free, editing: .cambridge)

    vm.selectShapeMode(.custom)

    #expect(vm.shapeMode == .circle)
  }

  /// Editing an existing custom-shape zone must open the editor already in
  /// Custom mode with its vertices populated, and render.
  @Test func body_renders_editingCustomShapeZone() throws {
    let boundary = try WatchZoneBoundary(vertices: [
      Coordinate(latitude: 51.50, longitude: -0.10),
      Coordinate(latitude: 51.51, longitude: -0.09),
      Coordinate(latitude: 51.50, longitude: -0.09),
    ])
    let zone = try WatchZone(
      id: WatchZoneId("zone-custom"),
      name: "Custom Area",
      centre: Coordinate(latitude: 51.5033, longitude: -0.0933),
      radiusMetres: 500,
      boundary: boundary
    )
    let vm = makeViewModel(tier: .personal, editing: zone)
    #expect(vm.shapeMode == .custom)
    #expect(vm.boundaryVertices.count == 3)

    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  /// Save stays disabled in Custom mode until the minimum vertex count is met.
  @Test func saveDisabledCondition_customMode_belowMinimumVertices() {
    let vm = makeViewModel(tier: .personal, editing: .cambridge)
    vm.selectShapeMode(.custom)

    #expect(!vm.canFinishCustomShape)
  }

  // MARK: - Locked custom shape (downgraded Free tier, GH#1031)

  /// A Free-tier user reopening the editor for a polygon zone they drew
  /// before downgrading (GH#1031's paused-zone posture: stays listed and
  /// editable, never silently converted) must render without crashing, and
  /// — the entitlement gap this covers — must not reach the boundary-
  /// drawing UI: `shapeMode` is `.custom` (it reflects the zone's actual
  /// stored shape) but `canDrawCustomShape` is `false`, so the View takes
  /// the locked branch, not `boundaryDrawingSection`.
  @Test func body_renders_freeTier_editingCustomShapeZone_locked() throws {
    let boundary = try WatchZoneBoundary(vertices: [
      Coordinate(latitude: 51.50, longitude: -0.10),
      Coordinate(latitude: 51.51, longitude: -0.09),
      Coordinate(latitude: 51.50, longitude: -0.09),
    ])
    let zone = try WatchZone(
      id: WatchZoneId("zone-custom-downgraded"),
      name: "Custom Area",
      centre: Coordinate(latitude: 51.5033, longitude: -0.0933),
      radiusMetres: 500,
      boundary: boundary
    )
    let vm = makeViewModel(tier: .free, editing: zone)
    #expect(vm.shapeMode == .custom)
    #expect(!vm.canDrawCustomShape)
    #expect(vm.isCustomShapeLocked)

    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  /// Save must stay reachable for a locked custom-shape zone (renaming or
  /// toggling notifications on a paused zone is still allowed — only the
  /// shape itself is locked) since `boundaryVertices` is untouched and
  /// already meets the minimum.
  @Test func saveDisabledCondition_lockedCustomShape_notBlockedByVertexCount() throws {
    let boundary = try WatchZoneBoundary(vertices: [
      Coordinate(latitude: 51.50, longitude: -0.10),
      Coordinate(latitude: 51.51, longitude: -0.09),
      Coordinate(latitude: 51.50, longitude: -0.09),
    ])
    let zone = try WatchZone(
      id: WatchZoneId("zone-custom-downgraded"),
      name: "Custom Area",
      centre: Coordinate(latitude: 51.5033, longitude: -0.0933),
      radiusMetres: 500,
      boundary: boundary
    )
    let vm = makeViewModel(tier: .free, editing: zone)

    #expect(vm.canFinishCustomShape)
  }
}
