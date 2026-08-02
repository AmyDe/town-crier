#if canImport(UIKit)
  import MapKit
  import SwiftUI
  import TownCrierDomain

  /// A UIKit `MKMapView` wrapped for SwiftUI for drawing a custom-shape watch
  /// zone boundary (GH#1031, bead tc-6he3x.8): tap an empty area to drop a
  /// vertex, drag an existing vertex pin to move it, tap the first vertex
  /// (once at least 3 exist) to close the ring.
  ///
  /// The representable is a thin adapter (MVVM-C): all vertex state lives on
  /// ``WatchZoneEditorViewModel``; this view translates taps/drags into
  /// ViewModel calls and renders the vertex pins plus a preview
  /// polygon/polyline overlay. Structure mirrors `ClusteredMapView`.
  @MainActor
  struct BoundaryDrawingMapView: UIViewRepresentable {
    /// Observed so `updateUIView` re-runs to re-diff vertices whenever the
    /// ViewModel publishes a change (add/move/remove/undo) — a plain stored
    /// reference is NOT enough: SwiftUI treats the representable as
    /// unchanged when its only stored property is the same
    /// `WatchZoneEditorViewModel` instance, so it skips `updateUIView` and
    /// new vertices never reach the map (mirrors `ClusteredMapView`).
    @ObservedObject var viewModel: WatchZoneEditorViewModel

    /// Where the map centres on first appearance — the postcode-geocoded
    /// coordinate the user already entered before switching to Custom.
    let initialCentre: Coordinate

    /// Screen-point tolerance for treating a tap as "on an existing vertex"
    /// (closes the ring, if it's the first one, or is otherwise ignored —
    /// moving a vertex is a drag) rather than "add a new vertex here".
    static let vertexHitTestTolerance: CGFloat = 22

    func makeCoordinator() -> Coordinator {
      Coordinator(viewModel: viewModel)
    }

    func makeUIView(context: Context) -> MKMapView {
      let mapView = MKMapView()
      mapView.delegate = context.coordinator
      mapView.pointOfInterestFilter = .excludingAll
      mapView.setRegion(
        MKCoordinateRegion(
          center: CLLocationCoordinate2D(
            latitude: initialCentre.latitude, longitude: initialCentre.longitude),
          latitudinalMeters: 1000,
          longitudinalMeters: 1000
        ),
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
    /// a pin drag into the corresponding ``WatchZoneEditorViewModel`` call —
    /// all validation (minimum vertices, self-intersection, UK bounds) stays
    /// in the domain/ViewModel layer.
    @MainActor
    final class Coordinator: NSObject, MKMapViewDelegate {
      static let vertexReuseIdentifier = "watch-zone-boundary-vertex"

      private let viewModel: WatchZoneEditorViewModel

      /// The shape preview overlay currently on the map (a polygon once at
      /// least 3 vertices exist, otherwise an open polyline), so it can be
      /// swapped rather than left to accumulate stale overlays.
      private var shapeOverlay: MKOverlay?

      init(viewModel: WatchZoneEditorViewModel) {
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
        let points = desired.map {
          CLLocationCoordinate2D(latitude: $0.latitude, longitude: $0.longitude)
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
          let vertexPoint = mapView.convert(vertex.coordinate, toPointIn: mapView)
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
  /// ``WatchZoneEditorViewModel/boundaryVertices``.
  final class BoundaryVertexAnnotation: NSObject, MKAnnotation {
    let index: Int
    @objc dynamic var coordinate: CLLocationCoordinate2D

    init(index: Int, coordinate: CLLocationCoordinate2D) {
      self.index = index
      self.coordinate = coordinate
    }
  }
#endif
