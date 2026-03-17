/// Persists and retrieves the user's watch zones.
public protocol WatchZoneRepository: Sendable {
    func save(_ zone: WatchZone) async throws
    func loadActive() async throws -> WatchZone?
}
