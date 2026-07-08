import MapKit

/// Pure, value-level detection of a "stacked" (same-address) map cluster tap
/// on the anonymous map (GH#877) — members whose coordinates are so close
/// together that no amount of MapKit zoom can ever visually separate them.
/// Deliberately free of `UIKit`/`UIViewRepresentable` (unlike
/// ``AnonymousClusteredMapView``, which is gated behind `canImport(UIKit)`)
/// so it is directly unit-testable everywhere `swift test` runs, including
/// macOS, with no simulator required.
///
/// Mirrors the authenticated map's server-computed `MapCluster.isStacked`
/// (GH#722) with an on-device rule: the anonymous map's client-side
/// `MKMapView` clustering (`clusteringIdentifier`) has no server
/// "unsplittable" signal to consult, so the decision is made here, on tap.
enum AnonymousClusterStackDetector {
  /// Same-address applications carry identical (or near-identical) coordinates
  /// from PlanIt, so this is generous — roughly 1 metre — well below any real
  /// spread between distinct sites.
  static let coincidenceEpsilonDegrees: Double = 1e-5

  /// MapKit's own zoom floor, mirroring the halving the coordinator's zoom-in
  /// branch already applies (`max(span / 2, 0.0005)`): below this span the map
  /// cannot zoom in any further, so a multi-member cluster here can never
  /// split visually — even when its members' coordinates exceed the
  /// coincidence epsilon.
  static let zoomFloorDegrees: Double = 0.001

  /// Whether a cluster tap should open the disambiguation list instead of
  /// zooming in further. A cluster of fewer than two members is never
  /// "stacked" — there is nothing to disambiguate.
  static func isStacked(
    memberCoordinates: [CLLocationCoordinate2D],
    regionSpanDegrees: Double
  ) -> Bool {
    guard memberCoordinates.count > 1 else { return false }
    if regionSpanDegrees <= zoomFloorDegrees {
      return true
    }
    return maxPairwiseDelta(memberCoordinates) <= coincidenceEpsilonDegrees
  }

  /// The largest latitude or longitude difference across every pair of
  /// members — the "can zoom ever split these apart" measure.
  private static func maxPairwiseDelta(_ coordinates: [CLLocationCoordinate2D]) -> Double {
    var maxDelta = 0.0
    for i in coordinates.indices {
      for j in coordinates.index(after: i)..<coordinates.endIndex {
        maxDelta = max(
          maxDelta,
          abs(coordinates[i].latitude - coordinates[j].latitude),
          abs(coordinates[i].longitude - coordinates[j].longitude))
      }
    }
    return maxDelta
  }
}
