import Foundation
import Testing

@testable import TownCrierDomain
@testable import TownCrierPresentation

/// Circle/Custom shape mode, vertex editing, and the custom-shape entitlement
/// gate on `WatchZoneEditorViewModel` (GH#1031, bead tc-6he3x.8).
@MainActor
@Suite("WatchZoneEditorViewModel — shape mode")
struct WatchZoneEditorViewModelShapeModeTests {
  private var spyGeocoder: SpyPostcodeGeocoder!
  private var spyRepository: SpyWatchZoneRepository!

  init() {
    spyGeocoder = SpyPostcodeGeocoder()
    spyRepository = SpyWatchZoneRepository()
  }

  private func makeSUT(
    tier: SubscriptionTier = .personal,
    editing zone: WatchZone? = nil
  ) -> WatchZoneEditorViewModel {
    WatchZoneEditorViewModel(
      geocoder: spyGeocoder,
      repository: spyRepository,
      tier: tier,
      editing: zone
    )
  }

  /// A valid, UK-bounded triangle — mirrors the fixture used by
  /// `WatchZoneBoundaryTests`.
  private func triangle() throws -> [Coordinate] {
    try [
      Coordinate(latitude: 51.50, longitude: -0.10),
      Coordinate(latitude: 51.51, longitude: -0.09),
      Coordinate(latitude: 51.50, longitude: -0.09),
    ]
  }

  /// Four taps where the second vertex is accidentally re-tapped as the
  /// fourth — a non-adjacent duplicate distinct from the vertex-count case
  /// (`WatchZoneBoundaryTests.init_fewerThanThreeVertices_throws` covers
  /// that one). Once auto-closed this hits `WatchZoneBoundary`'s duplicate
  /// check (position 1 vs. position 3), not its vertex-count check.
  private func ringWithNonAdjacentDuplicate() throws -> [Coordinate] {
    let repeated = try Coordinate(latitude: 51.51, longitude: -0.09)
    return try [
      Coordinate(latitude: 51.50, longitude: -0.10),
      repeated,
      Coordinate(latitude: 51.52, longitude: -0.08),
      repeated,
    ]
  }

  // MARK: - Initial state

  @Test func initialState_shapeModeIsCircle() {
    let sut = makeSUT()
    #expect(sut.shapeMode == .circle)
  }

  @Test func initialState_boundaryVerticesIsEmpty() {
    let sut = makeSUT()
    #expect(sut.boundaryVertices.isEmpty)
  }

  @Test func initialState_editingCustomShapeZone_populatesShapeModeAndVertices() throws {
    let boundary = try WatchZoneBoundary(vertices: triangle())
    let zone = try WatchZone(
      id: WatchZoneId("zone-custom"),
      name: "Custom Area",
      centre: Coordinate(latitude: 51.5033, longitude: -0.0933),
      radiusMetres: 500,
      boundary: boundary
    )

    let sut = makeSUT(editing: zone)

    #expect(sut.shapeMode == .custom)
    // The open sequence — the closing repeat of the first vertex is dropped.
    #expect(sut.boundaryVertices == Array(boundary.vertices.dropLast()))
  }

  @Test func initialState_editingCircleZone_staysCircleWithNoVertices() {
    let sut = makeSUT(editing: .cambridge)

    #expect(sut.shapeMode == .circle)
    #expect(sut.boundaryVertices.isEmpty)
  }

  // MARK: - canDrawCustomShape (tier gating)

  @Test func canDrawCustomShape_falseOnFreeTier() {
    #expect(!makeSUT(tier: .free).canDrawCustomShape)
  }

  @Test func canDrawCustomShape_trueOnPersonalTier() {
    #expect(makeSUT(tier: .personal).canDrawCustomShape)
  }

  @Test func canDrawCustomShape_trueOnProTier() {
    #expect(makeSUT(tier: .pro).canDrawCustomShape)
  }

  // MARK: - isCustomShapeLocked (downgraded Free tier holding a custom-shape zone)

  @Test func isCustomShapeLocked_falseByDefault() {
    #expect(!makeSUT().isCustomShapeLocked)
  }

  @Test func isCustomShapeLocked_falseOnPersonalTier_editingCustomShapeZone() throws {
    let boundary = try WatchZoneBoundary(vertices: triangle())
    let zone = try WatchZone(
      id: WatchZoneId("zone-custom"),
      name: "Custom Area",
      centre: Coordinate(latitude: 51.5033, longitude: -0.0933),
      radiusMetres: 500,
      boundary: boundary
    )

    let sut = makeSUT(tier: .personal, editing: zone)

    #expect(!sut.isCustomShapeLocked)
  }

