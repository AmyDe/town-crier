import MapKit
import TownCrierDomain

/// Fits a map region to a polygon's bounding box: the shared "zoom to fit
/// these vertices" math behind ``BoundaryDrawingRegion`` (the boundary-
/// drawing editor, GH#1031 bead tc-7se1w.2) and ``ClusteredMapView``'s
/// camera framing for a custom-shape zone on the main Map page (bead
/// tc-7se1w.3 — the polygon already drives application filtering; this
/// makes the camera framing match it instead of always fitting a circle).
///
/// A free-standing, non-generic type outside any `#if canImport(UIKit)`
/// guard — it only needs `MapKit`/`CoreLocation`, both available on macOS,
/// so `swift test`'s bare-macOS build (no UIKit) can still see it and the
/// zoom-to-fit math is directly unit-testable without a concrete
/// `UIViewRepresentable`/`MKMapView`.
enum PolygonBoundingRegion {
  /// Fits a region to `vertices`' bounding box, widened by `marginFraction`
  /// of the box's own span and floored at `minimumSpanDegrees` so a single
  /// vertex or a tightly-clustered few vertices doesn't produce an absurdly
  /// close-in zoom. Returns `nil` for empty `vertices` — callers decide
  /// their own no-vertices fallback (e.g. a fixed region centred elsewhere).
  static func fitting(
    vertices: [Coordinate],
    marginFraction: Double,
    minimumSpanDegrees: Double
  ) -> MKCoordinateRegion? {
    guard !vertices.isEmpty else { return nil }

    let latitudes = vertices.map(\.latitude)
    let longitudes = vertices.map(\.longitude)
    // Force-unwrap-free: `vertices` is non-empty here, so `min()`/`max()`
    // always succeed; the `??` fallback only guards the type-checker.
    let minLatitude = latitudes.min() ?? 0
    let maxLatitude = latitudes.max() ?? 0
    let minLongitude = longitudes.min() ?? 0
    let maxLongitude = longitudes.max() ?? 0

    let centre = CLLocationCoordinate2D(
      latitude: (minLatitude + maxLatitude) / 2,
      longitude: (minLongitude + maxLongitude) / 2
    )
    let span = MKCoordinateSpan(
      latitudeDelta: max((maxLatitude - minLatitude) * (1 + marginFraction), minimumSpanDegrees),
      longitudeDelta: max(
        (maxLongitude - minLongitude) * (1 + marginFraction), minimumSpanDegrees)
    )
    return MKCoordinateRegion(center: centre, span: span)
  }
}
