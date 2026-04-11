import Foundation
import TownCrierDomain

final class SpyApplicationCacheStore: ApplicationCacheStore, @unchecked Sendable {
  var storedEntry: CacheEntry<[PlanningApplication]>?
  private(set) var storeCalls: [(LocalAuthority, CacheEntry<[PlanningApplication]>)] = []
  private(set) var retrieveCalls: [LocalAuthority] = []

  func store(_ entry: CacheEntry<[PlanningApplication]>, for authority: LocalAuthority) async {
    storeCalls.append((authority, entry))
    storedEntry = entry
  }

  func retrieve(for authority: LocalAuthority) async -> CacheEntry<[PlanningApplication]>? {
    retrieveCalls.append(authority)
    return storedEntry
  }
}
