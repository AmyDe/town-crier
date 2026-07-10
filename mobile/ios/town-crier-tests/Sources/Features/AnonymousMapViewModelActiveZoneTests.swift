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
///
/// A second live-simulator-verified defect (crash) followed: switching
/// zones rapidly could SIGABRT inside MapKit's own deferred clustering pass.
/// `updateActiveZone(_:)`'s fetch (and therefore the annotation-set swap it
/// drives) is debounced — proven here; the MapKit-internal timing itself is
/// UIKit/on-sim-only and covered separately (see
/// `AnonymousClusteredMapView`'s file header).
///
/// GH#912 Phase 4: panning no longer drives a fetch at all (the fetch radius
/// IS the zone radius now, matching the drawn circle exactly), so the old
/// pan/zone-switch mutual-cancellation test that lived here is gone along
/// with `regionDidChange` itself.
@Suite("AnonymousMapViewModel — active-zone re-centring in place (GH#879 Phase 4)")
@MainActor
struct AnonymousMapViewModelActiveZoneTests {
  private func makeSUT(
    coordinate: Coordinate = .cambridge,
    radiusMetres: Double = 2000,
    debounceNanoseconds: UInt64 = 5_000_000
  ) -> (AnonymousMapViewModel, SpyAnonymousApplicationsRepository) {
    let repository = SpyAnonymousApplicationsRepository()
    let sut = AnonymousMapViewModel(
      repository: repository,
      coordinate: coordinate,
      radiusMetres: radiusMetres,
      debounceNanoseconds: debounceNanoseconds
    )
    return (sut, repository)
  }

  private func makeZone(radiusMetres: Double = 3000) throws -> DeviceLocalZone {
    try DeviceLocalZone(
      name: "London",
      centre: try Coordinate(latitude: 51.5074, longitude: -0.1278),
      radiusMetres: radiusMetres
    )
  }

  // MARK: - Immediate (non-debounced) state

  @Test func updateActiveZone_updatesAnchorToZoneCentreImmediately() throws {
    let (sut, _) = makeSUT(coordinate: .cambridge)
    let zone = try makeZone()

    sut.updateActiveZone(zone)

    #expect(sut.anchorCoordinate == zone.centre)
  }

  @Test func updateActiveZone_updatesRadiusMetresToZoneRadiusImmediately() throws {
    let (sut, _) = makeSUT(radiusMetres: 1000)
    let zone = try makeZone(radiusMetres: 4000)

    sut.updateActiveZone(zone)

    #expect(sut.radiusMetres == 4000)
  }

  /// GH#912 Phase 4 (honest anon map): the drawn circle/fetch radius takes
  /// the zone's ACTUAL radius, never clamped to the free-tier preview cap —
  /// otherwise pins between the cap and the zone's real radius would render
  /// outside the drawn circle, exactly the bug this phase fixes.
  @Test func updateActiveZone_setsRadiusToZonesActualRadius_evenAboveFreeTierCap() throws {
    let (sut, _) = makeSUT()
    let zone = try makeZone(radiusMetres: 5000)

    sut.updateActiveZone(zone)

    #expect(sut.radiusMetres == 5000)
  }

  @Test func updateActiveZone_clearsAnyPendingSelectionStateImmediately() throws {
    let (sut, _) = makeSUT()
    sut.selectApplication(.pendingReview)
    sut.selectStack([.pendingReview, .permitted])
    let zone = try makeZone()

    sut.updateActiveZone(zone)

    #expect(sut.selectedApplication == nil)
    #expect(sut.stackedApplications == nil)
  }

  // MARK: - Debounced fetch (crash fix: collapses rapid churn to one swap)

  @Test func updateActiveZone_refetchesApplicationsAtTheNewZonesCoordinateAndRadius() async throws {
    let (sut, repository) = makeSUT(coordinate: .cambridge)
    repository.fetchNearbyResult = .success([.pendingReview])
    let zone = try makeZone(radiusMetres: 4000)

    sut.updateActiveZone(zone)
    await sut.waitForPendingActiveZoneUpdate()

    let call = repository.fetchNearbyCalls.last
    #expect(call?.latitude == zone.centre.latitude)
    #expect(call?.longitude == zone.centre.longitude)
    #expect(call?.radiusMetres == 4000)
    #expect(sut.applications == [.pendingReview])
  }

  @Test func updateActiveZone_rapidSuccessiveCalls_onlyTheLastZoneFetches() async throws {
    let (sut, repository) = makeSUT()
    repository.fetchNearbyResult = .success([.pendingReview])
    let zoneA = try makeZone(radiusMetres: 1000)
    let zoneB = try makeZone(radiusMetres: 4000)

    sut.updateActiveZone(zoneA)
    sut.updateActiveZone(zoneB)
    await sut.waitForPendingActiveZoneUpdate()

    #expect(repository.fetchNearbyCalls.count == 1)
    #expect(repository.fetchNearbyCalls[0].radiusMetres == 4000)
    // The final state reflects zone B throughout, not a stale zone A value.
    #expect(sut.radiusMetres == 4000)
    #expect(sut.anchorCoordinate == zoneB.centre)
  }
}