  @Test func isCustomShapeLocked_trueOnFreeTier_editingCustomShapeZone() throws {
    let boundary = try WatchZoneBoundary(vertices: triangle())
    let zone = try WatchZone(
      id: WatchZoneId("zone-custom"),
      name: "Custom Area",
      centre: Coordinate(latitude: 51.5033, longitude: -0.0933),
      radiusMetres: 500,
      boundary: boundary
    )

    // A downgraded Free-tier user reopening the editor for a polygon zone
    // they drew before downgrading (GH#1031's paused-zone posture: the zone
    // stays listed and editable, never silently converted).
    let sut = makeSUT(tier: .free, editing: zone)

    #expect(sut.shapeMode == .custom)
    #expect(sut.isCustomShapeLocked)
  }

  @Test func isCustomShapeLocked_falseOnFreeTier_editingCircleZone() {
    let sut = makeSUT(tier: .free, editing: .cambridge)

    #expect(!sut.isCustomShapeLocked)
  }

  // MARK: - selectShapeMode

  @Test func selectShapeMode_custom_allowedOnPersonalTier() {
    let sut = makeSUT(tier: .personal)

    sut.selectShapeMode(.custom)

    #expect(sut.shapeMode == .custom)
  }

  @Test func selectShapeMode_custom_allowedOnProTier() {
    let sut = makeSUT(tier: .pro)

    sut.selectShapeMode(.custom)

    #expect(sut.shapeMode == .custom)
  }

  @Test func selectShapeMode_custom_ignoredOnFreeTier() {
    let sut = makeSUT(tier: .free)

    sut.selectShapeMode(.custom)

    #expect(sut.shapeMode == .circle)
  }

  @Test func selectShapeMode_backToCircle_alwaysAllowed() {
    let sut = makeSUT(tier: .personal)
    sut.selectShapeMode(.custom)

    sut.selectShapeMode(.circle)

    #expect(sut.shapeMode == .circle)
  }

  // MARK: - Vertex operations

  @Test func addVertex_appendsCoordinate() throws {
    let sut = makeSUT()
    let coordinate = try Coordinate(latitude: 51.50, longitude: -0.10)

    sut.addVertex(coordinate)

    #expect(sut.boundaryVertices == [coordinate])
  }

  @Test func addVertex_multipleAppendsInOrder() throws {
    let sut = makeSUT()
    let vertices = try triangle()

    for vertex in vertices {
      sut.addVertex(vertex)
    }

    #expect(sut.boundaryVertices == vertices)
  }

  @Test func moveVertex_updatesCoordinateAtIndex() throws {
    let sut = makeSUT()
    let vertices = try triangle()
    for vertex in vertices { sut.addVertex(vertex) }
    let moved = try Coordinate(latitude: 51.52, longitude: -0.08)

    sut.moveVertex(at: 1, to: moved)

    #expect(sut.boundaryVertices[1] == moved)
    #expect(sut.boundaryVertices.count == 3)
  }

  @Test func moveVertex_outOfRangeIndex_isNoOp() throws {
    let sut = makeSUT()
    let vertices = try triangle()
    for vertex in vertices { sut.addVertex(vertex) }
    let moved = try Coordinate(latitude: 51.52, longitude: -0.08)

    sut.moveVertex(at: 99, to: moved)

    #expect(sut.boundaryVertices == vertices)
  }

  @Test func removeVertex_removesAtIndex() throws {
    let sut = makeSUT()
    let vertices = try triangle()
    for vertex in vertices { sut.addVertex(vertex) }

    sut.removeVertex(at: 1)

    #expect(sut.boundaryVertices == [vertices[0], vertices[2]])
  }

  @Test func removeVertex_outOfRangeIndex_isNoOp() throws {
    let sut = makeSUT()
    let vertices = try triangle()
    for vertex in vertices { sut.addVertex(vertex) }

    sut.removeVertex(at: 99)

    #expect(sut.boundaryVertices == vertices)
  }

  @Test func undoLastVertex_removesLast() throws {
    let sut = makeSUT()
    let vertices = try triangle()
    for vertex in vertices { sut.addVertex(vertex) }

    sut.undoLastVertex()

    #expect(sut.boundaryVertices == [vertices[0], vertices[1]])
  }

  @Test func undoLastVertex_onEmpty_isNoOp() {
    let sut = makeSUT()

    sut.undoLastVertex()

    #expect(sut.boundaryVertices.isEmpty)
  }

