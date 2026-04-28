import Foundation

/// Port for local persistence of cached planning applications.
public protocol ApplicationCacheStore: Sendable {
  func store(_ entry: CacheEntry<[PlanningApplication]>, for zone: WatchZone) async
  func retrieve(for zone: WatchZone) async -> CacheEntry<[PlanningApplication]>?
  /// Removes any cached entry keyed to the given zone id.
  ///
  /// Callers must invoke this whenever a zone's geometry (radius/centre)
  /// changes, otherwise a fresh-cache hit can outlive the change for up to
  /// the cache TTL and serve applications matching the previous geometry.
  func invalidate(for zoneId: WatchZoneId) async
}
