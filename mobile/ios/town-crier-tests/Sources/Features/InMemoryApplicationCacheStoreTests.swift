import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("InMemoryApplicationCacheStore")
struct InMemoryApplicationCacheStoreTests {
  @Test func retrieve_returnsNil_whenEmpty() async {
    let sut = InMemoryApplicationCacheStore()

    let result = await sut.retrieve(for: WatchZone.cambridge)

    #expect(result == nil)
  }

  @Test func storeAndRetrieve_roundTrips() async {
    let sut = InMemoryApplicationCacheStore()
    let apps = [PlanningApplication.pendingReview, .permitted]
    let entry = CacheEntry(data: apps, fetchedAt: Date())

    await sut.store(entry, for: WatchZone.cambridge)
    let result = await sut.retrieve(for: WatchZone.cambridge)

    #expect(result != nil)
    #expect(result?.data.count == 2)
  }

  @Test func store_overwritesPreviousEntry() async {
    let sut = InMemoryApplicationCacheStore()
    let first = CacheEntry(data: [PlanningApplication.pendingReview], fetchedAt: Date())
    let second = CacheEntry(data: [PlanningApplication.permitted, .rejected], fetchedAt: Date())

    await sut.store(first, for: WatchZone.cambridge)
    await sut.store(second, for: WatchZone.cambridge)
    let result = await sut.retrieve(for: WatchZone.cambridge)

    #expect(result?.data.count == 2)
  }

  @Test func retrieve_isolatesByZone() async {
    let sut = InMemoryApplicationCacheStore()
    let camEntry = CacheEntry(data: [PlanningApplication.pendingReview], fetchedAt: Date())

    await sut.store(camEntry, for: WatchZone.cambridge)
    let result = await sut.retrieve(for: WatchZone.london)

    #expect(result == nil)
  }

  @Test func invalidate_removesEntryForZone() async {
    let sut = InMemoryApplicationCacheStore()
    let entry = CacheEntry(data: [PlanningApplication.pendingReview], fetchedAt: Date())
    await sut.store(entry, for: WatchZone.cambridge)

    await sut.invalidate(for: WatchZone.cambridge.id)
    let result = await sut.retrieve(for: WatchZone.cambridge)

    #expect(result == nil)
  }

  @Test func invalidate_doesNotAffectOtherZones() async {
    let sut = InMemoryApplicationCacheStore()
    let camEntry = CacheEntry(data: [PlanningApplication.pendingReview], fetchedAt: Date())
    let londonEntry = CacheEntry(data: [PlanningApplication.permitted], fetchedAt: Date())
    await sut.store(camEntry, for: WatchZone.cambridge)
    await sut.store(londonEntry, for: WatchZone.london)

    await sut.invalidate(for: WatchZone.cambridge.id)
    let camResult = await sut.retrieve(for: WatchZone.cambridge)
    let londonResult = await sut.retrieve(for: WatchZone.london)

    #expect(camResult == nil)
    #expect(londonResult != nil)
    #expect(londonResult?.data.count == 1)
  }

  @Test func invalidate_isNoOp_whenZoneNotCached() async {
    let sut = InMemoryApplicationCacheStore()

    await sut.invalidate(for: WatchZone.cambridge.id)
    let result = await sut.retrieve(for: WatchZone.cambridge)

    #expect(result == nil)
  }
}
