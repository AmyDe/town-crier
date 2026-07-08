import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Split out of `AnonymousMapViewModelTests` (file-length discipline) —
/// GH#879 Phase 4 defect fix: live simulator verification found that
/// switching the active zone on the Applications tab left the Map tab
/// frozen on the previous zone until a full relaunch. Root cause: the
/// coordinator was replacing `AnonymousMapViewModel` wholesale, which
/// `AnonymousMapView`'s `@StateObject` silently ignores (a replaced
/// constructor argument on an already-mounted view is a no-op). The fix
/// instead mutates a live instance via ``AnonymousMapViewModel/updateActiveZone(_:)``.
@Suite("AnonymousMapViewModel — active-zone re-centring in place (GH#879 Phase 4)")
@MainActor
struct AnonymousMapViewModelActiveZoneTests {
  private func makeSUT(
    coordinate: Coordinate = .cambridge,
    radiusMetres: Double = 2000
  ) -> (AnonymousMapViewModel, SpyAnonymousApplicationsRepository) {
    let repository = SpyAnonymousApplicationsRepository()
    let sut = AnonymousMapViewModel(
      repository: repository, coordinate: coordinate, radiusMetres: radiusMetres)
    return (sut, repository)
  }

  private func makeZone(radiusMetres: Double = 3000) throws -> DeviceLocalZone {
    try DeviceLocalZone(
      name: "London",
      centre: try Coordinate(latitude: 51.5074, longitude: -0.1278),
      radiusMetres: radiusMetres
    )
  }

  @Test func updateActiveZone_updatesCentreAndAnchorToZoneCentre() async throws {
    let (sut, repository) = makeSUT(coordinate: .cambridge)
    repository.fetchNearbyResult = .success([])
    let zone = try makeZone()

    await sut.updateActiveZone(zone)

    #expect(sut.centreLat == zone.centre.latitude)
    #expect(sut.centreLon == zone.centre.longitude)
    #expect(sut.anchorCoordinate == zone.centre)
  }

  @Test func updateActiveZone_updatesRadiusMetresToZoneRadius() async throws {
    let (sut, repository) = makeSUT(radiusMetres: 1000)
    repository.fetchNearbyResult = .success([])
    let zone = try makeZone(radiusMetres: 4000)

    await sut.updateActiveZone(zone)

    #expect(sut.radiusMetres == 4000)
  }

  @Test func updateActiveZone_clampsSelectedRadiusToFreeTierPreviewCap() async throws {
    let (sut, repository) = makeSUT()
    repository.fetchNearbyResult = .success([])
    let zone = try makeZone(radiusMetres: 5000)

    await sut.updateActiveZone(zone)

    #expect(sut.selectedRadiusMetres == AnonymousMapViewModel.maxSelectedRadiusMetres)
  }

  @Test func updateActiveZone_refetchesApplicationsAtTheNewZonesCoordinateAndRadius() async throws {
    let (sut, repository) = makeSUT(coordinate: .cambridge)
    repository.fetchNearbyResult = .success([.pendingReview])
    let zone = try makeZone(radiusMetres: 4000)

    await sut.updateActiveZone(zone)

    let call = repository.fetchNearbyCalls.last
    #expect(call?.latitude == zone.centre.latitude)
    #expect(call?.longitude == zone.centre.longitude)
    #expect(call?.radiusMetres == 4000)
    #expect(sut.applications == [.pendingReview])
  }

  @Test func updateActiveZone_clearsAnyPendingSelectionState() async throws {
    let (sut, repository) = makeSUT()
    repository.fetchNearbyResult = .success([])
    sut.selectApplication(.pendingReview)
    sut.selectStack([.pendingReview, .permitted])
    let zone = try makeZone()

    await sut.updateActiveZone(zone)

    #expect(sut.selectedApplication == nil)
    #expect(sut.stackedApplications == nil)
  }
}