  // MARK: - canFinishCustomShape (minimum vertex guard)

  @Test func canFinishCustomShape_falseWithZeroVertices() {
    #expect(!makeSUT().canFinishCustomShape)
  }

  @Test func canFinishCustomShape_falseWithTwoVertices() throws {
    let sut = makeSUT()
    sut.addVertex(try Coordinate(latitude: 51.50, longitude: -0.10))
    sut.addVertex(try Coordinate(latitude: 51.51, longitude: -0.09))

    #expect(!sut.canFinishCustomShape)
  }

  @Test func canFinishCustomShape_trueAtThreeVertices() throws {
    let sut = makeSUT()
    for vertex in try triangle() { sut.addVertex(vertex) }

    #expect(sut.canFinishCustomShape)
  }

  // MARK: - finishDrawing

  @Test func finishDrawing_belowMinimum_doesNothing() throws {
    let sut = makeSUT()
    sut.addVertex(try Coordinate(latitude: 51.50, longitude: -0.10))

    sut.finishDrawing()

    #expect(sut.error == nil)
  }

  @Test func finishDrawing_validRing_leavesErrorNil() throws {
    let sut = makeSUT()
    for vertex in try triangle() { sut.addVertex(vertex) }

    sut.finishDrawing()

    #expect(sut.error == nil)
  }

  @Test func finishDrawing_duplicateVertex_setsError() throws {
    let sut = makeSUT()
    for vertex in try ringWithNonAdjacentDuplicate() { sut.addVertex(vertex) }

    sut.finishDrawing()

    #expect(sut.error == .invalidWatchZoneBoundaryDuplicateVertex)
  }

  // MARK: - save() in custom mode

  private func geocode(_ sut: WatchZoneEditorViewModel) async {
    sut.postcodeInput = "CB1 2AD"
    spyGeocoder.geocodeResult = .success(.cambridge)
    await sut.submitPostcode()
  }

  @Test func save_customMode_belowMinimumVertices_returnsFalseWithoutCallingRepository() async throws {
    let sut = makeSUT()
    await geocode(sut)
    sut.selectShapeMode(.custom)
    sut.addVertex(try Coordinate(latitude: 51.50, longitude: -0.10))

    let didSave = await sut.save()

    #expect(!didSave)
    #expect(spyRepository.saveCalls.isEmpty)
  }

  @Test func save_customMode_buildsBoundaryAndSaves() async throws {
    let sut = makeSUT()
    await geocode(sut)
    sut.selectShapeMode(.custom)
    for vertex in try triangle() { sut.addVertex(vertex) }

    let didSave = await sut.save()

    #expect(didSave)
    let saved = try #require(spyRepository.saveCalls.first)
    #expect(saved.isCustomShape)
    #expect(saved.boundary?.vertices.dropLast().elementsEqual(try triangle()) == true)
  }

  @Test func save_customMode_derivesCentreAndRadiusFromBoundary() async throws {
    let sut = makeSUT()
    await geocode(sut)
    sut.selectShapeMode(.custom)
    let vertices = try triangle()
    for vertex in vertices { sut.addVertex(vertex) }
    let expectedBoundary = try WatchZoneBoundary(vertices: vertices)

    _ = await sut.save()

    let saved = try #require(spyRepository.saveCalls.first)
    let expectedCentroid = expectedBoundary.centroid
    #expect(saved.centre.latitude == expectedCentroid.latitude)
    #expect(saved.centre.longitude == expectedCentroid.longitude)
    #expect(saved.radiusMetres == expectedBoundary.enclosingRadiusMetres)
  }

  @Test func save_customMode_invalidRing_setsErrorAndDoesNotCallRepository() async throws {
    let sut = makeSUT()
    await geocode(sut)
    sut.selectShapeMode(.custom)
    for vertex in try ringWithNonAdjacentDuplicate() { sut.addVertex(vertex) }

    let didSave = await sut.save()

    #expect(!didSave)
    #expect(sut.error == .invalidWatchZoneBoundaryDuplicateVertex)
    #expect(spyRepository.saveCalls.isEmpty)
  }

  @Test func save_circleMode_stillOmitsBoundary() async {
    let sut = makeSUT()
    await geocode(sut)

    let didSave = await sut.save()

    #expect(didSave)
    let saved = spyRepository.saveCalls.first
    #expect(saved?.boundary == nil)
    #expect(saved?.isCustomShape == false)
  }
}
