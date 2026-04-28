import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Regression tests for tc-edxv: editing a watch zone in place left
/// `ApplicationListViewModel.selectedZone` stale because `loadApplications()`
/// only refreshed `selectedZone` when its id was missing from the reloaded
/// list. An in-place edit keeps the same id, so the stale value-typed copy
/// (with the old radius/centre) survived until the app cold-resumed.
@Suite("ApplicationListViewModel stale-zone refresh (tc-edxv)")
@MainActor
struct ApplicationListViewModelStaleZoneTests {
  @Test func loadApplications_refreshesSelectedZone_whenSameIdReloaded() async throws {
    let originalZone = try WatchZone(
      id: WatchZoneId("zone-home"),
      name: "Home",
      centre: try Coordinate(latitude: 51.5, longitude: -0.1),
      radiusMetres: 300
    )
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success([])
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([originalZone])
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    let sut = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy,
      userDefaults: defaults,
      zoneSelectionKey: "test.zone"
    )

    await sut.loadApplications()
    #expect(sut.selectedZone?.radiusMetres == 300)

    // Simulate an in-place edit: same id, new radius.
    let editedZone = try WatchZone(
      id: WatchZoneId("zone-home"),
      name: "Home",
      centre: try Coordinate(latitude: 51.5, longitude: -0.1),
      radiusMetres: 200
    )
    zoneSpy.loadAllResult = .success([editedZone])

    await sut.loadApplications()

    #expect(sut.selectedZone?.radiusMetres == 200)
  }

  @Test func loadApplications_refreshesSelectedZone_whenCentreChanged() async throws {
    let originalZone = try WatchZone(
      id: WatchZoneId("zone-home"),
      name: "Home",
      centre: try Coordinate(latitude: 51.5, longitude: -0.1),
      radiusMetres: 300
    )
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success([])
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([originalZone])
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    let sut = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy,
      userDefaults: defaults,
      zoneSelectionKey: "test.zone"
    )

    await sut.loadApplications()

    let editedZone = try WatchZone(
      id: WatchZoneId("zone-home"),
      name: "Home",
      centre: try Coordinate(latitude: 52.0, longitude: 0.5),
      radiusMetres: 300
    )
    zoneSpy.loadAllResult = .success([editedZone])

    await sut.loadApplications()

    #expect(sut.selectedZone?.centre.latitude == 52.0)
    #expect(sut.selectedZone?.centre.longitude == 0.5)
  }
}
