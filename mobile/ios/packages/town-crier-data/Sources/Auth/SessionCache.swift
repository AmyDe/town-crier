import Foundation
import TownCrierDomain

/// Thread-safe in-memory cache for the current Auth0 session, so most calls
/// skip the keychain (concurrent securityd reads contend and slow cold launch).
/// Concurrent callers on a cold cache share a single in-flight load via
/// `currentOrLoad`.
actor SessionCache {
  private var cached: AuthSession?
  private var inFlight: Task<AuthSession?, Never>?
  private var generation = 0
  private let leadTime: TimeInterval
  private let now: @Sendable () -> Date

  /// - Parameters:
  ///   - leadTime: How many seconds before `expiresAt` to consider the
  ///     cache stale. Mirrors Auth0's default 60-second renewal window so
  ///     we hand off to `CredentialsManager` before the access token
  ///     genuinely expires.
  ///   - now: Clock override for tests.
  init(
    leadTime: TimeInterval = 60,
    now: @Sendable @escaping () -> Date = { Date() }
  ) {
    self.leadTime = leadTime
    self.now = now
  }

  /// Returns a cached session if one is held and its access token is still
  /// at least `leadTime` seconds before `expiresAt`. Otherwise returns nil
  /// without populating the cache — callers should use `currentOrLoad` to
  /// fetch fresh credentials.
  func current() -> AuthSession? {
    guard let cached else { return nil }
    if cached.expiresAt.addingTimeInterval(-leadTime) > now() {
      return cached
    }
    return nil
  }

  /// Returns the cached session if valid, otherwise loads and caches one.
  /// Concurrent callers on a cold cache share one load. A load that
  /// overlaps `clear()` returns its result but does not cache it.
  func currentOrLoad(
    _ loader: @Sendable @escaping () async -> AuthSession?
  ) async -> AuthSession? {
    if let valid = current() {
      return valid
    }
    if let inFlight {
      return await inFlight.value
    }
    let startGeneration = generation
    let task = Task<AuthSession?, Never> {
      await loader()
    }
    inFlight = task
    let result = await task.value
    if generation == startGeneration {
      cached = result
      inFlight = nil
    }
    return result
  }

  /// Stores `session` as the current cached value. Used by `login` and
  /// `refreshSession` after a successful network round-trip so the next
  /// `currentSession()` short-circuits the keychain.
  func store(_ session: AuthSession) {
    cached = session
  }

  /// Drops the cached session and cancels any in-flight load. Called from
  /// `logout`, `deleteAccount`, and unrecoverable refresh failures.
  func clear() {
    generation += 1
    cached = nil
    inFlight?.cancel()
    inFlight = nil
  }
}
