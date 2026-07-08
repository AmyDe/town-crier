import Foundation
import Testing

@testable import TownCrierDomain
@testable import TownCrierPresentation

/// GH#879 Phase 5: the authed Zones tab's dismissible "N areas from before
/// you signed up" row — visible while any device-local zones remain
/// unconverted, dismissible for the session, and re-shown next launch (a
/// fresh view-model instance) while zones still remain.
@MainActor
@Suite("WatchZoneListViewModel -- Device-Local Zone Row")
struct WatchZoneListViewModelDeviceLocalZoneRowTests {
  private func makeLocalZone(name: String = "Home") throws -> DeviceLocalZone {
    try DeviceLocalZone(name: name, centre: .cambridge, radiusMetres: 1000)
  }

  @Test func load_withNoDeviceLocalZoneRepository_neverShowsRow() async {
    let sut = WatchZoneListViewModel(
      repository: SpyWatchZoneRepository(),
      featureGate: FeatureGate(tier: .free)
    )

    await sut.load()

    #expect(!sut.showsLocalZoneRow)
    #expect(sut.unconvertedLocalZones.isEmpty)
  }

  @Test func load_withUnconvertedLocalZones_populatesThemAndShowsRow() async throws {
    let localRepo = SpyDeviceLocalZoneRepository()
    let zone = try makeLocalZone(name: "Work")
    localRepo.loadAllResult = [zone]
    let sut = WatchZoneListViewModel(
      repository: SpyWatchZoneRepository(),
      featureGate: FeatureGate(tier: .free),
      deviceLocalZoneRepository: localRepo
    )

    await sut.load()

    #expect(sut.unconvertedLocalZones == [zone])
    #expect(sut.showsLocalZoneRow)
  }

  @Test func load_withNoUnconvertedLocalZones_doesNotShowRow() async {
    let localRepo = SpyDeviceLocalZoneRepository()
    localRepo.loadAllResult = []
    let sut = WatchZoneListViewModel(
      repository: SpyWatchZoneRepository(),
      featureGate: FeatureGate(tier: .free),
      deviceLocalZoneRepository: localRepo
    )

    await sut.load()

    #expect(!sut.showsLocalZoneRow)
  }

  @Test func dismissLocalZoneRow_hidesRowForTheSession() async throws {
    let localRepo = SpyDeviceLocalZoneRepository()
    localRepo.loadAllResult = [try makeLocalZone()]
    let sut = WatchZoneListViewModel(
      repository: SpyWatchZoneRepository(),
      featureGate: FeatureGate(tier: .free),
      deviceLocalZoneRepository: localRepo
    )
    await sut.load()
    #expect(sut.showsLocalZoneRow)

    sut.dismissLocalZoneRow()

    #expect(!sut.showsLocalZoneRow)
  }

  @Test func dismissLocalZoneRow_thenReload_staysDismissed() async throws {
    let localRepo = SpyDeviceLocalZoneRepository()
    localRepo.loadAllResult = [try makeLocalZone()]
    let sut = WatchZoneListViewModel(
      repository: SpyWatchZoneRepository(),
      featureGate: FeatureGate(tier: .free),
      deviceLocalZoneRepository: localRepo
    )
    await sut.load()
    sut.dismissLocalZoneRow()

    // A pull-to-refresh (another load()) must not un-dismiss the row within
    // the same session — only a fresh VM instance (next launch) resets it.
    await sut.load()

    #expect(!sut.showsLocalZoneRow)
  }

  @Test func load_withZonesRemainingAfterConversion_clearsRow() async throws {
    let localRepo = SpyDeviceLocalZoneRepository()
    localRepo.loadAllResult = [try makeLocalZone()]
    let sut = WatchZoneListViewModel(
      repository: SpyWatchZoneRepository(),
      featureGate: FeatureGate(tier: .free),
      deviceLocalZoneRepository: localRepo
    )
    await sut.load()
    #expect(sut.showsLocalZoneRow)

    // All remaining zones converted or deleted elsewhere; a later load()
    // (e.g. the coordinator's post-conversion refresh) must clear the row.
    localRepo.loadAllResult = []
    await sut.load()

    #expect(!sut.showsLocalZoneRow)
    #expect(sut.unconvertedLocalZones.isEmpty)
  }

  @Test func convertLocalZones_invokesOnConvertLocalZones() {
    let sut = WatchZoneListViewModel(
      repository: SpyWatchZoneRepository(),
      featureGate: FeatureGate(tier: .free)
    )
    var invoked = false
    sut.onConvertLocalZones = { invoked = true }

    sut.convertLocalZones()

    #expect(invoked)
  }
}
