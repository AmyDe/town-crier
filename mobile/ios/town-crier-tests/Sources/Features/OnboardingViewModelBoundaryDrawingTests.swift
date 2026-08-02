import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Custom-shape boundary drawing and the mid-flow tier change (GH#1031,
/// tc-6he3x.10) — split out of `OnboardingViewModelTests` to keep both files
/// under the project's file-length limit.
@Suite("OnboardingViewModel boundary drawing")
@MainActor
struct OnboardingViewModelBoundaryDrawingTests {
  private func makeSUT() -> (
    OnboardingViewModel,
    SpyPostcodeGeocoder,
    SpyWatchZoneRepository,
    SpyOnboardingRepository,
    SpyNotificationService
  ) {
    let geocoder = SpyPostcodeGeocoder()
    let zoneRepo = SpyWatchZoneRepository()
    let onboardingRepo = SpyOnboardingRepository()
    let notificationService = SpyNotificationService()
    let vm = OnboardingViewModel(
      geocoder: geocoder,
      watchZoneRepository: zoneRepo,
      onboardingRepository: onboardingRepo,
      notificationService: notificationService
    )
    return (vm, geocoder, zoneRepo, onboardingRepo, notificationService)
  }

  private func makeViewModel(tier: SubscriptionTier) -> OnboardingViewModel {
    OnboardingViewModel(
      geocoder: SpyPostcodeGeocoder(),
      watchZoneRepository: SpyWatchZoneRepository(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      subscriptionTier: tier
    )
  }

  // MARK: - Custom-shape boundary drawing (GH#1031, tc-6he3x.10)

  /// A valid, UK-bounded triangle — mirrors the fixture used by
  /// `WatchZoneBoundaryTests` and `WatchZoneEditorViewModelShapeModeTests`.
  private func triangle() throws -> [Coordinate] {
    try [
      Coordinate(latitude: 51.50, longitude: -0.10),
      Coordinate(latitude: 51.51, longitude: -0.09),
      Coordinate(latitude: 51.50, longitude: -0.09),
    ]
  }

  @Test func initialState_shapeModeIsCircle() {
    let (sut, _, _, _, _) = makeSUT()
    #expect(sut.shapeMode == .circle)
  }

  @Test func initialState_boundaryVerticesIsEmpty() {
    let (sut, _, _, _, _) = makeSUT()
    #expect(sut.boundaryVertices.isEmpty)
  }

  @Test func canDrawCustomShape_falseOnFreeTier() {
    #expect(!makeViewModel(tier: .free).canDrawCustomShape)
  }

  @Test func canDrawCustomShape_trueOnPersonalTier() {
    #expect(makeViewModel(tier: .personal).canDrawCustomShape)
  }

  @Test func canDrawCustomShape_trueOnProTier() {
    #expect(makeViewModel(tier: .pro).canDrawCustomShape)
  }

  @Test func requestCustomShapeUpgrade_presentsUpsellSheet() {
    let sut = makeViewModel(tier: .free)
    #expect(!sut.isCustomShapeUpsellPresented)

    sut.requestCustomShapeUpgrade()

    #expect(sut.isCustomShapeUpsellPresented)
  }

  @Test func addVertex_appendsCoordinate() throws {
    let (sut, _, _, _, _) = makeSUT()
    let coordinate = try Coordinate(latitude: 51.50, longitude: -0.10)

    sut.addVertex(coordinate)

    #expect(sut.boundaryVertices == [coordinate])
  }

  @Test func undoLastVertex_removesLast() throws {
    let (sut, _, _, _, _) = makeSUT()
    for vertex in try triangle() { sut.addVertex(vertex) }

    sut.undoLastVertex()

    #expect(sut.boundaryVertices.count == 2)
  }

  @Test func undoLastVertex_onEmpty_isNoOp() {
    let (sut, _, _, _, _) = makeSUT()

    sut.undoLastVertex()

    #expect(sut.boundaryVertices.isEmpty)
  }

  @Test func moveVertex_updatesCoordinateAtIndex() throws {
    let (sut, _, _, _, _) = makeSUT()
    for vertex in try triangle() { sut.addVertex(vertex) }
    let moved = try Coordinate(latitude: 51.52, longitude: -0.08)

    sut.moveVertex(at: 1, to: moved)

    #expect(sut.boundaryVertices[1] == moved)
  }

  @Test func removeVertex_removesAtIndex() throws {
    let (sut, _, _, _, _) = makeSUT()
    let vertices = try triangle()
    for vertex in vertices { sut.addVertex(vertex) }

    sut.removeVertex(at: 1)

    #expect(sut.boundaryVertices == [vertices[0], vertices[2]])
  }

  @Test func hasMinimumBoundaryVertices_falseBelowThree() throws {
    let (sut, _, _, _, _) = makeSUT()
    sut.addVertex(try Coordinate(latitude: 51.50, longitude: -0.10))

    #expect(!sut.hasMinimumBoundaryVertices)
  }

  @Test func hasMinimumBoundaryVertices_trueAtThree() throws {
    let (sut, _, _, _, _) = makeSUT()
    for vertex in try triangle() { sut.addVertex(vertex) }

    #expect(sut.hasMinimumBoundaryVertices)
  }

  @Test func finishDrawing_belowMinimum_doesNothing() throws {
    let (sut, _, _, _, _) = makeSUT()
    sut.addVertex(try Coordinate(latitude: 51.50, longitude: -0.10))

    sut.finishDrawing()

    #expect(sut.error == nil)
  }

  @Test func finishDrawing_validRing_leavesErrorNil() throws {
    let (sut, _, _, _, _) = makeSUT()
    for vertex in try triangle() { sut.addVertex(vertex) }

    sut.finishDrawing()

    #expect(sut.error == nil)
  }

  // MARK: - confirmBoundary()

  @Test func confirmBoundary_belowMinimumVertices_doesNotAdvance() async throws {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // -> postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // -> radiusPicker
    sut.addVertex(try Coordinate(latitude: 51.50, longitude: -0.10))

    sut.confirmBoundary()

    #expect(sut.currentStep == .radiusPicker)
  }

  @Test func confirmBoundary_buildsCustomShapeZoneAndAdvances() async throws {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // -> postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // -> radiusPicker
    for vertex in try triangle() { sut.addVertex(vertex) }

    var completedZone: WatchZone?
    sut.onComplete = { zone in completedZone = zone }
    sut.confirmBoundary()

    #expect(sut.currentStep == .notificationPermission)
    await sut.skipNotifications()
    let zone = try #require(completedZone)
    #expect(zone.isCustomShape)
    #expect(zone.boundary?.vertices.dropLast().elementsEqual(try triangle()) == true)
  }

  @Test func confirmBoundary_derivesCentreAndRadiusFromBoundary() async throws {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // -> postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // -> radiusPicker
    let vertices = try triangle()
    for vertex in vertices { sut.addVertex(vertex) }
    let expectedBoundary = try WatchZoneBoundary(vertices: vertices)

    var completedZone: WatchZone?
    sut.onComplete = { zone in completedZone = zone }
    sut.confirmBoundary()
    await sut.skipNotifications()

    let zone = try #require(completedZone)
    let expectedCentroid = expectedBoundary.centroid
    #expect(zone.centre.latitude == expectedCentroid.latitude)
    #expect(zone.centre.longitude == expectedCentroid.longitude)
    #expect(zone.radiusMetres == expectedBoundary.enclosingRadiusMetres)
  }

  // MARK: - Step transitions to/from boundaryDrawing

  @Test func goBack_fromBoundaryDrawing_returnsToRadiusPickerAsCircle() async throws {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // -> postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // -> radiusPicker
    sut.onUpgradeFlowCompleted = { sut.subscriptionTier = .personal }
    await sut.reconcileTierAfterCustomShapeUpgrade()
    #expect(sut.currentStep == .boundaryDrawing)

    sut.goBack()

    #expect(sut.currentStep == .radiusPicker)
    #expect(sut.shapeMode == .circle)
  }

  @Test func goBack_fromBoundaryDrawing_discardsInProgressVertices() async throws {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // -> postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // -> radiusPicker
    sut.onUpgradeFlowCompleted = { sut.subscriptionTier = .personal }
    await sut.reconcileTierAfterCustomShapeUpgrade()
    for vertex in try triangle() { sut.addVertex(vertex) }
    #expect(!sut.boundaryVertices.isEmpty)

    sut.goBack()

    #expect(sut.boundaryVertices.isEmpty)
  }

  // MARK: - selectCustomShape() re-entry (tc-6he3x.10)

  @Test func selectCustomShape_entitledOnRadiusPicker_entersBoundaryDrawing() async {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // -> postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // -> radiusPicker
    sut.onUpgradeFlowCompleted = { sut.subscriptionTier = .personal }
    await sut.reconcileTierAfterCustomShapeUpgrade()  // -> boundaryDrawing
    sut.goBack()  // -> radiusPicker, stranded without this re-entry point
    #expect(sut.currentStep == .radiusPicker)

    sut.selectCustomShape()

    #expect(sut.currentStep == .boundaryDrawing)
    #expect(sut.shapeMode == .custom)
  }

  @Test func selectCustomShape_notEntitled_doesNothing() async throws {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // -> postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // -> radiusPicker

    sut.selectCustomShape()

    #expect(sut.currentStep == .radiusPicker)
    #expect(sut.shapeMode == .circle)
  }

  @Test func selectCustomShape_notOnRadiusStep_doesNothing() {
    let sut = makeViewModel(tier: .personal)
    // Still on .welcome.

    sut.selectCustomShape()

    #expect(sut.currentStep == .welcome)
  }

  @Test func advance_fromBoundaryDrawing_goesToNotificationPermission() async {
    // advance() is exercised directly here (rather than via
    // confirmBoundary()) for exhaustive step-switch coverage, mirroring how
    // .radiusPicker's advance() case is tested independently of
    // confirmRadius().
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // -> postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // -> radiusPicker
    sut.onUpgradeFlowCompleted = { sut.subscriptionTier = .personal }
    await sut.reconcileTierAfterCustomShapeUpgrade()  // -> boundaryDrawing
    #expect(sut.currentStep == .boundaryDrawing)

    sut.advance()

    #expect(sut.currentStep == .notificationPermission)
  }

  // MARK: - Mid-flow tier change (GH#1031, tc-6he3x.10)

  @Test func reconcileTierAfterCustomShapeUpgrade_swapsRadiusStepForBoundaryDrawing() async {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // -> postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // -> radiusPicker
    #expect(sut.currentStep == .radiusPicker)
    var reconciled = false
    sut.onUpgradeFlowCompleted = {
      sut.subscriptionTier = .personal
      reconciled = true
    }

    await sut.reconcileTierAfterCustomShapeUpgrade()

    #expect(reconciled)
    // Swaps to the drawing step in place, without restarting onboarding.
    #expect(sut.currentStep == .boundaryDrawing)
    #expect(sut.shapeMode == .custom)
    // The already-entered postcode/geocode survives the swap.
    #expect(sut.postcodeInput == "CB1 2AD")
    #expect(sut.geocodedCoordinate != nil)
  }

  @Test func reconcileTierAfterCustomShapeUpgrade_stillFreeTier_staysOnRadiusPicker() async {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // -> postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // -> radiusPicker
    sut.onUpgradeFlowCompleted = {
      // Simulates a dismissed sheet with no purchase made — tier stays free.
    }

    await sut.reconcileTierAfterCustomShapeUpgrade()

    #expect(sut.currentStep == .radiusPicker)
    #expect(sut.shapeMode == .circle)
  }

  @Test func reconcileTierAfterCustomShapeUpgrade_notOnRadiusStep_doesNotSwapStep() async {
    let (sut, _, _, _, _) = makeSUT()
    // Still on .welcome — nothing to swap.
    sut.onUpgradeFlowCompleted = { sut.subscriptionTier = .personal }

    await sut.reconcileTierAfterCustomShapeUpgrade()

    #expect(sut.currentStep == .welcome)
  }
}
