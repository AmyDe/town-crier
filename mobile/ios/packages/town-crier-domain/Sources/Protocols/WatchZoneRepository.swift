/// Persists and retrieves the user's watch zones.
public protocol WatchZoneRepository: Sendable {
  func save(_ zone: WatchZone) async throws
  func update(_ zone: WatchZone) async throws
  func loadAll() async throws -> [WatchZone]
  func delete(_ id: WatchZoneId) async throws
}
