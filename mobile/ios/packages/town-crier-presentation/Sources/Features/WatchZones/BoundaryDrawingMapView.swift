import MapKit
import TownCrierDomain

/// Computes the map region ``BoundaryDrawingMapView`` shows on first
/// appearance (GH#1031, bead tc-7se1w.2): a fixed close-in region centred
/// on the postcode-geocoded centre when there are no vertices yet (a
/// brand-new custom shape — today's behaviour, unchanged), or a region
/// that fits the bounding box of the existing vertices with margin when
/// re-opening a zone that already has a drawn polygon. Without this, the
/// fixed 1000m default clips a large polygon's vertices/edges outside the
/// visible map on reopen.
///
/// A free-standing, non-generic type (rather than a member of the generic
/// `BoundaryDrawingMapView<ViewModel>`) so the zoom-to-fit math is
/// directly unit-testable without a concrete `ViewModel` or a
/// `UIViewRepresentable` context. Deliberately outside the `canImport(UIKit)`
/// guard below — it only needs `MapKit`/`CoreLocation`, both available on
/// macOS, so `swift test`'s bare-macOS build (no UIKit) can still see it.
enum BoundaryDrawingRegion {
  /// The fixed region size (metres) used when there are no vertices yet —
  /// matches the region this view always used before tc-7se1w.2.
  static let freshDrawRegionMetres: CLLocationDistance = 1000

  /// Extra room around the tightest bounding box of the vertices, as a
  /// fraction of the box's own span — keeps drawn edges/vertices from
  /// sitting flush against the map's edge rather than comfortably inside it.
  static let marginFraction = 0.3

  /// Smallest span (degrees of latitude/longitude) the fitted region is
  /// allowed to shrink to — stops a single vertex or a tightly clustered
  /// few vertices from producing an absurdly close-in zoom.
  static let minimumSpanDegrees = 0.006

