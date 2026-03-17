import Foundation
import Testing
import TownCrierDomain

@Suite("CacheEntry")
struct CacheEntryTests {
    // MARK: - Freshness

    @Test func isFresh_returnsTrueWhenWithinTTL() {
        let now = Date()
        let entry = CacheEntry(
            data: "test",
            fetchedAt: now.addingTimeInterval(-600),
            ttlSeconds: 900
        )

        #expect(entry.isFresh(at: now))
    }

    @Test func isFresh_returnsFalseWhenBeyondTTL() {
        let now = Date()
        let entry = CacheEntry(
            data: "test",
            fetchedAt: now.addingTimeInterval(-1000),
            ttlSeconds: 900
        )

        #expect(!entry.isFresh(at: now))
    }

    @Test func isFresh_returnsFalseWhenExactlyAtTTL() {
        let now = Date()
        let entry = CacheEntry(
            data: "test",
            fetchedAt: now.addingTimeInterval(-900),
            ttlSeconds: 900
        )

        #expect(!entry.isFresh(at: now))
    }

    @Test func age_returnsElapsedSecondsFromFetchedAt() {
        let now = Date()
        let entry = CacheEntry(
            data: "test",
            fetchedAt: now.addingTimeInterval(-300),
            ttlSeconds: 900
        )

        let age = entry.age(at: now)
        #expect(abs(age - 300) < 0.01)
    }

    // MARK: - Default TTL

    @Test func defaultTTL_is15Minutes() {
        #expect(CacheEntry<String>.defaultTTLSeconds == 900)
    }

    @Test func init_usesDefaultTTLWhenNotSpecified() {
        let entry = CacheEntry(data: "test", fetchedAt: Date())

        #expect(entry.ttlSeconds == 900)
    }
}
