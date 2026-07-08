/// Persists device-local (pre-signup) zones (GH#879 Phase 4): up to
/// ``DeviceLocalZone/maxZoneCount`` areas, entirely on-device — no server
/// zone, no alerts. A distinct value object/repository pair from
/// `WatchZone`/`WatchZoneRepository`: `WatchZone.authorityId` is
/// server-resolved and unavailable to an anonymous session.
public protocol DeviceLocalZoneRepository: Sendable {
  /// All persisted zones, in the order they were created.
  func loadAll() -> [DeviceLocalZone]

  /// Persists `zone` — inserting it if `zone.id` is new, replacing the
  /// existing entry in place if not. Throws
  /// `DomainError.deviceLocalZoneLimitReached` when inserting a NEW zone
  /// would exceed ``DeviceLocalZone/maxZoneCount``; editing an existing zone
  /// is never blocked by the cap. The first zone ever saved (whether via this
  /// call or the legacy-state migration) becomes the active zone
  /// automatically.
  func save(_ zone: DeviceLocalZone) throws

  /// Removes the zone with `id`, if present. Idempotent. If the removed zone
  /// was active, the active zone becomes the first remaining zone, or `nil`
  /// if none remain.
  func delete(_ id: DeviceLocalZoneId)

  /// The currently active zone's id — the one driving the anonymous
  /// Applications list query and the Map centre/radius — or `nil` if no zone
  /// has ever been saved.
  func activeZoneId() -> DeviceLocalZoneId?

  /// Sets the active zone. Pass `nil` to clear it explicitly.
  func setActiveZoneId(_ id: DeviceLocalZoneId?)
}
