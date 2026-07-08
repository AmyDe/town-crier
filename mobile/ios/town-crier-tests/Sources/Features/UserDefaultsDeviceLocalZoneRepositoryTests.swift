import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

/// GH#879 Phase 4 acceptance criteria: repository round-trip, cap-at-3
/// enforcement, radius clamp [100, 5000], migration seeds zone 1 from the
/// legacy `AnonymousBrowseState`.
@Suite("UserDefaultsDeviceLocalZoneRepository")
struct UserDefaultsDeviceLocalZoneRepositoryTests {
  private func makeSUT(
    legacyState: AnonymousBrowseState? = nil
  ) -> (UserDefaultsDeviceLocalZoneRepository, SpyAnonymousBrowseStateRepository) {
    // Isolated suite per test so parallel test runs never share state.
    let defaults = UserDefaults(suiteName: UUID().uuidString)
    let legacyRepository = SpyAnonymousBrowseStateRepository()
    legacyRepository.loadResult = legacyState
    let sut = UserDefaultsDeviceLocalZoneRepository(
      // swiftlint:disable:next force_unwrapping
      defaults: defaults!, legacyStateRepository: legacyRepository)
    return (sut, legacyRepository)
  }

  private func makeZone(
    name: String = "Home", radiusMetres: Double = 1000
  ) throws -> DeviceLocalZone {
    try DeviceLocalZone(name: name, centre: .cambridge, radiusMetres: radiusMetres)
  }

  // MARK: - Round-trip

  @Test func loadAll_returnsEmpty_whenNothingSaved() {
    let (sut, _) = makeSUT()

    #expect(sut.loadAll().isEmpty)
  }

  @Test func saveThenLoadAll_roundTripsTheZone() throws {
    let (sut, _) = makeSUT()
    let zone = try makeZone()

    try sut.save(zone)

    #expect(sut.loadAll() == [zone])
  }

  @Test func save_existingId_replacesInPlace() throws {
    let (sut, _) = makeSUT()
    let zone = try makeZone(name: "Home")
    try sut.save(zone)
    let updated = try DeviceLocalZone(
      id: zone.id, name: "Updated Home", centre: .cambridge, radiusMetres: 2000)

    try sut.save(updated)

    #expect(sut.loadAll() == [updated])
  }

  @Test func delete_removesTheZone() throws {
    let (sut, _) = makeSUT()
    let zone = try makeZone()
    try sut.save(zone)

    sut.delete(zone.id)

    #expect(sut.loadAll().isEmpty)
  }

  @Test func delete_missingId_isNoOp() throws {
    let (sut, _) = makeSUT()
    let zone = try makeZone()
    try sut.save(zone)

    sut.delete(DeviceLocalZoneId())

    #expect(sut.loadAll() == [zone])
  }

  // MARK: - Cap at 3

  @Test func save_thirdZone_succeeds() throws {
    let (sut, _) = makeSUT()
    try sut.save(try makeZone(name: "One"))
    try sut.save(try makeZone(name: "Two"))

    try sut.save(try makeZone(name: "Three"))

    #expect(sut.loadAll().count == 3)
  }

