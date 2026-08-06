import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// `MapViewModel.boundaryVertices` — threads the selected zone's custom-shape
/// polygon (GH#1031) through to `ClusteredMapView` so it can draw an
/// `MKPolygon` overlay instead of always drawing an `MKCircle` (tc-7se1w.3).
/// `nil` means "this zone is a circle" — the map view's existing circle path
/// stays untouched for that case.
@Suite("MapViewModel boundary vertices")
@MainActor
struct MapViewModelBoundaryVerticesTests {
  private func makeSUT(
    watchZones: [WatchZone]
  ) -> (MapViewModel, SpyWatchZoneRepository) {
    let spy = SpyPlanningApplicationRepository()
    spy.fetchClustersResult = .success([])
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success(watchZones)
    let vm = MapViewModel(repository: spy, watchZoneRepository: watchZoneSpy)
    return (vm, watchZoneSpy)
  }

  @Test func boundaryVertices_nilForCircleZone() async {
    let (sut, _) = makeSUT(watchZones: [.cambridge])

    await sut.loadApplications()

    #expect(sut.boundaryVertices == nil)
  }

  @Test func boundaryVertices_returnsVerticesForCustomShapeZone() async throws {
    let boundary = try WatchZoneBoundary(vertices: [
      Coordinate(latitude: 51.50, longitude: -0.10),
      Coordinate(latitude: 51.51, longitude: -0.09),
      Coordinate(latitude: 51.50, longitude: -0.09),
    ])
    let zone = try WatchZone(
      name: "Custom Area",
      centre: Coordinate(latitude: 51.505, longitude: -0.095),
      radiusMetres: 1000,
      boundary: boundary
    )
    let (sut, _) = makeSUT(watchZones: [zone])

    await sut.loadApplications()

    #expect(sut.boundaryVertices == boundary.vertices)
  }

  @Test func boundaryVertices_updatesOnSelectZone() async throws {
    let boundary = try WatchZoneBoundary(vertices: [
      Coordinate(latitude: 51.50, longitude: -0.10),
      Coordinate(latitude: 51.51, longitude: -0.09),
      Coordinate(latitude: 51.50, longitude: -0.09),
    ])
    let customZone = try WatchZone(
      id: WatchZoneId("zone-custom"),
      name: "Custom Area",
      centre: Coordinate(latitude: 51.505, longitude: -0.095),
      radiusMetres: 1000,
      boundary: boundary
    )
    let (sut, _) = makeSUT(watchZones: [.cambridge, customZone])
    await sut.loadApplications()
    #expect(sut.boundaryVertices == nil)

    await sut.selectZone(customZone)

    #expect(sut.boundaryVertices == boundary.vertices)
  }
}
