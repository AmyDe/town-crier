import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("InMemoryApplicationCacheStore")
struct InMemoryApplicationCacheStoreTests {
  @Test func retrieve_returnsNil_whenEmpty() async {
    let sut = InMemoryApplicationCacheStore()

    let result = await sut.retrieve(for: .cambridge)

    #expect(result == nil)
  }

  @Test func storeAndRetrieve_roundTrips() async {
    let sut = InMemoryApplicationCacheStore()
    let apps = [PlanningApplication.pendingReview, .approved]
    let entry = CacheEntry(data: apps, fetchedAt: Date())

    await sut.store(entry, for: .cambridge)
    let result = await sut.retrieve(for: .cambridge)

    #expect(result != nil)
    #expect(result?.data.count == 2)
  }

  @Test func store_overwritesPreviousEntry() async {
    let sut = InMemoryApplicationCacheStore()
    let first = CacheEntry(data: [PlanningApplication.pendingReview], fetchedAt: Date())
    let second = CacheEntry(data: [PlanningApplication.approved, .refused], fetchedAt: Date())

    await sut.store(first, for: .cambridge)
    await sut.store(second, for: .cambridge)
    let result = await sut.retrieve(for: .cambridge)

    #expect(result?.data.count == 2)
  }

  @Test func retrieve_isolatesByAuthority() async {
    let sut = InMemoryApplicationCacheStore()
    let camEntry = CacheEntry(data: [PlanningApplication.pendingReview], fetchedAt: Date())
    let other = LocalAuthority(code: "OXF", name: "Oxford")

    await sut.store(camEntry, for: .cambridge)
    let result = await sut.retrieve(for: other)

    #expect(result == nil)
  }
}
