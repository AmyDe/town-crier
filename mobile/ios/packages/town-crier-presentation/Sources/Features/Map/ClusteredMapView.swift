#if canImport(UIKit)
  import MapKit
  import SwiftUI
  import TownCrierDomain

  /// A UIKit `MKMapView` wrapped for SwiftUI that renders the server-computed
  /// cluster aggregates for the current viewport (GH#698). The device holds only
  /// the handful of cells on screen — not the whole zone's 22k pins — so panning
  /// and zooming stay smooth. On a region change it tells the ViewModel
  /// (debounced ~250ms) to refetch clusters for the new visible rect.
  ///
  /// The representable is a thin adapter (MVVM-C): all state lives on
  /// ``MapViewModel``; this view translates its published clusters into
  /// annotations, styles them, and routes taps back to the ViewModel.
  @MainActor
  struct ClusteredMapView: UIViewRepresentable {
    /// Observed so `updateUIView` re-runs to re-diff the (small) cluster set and
    /// re-frame whenever the ViewModel publishes — a refetch, a zone switch, a
    /// status-chip change. A plain stored reference is NOT enough: SwiftUI treats
    /// the representable as unchanged when its only stored property is the same
    /// `MapViewModel` instance, so it skips `updateUIView` and new clusters never
    /// reach the map.
    @ObservedObject var viewModel: MapViewModel

    static let markerReuseIdentifier = "planning-application-cluster"

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

      let coordinator = context.coordinator
      coordinator.frameCamera(
        on: mapView,
        centre: CLLocationCoordinate2D(
          latitude: viewModel.centreLat, longitude: viewModel.centreLon),
        radius: viewModel.radiusMetres,
        zoneId: viewModel.selectedZone?.id,
        animated: false)
      coordinator.syncAnnotations(on: mapView, desired: viewModel.clusters)
      coordinator.applyRadiusOverlay(
        to: mapView,
        centreLat: viewModel.centreLat,
        centreLon: viewModel.centreLon,
        radius: viewModel.radiusMetres)
      return mapView
    }

    func updateUIView(_ mapView: MKMapView, context: Context) {
      let coordinator = context.coordinator
      coordinator.syncAnnotations(on: mapView, desired: viewModel.clusters)
      coordinator.applyRadiusOverlay(
        to: mapView,
        centreLat: viewModel.centreLat,
        centreLon: viewModel.centreLon,
        radius: viewModel.radiusMetres)
      // Reframe only when the selected zone actually changes, so a refetch or a
      // status-chip change never yanks the user's current pan/zoom back.
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
    /// styles cluster markers, routes a single-member tap to
    /// ``MapViewModel/selectCluster(_:)``, zooms into a multi-member cell, debounces
    /// the region-change refetch, and renders the zone radius circle. All
    /// callbacks run on the main thread (MapKit guarantees it), matching the
    /// `@MainActor` isolation.
    @MainActor
    final class Coordinator: NSObject, MKMapViewDelegate {
      private let viewModel: MapViewModel

      /// The zone the camera is currently framed on, so we only reframe on a real
      /// zone change rather than on every cluster/filter update.
      private var framedZoneId: WatchZoneId?
      /// The currently-rendered radius circle and the centre/radius it was drawn
      /// for, so we redraw it only when the zone's geometry changes.
      private var radiusOverlay: MKCircle?
      private var renderedCentreLat: Double?
      private var renderedCentreLon: Double?
      private var renderedRadius: Double?

      /// The pending debounced refetch, cancelled and rescheduled on each region
      /// change so a pan/zoom flurry issues a single fetch when it settles.
      private var refetchTask: Task<Void, Never>?

      init(viewModel: MapViewModel) {
        self.viewModel = viewModel
      }

      // MARK: - Annotation diffing

      /// Applies only the delta between the displayed cluster markers and
      /// `desired`. The set is the handful of cells in the viewport, not the full
      /// zone, so this stays cheap and never churns the whole map.
      func syncAnnotations(on mapView: MKMapView, desired: [MapCluster]) {
        let current = mapView.annotations.compactMap { $0 as? MapClusterAnnotation }
        let currentIds = Set(current.map(\.clusterId))
        let desiredIds = Set(desired.map(\.id))

        let toRemove = current.filter { !desiredIds.contains($0.clusterId) }
        if !toRemove.isEmpty {
          mapView.removeAnnotations(toRemove)
        }

        let toAdd =
          desired
          .filter { !currentIds.contains($0.id) }
          .map(MapClusterAnnotation.init(cluster:))
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
        // Span 2.5x the zone radius so the whole circle plus a margin is visible.
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
        guard let cluster = annotation as? MapClusterAnnotation else { return nil }
        return markerView(for: cluster, on: mapView)
      }

      private func markerView(
        for annotation: MapClusterAnnotation, on mapView: MKMapView
      ) -> MKAnnotationView {
        let view = mapView.dequeueReusableAnnotationView(
          withIdentifier: ClusteredMapView.markerReuseIdentifier, for: annotation)
        guard let marker = view as? MKMarkerAnnotationView else { return view }
        marker.annotation = annotation
        marker.canShowCallout = false

        if annotation.cluster.count > 1 {
          // Brand amber: a cluster is a navigational aggregate, not a status, so
          // it takes the design system's brand accent rather than any `tcStatus*`
          // colour (which would falsely imply a single status for the group).
          marker.markerTintColor = UIColor(Color.tcAmber)
          marker.glyphImage = nil
          marker.glyphText = Self.bubbleGlyph(for: annotation.cluster.count)
          marker.displayPriority = .required
        } else {
          let status = annotation.cluster.memberStatus ?? .unknown
          marker.markerTintColor = UIColor(status.displayColor)
          marker.glyphText = nil
          marker.glyphImage = UIImage(systemName: "mappin.circle.fill")
          marker.displayPriority = .defaultHigh
        }
        return marker
      }

      /// The count shown inside an amber bubble, capped so a fully-zoomed-out
      /// dense cell stays legible.
      private static func bubbleGlyph(for count: Int) -> String {
        count > 999 ? "999+" : "\(count)"
      }

      func mapView(_ mapView: MKMapView, didSelect view: MKAnnotationView) {
        guard let annotation = view.annotation as? MapClusterAnnotation else { return }
        let cluster = annotation.cluster
        mapView.deselectAnnotation(annotation, animated: false)

        if cluster.count > 1 {
          // Zoom into the cell so its members spread into finer cells on the next
          // (debounced) refetch.
          var region = mapView.region
          region.center = annotation.coordinate
          region.span = MKCoordinateSpan(
            latitudeDelta: max(region.span.latitudeDelta / 2, 0.0005),
            longitudeDelta: max(region.span.longitudeDelta / 2, 0.0005))
          mapView.setRegion(region, animated: true)
        } else {
          let viewModel = self.viewModel
          Task { await viewModel.selectCluster(cluster) }
        }
      }

      func mapView(_ mapView: MKMapView, regionDidChangeAnimated animated: Bool) {
        scheduleClusterRefetch(for: mapView)
      }

      /// Debounces the viewport refetch: cancels any pending fetch and schedules a
      /// fresh one ~250ms out, so a continuous pan/zoom gesture issues one fetch
      /// when it settles rather than dozens mid-gesture.
      private func scheduleClusterRefetch(for mapView: MKMapView) {
        let viewport = Self.viewport(from: mapView)
        let zoom = Self.zoom(from: mapView)
        let viewModel = self.viewModel
        refetchTask?.cancel()
        refetchTask = Task { @MainActor in
          try? await Task.sleep(nanoseconds: 250_000_000)
          guard !Task.isCancelled else { return }
          await viewModel.loadClusters(viewport: viewport, zoom: zoom)
        }
      }

      static func viewport(from mapView: MKMapView) -> MapViewport {
        let region = mapView.region
        return MapViewport(
          west: region.center.longitude - region.span.longitudeDelta / 2,
          south: region.center.latitude - region.span.latitudeDelta / 2,
          east: region.center.longitude + region.span.longitudeDelta / 2,
          north: region.center.latitude + region.span.latitudeDelta / 2)
      }

      static func zoom(from mapView: MKMapView) -> Int {
        MapViewModel.slippyZoom(forLongitudeSpanDegrees: mapView.region.span.longitudeDelta)
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
    }
  }

  /// A reference-type `MKAnnotation` wrapping a value-type ``MapCluster`` so
  /// MapKit can hold it. Carries the cluster the coordinator needs to style the
  /// marker and route a tap.
  final class MapClusterAnnotation: NSObject, MKAnnotation {
    let cluster: MapCluster
    let clusterId: String
    let coordinate: CLLocationCoordinate2D

    init(cluster: MapCluster) {
      self.cluster = cluster
      self.clusterId = cluster.id
      self.coordinate = CLLocationCoordinate2D(
        latitude: cluster.coordinate.latitude, longitude: cluster.coordinate.longitude)
    }
  }
#endif
