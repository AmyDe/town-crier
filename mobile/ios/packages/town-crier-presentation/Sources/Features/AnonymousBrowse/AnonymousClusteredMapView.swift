#if canImport(UIKit)
  import MapKit
  import SwiftUI
  import TownCrierDomain

  /// A UIKit `MKMapView` wrapped for SwiftUI that renders the anonymous map's
  /// server-computed cluster aggregates for the current viewport, 1:1 like the
  /// authenticated map's `ClusteredMapView`. On a region change it tells the
  /// ViewModel (debounced ~250ms) to refetch clusters for the new visible rect.
  /// It uses no MapKit auto-clustering identifier.
  ///
  /// `Coordinator.scheduleUpdate(_:)` defers and coalesces `updateUIView`
  /// mutations onto one later main-actor hop. Mutating annotations, overlay or
  /// camera synchronously and re-entrantly crashed (`SIGABRT`) inside MapKit's
  /// deferred clustering pass
  /// (`-[MKMapView annotationContainer:requestAddingClusterForAnnotationViews:]`)
  /// on rapid zone or tab switches. Do not remove it; the race cannot be
  /// reproduced in `swift test`.
  @MainActor
  struct AnonymousClusteredMapView: UIViewRepresentable {
    /// Observed so `updateUIView` re-runs whenever the ViewModel publishes —
    /// a refetch, a selection, an active-zone switch. A plain stored
    /// reference is NOT enough: SwiftUI treats the representable as unchanged
    /// when its only stored property is the same `AnonymousMapViewModel`
    /// instance, so it skips `updateUIView` and new clusters/radius changes
    /// never reach the map (mirrors `ClusteredMapView`).
    @ObservedObject var viewModel: AnonymousMapViewModel

    static let markerReuseIdentifier = "anonymous-planning-application-cluster"

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
        centre: Self.coordinate(for: viewModel.anchorCoordinate),
        radius: viewModel.radiusMetres,
        animated: false)
      coordinator.syncAnnotations(on: mapView, desired: viewModel.clusters)
      coordinator.applyRadiusOverlay(
        to: mapView,
        anchor: viewModel.anchorCoordinate,
        radius: viewModel.radiusMetres)
      return mapView
    }

    func updateUIView(_ mapView: MKMapView, context: Context) {
      // Deferred and coalesced to avoid a MapKit crash; see this file's header.
      let coordinator = context.coordinator
      let currentViewModel = viewModel
      coordinator.scheduleUpdate {
        coordinator.syncAnnotations(on: mapView, desired: currentViewModel.clusters)
        coordinator.applyRadiusOverlay(
          to: mapView,
          anchor: currentViewModel.anchorCoordinate,
          radius: currentViewModel.radiusMetres)
        // Reframe only when the radius actually changed (an active-zone
        // switch), so a cluster refetch never yanks the user's current
        // viewport back — mirrors `ClusteredMapView.frameCameraIfZoneChanged`.
        coordinator.reframeIfRadiusChanged(
          on: mapView,
          centre: Self.coordinate(for: currentViewModel.anchorCoordinate),
          radius: currentViewModel.radiusMetres)
      }
    }

    static func coordinate(for coordinate: Coordinate) -> CLLocationCoordinate2D {
      CLLocationCoordinate2D(latitude: coordinate.latitude, longitude: coordinate.longitude)
    }
  }

  extension AnonymousClusteredMapView {
    /// `MKMapViewDelegate` for ``AnonymousClusteredMapView``. Holds no
    /// business logic — it styles cluster markers, routes a single-member tap
    /// to ``AnonymousMapViewModel/selectCluster(_:)``, opens the
    /// disambiguation list for a stacked (unsplittable) multi-member cell via
    /// ``AnonymousMapViewModel/selectStack(_:)`` while zooming into a
    /// splittable one, debounces the region-change refetch, and renders the
    /// radius circle — mirrors `ClusteredMapView.Coordinator` throughout.
    @MainActor
    final class Coordinator: NSObject, MKMapViewDelegate {
      private let viewModel: AnonymousMapViewModel

      /// Guards ``scheduleUpdate(_:)``'s coalescing; see this file's header.
      private var hasScheduledUpdate = false

      /// The radius the camera is currently framed on, so a cluster refetch
      /// never reframes — only an actual radius change (active-zone switch)
      /// does.
      private var framedRadius: Double?
      /// The currently-rendered radius circle and the radius it was drawn
      /// for, so it's redrawn only when the radius actually changes.
      private var radiusOverlay: MKCircle?
      private var renderedRadius: Double?

      /// The pending debounced viewport refetch, cancelled and rescheduled on
      /// each region change so a pan/zoom flurry issues a single fetch when
      /// it settles.
      private var refetchTask: Task<Void, Never>?

      init(viewModel: AnonymousMapViewModel) {
        self.viewModel = viewModel
      }

      // MARK: - Update coalescing

      /// Coalesces bursts of `updateUIView` calls (a refetch, a selection, an
      /// active-zone switch — several `@Published` properties can change
      /// together in one shot, e.g. `AnonymousMapViewModel.updateActiveZone(_:)`)
      /// onto a SINGLE deferred application, one main-actor hop later, rather
      /// than mutating MapKit's annotation/overlay/camera state synchronously
      /// and possibly re-entrantly on every call — see this file's header for
      /// the crash this closes. `apply` reads the ViewModel's state fresh at
      /// the point it actually runs, so a coalesced burst always applies the
      /// LATEST state, never a stale intermediate one; calls arriving while
      /// one is already pending are simply dropped (the pending one will pick
      /// up their state too).
      func scheduleUpdate(_ apply: @escaping @MainActor () -> Void) {
        guard !hasScheduledUpdate else { return }
        hasScheduledUpdate = true
        Task { @MainActor [weak self] in
          self?.hasScheduledUpdate = false
          apply()
        }
      }

      // MARK: - Annotation diffing

      /// Applies only the delta between the displayed cluster markers and
      /// `desired`. The set is the handful of cells in the viewport, not
      /// every application in the radius, so this stays cheap and never
      /// churns the whole map.
      func syncAnnotations(on mapView: MKMapView, desired: [AnonymousMapCluster]) {
        let current = mapView.annotations.compactMap { $0 as? AnonymousMapClusterAnnotation }
        let currentIds = Set(current.map(\.clusterId))
        let desiredIds = Set(desired.map(\.id))

        let toRemove = current.filter { !desiredIds.contains($0.clusterId) }
        if !toRemove.isEmpty {
          mapView.removeAnnotations(toRemove)
        }

        let toAdd =
          desired
          .filter { !currentIds.contains($0.id) }
          .map(AnonymousMapClusterAnnotation.init(cluster:))
        if !toAdd.isEmpty {
          mapView.addAnnotations(toAdd)
        }
      }

      // MARK: - Radius overlay

      func applyRadiusOverlay(to mapView: MKMapView, anchor: Coordinate, radius: Double) {
        if renderedRadius == radius {
          return
        }
        if let radiusOverlay {
          mapView.removeOverlay(radiusOverlay)
        }
        let circle = MKCircle(
          center: AnonymousClusteredMapView.coordinate(for: anchor), radius: radius)
        mapView.addOverlay(circle, level: .aboveRoads)
        radiusOverlay = circle
        renderedRadius = radius
      }

      // MARK: - Camera framing

      func frameCamera(
        on mapView: MKMapView, centre: CLLocationCoordinate2D, radius: Double, animated: Bool
      ) {
        // Span 2.5x the radius so the whole circle plus a margin is visible.
        let region = MKCoordinateRegion(
          center: centre, latitudinalMeters: radius * 2.5, longitudinalMeters: radius * 2.5)
        mapView.setRegion(region, animated: animated)
        framedRadius = radius
      }

      func reframeIfRadiusChanged(
        on mapView: MKMapView, centre: CLLocationCoordinate2D, radius: Double
      ) {
        guard radius != framedRadius else { return }
        frameCamera(on: mapView, centre: centre, radius: radius, animated: true)
      }

      // MARK: - MKMapViewDelegate

      func mapView(_ mapView: MKMapView, viewFor annotation: MKAnnotation) -> MKAnnotationView? {
        if annotation is MKUserLocation { return nil }
        guard let cluster = annotation as? AnonymousMapClusterAnnotation else { return nil }
        return markerView(for: cluster, on: mapView)
      }

      private func markerView(
        for annotation: AnonymousMapClusterAnnotation, on mapView: MKMapView
      ) -> MKAnnotationView {
        let view = mapView.dequeueReusableAnnotationView(
          withIdentifier: AnonymousClusteredMapView.markerReuseIdentifier, for: annotation)
        guard let marker = view as? MKMarkerAnnotationView else { return view }
        marker.annotation = annotation
        marker.canShowCallout = false

        if annotation.cluster.count > 1 {
          // Brand amber: a cluster is a navigational aggregate, not a status,
          // so it takes the design system's brand accent rather than any
          // `tcStatus*` colour (which would falsely imply a single status for
          // the group) — mirrors `ClusteredMapView`.
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
        guard let annotation = view.annotation as? AnonymousMapClusterAnnotation else { return }
        let cluster = annotation.cluster
        mapView.deselectAnnotation(annotation, animated: false)

        if cluster.count > 1 {
          if cluster.isStacked {
            // Members are coincident (or closer than the finest grid cell),
            // so no zoom level can ever split them. Open the disambiguation
            // list of the stacked applications instead of zooming forever.
            Task { [viewModel] in await viewModel.selectStack(cluster) }
          } else {
            // Splittable cell: zoom into it so its members spread into finer
            // cells on the next (debounced) refetch.
            var region = mapView.region
            region.center = annotation.coordinate
            region.span = MKCoordinateSpan(
              latitudeDelta: max(region.span.latitudeDelta / 2, 0.0005),
              longitudeDelta: max(region.span.longitudeDelta / 2, 0.0005))
            mapView.setRegion(region, animated: true)
          }
        } else {
          Task { [viewModel] in await viewModel.selectCluster(cluster) }
        }
      }

      func mapView(_ mapView: MKMapView, regionDidChangeAnimated animated: Bool) {
        scheduleClusterRefetch(for: mapView)
      }

      /// Debounces the viewport refetch: cancels any pending fetch and
      /// schedules a fresh one ~250ms out, so a continuous pan/zoom gesture
      /// issues one fetch when it settles rather than dozens mid-gesture —
      /// mirrors `ClusteredMapView.Coordinator.scheduleClusterRefetch`.
      private func scheduleClusterRefetch(for mapView: MKMapView) {
        let viewport = Self.viewport(from: mapView)
        let zoom = Self.zoom(from: mapView)
        refetchTask?.cancel()
        refetchTask = Task { @MainActor [viewModel] in
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

  /// A reference-type `MKAnnotation` wrapping a value-type ``AnonymousMapCluster``
  /// so MapKit can hold it. Carries the cluster the coordinator needs to
  /// style the marker and route a tap — mirrors `MapClusterAnnotation`.
  final class AnonymousMapClusterAnnotation: NSObject, MKAnnotation {
    let cluster: AnonymousMapCluster
    let clusterId: String
    let coordinate: CLLocationCoordinate2D

    init(cluster: AnonymousMapCluster) {
      self.cluster = cluster
      self.clusterId = cluster.id
      self.coordinate = CLLocationCoordinate2D(
        latitude: cluster.coordinate.latitude, longitude: cluster.coordinate.longitude)
    }
  }
#endif
