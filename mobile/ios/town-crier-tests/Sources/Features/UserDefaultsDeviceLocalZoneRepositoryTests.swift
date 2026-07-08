import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

/// GH#879 Phase 4 / GH#888 acceptance criteria: repository round-trip,
/// cap-at-1 enforcement (GH#888 reversed the original cap-at-3), radius
/// clamp [100, 5000], migration seeds zone 1 from the legacy
/// `AnonymousBrowseState`, and an idempotent trim-to-active-zone on every
/// `loadAll()` call self-heals any pre-GH#888 multi-zone install.
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

  /// A repository/defaults pair with no legacy-state repository injected —
  /// used by the trim tests (GH#888), which need direct access to the
  /// `UserDefaults` suite to seed pre-existing (pre-GH#888) multi-zone
  /// storage.
  private func makeSUTAndDefaults() -> (UserDefaultsDeviceLocalZoneRepository, UserDefaults) {
    let defaults = UserDefaults(suiteName: UUID().uuidString)
    // swiftlint:disable:next force_unwrapping
    let sut = UserDefaultsDeviceLocalZoneRepository(defaults: defaults!)
    // swiftlint:disable:next force_unwrapping
    return (sut, defaults!)
  }

  /// Wire-format fixture mirroring the production repository's private
  /// `StoredZone` shape — lets the trim tests (GH#888) seed the UserDefaults
  /// suite directly, as if it were a pre-GH#888 multi-zone TestFlight
  /// install, bypassing the (now much lower) `save()` cap entirely.
  private struct RawStoredZone: Codable {
    let id: String
    let name: String
    let latitude: Double
    let longitude: Double
    let radiusMetres: Double

    init(_ zone: DeviceLocalZone) {
      id = zone.id.value
      name = zone.name
      latitude = zone.centre.latitude
      longitude = zone.centre.longitude
      radiusMetres = zone.radiusMetres
    }
  }

  private func seedRawZones(
    _ zones: [DeviceLocalZone], activeId: DeviceLocalZoneId?, in defaults: UserDefaults
  ) throws {
    let data = try JSONEncoder().encode(zones.map(RawStoredZone.init))
    defaults.set(data, forKey: "deviceLocalZones")
    if let activeId {
      defaults.set(activeId.value, forKey: "deviceLocalZoneActiveId")
    }
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

  // MARK: - Cap at 1 (GH#888 — reversed the original cap-at-3)

  @Test func save_secondZone_throwsLimitReached() throws {
    let (sut, _) = makeSUT()
    try sut.save(try makeZone(name: "One"))

    #expect(throws: DomainError.deviceLocalZoneLimitReached) {
      try sut.save(try makeZone(name: "Two"))
    }
    #expect(sut.loadAll().count == 1)
  }

  @Test func save_editingExistingZone_atCap_doesNotThrow() throws {
    let (sut, _) = makeSUT()
    let first = try makeZone(name: "One")
    try sut.save(first)

    let renamed = try DeviceLocalZone(
      id: first.id, name: "Renamed", centre: .cambridge, radiusMetres: 1000)
    try sut.save(renamed)

    #expect(sut.loadAll() == [renamed])
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

  @Test func setActiveZoneId_updatesTheActiveZone() throws {
    let (sut, defaults) = makeSUTAndDefaults()
    let first = try makeZone(name: "One")
    let second = try makeZone(name: "Two")
    try seedRawZones([first, second], activeId: first.id, in: defaults)

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
    let (sut, defaults) = makeSUTAndDefaults()
    let first = try makeZone(name: "One")
    let second = try makeZone(name: "Two")
    try seedRawZones([first, second], activeId: first.id, in: defaults)

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
    let (sut, defaults) = makeSUTAndDefaults()
    let first = try makeZone(name: "One")
    let second = try makeZone(name: "Two")
    try seedRawZones([first, second], activeId: first.id, in: defaults)

    sut.delete(second.id)

    #expect(sut.activeZoneId() == first.id)
  }

  // MARK: - Trim to active zone (GH#888)

  /// The exact acceptance-criteria scenario: storage seeded with 3 zones
  /// (active = the 2nd) — `loadAll()` returns only the active zone.
  @Test func loadAll_moreThanOneZoneStored_activeSet_returnsOnlyTheActiveZone() throws {
    let (sut, defaults) = makeSUTAndDefaults()
    let one = try makeZone(name: "One")
    let two = try makeZone(name: "Two")
    let three = try makeZone(name: "Three")
    try seedRawZones([one, two, three], activeId: two.id, in: defaults)

    let zones = sut.loadAll()

    #expect(zones == [two])
  }

  /// The trim is PERSISTED, not just returned transiently — a fresh
  /// repository instance over the same `UserDefaults` suite (mirroring an
  /// app relaunch) sees the same single zone.
  @Test func loadAll_moreThanOneZoneStored_persistsTheTrim() throws {
    let (sut, defaults) = makeSUTAndDefaults()
    let one = try makeZone(name: "One")
    let two = try makeZone(name: "Two")
    let three = try makeZone(name: "Three")
    try seedRawZones([one, two, three], activeId: two.id, in: defaults)

    _ = sut.loadAll()

    let reloaded = UserDefaultsDeviceLocalZoneRepository(defaults: defaults)
    #expect(reloaded.loadAll() == [two])
    #expect(reloaded.activeZoneId() == two.id)
  }

  @Test func loadAll_moreThanOneZoneStored_noActiveSet_keepsTheFirstZone() throws {
    let (sut, defaults) = makeSUTAndDefaults()
    let one = try makeZone(name: "One")
    let two = try makeZone(name: "Two")
    try seedRawZones([one, two], activeId: nil, in: defaults)

    let zones = sut.loadAll()

    #expect(zones == [one])
    #expect(sut.activeZoneId() == one.id)
  }

  /// Idempotent: once at most one zone remains, repeated calls are a no-op.
  @Test func loadAll_alreadyAtMostOneZone_isANoOp() throws {
    let (sut, _) = makeSUT()
    let zone = try makeZone()
    try sut.save(zone)

    #expect(sut.loadAll() == [zone])
    #expect(sut.loadAll() == [zone])
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
