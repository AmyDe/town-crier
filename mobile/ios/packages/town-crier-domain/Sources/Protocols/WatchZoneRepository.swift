/// Persists and retrieves the user's watch zones.
public protocol WatchZoneRepository: Sendable {
  func save(_ zone: WatchZone) async throws
  func update(_ zone: WatchZone) async throws
  func loadAll() async throws -> [WatchZone]
  func delete(_ id: WatchZoneId) async throws

  /// Fetches the pre-canned watch-zone filter catalog from
  /// `GET /v1/watch-zones/filter-catalog` (GH#1104, bead tc-m8j90.2) --
  /// display copy only, resolved separately from ``WatchZone/filterKey``,
  /// which is never validated against this catalog client-side.
  func filterCatalog() async throws -> [FilterCatalogEntry]
}
