import MapKit
import SwiftUI
import TownCrierDomain

/// A non-interactive map preview showing a zone's shape around a centre
/// coordinate: a circle by default, or a custom-drawn polygon outline when
/// `boundaryVertices` describes one (tc-7se1w.1).
///
/// Size is controlled externally via SwiftUI `.frame()` modifiers.
/// The `strokeWidth` parameter allows callers to adjust border
/// thickness for different display contexts (compact vs. large).
struct ZoneMapPreview: View {
  let centre: Coordinate
  let radiusMetres: Double
  let strokeWidth: CGFloat
  /// The zone's custom-shape polygon ring, or `nil` for a circle. Mirrors
  /// ``WatchZone/isCustomShape`` — callers pass a non-nil
  /// `WatchZoneBoundary.vertices` for a custom-shape zone.
  let boundaryVertices: [Coordinate]?

  init(
    centre: Coordinate,
    radiusMetres: Double,
    // swiftlint:disable:next discouraged_default_parameter
    strokeWidth: CGFloat = 1,
    // swiftlint:disable:next discouraged_default_parameter
    boundaryVertices: [Coordinate]? = nil
  ) {
    self.centre = centre
    self.radiusMetres = radiusMetres
    self.strokeWidth = strokeWidth
    self.boundaryVertices = boundaryVertices
  }

  /// Whether `boundaryVertices` describes a polygon (3 or more vertices) —
  /// the minimum needed to enclose an area, matching the domain's
  /// `WatchZoneBoundary` validation.
  var isCustomShape: Bool {
    (boundaryVertices?.count ?? 0) >= 3
  }

  var body: some View {
    Map(initialPosition: .region(region)) {
      if isCustomShape, let boundaryVertices {
        MapPolygon(coordinates: boundaryVertices.map(Self.clLocation))
          .foregroundStyle(Color.tcAmber.opacity(0.2))
          .stroke(Color.tcAmber, style: StrokeStyle(lineWidth: strokeWidth, dash: [6, 4]))
      } else {
        MapCircle(center: clLocation, radius: radiusMetres)
          .foregroundStyle(Color.tcAmber.opacity(0.2))
          .stroke(Color.tcAmber, style: StrokeStyle(lineWidth: strokeWidth, dash: [6, 4]))
      }
    }
    .mapStyle(.standard(elevation: .flat))
    .allowsHitTesting(false)
  }

  private var clLocation: CLLocationCoordinate2D {
    Self.clLocation(centre)
  }

  private static func clLocation(_ coordinate: Coordinate) -> CLLocationCoordinate2D {
    CLLocationCoordinate2D(latitude: coordinate.latitude, longitude: coordinate.longitude)
  }

  private var region: MKCoordinateRegion {
    MKCoordinateRegion(
      center: clLocation,
      latitudinalMeters: radiusMetres * 2.5,
      longitudinalMeters: radiusMetres * 2.5
    )
  }
}
