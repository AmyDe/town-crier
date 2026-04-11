import Foundation
import TownCrierDomain

/// An in-memory cache store for development and testing.
public actor InMemoryApplicationCacheStore: ApplicationCacheStore {
  private var entries: [String: CacheEntry<[PlanningApplication]>] = [:]

  public init() {}

  public func store(_ entry: CacheEntry<[PlanningApplication]>, for authority: LocalAuthority) {
    entries[authority.code] = entry
  }

  public func retrieve(for authority: LocalAuthority) -> CacheEntry<[PlanningApplication]>? {
    entries[authority.code]
  }
}
