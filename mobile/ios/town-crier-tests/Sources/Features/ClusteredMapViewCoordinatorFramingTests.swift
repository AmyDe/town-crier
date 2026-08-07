#if canImport(UIKit)
  import MapKit
  import Testing
  import TownCrierDomain

  @testable import TownCrierPresentation

  /// `ClusteredMapView.Coordinator.frameCameraIfZoneChanged(on:framing:zoneId:)`
  /// — regression coverage for a bug flagged by CodeRabbit on PR #1069: the
  /// guard compared only `zoneId`, so reloading the same zone with an edited
  /// boundary, centre, or radius left the camera framed on the stale
  /// geometry. Gated behind `#if canImport(UIKit)` like `ClusteredMapView`
  /// itself (see `reference_ios_uikit_gated_files_invisible_to_swift_test`).
  @Suite("ClusteredMapView.Coordinator camera framing")
  @MainActor
  struct ClusteredMapViewCoordinatorFramingTests {
    private static let zoneId = WatchZoneId("zone-1")

    private func makeSUT() -> (ClusteredMapView.Coordinator, MKMapView) {
      let spy = SpyPlanningApplicationRepository()
      spy.fetchClustersResult = .success([])
      let watchZoneSpy = SpyWatchZoneRepository()
      watchZoneSpy.loadAllResult = .success([])
      let viewModel = MapViewModel(repository: spy, watchZoneRepository: watchZoneSpy)
      let coordinator = ClusteredMapView.Coordinator(viewModel: viewModel)
      // A zero-size MKMapView clamps/ignores setRegion, so give it a real frame.
      let mapView = MKMapView(frame: CGRect(x: 0, y: 0, width: 390, height: 844))
      return (coordinator, mapView)
    }

    private func framing(centreLat: Double, radius: Double = 500) -> ClusteredZoneFraming {
      ClusteredZoneFraming(
        centreLat: centreLat, centreLon: 0.1218, radius: radius, boundaryVertices: nil)
    }

    @Test func sameZoneIdChangedGeometry_reframesCamera() {
      let (sut, mapView) = makeSUT()
      let original = framing(centreLat: 52.2053)
      let edited = framing(centreLat: 52.9000)

      sut.frameCameraIfZoneChanged(on: mapView, framing: original, zoneId: Self.zoneId)
      let framedLatitude = mapView.region.center.latitude

      sut.frameCameraIfZoneChanged(on: mapView, framing: edited, zoneId: Self.zoneId)

      #expect(mapView.region.center.latitude != framedLatitude)
      #expect(abs(mapView.region.center.latitude - edited.centreLat) < 0.01)
    }

    @Test func sameZoneIdSameGeometry_doesNotReframe() {
      let (sut, mapView) = makeSUT()
      let unchanged = framing(centreLat: 52.2053)

      sut.frameCameraIfZoneChanged(on: mapView, framing: unchanged, zoneId: Self.zoneId)

      // Nudge the map away from the framed region so a no-op reframe call is
      // distinguishable from a real one: a real reframe would snap it back.
      let sentinelRegion = MKCoordinateRegion(
        center: CLLocationCoordinate2D(latitude: 10, longitude: 10),
        latitudinalMeters: 1000,
        longitudinalMeters: 1000
      )
      mapView.setRegion(sentinelRegion, animated: false)

      sut.frameCameraIfZoneChanged(on: mapView, framing: unchanged, zoneId: Self.zoneId)

      #expect(abs(mapView.region.center.latitude - 10) < 0.01)
    }
  }
#endif
