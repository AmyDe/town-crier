#if canImport(UIKit)
  import MapKit
  import Testing
  import TownCrierDomain

  @testable import TownCrierPresentation

  /// `ClusteredMapView.Coordinator.applyZoneOverlay(to:framing:)` — regression
  /// coverage for a bug found in live simulator verification of tc-7se1w.3:
  /// selecting a custom-shape zone (drawing an `MKPolygon`) and then selecting
  /// a plain radius/circle zone left a stale (or wrongly-shaped) overlay
  /// instead of showing an `MKCircle`. Gated behind `#if canImport(UIKit)`
  /// like `ClusteredMapView` itself — `UIViewRepresentable` and `MKMapView`'s
  /// overlay APIs the Coordinator drives are UIKit-only, so this only compiles
  /// (and only catches this class of bug) when the test target builds for
  /// iOS, not the bare-macOS `swift test` run (see
  /// `reference_ios_uikit_gated_files_invisible_to_swift_test`).
  @Suite("ClusteredMapView.Coordinator zone overlay")
  @MainActor
  struct ClusteredMapViewCoordinatorOverlayTests {
    private func makeSUT() -> (ClusteredMapView.Coordinator, MKMapView) {
      let spy = SpyPlanningApplicationRepository()
      spy.fetchClustersResult = .success([])
      let watchZoneSpy = SpyWatchZoneRepository()
      watchZoneSpy.loadAllResult = .success([])
      let viewModel = MapViewModel(repository: spy, watchZoneRepository: watchZoneSpy)
      let coordinator = ClusteredMapView.Coordinator(viewModel: viewModel)
      let mapView = MKMapView()
      return (coordinator, mapView)
    }

    private func polygonFraming() throws -> ClusteredZoneFraming {
      let vertices = try [
        Coordinate(latitude: 52.2043, longitude: 0.1218),
        Coordinate(latitude: 52.2063, longitude: 0.1238),
        Coordinate(latitude: 52.2043, longitude: 0.1258),
      ]
      return ClusteredZoneFraming(
        centreLat: 52.2053, centreLon: 0.1218, radius: 500, boundaryVertices: vertices)
    }

    private func circleFraming() -> ClusteredZoneFraming {
      // KT2 6EJ-shaped fixture: a plain radius zone, nowhere near the polygon
      // fixture's coordinates, so a leftover polygon at the wrong location
      // would be as detectable as a leftover polygon shape at the right one.
      ClusteredZoneFraming(
        centreLat: 51.4095, centreLon: -0.2864, radius: 2000, boundaryVertices: nil)
    }

    @Test func customShapeThenCircle_showsOnlyACircleOverlay() throws {
      let (sut, mapView) = makeSUT()

      sut.applyZoneOverlay(to: mapView, framing: try polygonFraming())
      #expect(mapView.overlays.contains { $0 is MKPolygon })

      sut.applyZoneOverlay(to: mapView, framing: circleFraming())

      #expect(mapView.overlays.count == 1)
      #expect(mapView.overlays.first is MKCircle)
      #expect(!mapView.overlays.contains { $0 is MKPolygon })
    }

    @Test func circleThenCustomShape_showsOnlyAPolygonOverlay() throws {
      let (sut, mapView) = makeSUT()

      sut.applyZoneOverlay(to: mapView, framing: circleFraming())
      #expect(mapView.overlays.contains { $0 is MKCircle })

      sut.applyZoneOverlay(to: mapView, framing: try polygonFraming())

      #expect(mapView.overlays.count == 1)
      #expect(mapView.overlays.first is MKPolygon)
      #expect(!mapView.overlays.contains { $0 is MKCircle })
    }

    @Test func circleOnly_showsACircleOverlay() {
      let (sut, mapView) = makeSUT()

      sut.applyZoneOverlay(to: mapView, framing: circleFraming())

      #expect(mapView.overlays.count == 1)
      #expect(mapView.overlays.first is MKCircle)
    }
  }
#endif