  static func fitting(vertices: [Coordinate], initialCentre: Coordinate) -> MKCoordinateRegion {
    guard !vertices.isEmpty else {
      return MKCoordinateRegion(
        center: CLLocationCoordinate2D(
          latitude: initialCentre.latitude, longitude: initialCentre.longitude),
        latitudinalMeters: freshDrawRegionMetres,
        longitudinalMeters: freshDrawRegionMetres
      )
    }

    let latitudes = vertices.map(\.latitude)
    let longitudes = vertices.map(\.longitude)
    // Force-unwrap-free: `vertices` is non-empty here, so `min()`/`max()`
    // always succeed; the `??` fallback only guards the type-checker.
    let minLatitude = latitudes.min() ?? initialCentre.latitude
    let maxLatitude = latitudes.max() ?? initialCentre.latitude
    let minLongitude = longitudes.min() ?? initialCentre.longitude
    let maxLongitude = longitudes.max() ?? initialCentre.longitude

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

#if canImport(UIKit)
  import SwiftUI

  /// A UIKit `MKMapView` wrapped for SwiftUI for drawing a custom-shape watch
  /// zone boundary (GH#1031, bead tc-6he3x.8): tap an empty area to drop a
  /// vertex, drag an existing vertex pin to move it, tap the first vertex
  /// (once at least 3 exist) to close the ring.
  ///
  /// The representable is a thin adapter (MVVM-C): all vertex state lives on
  /// the driving ViewModel; this view translates taps/drags into ViewModel
  /// calls and renders the vertex pins plus a preview polygon/polyline
  /// overlay. Structure mirrors `ClusteredMapView`.
  ///
  /// Generic over ``BoundaryDrawingViewModel`` rather than tied to the
  /// concrete `WatchZoneEditorViewModel` (tc-6he3x.10): the onboarding
  /// wizard drives its own boundary-drawing state on `OnboardingViewModel`,
  /// and this view only needs the narrow vertex-editing surface both
  /// ViewModels share, not the rest of either one.
  @MainActor
  struct BoundaryDrawingMapView<ViewModel: BoundaryDrawingViewModel>: UIViewRepresentable {
    /// Observed so `updateUIView` re-runs to re-diff vertices whenever the
    /// ViewModel publishes a change (add/move/remove/undo) — a plain stored
    /// reference is NOT enough: SwiftUI treats the representable as
    /// unchanged when its only stored property is the same ViewModel
    /// instance, so it skips `updateUIView` and new vertices never reach the
    /// map (mirrors `ClusteredMapView`).
    @ObservedObject var viewModel: ViewModel

    /// Where the map centres on first appearance — the postcode-geocoded
    /// coordinate the user already entered before switching to Custom.
    let initialCentre: Coordinate

    /// Screen-point tolerance for treating a tap as "on an existing vertex"
    /// (closes the ring, if it's the first one, or is otherwise ignored —
    /// moving a vertex is a drag) rather than "add a new vertex here".
    ///
    /// A computed property, not a stored one — Swift disallows stored
    /// `static` properties on a generic type.
    static var vertexHitTestTolerance: CGFloat { 22 }

    func makeCoordinator() -> Coordinator {
      Coordinator(viewModel: viewModel)
    }

    func makeUIView(context: Context) -> MKMapView {
      let mapView = MKMapView()
      mapView.delegate = context.coordinator
      mapView.pointOfInterestFilter = .excludingAll
      mapView.setRegion(
        BoundaryDrawingRegion.fitting(
          vertices: viewModel.boundaryVertices, initialCentre: initialCentre),
        animated: false
      )

      let tapRecognizer = UITapGestureRecognizer(
        target: context.coordinator, action: #selector(Coordinator.handleTap(_:)))
      mapView.addGestureRecognizer(tapRecognizer)

      context.coordinator.applyVertices(on: mapView, desired: viewModel.boundaryVertices)
      return mapView
    }

    func updateUIView(_ mapView: MKMapView, context: Context) {
      context.coordinator.applyVertices(on: mapView, desired: viewModel.boundaryVertices)
    }
  }

  extension BoundaryDrawingMapView {
    /// `MKMapViewDelegate` + tap-gesture target for ``BoundaryDrawingMapView``.
    /// Holds no business logic of its own beyond translating a screen tap or
    /// a pin drag into the corresponding ``BoundaryDrawingViewModel`` call —
    /// all validation (minimum vertices, self-intersection, UK bounds) stays
    /// in the domain/ViewModel layer.
    @MainActor
    final class Coordinator: NSObject, MKMapViewDelegate {
      // A computed property, not a stored one — Swift disallows stored
      // `static` properties on a type nested inside a generic type.
      static var vertexReuseIdentifier: String { "watch-zone-boundary-vertex" }

      private let viewModel: ViewModel

      /// The shape preview overlay currently on the map (a polygon once at
      /// least 3 vertices exist, otherwise an open polyline), so it can be
      /// swapped rather than left to accumulate stale overlays.
      private var shapeOverlay: MKOverlay?

      init(viewModel: ViewModel) {
        self.viewModel = viewModel
      }

      // MARK: - Vertex + overlay diffing

      /// Applies only the delta between the displayed vertex pins and
      /// `desired`, keyed by ordinal position (a vertex's identity is its
      /// index in the ring, not its coordinate — two vertices may
      /// legitimately sit close together while drawing), then redraws the
      /// preview shape overlay.
      func applyVertices(on mapView: MKMapView, desired: [Coordinate]) {
        let current = mapView.annotations
          .compactMap { $0 as? BoundaryVertexAnnotation }
          .sorted { $0.index < $1.index }

        if current.count > desired.count {
          mapView.removeAnnotations(Array(current[desired.count...]))
        }

        for (index, coordinate) in desired.enumerated() {
          let clCoordinate = CLLocationCoordinate2D(
            latitude: coordinate.latitude, longitude: coordinate.longitude)
          if index < current.count {
            let existing = current[index]
            if existing.coordinate.latitude != clCoordinate.latitude
              || existing.coordinate.longitude != clCoordinate.longitude {
              existing.coordinate = clCoordinate
            }
          } else {
            mapView.addAnnotation(BoundaryVertexAnnotation(index: index, coordinate: clCoordinate))
          }
        }

        applyShapeOverlay(on: mapView, desired: desired)
      }

      private func applyShapeOverlay(on mapView: MKMapView, desired: [Coordinate]) {
        if let shapeOverlay {
          mapView.removeOverlay(shapeOverlay)
          self.shapeOverlay = nil
        }
        guard desired.count >= 2 else { return }
        let points = desired.map { vertex in
          CLLocationCoordinate2D(latitude: vertex.latitude, longitude: vertex.longitude)
        }
        let overlay: MKOverlay =
          desired.count >= 3
          ? MKPolygon(coordinates: points, count: points.count)
          : MKPolyline(coordinates: points, count: points.count)
        mapView.addOverlay(overlay, level: .aboveRoads)
        shapeOverlay = overlay
      }

      // MARK: - Tap gesture (add vertex / close ring)

      @objc func handleTap(_ recognizer: UITapGestureRecognizer) {
        guard recognizer.state == .ended, let mapView = recognizer.view as? MKMapView else {
          return
        }
        let point = recognizer.location(in: mapView)

        if let hitIndex = nearestVertexIndex(to: point, on: mapView) {
          if hitIndex == 0, viewModel.boundaryVertices.count >= 3 {
            viewModel.finishDrawing()
          }
          // A tap on any other existing vertex is a no-op here — moving one
          // is a drag, handled by `mapView(_:annotationView:didChange:...)`.
          return
        }

        let tapped = mapView.convert(point, toCoordinateFrom: mapView)
        guard
          let coordinate = try? Coordinate(latitude: tapped.latitude, longitude: tapped.longitude)
        else { return }
        viewModel.addVertex(coordinate)
      }

      /// The index of the displayed vertex nearest `point`, if any lies
      /// within `BoundaryDrawingMapView.vertexHitTestTolerance` screen
      /// points of it.
      private func nearestVertexIndex(to point: CGPoint, on mapView: MKMapView) -> Int? {
        let vertices = mapView.annotations.compactMap { $0 as? BoundaryVertexAnnotation }
        var closestIndex: Int?
        var closestDistance = BoundaryDrawingMapView.vertexHitTestTolerance
        for vertex in vertices {
          let vertexPoint = mapView.convert(vertex.coordinate, toPointTo: mapView)
          let distance = hypot(vertexPoint.x - point.x, vertexPoint.y - point.y)
          if distance <= closestDistance {
            closestDistance = distance
            closestIndex = vertex.index
          }
        }
        return closestIndex
      }

      // MARK: - MKMapViewDelegate

      func mapView(_ mapView: MKMapView, viewFor annotation: MKAnnotation) -> MKAnnotationView? {
        guard let vertex = annotation as? BoundaryVertexAnnotation else { return nil }
        let view =
          (mapView.dequeueReusableAnnotationView(withIdentifier: Self.vertexReuseIdentifier)
            as? MKMarkerAnnotationView)
          ?? MKMarkerAnnotationView(annotation: vertex, reuseIdentifier: Self.vertexReuseIdentifier)
        view.annotation = vertex
        view.canShowCallout = false
        view.isDraggable = true
        view.markerTintColor =
          vertex.index == 0 ? UIColor(Color.tcAmber) : UIColor(Color.tcAmberMuted)
        view.glyphText = "\(vertex.index + 1)"
        return view
      }

      func mapView(
        _ mapView: MKMapView,
        annotationView view: MKAnnotationView,
        didChange newState: MKAnnotationView.DragState,
        fromOldState oldState: MKAnnotationView.DragState
      ) {
        guard newState == .ending || newState == .canceling else { return }
        defer { view.dragState = .none }
        guard newState == .ending, let vertex = view.annotation as? BoundaryVertexAnnotation
        else { return }
        let moved = vertex.coordinate
        if let coordinate = try? Coordinate(latitude: moved.latitude, longitude: moved.longitude) {
          viewModel.moveVertex(at: vertex.index, to: coordinate)
        }
      }

      func mapView(_ mapView: MKMapView, rendererFor overlay: MKOverlay) -> MKOverlayRenderer {
        if let polygon = overlay as? MKPolygon {
          let renderer = MKPolygonRenderer(polygon: polygon)
          renderer.strokeColor = UIColor(Color.tcAmber)
          renderer.fillColor = UIColor(Color.tcAmber.opacity(0.15))
          renderer.lineWidth = 2
          return renderer
        }
        if let polyline = overlay as? MKPolyline {
          let renderer = MKPolylineRenderer(polyline: polyline)
          renderer.strokeColor = UIColor(Color.tcAmber)
          renderer.lineWidth = 2
          return renderer
        }
        return MKOverlayRenderer(overlay: overlay)
      }
    }
  }

  /// A reference-type `MKAnnotation` for a single boundary vertex pin,
  /// carrying its ordinal position in the ring so the coordinator can route
  /// a drag, or a "tap first vertex to close", back to the right index in
  /// ``BoundaryDrawingViewModel/boundaryVertices``.
  final class BoundaryVertexAnnotation: NSObject, MKAnnotation {
    let index: Int
    @objc dynamic var coordinate: CLLocationCoordinate2D

    init(index: Int, coordinate: CLLocationCoordinate2D) {
      self.index = index
      self.coordinate = coordinate
    }
  }

  /// `WatchZoneEditorViewModel` (tc-6he3x.8) already exposes exactly the
  /// vertex-editing surface ``BoundaryDrawingViewModel`` requires, so no
  /// changes to that type are needed. The conformance is declared here,
  /// alongside its sole reason for existing, rather than on the type
  /// itself, so the already-shipped `WatchZoneEditorViewModel.swift` stays
  /// untouched (tc-6he3x.10).
  extension WatchZoneEditorViewModel: BoundaryDrawingViewModel {}
#endif
