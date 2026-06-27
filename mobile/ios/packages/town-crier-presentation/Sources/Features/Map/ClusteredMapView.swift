#if canImport(UIKit)
  import MapKit
  import SwiftUI
  import TownCrierDomain

  /// A UIKit `MKMapView` wrapped for SwiftUI so the map gets native
  /// `MKClusterAnnotation` clustering (GH#542). SwiftUI's `Map` has no clustering
  /// and materialises every annotation, so a dense watch zone with thousands of
  /// applications stalls. MapKit instead bounds the rendered marker count by
  /// screen area regardless of how many pins the zone holds — which is what lets
  /// the eager-drained full set (GH#682 slice 5) render smoothly.
  ///
  /// The representable is a thin adapter (MVVM-C): all state lives on
  /// ``MapViewModel``; this view only translates its published annotations into
  /// `MKAnnotation`s and forwards taps back to the ViewModel.
  @MainActor
  struct ClusteredMapView: UIViewRepresentable {
    /// Observed so the representable is invalidated — and `updateUIView` re-runs to
    /// re-diff annotations and re-frame — whenever the ViewModel publishes (filter
    /// change, zone switch, drained pages). A plain stored reference is NOT enough:
    /// SwiftUI treats the representable as unchanged when its only stored property
    /// is the same `MapViewModel` instance, so it skips `updateUIView` and filter
    /// toggles never reach the map.
    @ObservedObject var viewModel: MapViewModel

    /// Shared `clusteringIdentifier` on the individual marker views — MapKit
    /// collapses any markers carrying the same identifier into one
    /// `MKClusterAnnotation` when they overlap on screen.
    static let clusteringIdentifier = "planning-application"
    static let markerReuseIdentifier = "planning-application-marker"
    static let clusterReuseIdentifier = "planning-application-cluster"

    func makeCoordinator() -> Coordinator {
      Coordinator(viewModel: viewModel)
    }

    func makeUIView(context: Context) -> MKMapView {
      let mapView = MKMapView()
      mapView.delegate = context.coordinator
      mapView.pointOfInterestFilter = .excludingAll
      mapView.register(
        MKMarkerAnnotationView.self,
        forAnnotationViewWithReuseIdentifier: Self.markerReuseIdentifier)
      mapView.register(
        MKMarkerAnnotationView.self,
        forAnnotationViewWithReuseIdentifier: Self.clusterReuseIdentifier)

      let coordinator = context.coordinator
      coordinator.syncAnnotations(on: mapView, desired: viewModel.filteredAnnotations)
      coordinator.applyRadiusOverlay(
        to: mapView,
        centreLat: viewModel.centreLat,
        centreLon: viewModel.centreLon,
        radius: viewModel.radiusMetres)
      coordinator.frameCamera(
        on: mapView,
        centre: CLLocationCoordinate2D(
          latitude: viewModel.centreLat, longitude: viewModel.centreLon),
        radius: viewModel.radiusMetres,
        zoneId: viewModel.selectedZone?.id,
        animated: false)
      return mapView
    }

    func updateUIView(_ mapView: MKMapView, context: Context) {
      let coordinator = context.coordinator
      coordinator.syncAnnotations(on: mapView, desired: viewModel.filteredAnnotations)
      coordinator.applyRadiusOverlay(
        to: mapView,
        centreLat: viewModel.centreLat,
        centreLon: viewModel.centreLon,
        radius: viewModel.radiusMetres)
      // Reframe the camera only when the selected zone actually changes, so a
      // filter toggle or a background annotation refresh never yanks the user's
      // current pan/zoom back to the zone framing.
      coordinator.frameCameraIfZoneChanged(
        on: mapView,
        centreLat: viewModel.centreLat,
        centreLon: viewModel.centreLon,
        radius: viewModel.radiusMetres,
        zoneId: viewModel.selectedZone?.id)
    }
  }

  extension ClusteredMapView {
    /// `MKMapViewDelegate` for ``ClusteredMapView``. Holds no business logic — it
    /// styles markers, forwards a single-pin tap to ``MapViewModel/selectApplication(_:)``,
    /// expands a cluster tap by zooming, and renders the zone radius circle. All
    /// callbacks run on the main thread (MapKit guarantees it), matching the
    /// `@MainActor` isolation.
    @MainActor
    final class Coordinator: NSObject, MKMapViewDelegate {
      private let viewModel: MapViewModel

      /// The zone the camera is currently framed on, so we only reframe on a real
      /// zone change rather than on every annotation/filter update.
      private var framedZoneId: WatchZoneId?
      /// The currently-rendered radius circle and the centre/radius it was drawn
      /// for, so we redraw it only when the zone's geometry changes.
      private var radiusOverlay: MKCircle?
      private var renderedCentreLat: Double?
      private var renderedCentreLon: Double?
      private var renderedRadius: Double?

      init(viewModel: MapViewModel) {
        self.viewModel = viewModel
      }

      // MARK: - Annotation diffing

      /// Applies only the delta between the displayed pins and `desired`, so a
      /// filter change or a partial refresh doesn't churn the whole annotation set
      /// (which would drop clustering animations and the current selection).
      func syncAnnotations(on mapView: MKMapView, desired: [MapAnnotationItem]) {
        let current = mapView.annotations.compactMap { $0 as? PlanningApplicationAnnotation }
        let currentIds = Set(current.map(\.annotationId))
        let desiredIds = Set(desired.map(\.id))

        let toRemove = current.filter { !desiredIds.contains($0.annotationId) }
        if !toRemove.isEmpty {
          mapView.removeAnnotations(toRemove)
        }

        let toAdd =
          desired
          .filter { !currentIds.contains($0.id) }
          .map(PlanningApplicationAnnotation.init(item:))
        if !toAdd.isEmpty {
          mapView.addAnnotations(toAdd)
        }
      }

      // MARK: - Radius overlay

      func applyRadiusOverlay(
        to mapView: MKMapView, centreLat: Double, centreLon: Double, radius: Double
      ) {
        if renderedRadius == radius, renderedCentreLat == centreLat, renderedCentreLon == centreLon {
          return
        }
        if let radiusOverlay {
          mapView.removeOverlay(radiusOverlay)
        }
        let circle = MKCircle(
          center: CLLocationCoordinate2D(latitude: centreLat, longitude: centreLon),
          radius: radius)
        mapView.addOverlay(circle, level: .aboveRoads)
        radiusOverlay = circle
        renderedCentreLat = centreLat
        renderedCentreLon = centreLon
        renderedRadius = radius
      }

      // MARK: - Camera framing

      func frameCamera(
        on mapView: MKMapView,
        centre: CLLocationCoordinate2D,
        radius: Double,
        zoneId: WatchZoneId?,
        animated: Bool
      ) {
        // Mirror the previous SwiftUI framing: span 2.5x the zone radius so the
        // whole circle plus a margin is visible.
        let region = MKCoordinateRegion(
          center: centre,
          latitudinalMeters: radius * 2.5,
          longitudinalMeters: radius * 2.5)
        mapView.setRegion(region, animated: animated)
        framedZoneId = zoneId
      }

      func frameCameraIfZoneChanged(
        on mapView: MKMapView,
        centreLat: Double,
        centreLon: Double,
        radius: Double,
        zoneId: WatchZoneId?
      ) {
        guard zoneId != framedZoneId else { return }
        frameCamera(
          on: mapView,
          centre: CLLocationCoordinate2D(latitude: centreLat, longitude: centreLon),
          radius: radius,
          zoneId: zoneId,
          animated: true)
      }

      // MARK: - MKMapViewDelegate

      func mapView(_ mapView: MKMapView, viewFor annotation: MKAnnotation) -> MKAnnotationView? {
        if let cluster = annotation as? MKClusterAnnotation {
          return clusterView(for: cluster, on: mapView)
        }
        if let planning = annotation as? PlanningApplicationAnnotation {
          return markerView(for: planning, on: mapView)
        }
        return nil
      }

      private func markerView(
        for annotation: PlanningApplicationAnnotation, on mapView: MKMapView
      ) -> MKAnnotationView {
        let view = mapView.dequeueReusableAnnotationView(
          withIdentifier: ClusteredMapView.markerReuseIdentifier, for: annotation)
        guard let marker = view as? MKMarkerAnnotationView else { return view }
        marker.annotation = annotation
        marker.clusteringIdentifier = ClusteredMapView.clusteringIdentifier
        marker.glyphImage = UIImage(systemName: "mappin.circle.fill")
        marker.markerTintColor = UIColor(annotation.status.displayColor)
        marker.canShowCallout = false
        // Low priority lets MapKit cluster overlapping pins; the cluster bubble
        // (required priority) always wins.
        marker.displayPriority = .defaultLow
        return marker
      }

      private func clusterView(
        for cluster: MKClusterAnnotation, on mapView: MKMapView
      ) -> MKAnnotationView {
        let view = mapView.dequeueReusableAnnotationView(
          withIdentifier: ClusteredMapView.clusterReuseIdentifier, for: cluster)
        guard let marker = view as? MKMarkerAnnotationView else { return view }
        marker.annotation = cluster
        // Brand amber: a cluster is a navigational aggregate, not a status, so it
        // takes the design system's brand accent rather than any `tcStatus*`
        // colour (which would falsely imply a single status for the group).
        marker.markerTintColor = UIColor(Color.tcAmber)
        marker.glyphText = "\(cluster.memberAnnotations.count)"
        marker.canShowCallout = false
        marker.displayPriority = .required
        return marker
      }

      func mapView(_ mapView: MKMapView, didSelect view: MKAnnotationView) {
        if let cluster = view.annotation as? MKClusterAnnotation {
          let rect = enclosingMapRect(for: cluster.memberAnnotations)
          mapView.setVisibleMapRect(
            rect,
            edgePadding: UIEdgeInsets(top: 60, left: 60, bottom: 60, right: 60),
            animated: true)
          mapView.deselectAnnotation(cluster, animated: false)
          return
        }
        if let planning = view.annotation as? PlanningApplicationAnnotation {
          viewModel.selectApplication(planning.applicationId)
          // Deselect so tapping the same pin again after dismissing the sheet
          // fires `didSelect` once more.
          mapView.deselectAnnotation(planning, animated: false)
        }
      }

      func mapView(_ mapView: MKMapView, rendererFor overlay: MKOverlay) -> MKOverlayRenderer {
        guard let circle = overlay as? MKCircle else {
          return MKOverlayRenderer(overlay: overlay)
        }
        let renderer = MKCircleRenderer(circle: circle)
        renderer.strokeColor = UIColor(Color.tcAmber.opacity(0.3))
        renderer.fillColor = UIColor(Color.tcAmber.opacity(0.08))
        renderer.lineWidth = 1.5
        return renderer
      }

      private func enclosingMapRect(for annotations: [MKAnnotation]) -> MKMapRect {
        annotations.reduce(MKMapRect.null) { rect, annotation in
          let point = MKMapPoint(annotation.coordinate)
          return rect.union(MKMapRect(x: point.x, y: point.y, width: 0, height: 0))
        }
      }
    }
  }

  /// A reference-type `MKAnnotation` wrapping a value-type ``MapAnnotationItem`` so
  /// MapKit can hold and cluster it. Carries the `applicationId` and `status` the
  /// coordinator needs to colour the marker and route a tap.
  final class PlanningApplicationAnnotation: NSObject, MKAnnotation {
    let annotationId: String
    let applicationId: PlanningApplicationId
    let status: ApplicationStatus
    let coordinate: CLLocationCoordinate2D
    let title: String?
    let subtitle: String?

    init(item: MapAnnotationItem) {
      self.annotationId = item.id
      self.applicationId = item.applicationId
      self.status = item.status
      self.coordinate = CLLocationCoordinate2D(latitude: item.latitude, longitude: item.longitude)
      self.title = item.title
      self.subtitle = item.address
    }
  }
#endif
