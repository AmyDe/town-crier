import Foundation
import Testing
import TownCrierDomain

@Suite("DataFreshness")
struct DataFreshnessTests {
    @Test func fresh_whenCacheEntryIsFresh() {
        let now = Date()
        let entry = CacheEntry(data: "test", fetchedAt: now.addingTimeInterval(-60), ttlSeconds: 900)
        let freshness = DataFreshness.from(entry, at: now)

        #expect(freshness == .fresh)
    }

    @Test func stale_whenCacheEntryIsExpired() {
        let now = Date()
        let entry = CacheEntry(data: "test", fetchedAt: now.addingTimeInterval(-1000), ttlSeconds: 900)
        let freshness = DataFreshness.from(entry, at: now)

        #expect(freshness == .stale)
    }

    @Test func offline_isDistinctValue() {
        let freshness: DataFreshness = .offline
        #expect(freshness == .offline)
    }
}
