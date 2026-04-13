import Foundation
import TownCrierDomain

/// An in-memory cache store for development and testing.
public actor InMemoryApplicationCacheStore: ApplicationCacheStore {
  private var entries: [String: CacheEntry<[PlanningApplication]>] = [:]

  public init() {}

  public func store(_ entry: CacheEntry<[PlanningApplication]>, for zone: WatchZone) {
    entries[zone.id.value] = entry
  }

  public func retrieve(for zone: WatchZone) -> CacheEntry<[PlanningApplication]>? {
    entries[zone.id.value]
  }
}
