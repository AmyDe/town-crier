import Foundation
import TownCrierDomain

/// Thread-safe in-memory cache for the current Auth0 session.
///
/// `Auth0AuthenticationService.currentSession()` was hitting
/// `SimpleKeychain.data(forKey:)` -> `SecItemCopyMatching` -> securityd on
/// every call, via `CredentialsManager.canRenew()` / `hasValid()` /
/// `credentials()`. Concurrent foreground bursts (e.g. a push tap arriving
/// while `LoginViewModel.checkExistingSession` and
/// `AppCoordinator.resolveSubscriptionTier` are already in flight) caused
/// four-way contention on securityd's connection pool, adding tens to
/// hundreds of milliseconds to cold-launch latency and amplifying the
/// resumption gap that drove the tc-cbmk notification-tap crash.
///
/// This cache holds the most recent valid `AuthSession` in memory. When the
/// session's access token has at least `leadTime` seconds before
/// `expiresAt`, callers get the cached value without touching the keychain.
/// Concurrent callers on a cold cache share a single in-flight load via
/// `currentOrLoad`, so a four-way burst issues at most one
/// `SecItemCopyMatching` (tc-3d7b).
actor SessionCache {
  private var cached: AuthSession?
  private var inFlight: Task<AuthSession?, Never>?
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

  /// Returns the cached session if valid, otherwise invokes `loader` and
  /// caches the result. Concurrent callers on a cold cache share the same
  /// in-flight load — this is the single-flight behaviour that prevents
  /// a foreground burst from issuing N keychain reads for the same token.
  func currentOrLoad(
    _ loader: @Sendable @escaping () async -> AuthSession?
  ) async -> AuthSession? {
    if let valid = current() {
      return valid
    }
    if let inFlight {
      return await inFlight.value
    }
    let task = Task<AuthSession?, Never> {
      await loader()
    }
    inFlight = task
    let result = await task.value
    cached = result
    inFlight = nil
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
    cached = nil
    inFlight?.cancel()
    inFlight = nil
  }
}
