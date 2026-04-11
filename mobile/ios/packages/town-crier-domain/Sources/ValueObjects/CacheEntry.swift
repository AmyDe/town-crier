import Foundation

/// A cached piece of data with freshness tracking.
public struct CacheEntry<T: Sendable>: Sendable {
  public static var defaultTTLSeconds: TimeInterval { 900 }

  public let data: T
  public let fetchedAt: Date
  public let ttlSeconds: TimeInterval

  public init(data: T, fetchedAt: Date, ttlSeconds: TimeInterval = defaultTTLSeconds) {
    self.data = data
    self.fetchedAt = fetchedAt
    self.ttlSeconds = ttlSeconds
  }

  /// Whether the cached data is still within its time-to-live.
  public func isFresh(at now: Date = Date()) -> Bool {
    age(at: now) < ttlSeconds
  }

  /// Elapsed seconds since the data was fetched.
  public func age(at now: Date = Date()) -> TimeInterval {
    now.timeIntervalSince(fetchedAt)
  }
}
