import MapKit
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// `BoundaryDrawingRegion.fitting(vertices:initialCentre:)` — the initial
/// map region `BoundaryDrawingMapView` shows on first appearance (GH#1031,
/// bead tc-7se1w.2). Extracted as a pure function so the zoom-to-fit math
/// is testable without standing up a `UIViewRepresentable`.
@Suite("BoundaryDrawingRegion")
struct BoundaryDrawingRegionTests {
  private func coordinate(_ latitude: Double, _ longitude: Double) throws -> Coordinate {
    try Coordinate(latitude: latitude, longitude: longitude)
  }

  // MARK: - No vertices yet (fresh draw) — unchanged fixed-region behaviour

  @Test func noVertices_returnsFixedRegionCentredOnInitialCentre() throws {
    let centre = try coordinate(52.2053, 0.1218)

    let region = BoundaryDrawingRegion.fitting(vertices: [], initialCentre: centre)

    let expected = MKCoordinateRegion(
      center: CLLocationCoordinate2D(latitude: centre.latitude, longitude: centre.longitude),
      latitudinalMeters: BoundaryDrawingRegion.freshDrawRegionMetres,
      longitudinalMeters: BoundaryDrawingRegion.freshDrawRegionMetres
    )
    #expect(region.center.latitude == expected.center.latitude)
    #expect(region.center.longitude == expected.center.longitude)
    #expect(region.span.latitudeDelta == expected.span.latitudeDelta)
    #expect(region.span.longitudeDelta == expected.span.longitudeDelta)
  }

  // MARK: - A large polygon — must fit the whole bounding box, with margin

  @Test func largePolygon_fitsBoundingBoxWithMargin() throws {
    // A ~1.1km x 1.1km box around Cambridge — bigger than the old fixed
    // 1000m default, so the old behaviour would clip it.
    let vertices = try [
      coordinate(51.50, -0.10),
      coordinate(51.51, -0.10),
      coordinate(51.51, -0.09),
      coordinate(51.50, -0.09),
    ]

    let region = BoundaryDrawingRegion.fitting(
      vertices: vertices, initialCentre: try coordinate(51.505, -0.095))

    // Centred on the bounding box, not `initialCentre`.
    #expect(abs(region.center.latitude - 51.505) < 0.0001)
    #expect(abs(region.center.longitude - (-0.095)) < 0.0001)

    // Every vertex must lie strictly inside the region (with margin, not
    // flush against the edge).
    for vertex in vertices {
      let latHalfSpan = region.span.latitudeDelta / 2
      let lonHalfSpan = region.span.longitudeDelta / 2
      #expect(abs(vertex.latitude - region.center.latitude) < latHalfSpan)
      #expect(abs(vertex.longitude - region.center.longitude) < lonHalfSpan)
    }

    // Margin actually widens the span beyond the raw bounding box (0.01 deg
    // in both directions here) rather than fitting it exactly flush.
    #expect(region.span.latitudeDelta > 0.01)
    #expect(region.span.longitudeDelta > 0.01)
  }

  // MARK: - A single vertex / tight cluster — minimum span, not absurd zoom

  @Test func singleVertex_appliesMinimumSpanRatherThanZoomingToNothing() throws {
    let vertex = try coordinate(51.50, -0.10)

    let region = BoundaryDrawingRegion.fitting(
      vertices: [vertex], initialCentre: try coordinate(51.60, -0.20))

    #expect(region.center.latitude == vertex.latitude)
    #expect(region.center.longitude == vertex.longitude)
    #expect(region.span.latitudeDelta == BoundaryDrawingRegion.minimumSpanDegrees)
    #expect(region.span.longitudeDelta == BoundaryDrawingRegion.minimumSpanDegrees)
  }

  @Test func tightCluster_appliesMinimumSpanWhenBoundingBoxIsSmallerThanMinimum() throws {
    // Two vertices a few metres apart — a real but tiny polygon.
    let vertices = try [
      coordinate(51.50000, -0.10000),
      coordinate(51.50002, -0.10002),
    ]

    let region = BoundaryDrawingRegion.fitting(
      vertices: vertices, initialCentre: try coordinate(51.60, -0.20))

    #expect(region.span.latitudeDelta == BoundaryDrawingRegion.minimumSpanDegrees)
    #expect(region.span.longitudeDelta == BoundaryDrawingRegion.minimumSpanDegrees)
  }
}
