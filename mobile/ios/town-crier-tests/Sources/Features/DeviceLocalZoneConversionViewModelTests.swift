import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// GH#879 Phase 5: the post-signup "Add your other areas" conversion sheet.
/// Offers device-local zones the wizard didn't already convert for
/// server-side creation — sequential saves, in list order, stopping at the
/// first tier-quota breach and leaving everything from that point on in
/// local storage untouched.
@Suite("DeviceLocalZoneConversionViewModel")
@MainActor
struct DeviceLocalZoneConversionViewModelTests {
  private func makeZone(id: String, name: String, radiusMetres: Double = 1000) throws -> DeviceLocalZone {
    try DeviceLocalZone(
      id: DeviceLocalZoneId(id), name: name, centre: .cambridge, radiusMetres: radiusMetres)
  }

  private func makeSUT(zones: [DeviceLocalZone]) -> (
    DeviceLocalZoneConversionViewModel, SpyWatchZoneRepository, SpyDeviceLocalZoneRepository
  ) {
    let watchZoneRepo = SpyWatchZoneRepository()
    let localRepo = SpyDeviceLocalZoneRepository()
    let sut = DeviceLocalZoneConversionViewModel(
      zones: zones,
      watchZoneRepository: watchZoneRepo,
      deviceLocalZoneRepository: localRepo
    )
    return (sut, watchZoneRepo, localRepo)
  }

  // MARK: - Initial state

  @Test func zones_seededFromInit() throws {
    let zone = try makeZone(id: "a", name: "Home")
    let (sut, _, _) = makeSUT(zones: [zone])

    #expect(sut.zones == [zone])
    #expect(!sut.isConverting)
  }

  // MARK: - Sequential conversion, all succeed

  @Test func convertAll_savesEachZoneSequentiallyInListOrder() async throws {
    let zoneA = try makeZone(id: "a", name: "Home")
    let zoneB = try makeZone(id: "b", name: "Work")
    let (sut, watchZoneRepo, _) = makeSUT(zones: [zoneA, zoneB])

    await sut.convertAll()

    #expect(watchZoneRepo.saveCalls.count == 2)
    #expect(watchZoneRepo.saveCalls[0].name == "Home")
    #expect(watchZoneRepo.saveCalls[1].name == "Work")
  }

  @Test func convertAll_deletesEachConvertedZoneFromLocalStorageImmediately() async throws {
    let zoneA = try makeZone(id: "a", name: "Home")
    let zoneB = try makeZone(id: "b", name: "Work")
    let (sut, _, localRepo) = makeSUT(zones: [zoneA, zoneB])

    await sut.convertAll()

    #expect(localRepo.deleteCalls == [zoneA.id, zoneB.id])
  }

  @Test func convertAll_removesConvertedZonesFromPublishedList() async throws {
    let zoneA = try makeZone(id: "a", name: "Home")
    let zoneB = try makeZone(id: "b", name: "Work")
    let (sut, _, _) = makeSUT(zones: [zoneA, zoneB])

    await sut.convertAll()

    #expect(sut.zones.isEmpty)
  }

  @Test func convertAll_whenAllSucceed_invokesOnFinished() async throws {
    let zone = try makeZone(id: "a", name: "Home")
    let (sut, _, _) = makeSUT(zones: [zone])
    var finished = false
    sut.onFinished = { finished = true }

    await sut.convertAll()

    #expect(finished)
    #expect(!sut.isConverting)
  }

  @Test func convertAll_constructsWatchZoneFromDeviceLocalZoneNameCentreRadius() async throws {
    let zone = try makeZone(id: "a", name: "Mum's House", radiusMetres: 1500)
    let (sut, watchZoneRepo, _) = makeSUT(zones: [zone])

    await sut.convertAll()

    let saved = try #require(watchZoneRepo.saveCalls.first)
    #expect(saved.name == "Mum's House")
    #expect(saved.centre == zone.centre)
    #expect(saved.radiusMetres == 1500)
    // Reuses exactly the same "no authorityId, let the server resolve it
    // from lat/lng" path the wizard and authed editor already use.
    #expect(saved.authorityId == 0)
  }

  // MARK: - Quota breach stops conversion

  @Test func convertAll_onInsufficientEntitlement_stopsAndInvokesCallback() async throws {
    let zoneA = try makeZone(id: "a", name: "Home")
    let zoneB = try makeZone(id: "b", name: "Work")
    let zoneC = try makeZone(id: "c", name: "Allotment")
    let (sut, watchZoneRepo, _) = makeSUT(zones: [zoneA, zoneB, zoneC])
    watchZoneRepo.saveResults = [
      .success(()),
      .failure(DomainError.insufficientEntitlement(required: "personal")),
    ]
    var entitlementRouted = false
    sut.onInsufficientEntitlement = { entitlementRouted = true }
    var finished = false
    sut.onFinished = { finished = true }

    await sut.convertAll()

    #expect(entitlementRouted)
    #expect(!finished)
    // Only the first zone was attempted+converted; the second attempt failed
    // and the third was never reached.
    #expect(watchZoneRepo.saveCalls.count == 2)
    #expect(!sut.isConverting)
  }

  @Test func convertAll_onInsufficientEntitlement_leavesTriggeringAndLaterZonesLocal() async throws {
    let zoneA = try makeZone(id: "a", name: "Home")
    let zoneB = try makeZone(id: "b", name: "Work")
    let zoneC = try makeZone(id: "c", name: "Allotment")
    let (sut, watchZoneRepo, localRepo) = makeSUT(zones: [zoneA, zoneB, zoneC])
    watchZoneRepo.saveResults = [
      .success(()),
      .failure(DomainError.insufficientEntitlement(required: "personal")),
    ]

    await sut.convertAll()

    // Only the successfully-converted zone was deleted locally.
    #expect(localRepo.deleteCalls == [zoneA.id])
    // The zone that hit the quota, and the one after it, stay in the
    // published list untouched — never silently discarded.
    #expect(sut.zones == [zoneB, zoneC])
  }

  // MARK: - Generic failure also stops conversion (never silently discard data)

  @Test func convertAll_onGenericFailure_stopsAndSetsError() async throws {
    let zoneA = try makeZone(id: "a", name: "Home")
    let zoneB = try makeZone(id: "b", name: "Work")
    let (sut, watchZoneRepo, localRepo) = makeSUT(zones: [zoneA, zoneB])
    watchZoneRepo.saveResults = [.failure(DomainError.networkUnavailable)]

    await sut.convertAll()

    #expect(sut.error == .networkUnavailable)
    #expect(localRepo.deleteCalls.isEmpty)
    #expect(sut.zones == [zoneA, zoneB])
    #expect(!sut.isConverting)
  }

  // MARK: - Explicit dismiss

  @Test func dismiss_invokesOnFinishedWithoutConvertingAnything() async throws {
    let zone = try makeZone(id: "a", name: "Home")
    let (sut, watchZoneRepo, localRepo) = makeSUT(zones: [zone])
    var finished = false
    sut.onFinished = { finished = true }

    sut.dismiss()

    #expect(finished)
    #expect(watchZoneRepo.saveCalls.isEmpty)
    #expect(localRepo.deleteCalls.isEmpty)
    #expect(sut.zones == [zone])
  }
}
