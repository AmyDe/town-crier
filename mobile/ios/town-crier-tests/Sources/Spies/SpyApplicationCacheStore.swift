import Foundation
import TownCrierDomain

final class SpyApplicationCacheStore: ApplicationCacheStore, @unchecked Sendable {
  var storedEntry: CacheEntry<[PlanningApplication]>?
  private(set) var storeCalls: [(WatchZone, CacheEntry<[PlanningApplication]>)] = []
  private(set) var retrieveCalls: [WatchZone] = []
  private(set) var invalidateCalls: [WatchZoneId] = []

  func store(_ entry: CacheEntry<[PlanningApplication]>, for zone: WatchZone) async {
    storeCalls.append((zone, entry))
    storedEntry = entry
  }

  func retrieve(for zone: WatchZone) async -> CacheEntry<[PlanningApplication]>? {
    retrieveCalls.append(zone)
    return storedEntry
  }

  func invalidate(for zoneId: WatchZoneId) async {
    invalidateCalls.append(zoneId)
    storedEntry = nil
  }
}
