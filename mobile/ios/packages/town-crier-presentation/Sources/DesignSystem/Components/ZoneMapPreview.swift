import MapKit
import SwiftUI
import TownCrierDomain

/// A non-interactive map preview showing a zone circle around a centre coordinate.
///
/// Size is controlled externally via SwiftUI `.frame()` modifiers.
/// The `strokeWidth` parameter allows callers to adjust circle border
/// thickness for different display contexts (compact vs. large).
struct ZoneMapPreview: View {
    let centre: Coordinate
    let radiusMetres: Double
    let strokeWidth: CGFloat

    init(centre: Coordinate, radiusMetres: Double, strokeWidth: CGFloat = 1) {
        self.centre = centre
        self.radiusMetres = radiusMetres
        self.strokeWidth = strokeWidth
    }

    var body: some View {
        Map(initialPosition: .region(region)) {
            MapCircle(center: clLocation, radius: radiusMetres)
                .foregroundStyle(Color.tcAmber.opacity(0.2))
                .stroke(Color.tcAmber, lineWidth: strokeWidth)
        }
        .mapStyle(.standard(elevation: .flat))
        .allowsHitTesting(false)
    }

    private var clLocation: CLLocationCoordinate2D {
        CLLocationCoordinate2D(latitude: centre.latitude, longitude: centre.longitude)
    }

    private var region: MKCoordinateRegion {
        MKCoordinateRegion(
            center: clLocation,
            latitudinalMeters: radiusMetres * 2.5,
            longitudinalMeters: radiusMetres * 2.5
        )
    }
}
