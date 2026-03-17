import Foundation

/// Indicates how current the displayed data is.
public enum DataFreshness: Equatable, Sendable {
    /// Data is within its TTL — no indicator needed.
    case fresh
    /// Data is beyond its TTL but still usable — show stale indicator.
    case stale
    /// Device is offline, serving cached data — show offline indicator.
    case offline

    /// Derives freshness from a cache entry at a given point in time.
    public static func from<T>(_ entry: CacheEntry<T>, at now: Date = Date()) -> DataFreshness {
        entry.isFresh(at: now) ? .fresh : .stale
    }
}