  @Test func save_fourthZone_throwsLimitReached() throws {
    let (sut, _) = makeSUT()
    try sut.save(try makeZone(name: "One"))
    try sut.save(try makeZone(name: "Two"))
    try sut.save(try makeZone(name: "Three"))

    #expect(throws: DomainError.deviceLocalZoneLimitReached) {
      try sut.save(try makeZone(name: "Four"))
    }
    #expect(sut.loadAll().count == 3)
  }

  @Test func save_editingExistingZone_atCap_doesNotThrow() throws {
    let (sut, _) = makeSUT()
    let first = try makeZone(name: "One")
    try sut.save(first)
    try sut.save(try makeZone(name: "Two"))
    try sut.save(try makeZone(name: "Three"))

    let renamed = try DeviceLocalZone(
      id: first.id, name: "Renamed", centre: .cambridge, radiusMetres: 1000)
    try sut.save(renamed)

    #expect(sut.loadAll().contains(renamed))
  }

  // MARK: - Active zone

  @Test func activeZoneId_isNil_whenNothingSaved() {
    let (sut, _) = makeSUT()

    #expect(sut.activeZoneId() == nil)
  }

  @Test func save_firstEverZone_becomesActive() throws {
    let (sut, _) = makeSUT()
    let zone = try makeZone()

    try sut.save(zone)

    #expect(sut.activeZoneId() == zone.id)
  }

  @Test func save_secondZone_doesNotChangeActiveZone() throws {
    let (sut, _) = makeSUT()
    let first = try makeZone(name: "One")
    try sut.save(first)

    try sut.save(try makeZone(name: "Two"))

    #expect(sut.activeZoneId() == first.id)
  }

  @Test func setActiveZoneId_updatesTheActiveZone() throws {
    let (sut, _) = makeSUT()
    let first = try makeZone(name: "One")
    let second = try makeZone(name: "Two")
    try sut.save(first)
    try sut.save(second)

    sut.setActiveZoneId(second.id)

    #expect(sut.activeZoneId() == second.id)
  }

  @Test func setActiveZoneId_nil_clearsTheActiveZone() throws {
    let (sut, _) = makeSUT()
    try sut.save(try makeZone())

    sut.setActiveZoneId(nil)

    #expect(sut.activeZoneId() == nil)
  }

  @Test func delete_activeZone_promotesFirstRemainingZone() throws {
    let (sut, _) = makeSUT()
    let first = try makeZone(name: "One")
    let second = try makeZone(name: "Two")
    try sut.save(first)
    try sut.save(second)

    sut.delete(first.id)

    #expect(sut.activeZoneId() == second.id)
  }

  @Test func delete_lastZone_clearsActiveZone() throws {
    let (sut, _) = makeSUT()
    let zone = try makeZone()
    try sut.save(zone)

    sut.delete(zone.id)

    #expect(sut.activeZoneId() == nil)
  }

  @Test func delete_nonActiveZone_doesNotChangeActiveZone() throws {
    let (sut, _) = makeSUT()
    let first = try makeZone(name: "One")
    let second = try makeZone(name: "Two")
    try sut.save(first)
    try sut.save(second)

    sut.delete(second.id)

    #expect(sut.activeZoneId() == first.id)
  }

  // MARK: - Migration from legacy AnonymousBrowseState

  @Test func loadAll_noStoredZonesNoLegacyState_seedsNothing() {
    let (sut, _) = makeSUT(legacyState: nil)

    #expect(sut.loadAll().isEmpty)
    #expect(sut.activeZoneId() == nil)
  }

  @Test func loadAll_noStoredZones_seedsZoneFromLegacyState() throws {
    let legacyState = AnonymousBrowseState(
      postcode: try Postcode("CB1 2AD"),
      coordinate: .cambridge,
      radiusMetres: 1500,
      createdAt: Date())
    let (sut, _) = makeSUT(legacyState: legacyState)

    let zones = sut.loadAll()

    #expect(zones.count == 1)
    #expect(zones.first?.name == "CB1 2AD")
    #expect(zones.first?.centre == .cambridge)
    #expect(zones.first?.radiusMetres == 1500)
  }

  @Test func loadAll_seedsFromLegacyState_setsItActive() throws {
    let legacyState = AnonymousBrowseState(
      postcode: try Postcode("CB1 2AD"), coordinate: .cambridge, createdAt: Date())
    let (sut, _) = makeSUT(legacyState: legacyState)

    let zones = sut.loadAll()

    #expect(sut.activeZoneId() == zones.first?.id)
  }

  @Test func loadAll_whenZonesAlreadyExist_doesNotMigrateLegacyState() throws {
    let legacyState = AnonymousBrowseState(
      postcode: try Postcode("CB1 2AD"), coordinate: .cambridge, createdAt: Date())
    let (sut, _) = makeSUT(legacyState: legacyState)
    try sut.save(try makeZone(name: "Manually Added"))

    let zones = sut.loadAll()

    #expect(zones.count == 1)
    #expect(zones.first?.name == "Manually Added")
  }

  @Test func loadAll_afterMigrationAndDeletion_doesNotReSeedFromLegacyState() throws {
    let legacyState = AnonymousBrowseState(
      postcode: try Postcode("CB1 2AD"), coordinate: .cambridge, createdAt: Date())
    let (sut, _) = makeSUT(legacyState: legacyState)
    let migrated = sut.loadAll()
    #expect(migrated.count == 1)
    sut.delete(migrated[0].id)

    let zonesAfterDelete = sut.loadAll()

    #expect(zonesAfterDelete.isEmpty)
  }

  @Test func loadAll_noLegacyStateRepositoryInjected_doesNotCrash() {
    let defaults = UserDefaults(suiteName: UUID().uuidString)
    // swiftlint:disable:next force_unwrapping
    let sut = UserDefaultsDeviceLocalZoneRepository(defaults: defaults!)

    #expect(sut.loadAll().isEmpty)
  }
}
