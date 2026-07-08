#if canImport(UIKit)
  import MapKit
  import SwiftUI
  import TownCrierDomain

  /// A UIKit `MKMapView` wrapped for SwiftUI that renders the anonymous map's
  /// pins with MapKit's built-in CLIENT-SIDE clustering (`clusteringIdentifier`)
  /// — unlike the authenticated map's `ClusteredMapView`, which renders
  /// server-computed cluster aggregates. near-point returns at most 200
  /// individual points (GH#868 Phase 2), a small enough set that on-device
  /// grid clustering is correct and needs no new backend endpoint (GH#868
  /// Phase 3 refinement).
  ///
  /// Reuses `ClusteredMapView`'s visual language (amber count bubble,
  /// status-coloured single pin, `MKCircle` radius overlay) without touching
  /// that file — the anonymous surface is a deliberately reduced,
  /// self-contained screen (see `AnonymousMapView`'s header).
  @MainActor
  struct AnonymousClusteredMapView: UIViewRepresentable {
    /// Observed so `updateUIView` re-runs whenever the ViewModel publishes —
    /// a refetch, a selection, a radius-slider drag. A plain stored reference
    /// is NOT enough: SwiftUI treats the representable as unchanged when its
    /// only stored property is the same `AnonymousMapViewModel` instance, so
    /// it skips `updateUIView` and new pins/radius changes never reach the
    /// map (mirrors `ClusteredMapView`).
    @ObservedObject var viewModel: AnonymousMapViewModel

    static let markerReuseIdentifier = "anonymous-planning-application"
    static let clusteringIdentifier = "anonymous-planning-application-cluster"

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
        radius: viewModel.selectedRadiusMetres,
        animated: false)
      coordinator.syncAnnotations(on: mapView, desired: viewModel.applications)
      coordinator.applyRadiusOverlay(
        to: mapView,
        anchor: viewModel.anchorCoordinate,
        radius: viewModel.selectedRadiusMetres)
      return mapView
    }

    func updateUIView(_ mapView: MKMapView, context: Context) {
      let coordinator = context.coordinator
      coordinator.syncAnnotations(on: mapView, desired: viewModel.applications)
      coordinator.applyRadiusOverlay(
        to: mapView,
        anchor: viewModel.anchorCoordinate,
        radius: viewModel.selectedRadiusMetres)
      // Reframe only when the selected radius actually changed, so a pin
      // refetch (pan/zoom exploring) never yanks the user's current viewport
      // back — mirrors `ClusteredMapView.frameCameraIfZoneChanged`.
      coordinator.reframeIfRadiusChanged(
        on: mapView,
        centre: Self.coordinate(for: viewModel.anchorCoordinate),
        radius: viewModel.selectedRadiusMetres)
    }

    static func coordinate(for coordinate: Coordinate) -> CLLocationCoordinate2D {
      CLLocationCoordinate2D(latitude: coordinate.latitude, longitude: coordinate.longitude)
    }
  }

  extension AnonymousClusteredMapView {
    /// `MKMapViewDelegate` for ``AnonymousClusteredMapView``. Styles individual
    /// and MapKit-synthesised cluster markers, routes a single-pin tap to
    /// ``AnonymousMapViewModel/selectApplication(_:)``, and a cluster tap
    /// either to a zoom-in (MapKit's own clustering re-splits the group at the
    /// finer zoom level) or, when ``AnonymousClusterStackDetector`` finds the
    /// members coincident (or the region is already at its zoom floor), to
    /// ``AnonymousMapViewModel/selectStack(_:)`` — there is no server-side
    /// "stacked/unsplittable" signal here, so the decision is made on-device
    /// (GH#877). Also renders the radius preview circle and forwards viewport
    /// changes to
    /// ``AnonymousMapViewModel/regionDidChange(centreLat:centreLon:radiusMetres:)``,
    /// which debounces internally.
    @MainActor
    final class Coordinator: NSObject, MKMapViewDelegate {
      private let viewModel: AnonymousMapViewModel

      /// The radius the camera is currently framed on, so a pin refetch never
      /// reframes — only an actual radius-slider change does.
      private var framedRadius: Double?
      /// The currently-rendered radius circle and the radius it was drawn
      /// for, so it's redrawn only when the radius actually changes.
      private var radiusOverlay: MKCircle?
      private var renderedRadius: Double?

      init(viewModel: AnonymousMapViewModel) {
        self.viewModel = viewModel
      }

      // MARK: - Annotation diffing

      /// Applies only the delta between the displayed pins and `desired` — at
      /// most 200 points, so a full diff stays cheap.
      func syncAnnotations(on mapView: MKMapView, desired: [PlanningApplication]) {
        let current = mapView.annotations.compactMap { $0 as? AnonymousApplicationAnnotation }
        let currentIds = Set(current.map(\.applicationId))
        let desiredById = Dictionary(
          uniqueKeysWithValues: desired.compactMap { application in
            application.location != nil ? (application.id, application) : nil
          })

        let toRemove = current.filter { !desiredById.keys.contains($0.applicationId) }
        if !toRemove.isEmpty {
          mapView.removeAnnotations(toRemove)
        }

        let toAdd =
          desiredById.values
          .filter { !currentIds.contains($0.id) }
          .compactMap(AnonymousApplicationAnnotation.init(application:))
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
        let view = mapView.dequeueReusableAnnotationView(
          withIdentifier: AnonymousClusteredMapView.markerReuseIdentifier, for: annotation)
        guard let marker = view as? MKMarkerAnnotationView else { return view }
        marker.canShowCallout = false

        if let cluster = annotation as? MKClusterAnnotation {
          // Brand amber: a cluster is a navigational aggregate, not a status,
          // so it takes the design system's brand accent rather than any
          // `tcStatus*` colour (which would falsely imply a single status for
          // the group) — mirrors `ClusteredMapView`.
          marker.markerTintColor = UIColor(Color.tcAmber)
          marker.glyphImage = nil
          marker.glyphText = Self.bubbleGlyph(for: cluster.memberAnnotations.count)
          marker.displayPriority = .required
        } else if let point = annotation as? AnonymousApplicationAnnotation {
          marker.clusteringIdentifier = AnonymousClusteredMapView.clusteringIdentifier
          marker.markerTintColor = UIColor(point.status.displayColor)
          marker.glyphText = nil
          marker.glyphImage = UIImage(systemName: "mappin.circle.fill")
          marker.displayPriority = .defaultHigh
        }
        return marker
      }

      /// The count shown inside an amber bubble, capped so a fully-zoomed-out
      /// dense cluster stays legible.
      private static func bubbleGlyph(for count: Int) -> String {
        count > 999 ? "999+" : "\(count)"
      }

      func mapView(_ mapView: MKMapView, didSelect view: MKAnnotationView) {
        guard let annotation = view.annotation else { return }
        mapView.deselectAnnotation(annotation, animated: false)

        if let cluster = annotation as? MKClusterAnnotation {
          let members = cluster.memberAnnotations.compactMap { memberAnnotation in
            memberAnnotation as? AnonymousApplicationAnnotation
          }
          let regionSpan = max(mapView.region.span.latitudeDelta, mapView.region.span.longitudeDelta)
          let isStacked = AnonymousClusterStackDetector.isStacked(
            memberCoordinates: members.map(\.coordinate), regionSpanDegrees: regionSpan)

          if isStacked {
            // Coincident (or effectively coincident) members: no zoom level
            // can ever split them, so open the disambiguation list instead
            // (GH#877). No network call — every member's full
            // `PlanningApplication` is already loaded from `near-point`.
            viewModel.selectStack(members.map(\.application))
          } else {
            // Splittable cluster: MapKit's own clustering re-splits the group
            // once the map zooms in far enough.
            var region = mapView.region
            region.center = cluster.coordinate
            region.span = MKCoordinateSpan(
              latitudeDelta: max(region.span.latitudeDelta / 2, 0.0005),
              longitudeDelta: max(region.span.longitudeDelta / 2, 0.0005))
            mapView.setRegion(region, animated: true)
          }
        } else if let point = annotation as? AnonymousApplicationAnnotation {
          viewModel.selectApplication(point.application)
        }
      }

      func mapView(_ mapView: MKMapView, regionDidChangeAnimated animated: Bool) {
        // `AnonymousMapViewModel.regionDidChange` debounces internally
        // (cancels/reschedules its own pending fetch), so forwarding every
        // intermediate `regionDidChangeAnimated` call here still collapses to
        // one fetch once a pan/zoom gesture settles.
        let region = mapView.region
        let span = max(region.span.latitudeDelta, region.span.longitudeDelta)
        viewModel.regionDidChange(
          centreLat: region.center.latitude,
          centreLon: region.center.longitude,
          radiusMetres: span * 111_320 / 2)
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

  /// A reference-type `MKAnnotation` wrapping a value-type `PlanningApplication`
  /// so MapKit can hold it and cluster it client-side. Carries the application
  /// the coordinator needs to style the marker and route a tap. Fails to
  /// initialise for an application with no `location` — such applications are
  /// filtered out before reaching the map.
  final class AnonymousApplicationAnnotation: NSObject, MKAnnotation {
    let application: PlanningApplication
    let applicationId: PlanningApplicationId
    let coordinate: CLLocationCoordinate2D
    var status: ApplicationStatus { application.status }

    init?(application: PlanningApplication) {
      guard let location = application.location else { return nil }
      self.application = application
      self.applicationId = application.id
      self.coordinate = CLLocationCoordinate2D(
        latitude: location.latitude, longitude: location.longitude)
    }
  }
#endif
