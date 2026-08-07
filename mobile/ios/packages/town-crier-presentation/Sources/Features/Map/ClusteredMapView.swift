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
      let framing = zoneFraming
      coordinator.frameCamera(
        on: mapView, framing: framing, zoneId: viewModel.selectedZone?.id, animated: false)
      coordinator.syncAnnotations(on: mapView, desired: viewModel.clusters)
      coordinator.applyZoneOverlay(to: mapView, framing: framing)
      return mapView
    }

    func updateUIView(_ mapView: MKMapView, context: Context) {
      let coordinator = context.coordinator
      let framing = zoneFraming
      coordinator.syncAnnotations(on: mapView, desired: viewModel.clusters)
      coordinator.applyZoneOverlay(to: mapView, framing: framing)
      // Reframe only when the selected zone actually changes, so a refetch or a
      // status-chip change never yanks the user's current pan/zoom back.
      coordinator.frameCameraIfZoneChanged(
        on: mapView, framing: framing, zoneId: viewModel.selectedZone?.id)
    }

    private var zoneFraming: ClusteredZoneFraming {
      ClusteredZoneFraming(
        centreLat: viewModel.centreLat,
        centreLon: viewModel.centreLon,
        radius: viewModel.radiusMetres,
        boundaryVertices: viewModel.boundaryVertices)
    }
  }

  /// The zone geometry ``ClusteredMapView``'s overlay and camera-framing logic
  /// need, bundled into one value so passing it to ``ClusteredMapView/Coordinator``
  /// stays within SwiftLint's function-parameter-count limit. `boundaryVertices`
  /// non-nil/non-empty means a custom-shape zone (GH#1031, tc-7se1w.3); `nil`
  /// means a circle, framed/overlaid from `centreLat`/`centreLon`/`radius` as
  /// before.
  struct ClusteredZoneFraming: Equatable {
    let centreLat: Double
    let centreLon: Double
    let radius: Double
    let boundaryVertices: [Coordinate]?

    var centre: CLLocationCoordinate2D {
      CLLocationCoordinate2D(latitude: centreLat, longitude: centreLon)
    }
  }

  extension ClusteredMapView {
    /// `MKMapViewDelegate` for ``ClusteredMapView``. Holds no business logic — it
    /// styles cluster markers, routes a single-member tap to
    /// ``MapViewModel/selectCluster(_:)``, opens the disambiguation list for an
    /// unsplittable (stacked) multi-member cell via
    /// ``MapViewModel/selectStack(_:)`` while zooming into a splittable one,
    /// debounces the region-change refetch, and renders the zone overlay (a
    /// circle, or an `MKPolygon` outline for a custom-shape zone). All callbacks
    /// run on the main thread (MapKit guarantees it), matching the `@MainActor`
    /// isolation.
    @MainActor
    final class Coordinator: NSObject, MKMapViewDelegate {
      private let viewModel: MapViewModel

      /// The zone the camera is currently framed on, so we only reframe on a real
      /// zone change rather than on every cluster/filter update.
      private var framedZoneId: WatchZoneId?
      /// The geometry the camera was last framed to, so a same-ID zone whose
      /// boundary, centre, or radius changes (e.g. a mid-session edit) still
      /// triggers a reframe.
      private var framedZoneFraming: ClusteredZoneFraming?
      /// The currently-rendered radius circle (circle-shaped zones) and the
      /// centre/radius it was drawn for, so we redraw it only when the zone's
      /// geometry changes.
      private var radiusOverlay: MKCircle?
      private var renderedCentreLat: Double?
      private var renderedCentreLon: Double?
      private var renderedRadius: Double?
      /// The currently-rendered polygon (custom-shape zones, GH#1031) and the
      /// vertices it was drawn for, so we redraw it only when the boundary
      /// actually changes. ``applyZoneOverlay(to:framing:)`` shows at most one
      /// of `radiusOverlay`/`polygonOverlay` at a time — the other is torn down
      /// when the selected zone's shape mode differs from what's currently on
      /// screen (tc-7se1w.3: the overlay must match whichever shape drives
      /// application filtering for this zone).
      private var polygonOverlay: MKPolygon?
      private var renderedVertices: [Coordinate]?

      /// The pending debounced refetch, cancelled and rescheduled on each region
      /// change so a pan/zoom flurry issues a single fetch when it settles.
      private var refetchTask: Task<Void, Never>?

      /// Extra room around the polygon's bounding box when framing the camera
      /// for a custom-shape zone — mirrors `BoundaryDrawingRegion.marginFraction`,
      /// which tc-7se1w.2 uses for the same "zoom to fit" need in the
      /// boundary-drawing editor.
      private static let polygonMarginFraction = 0.3
      /// Mirrors `BoundaryDrawingRegion.minimumSpanDegrees`.
      private static let polygonMinimumSpanDegrees = 0.006

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

      // MARK: - Zone overlay (circle or custom-shape polygon)

      /// Shows exactly one overlay for the selected zone: an `MKPolygon`
      /// outline for a custom shape (GH#1031), or the existing `MKCircle`
      /// radius overlay otherwise. Tears down the other kind first, so
      /// switching between a circle zone and a custom-shape zone never leaves
      /// a stale overlay of the wrong kind on screen (tc-7se1w.3 — the overlay
      /// must match whichever shape actually drives application filtering for
      /// this zone).
      func applyZoneOverlay(to mapView: MKMapView, framing: ClusteredZoneFraming) {
        guard let boundaryVertices = framing.boundaryVertices, !boundaryVertices.isEmpty else {
          clearPolygonOverlay(from: mapView)
          applyRadiusOverlay(
            to: mapView,
            centreLat: framing.centreLat,
            centreLon: framing.centreLon,
            radius: framing.radius)
          return
        }
        clearRadiusOverlay(from: mapView)
        applyPolygonOverlay(to: mapView, vertices: boundaryVertices)
      }

      private func applyRadiusOverlay(
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

      private func applyPolygonOverlay(to mapView: MKMapView, vertices: [Coordinate]) {
        if renderedVertices == vertices {
          return
        }
        if let polygonOverlay {
          mapView.removeOverlay(polygonOverlay)
        }
        let coordinates = vertices.map { vertex in
          CLLocationCoordinate2D(latitude: vertex.latitude, longitude: vertex.longitude)
        }
        let polygon = MKPolygon(coordinates: coordinates, count: coordinates.count)
        mapView.addOverlay(polygon, level: .aboveRoads)
        polygonOverlay = polygon
        renderedVertices = vertices
      }

      private func clearRadiusOverlay(from mapView: MKMapView) {
        guard let radiusOverlay else { return }
        mapView.removeOverlay(radiusOverlay)
        self.radiusOverlay = nil
        renderedCentreLat = nil
        renderedCentreLon = nil
        renderedRadius = nil
      }

      private func clearPolygonOverlay(from mapView: MKMapView) {
        guard let polygonOverlay else { return }
        mapView.removeOverlay(polygonOverlay)
        self.polygonOverlay = nil
        renderedVertices = nil
      }

      // MARK: - Camera framing

      func frameCamera(
        on mapView: MKMapView, framing: ClusteredZoneFraming, zoneId: WatchZoneId?, animated: Bool
      ) {
        mapView.setRegion(Self.cameraRegion(framing: framing), animated: animated)
        framedZoneId = zoneId
        framedZoneFraming = framing
      }

      func frameCameraIfZoneChanged(
        on mapView: MKMapView, framing: ClusteredZoneFraming, zoneId: WatchZoneId?
      ) {
        guard zoneId != framedZoneId || framing != framedZoneFraming else { return }
        frameCamera(on: mapView, framing: framing, zoneId: zoneId, animated: true)
      }

      /// The region to frame for a zone: fitted to the polygon's bounding box
      /// for a custom shape (tc-7se1w.3), or the existing 2.5x-radius span for
      /// a circle.
      private static func cameraRegion(framing: ClusteredZoneFraming) -> MKCoordinateRegion {
        guard let boundaryVertices = framing.boundaryVertices, !boundaryVertices.isEmpty else {
          return circleRegion(framing: framing)
        }
        return PolygonBoundingRegion.fitting(
          vertices: boundaryVertices,
          marginFraction: polygonMarginFraction,
          minimumSpanDegrees: polygonMinimumSpanDegrees
        ) ?? circleRegion(framing: framing)
      }

      private static func circleRegion(framing: ClusteredZoneFraming) -> MKCoordinateRegion {
        // Span 2.5x the zone radius so the whole circle plus a margin is visible.
        MKCoordinateRegion(
          center: framing.centre,
          latitudinalMeters: framing.radius * 2.5,
          longitudinalMeters: framing.radius * 2.5)
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
          if cluster.isStacked {
            // Members are coincident (or closer than the finest grid cell), so no
            // zoom level can ever split them. Open the disambiguation list of the
            // stacked applications instead of zooming forever (GH#722).
            let capturedViewModel = self.viewModel
            Task { await capturedViewModel.selectStack(cluster) }
          } else {
            // Splittable cell: zoom into it so its members spread into finer cells
            // on the next (debounced) refetch.
            var region = mapView.region
            region.center = annotation.coordinate
            region.span = MKCoordinateSpan(
              latitudeDelta: max(region.span.latitudeDelta / 2, 0.0005),
              longitudeDelta: max(region.span.longitudeDelta / 2, 0.0005))
            mapView.setRegion(region, animated: true)
          }
        } else {
          let capturedViewModel = self.viewModel
          Task { await capturedViewModel.selectCluster(cluster) }
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
        let capturedViewModel = self.viewModel
        refetchTask?.cancel()
        refetchTask = Task { @MainActor in
          try? await Task.sleep(nanoseconds: 250_000_000)
          guard !Task.isCancelled else { return }
          await capturedViewModel.loadClusters(viewport: viewport, zoom: zoom)
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
        if let circle = overlay as? MKCircle {
          let renderer = MKCircleRenderer(circle: circle)
          renderer.strokeColor = UIColor(Color.tcAmber.opacity(0.3))
          renderer.fillColor = UIColor(Color.tcAmber.opacity(0.08))
          renderer.lineWidth = 1.5
          return renderer
        }
        if let polygon = overlay as? MKPolygon {
          let renderer = MKPolygonRenderer(polygon: polygon)
          renderer.strokeColor = UIColor(Color.tcAmber.opacity(0.3))
          renderer.fillColor = UIColor(Color.tcAmber.opacity(0.08))
          renderer.lineWidth = 1.5
          return renderer
        }
        return MKOverlayRenderer(overlay: overlay)
      }
    }
  }
#endif
