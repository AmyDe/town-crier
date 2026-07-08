import MapKit
import Testing

@testable import TownCrierPresentation

/// Covers the pure, value-level "stacked" (same-address) cluster-tap
/// detection rule (GH#877) — the epsilon/zoom-floor decision that
/// ``AnonymousClusteredMapView/Coordinator`` uses in place of the
/// authenticated map's server-computed `MapCluster.isStacked`. Deliberately
/// free of `MKMapView`/`UIViewRepresentable` so it runs without a simulator.
@Suite("AnonymousClusterStackDetector")
struct AnonymousClusterStackDetectorTests {
  private func coordinate(_ latitude: Double, _ longitude: Double) -> CLLocationCoordinate2D {
    CLLocationCoordinate2D(latitude: latitude, longitude: longitude)
  }

  @Test func isStacked_allMembersShareOneCoordinate_returnsTrue() {
    let members = [
      coordinate(51.5, -0.12),
      coordinate(51.5, -0.12),
      coordinate(51.5, -0.12),
    ]

    let result = AnonymousClusterStackDetector.isStacked(
      memberCoordinates: members, regionSpanDegrees: 0.05)

    #expect(result)
  }

  @Test func isStacked_membersWithinEpsilon_returnsTrue() {
    // Same-address applications from PlanIt carry near-identical but not
    // bit-for-bit-equal coordinates — must still count as coincident.
    let members = [
      coordinate(51.500_000, -0.120_000),
      coordinate(51.500_001, -0.120_001),
    ]

    let result = AnonymousClusterStackDetector.isStacked(
      memberCoordinates: members, regionSpanDegrees: 0.05)

    #expect(result)
  }

  @Test func isStacked_spreadMembers_returnsFalse() {
    let members = [
      coordinate(51.5, -0.12),
      coordinate(51.6, -0.20),
    ]

    let result = AnonymousClusterStackDetector.isStacked(
      memberCoordinates: members, regionSpanDegrees: 0.05)

    #expect(!result)
  }

  @Test func isStacked_regionSpanAtZoomFloor_treatsSpreadMembersAsStacked() {
    // Below the zoom floor MapKit cannot zoom in any further, so a
    // multi-member cluster here can never split visually — even when its
    // members exceed the coincidence epsilon.
    let members = [
      coordinate(51.5, -0.12),
      coordinate(51.50005, -0.12005),
    ]

    let result = AnonymousClusterStackDetector.isStacked(
      memberCoordinates: members, regionSpanDegrees: 0.001)

    #expect(result)
  }

  @Test func isStacked_regionSpanBelowZoomFloor_treatsSpreadMembersAsStacked() {
    let members = [
      coordinate(51.5, -0.12),
      coordinate(51.6, -0.20),
    ]

    let result = AnonymousClusterStackDetector.isStacked(
      memberCoordinates: members, regionSpanDegrees: 0.0005)

    #expect(result)
  }

  @Test func isStacked_regionSpanAboveZoomFloor_spreadMembersReturnFalse() {
    let members = [
      coordinate(51.5, -0.12),
      coordinate(51.6, -0.20),
    ]

    let result = AnonymousClusterStackDetector.isStacked(
      memberCoordinates: members, regionSpanDegrees: 0.002)

    #expect(!result)
  }

  @Test func isStacked_singleMember_returnsFalse() {
    let members = [coordinate(51.5, -0.12)]

    let result = AnonymousClusterStackDetector.isStacked(
      memberCoordinates: members, regionSpanDegrees: 0.05)

    #expect(!result)
  }

  @Test func isStacked_noMembers_returnsFalse() {
    let result = AnonymousClusterStackDetector.isStacked(
      memberCoordinates: [], regionSpanDegrees: 0.05)

    #expect(!result)
  }
}
