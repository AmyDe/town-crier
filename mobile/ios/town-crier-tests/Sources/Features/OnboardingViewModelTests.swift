import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("OnboardingViewModel")
@MainActor
struct OnboardingViewModelTests {
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

  // MARK: - Step Progression

  @Test func initialStep_isWelcome() {
    let (sut, _, _, _, _) = makeSUT()

    #expect(sut.currentStep == .welcome)
  }

  @Test func advanceFromWelcome_goesToPostcodeEntry() {
    let (sut, _, _, _, _) = makeSUT()

    sut.advance()

    #expect(sut.currentStep == .postcodeEntry)
  }

  @Test func advanceFromPostcodeEntry_requiresGeocodedPostcode() {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // → postcodeEntry

    sut.advance()  // should not advance without geocoded postcode

    #expect(sut.currentStep == .postcodeEntry)
  }

  @Test func goBack_fromPostcodeEntry_returnsToWelcome() {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // → postcodeEntry

    sut.goBack()

    #expect(sut.currentStep == .welcome)
  }

  @Test func goBack_fromWelcome_staysOnWelcome() {
    let (sut, _, _, _, _) = makeSUT()

    sut.goBack()

    #expect(sut.currentStep == .welcome)
  }

  // MARK: - Postcode Entry

  @Test func submitPostcode_geocodesAndAdvancesToRadiusPicker() async {
    let (sut, geocoder, _, _, _) = makeSUT()
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "CB1 2AD"

    await sut.submitPostcode()

    #expect(geocoder.geocodeCalls.count == 1)
    #expect(geocoder.geocodeCalls.first?.value == "CB1 2AD")
    #expect(sut.currentStep == .radiusPicker)
    #expect(sut.geocodedCoordinate != nil)
  }

  @Test func submitPostcode_invalidPostcode_setsError() async {
    let (sut, geocoder, _, _, _) = makeSUT()
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "INVALID"

    await sut.submitPostcode()

    #expect(geocoder.geocodeCalls.isEmpty)
    #expect(sut.error != nil)
    #expect(sut.currentStep == .postcodeEntry)
  }

  @Test func submitPostcode_geocodingFailure_setsError() async {
    let (sut, geocoder, _, _, _) = makeSUT()
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    geocoder.geocodeResult = .failure(DomainError.geocodingFailed("Not found"))

    await sut.submitPostcode()

    #expect(sut.error == .geocodingFailed("Not found"))
    #expect(sut.currentStep == .postcodeEntry)
  }

  @Test func submitPostcode_clearsExistingError() async {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "INVALID"
    await sut.submitPostcode()
    #expect(sut.error != nil)

    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()

    #expect(sut.error == nil)
  }

  @Test func submitPostcode_setsIsLoadingDuringGeocode() async {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "CB1 2AD"

    #expect(!sut.isLoading)
    await sut.submitPostcode()
    #expect(!sut.isLoading)
  }

  // MARK: - Radius Selection

  @Test func selectedRadiusMetres_defaultsTo1000() {
    let (sut, _, _, _, _) = makeSUT()

    #expect(sut.selectedRadiusMetres == 1000)
  }

  @Test func confirmRadius_createsWatchZoneAndAdvancesToNotification() async {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // → radiusPicker
    sut.selectedRadiusMetres = 2000

    var completedZone: WatchZone?
    sut.onComplete = { zone in completedZone = zone }
    sut.confirmRadius()

    #expect(sut.currentStep == .notificationPermission)
    // Verify zone was created via the completion callback after onboarding finishes
    await sut.skipNotifications()
    #expect(completedZone != nil)
    #expect(completedZone?.radiusMetres == 2000)
  }

  @Test func goBack_fromRadiusPicker_returnsToPostcodeEntry() async {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // → radiusPicker

    sut.goBack()

    #expect(sut.currentStep == .postcodeEntry)
  }

  // MARK: - Notification Permission

  @Test func requestNotifications_requestsPermissionAndCompletes() async {
    let (sut, _, zoneRepo, onboardingRepo, notificationService) = makeSUT()
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // → radiusPicker
    sut.confirmRadius()  // → notificationPermission

    await sut.requestNotificationPermission()

    #expect(notificationService.requestPermissionCallCount == 1)
    #expect(zoneRepo.saveCalls.count == 1)
    #expect(onboardingRepo.markOnboardingCompleteCallCount == 1)
    #expect(sut.isComplete)
  }

  @Test func skipNotifications_completesWithoutRequestingPermission() async {
    let (sut, _, zoneRepo, onboardingRepo, notificationService) = makeSUT()
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // → radiusPicker
    sut.confirmRadius()  // → notificationPermission

    await sut.skipNotifications()

    #expect(notificationService.requestPermissionCallCount == 0)
    #expect(zoneRepo.saveCalls.count == 1)
    #expect(onboardingRepo.markOnboardingCompleteCallCount == 1)
    #expect(sut.isComplete)
  }

  @Test func requestNotifications_denied_stillCompletes() async {
    let (sut, _, zoneRepo, onboardingRepo, notificationService) = makeSUT()
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // → radiusPicker
    sut.confirmRadius()  // → notificationPermission
    notificationService.requestPermissionResult = .success(false)

    await sut.requestNotificationPermission()

    #expect(zoneRepo.saveCalls.count == 1)
    #expect(onboardingRepo.markOnboardingCompleteCallCount == 1)
    #expect(sut.isComplete)
  }

