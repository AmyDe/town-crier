import Foundation

/// Port for local persistence of cached planning applications.
public protocol ApplicationCacheStore: Sendable {
  func store(_ entry: CacheEntry<[PlanningApplication]>, for authority: LocalAuthority) async
  func retrieve(for authority: LocalAuthority) async -> CacheEntry<[PlanningApplication]>?
}
