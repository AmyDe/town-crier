import Foundation
import TownCrierDomain

final class SpyApplicationCacheStore: ApplicationCacheStore, @unchecked Sendable {
  var storedEntry: CacheEntry<[PlanningApplication]>?
  private(set) var storeCalls: [(WatchZone, CacheEntry<[PlanningApplication]>)] = []
  private(set) var retrieveCalls: [WatchZone] = []

  func store(_ entry: CacheEntry<[PlanningApplication]>, for zone: WatchZone) async {
    storeCalls.append((zone, entry))
    storedEntry = entry
  }

  func retrieve(for zone: WatchZone) async -> CacheEntry<[PlanningApplication]>? {
    retrieveCalls.append(zone)
    return storedEntry
  }
}