  // MARK: - Completion Callback

  @Test func completion_invokesOnComplete() async {
    let (sut, _, _, _, _) = makeSUT()
    var completedZone: WatchZone?
    sut.onComplete = { zone in completedZone = zone }
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // → radiusPicker
    sut.confirmRadius()  // → notificationPermission

    await sut.skipNotifications()

    #expect(completedZone != nil)
    #expect(completedZone?.radiusMetres == 1000)
  }

  // MARK: - Tier-bounded radius (tc-w3cb.2)

  private func makeViewModel(tier: SubscriptionTier) -> OnboardingViewModel {
    OnboardingViewModel(
      geocoder: SpyPostcodeGeocoder(),
      watchZoneRepository: SpyWatchZoneRepository(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      subscriptionTier: tier
    )
  }

  @Test func maxRadiusMetres_freeTier_capsAt2km() {
    #expect(makeViewModel(tier: .free).maxRadiusMetres == 2000)
  }

  @Test func maxRadiusMetres_personalTier_capsAt5km() {
    #expect(makeViewModel(tier: .personal).maxRadiusMetres == 5000)
  }

  @Test func maxRadiusMetres_proTier_capsAt10km() {
    #expect(makeViewModel(tier: .pro).maxRadiusMetres == 10000)
  }

  // MARK: - Radius upsell (tc-w3cb.3)

  @Test func canUnlockLargerRadius_isTrueBelowPro() {
    #expect(makeViewModel(tier: .free).canUnlockLargerRadius)
    #expect(makeViewModel(tier: .personal).canUnlockLargerRadius)
  }

  @Test func canUnlockLargerRadius_isFalseAtTopTier() {
    #expect(!makeViewModel(tier: .pro).canUnlockLargerRadius)
  }

  @Test func requestLargerRadiusUpgrade_presentsUpsellSheet() {
    let sut = makeViewModel(tier: .free)
    #expect(!sut.isRadiusUpsellPresented)

    sut.requestLargerRadiusUpgrade()

    #expect(sut.isRadiusUpsellPresented)
  }

  // MARK: - Tier-aware notification copy (tc-w3cb.4)

  @Test func deliversInstantAlerts_isFalseForFree() {
    #expect(!makeViewModel(tier: .free).deliversInstantAlerts)
  }

  @Test func deliversInstantAlerts_isTrueForPaidTiers() {
    #expect(makeViewModel(tier: .personal).deliversInstantAlerts)
    #expect(makeViewModel(tier: .pro).deliversInstantAlerts)
  }

  // MARK: - Anonymous browse post-signup handoff (GH#868 Phase 3.5)

  @Test func prefill_seedsPostcodeAndCoordinate_andJumpsToRadiusPicker() throws {
    let (sut, _, _, _, _) = makeSUT()
    let postcode = try Postcode("CB1 2AD")
    let coordinate = try Coordinate(latitude: 52.2053, longitude: 0.1218)

    sut.prefill(postcode: postcode, coordinate: coordinate, radiusMetres: 1500)

    #expect(sut.postcodeInput == "CB1 2AD")
    #expect(sut.geocodedCoordinate == coordinate)
    #expect(sut.currentStep == .radiusPicker)
    #expect(sut.selectedRadiusMetres == 1500)
  }

  @Test func prefill_thenConfirmRadius_createsZoneAtPrefilledLocation() async throws {
    let (sut, _, _, _, _) = makeSUT()
    let postcode = try Postcode("CB1 2AD")
    let coordinate = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    sut.prefill(postcode: postcode, coordinate: coordinate, radiusMetres: 1500)

    var completedZone: WatchZone?
    sut.onComplete = { zone in completedZone = zone }
    sut.confirmRadius()
    await sut.skipNotifications()

    #expect(completedZone != nil)
    #expect(completedZone?.centre == coordinate)
    #expect(completedZone?.radiusMetres == 1500)
  }

  @Test func prefill_withRadiusAboveTierMax_clampsToMaxRadiusMetres() throws {
    let (sut, _, _, _, _) = makeSUT()
    let postcode = try Postcode("CB1 2AD")
    let coordinate = try Coordinate(latitude: 52.2053, longitude: 0.1218)

    // Free tier's cap is 2000; the anonymous picker itself is bounded there
    // too, so this only guards against a stale/legacy persisted radius.
    sut.prefill(postcode: postcode, coordinate: coordinate, radiusMetres: 5000)

    #expect(sut.selectedRadiusMetres == 2000)
  }

  @Test func prefill_withRadiusBelowMinimum_clampsTo100() throws {
    let (sut, _, _, _, _) = makeSUT()
    let postcode = try Postcode("CB1 2AD")
    let coordinate = try Coordinate(latitude: 52.2053, longitude: 0.1218)

    sut.prefill(postcode: postcode, coordinate: coordinate, radiusMetres: 10)

    #expect(sut.selectedRadiusMetres == 100)
  }

  @Test func prefill_doesNotAffectNormalNonPrefilledFlow() async {
    // The additive prefill entry point leaves the existing postcode-entry ->
    // submitPostcode() path completely unchanged when never called.
    let (sut, geocoder, _, _, _) = makeSUT()
    sut.advance()  // -> postcodeEntry
    sut.postcodeInput = "CB1 2AD"

    await sut.submitPostcode()

    #expect(geocoder.geocodeCalls.count == 1)
    #expect(sut.currentStep == .radiusPicker)
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
