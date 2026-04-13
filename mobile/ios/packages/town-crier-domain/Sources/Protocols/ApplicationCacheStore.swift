import Foundation

/// Port for local persistence of cached planning applications.
public protocol ApplicationCacheStore: Sendable {
  func store(_ entry: CacheEntry<[PlanningApplication]>, for zone: WatchZone) async
  func retrieve(for zone: WatchZone) async -> CacheEntry<[PlanningApplication]>?
}
