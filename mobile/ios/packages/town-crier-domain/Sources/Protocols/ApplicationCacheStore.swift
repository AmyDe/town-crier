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

  /// Removes every cached entry, regardless of zone.
  ///
  /// Used after a global state mutation that affects every zone's view of
  /// applications — e.g. mark-all-read, which clears each row's
  /// `latestUnreadEvent`, where a TTL-fresh cache hit would otherwise serve
  /// stale unread flags for up to the cache TTL (tc-e3bu).
  func invalidateAll() async
}
