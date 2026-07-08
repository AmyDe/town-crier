import Foundation
import TownCrierDomain

/// Persists device-local (pre-signup) zones to UserDefaults as a JSON-encoded
/// array (GH#879 Phase 4). Device-local, like
/// ``UserDefaultsAnonymousBrowseStateRepository`` — reinstalling the app
/// resets it.
///
/// Migration: the first time ``loadAll()`` is called (tracked by a one-shot
/// flag, not "storage is currently empty" — so a user who later deletes
/// every zone is never silently re-seeded), if there are no stored zones and
/// a legacy ``AnonymousBrowseState`` exists, seeds zone 1 from it (name = its
/// postcode string, centre/radius from the state) and makes it active.
/// `AnonymousBrowseState` itself is left untouched —
/// `AppCoordinator+Onboarding`'s post-signup prefill still reads it.
///
/// Trim (GH#888): every ``loadAll()`` call also idempotently enforces the
/// cap dropping from 3 to 1 — if more than one zone is stored (a device that
/// installed a Phase 4/5 TestFlight build before this change), it keeps the
/// ACTIVE zone (falling back to the first if none is active), persists the
/// reduced set, and clears the discarded zones. No new one-shot flag is
/// needed for this: it is safe to re-run on every call because it is a
/// no-op once at most one zone remains. User-approved 2026-07-08 — existing
/// multi-zone TestFlight installs are not a migration concern beyond this.
public final class UserDefaultsDeviceLocalZoneRepository: DeviceLocalZoneRepository,
  @unchecked Sendable {
  private let defaults: UserDefaults
  private let legacyStateRepository: AnonymousBrowseStateRepository?
  private let zonesKey = "deviceLocalZones"
  private let activeZoneIdKey = "deviceLocalZoneActiveId"
  private let migrationKey = "deviceLocalZoneMigrationComplete"
  private let decoder = JSONDecoder()
  private let encoder = JSONEncoder()

  public init(
    defaults: UserDefaults = .standard,
    legacyStateRepository: AnonymousBrowseStateRepository? = nil
  ) {
    self.defaults = defaults
    self.legacyStateRepository = legacyStateRepository
  }

  public func loadAll() -> [DeviceLocalZone] {
    migrateIfNeeded()
    return trimToActiveZoneIfNeeded()
  }

  public func save(_ zone: DeviceLocalZone) throws {
    var zones = storedZones()
    if let index = zones.firstIndex(where: { $0.id == zone.id }) {
      zones[index] = zone
    } else {
      guard zones.count < DeviceLocalZone.maxZoneCount else {
        throw DomainError.deviceLocalZoneLimitReached
      }
      zones.append(zone)
    }
    persist(zones)
    if activeZoneId() == nil {
      setActiveZoneId(zone.id)
    }
  }

  public func delete(_ id: DeviceLocalZoneId) {
    var zones = storedZones()
    zones.removeAll { $0.id == id }
    persist(zones)
    if activeZoneId() == id {
      setActiveZoneId(zones.first?.id)
    }
  }

  public func activeZoneId() -> DeviceLocalZoneId? {
    guard let value = defaults.string(forKey: activeZoneIdKey) else { return nil }
    return DeviceLocalZoneId(value)
  }

  public func setActiveZoneId(_ id: DeviceLocalZoneId?) {
    guard let id else {
      defaults.removeObject(forKey: activeZoneIdKey)
      return
    }
    defaults.set(id.value, forKey: activeZoneIdKey)
  }

  // MARK: - Trim (GH#888)

  /// Enforces the cap on read: keeps only the active zone (or the first
  /// zone, if none is marked active) when more than one is stored, and
  /// persists the reduction so subsequent calls are a no-op. Returns the
  /// (possibly already-compliant) stored zones.
  private func trimToActiveZoneIfNeeded() -> [DeviceLocalZone] {
    let zones = storedZones()
    guard zones.count > 1 else { return zones }
    let activeId = activeZoneId()
    let keeper = zones.first { $0.id == activeId } ?? zones[0]
    persist([keeper])
    setActiveZoneId(keeper.id)
    return [keeper]
  }

  // MARK: - Migration

  private func migrateIfNeeded() {
    guard !defaults.bool(forKey: migrationKey) else { return }
    defaults.set(true, forKey: migrationKey)
    guard storedZones().isEmpty else { return }
    guard let legacyState = legacyStateRepository?.load() else { return }
    guard
      let zone = try? DeviceLocalZone(
        name: legacyState.postcode.value,
        centre: legacyState.coordinate,
        radiusMetres: legacyState.radiusMetres)
    else { return }
    persist([zone])
    setActiveZoneId(zone.id)
  }

  // MARK: - Storage

  private func storedZones() -> [DeviceLocalZone] {
    guard let data = defaults.data(forKey: zonesKey) else { return [] }
    guard let stored = try? decoder.decode([StoredZone].self, from: data) else { return [] }
    return stored.compactMap { $0.toDomain() }
  }

  private func persist(_ zones: [DeviceLocalZone]) {
    let stored = zones.map { StoredZone(zone: $0) }
    guard let data = try? encoder.encode(stored) else { return }
    defaults.set(data, forKey: zonesKey)
  }

  /// Flat, versionless wire shape for the persisted blob — deliberately not
  /// `DeviceLocalZone` itself, so the domain type never needs to conform to
  /// `Codable` (Domain stays free of persistence concerns).
  private struct StoredZone: Codable {
    let id: String
    let name: String
    let latitude: Double
    let longitude: Double
    let radiusMetres: Double

    init(zone: DeviceLocalZone) {
      id = zone.id.value
      name = zone.name
      latitude = zone.centre.latitude
      longitude = zone.centre.longitude
      radiusMetres = zone.radiusMetres
    }

    func toDomain() -> DeviceLocalZone? {
      guard let centre = try? Coordinate(latitude: latitude, longitude: longitude) else {
        return nil
      }
      return try? DeviceLocalZone(
        id: DeviceLocalZoneId(id), name: name, centre: centre, radiusMetres: radiusMetres)
    }
  }
}
