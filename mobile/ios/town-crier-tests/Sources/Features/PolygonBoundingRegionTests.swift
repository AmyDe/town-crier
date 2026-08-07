import MapKit
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// `PolygonBoundingRegion.fitting(vertices:marginFraction:minimumSpanDegrees:)`
/// — the shared "fit a map region to a polygon's bounding box" math behind
/// both `BoundaryDrawingRegion` (the boundary-drawing editor, tc-7se1w.2) and
/// `ClusteredMapView`'s camera framing for custom-shape zones on the main Map
/// page (tc-7se1w.3). A free-standing, non-UIKit type so the zoom-to-fit math
/// is directly unit-testable without a `UIViewRepresentable`/`MKMapView`.
@Suite("PolygonBoundingRegion")
struct PolygonBoundingRegionTests {
  private let marginFraction = 0.3
  private let minimumSpanDegrees = 0.006

  private func coordinate(_ latitude: Double, _ longitude: Double) throws -> Coordinate {
    try Coordinate(latitude: latitude, longitude: longitude)
  }

  @Test func emptyVertices_returnsNil() {
    let region = PolygonBoundingRegion.fitting(
      vertices: [], marginFraction: marginFraction, minimumSpanDegrees: minimumSpanDegrees)

    #expect(region == nil)
  }

  @Test func largePolygon_fitsBoundingBoxWithMargin() throws {
    // A ~1.1km x 1.1km box around Cambridge.
    let vertices = try [
      coordinate(51.50, -0.10),
      coordinate(51.51, -0.10),
      coordinate(51.51, -0.09),
      coordinate(51.50, -0.09),
    ]

    let region = try #require(
      PolygonBoundingRegion.fitting(
        vertices: vertices, marginFraction: marginFraction, minimumSpanDegrees: minimumSpanDegrees)
    )

    // Centred on the bounding box.
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

    // Margin actually applies the expected 30% widening (0.01deg raw box *
    // 1.3 = 0.013deg) rather than merely exceeding the raw bounding box.
    #expect(abs(region.span.latitudeDelta - 0.013) < 0.0001)
    #expect(abs(region.span.longitudeDelta - 0.013) < 0.0001)
  }

  @Test func singleVertex_appliesMinimumSpanRatherThanZoomingToNothing() throws {
    let vertex = try coordinate(51.50, -0.10)

    let region = try #require(
      PolygonBoundingRegion.fitting(
        vertices: [vertex], marginFraction: marginFraction, minimumSpanDegrees: minimumSpanDegrees)
    )

    #expect(region.center.latitude == vertex.latitude)
    #expect(region.center.longitude == vertex.longitude)
    #expect(region.span.latitudeDelta == minimumSpanDegrees)
    #expect(region.span.longitudeDelta == minimumSpanDegrees)
  }

  @Test func tightCluster_appliesMinimumSpanWhenBoundingBoxIsSmallerThanMinimum() throws {
    let vertices = try [
      coordinate(51.50000, -0.10000),
      coordinate(51.50002, -0.10002),
    ]

    let region = try #require(
      PolygonBoundingRegion.fitting(
        vertices: vertices, marginFraction: marginFraction, minimumSpanDegrees: minimumSpanDegrees)
    )

    #expect(region.span.latitudeDelta == minimumSpanDegrees)
    #expect(region.span.longitudeDelta == minimumSpanDegrees)
  }
}
