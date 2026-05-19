import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

/// Behaviour contract for `SessionCache` (tc-3d7b).
///
/// The cache lives inside `Auth0AuthenticationService` and is the reason
/// repeated `currentSession()` calls within a single foreground burst issue
/// at most one `SecItemCopyMatching`. It must:
///
/// - Return cached sessions while their access token is still beyond the
///   renewal lead time.
/// - Single-flight concurrent cold-cache callers so a four-way burst loads
///   credentials from the keychain just once.
/// - Drop the cache on logout/refresh-failure so subsequent callers cannot
///   reuse a torn-down session.
@Suite("SessionCache (tc-3d7b)")
struct SessionCacheTests {

  private static let fixedNow = Date(timeIntervalSince1970: 1_700_000_000)
  private let fixedClock: @Sendable () -> Date = { Self.fixedNow }

  private func makeSession(expiresIn seconds: TimeInterval) -> AuthSession {
    AuthSession(
      accessToken: "a",
      idToken: "i",
      expiresAt: Self.fixedNow.addingTimeInterval(seconds),
      userProfile: UserProfile(userId: "u", email: "u@example.com", name: nil)
    )
  }

  // MARK: - current()

  @Test func current_returnsNil_whenEmpty() async {
    let sut = SessionCache(leadTime: 60, now: fixedClock)

    let result = await sut.current()

    #expect(result == nil)
  }

  @Test func current_returnsCached_whenAccessTokenIsBeyondLeadTime() async {
    let sut = SessionCache(leadTime: 60, now: fixedClock)
    let valid = makeSession(expiresIn: 300)  // 5 min from now, past 60s lead
    await sut.store(valid)

    let result = await sut.current()

    #expect(result == valid)
  }

  @Test func current_returnsNil_whenAccessTokenIsWithinLeadTime() async {
    let sut = SessionCache(leadTime: 60, now: fixedClock)
    let stale = makeSession(expiresIn: 30)  // inside the 60s renewal window
    await sut.store(stale)

    let result = await sut.current()

    #expect(result == nil)
  }

  @Test func current_returnsNil_whenAccessTokenAlreadyExpired() async {
    let sut = SessionCache(leadTime: 60, now: fixedClock)
    let expired = makeSession(expiresIn: -10)
    await sut.store(expired)

    let result = await sut.current()

    #expect(result == nil)
  }

  // MARK: - currentOrLoad()

  @Test func currentOrLoad_returnsCachedSession_withoutInvokingLoader() async {
    let sut = SessionCache(leadTime: 60, now: fixedClock)
    let valid = makeSession(expiresIn: 300)
    await sut.store(valid)
    let counter = CallCounter()
    let fallback = makeSession(expiresIn: 300)

    let result = await sut.currentOrLoad {
      await counter.increment()
      return fallback
    }

    #expect(result == valid)
    #expect(await counter.invocations == 0)
  }

  @Test func currentOrLoad_invokesLoader_whenCacheIsCold() async {
    let sut = SessionCache(leadTime: 60, now: fixedClock)
    let loaded = makeSession(expiresIn: 300)
    let counter = CallCounter()

    let result = await sut.currentOrLoad {
      await counter.increment()
      return loaded
    }

    #expect(result == loaded)
    #expect(await counter.invocations == 1)
  }

  @Test func currentOrLoad_cachesLoaderResult_forNextCall() async {
    let sut = SessionCache(leadTime: 60, now: fixedClock)
    let loaded = makeSession(expiresIn: 300)
    let counter = CallCounter()
    let other = makeSession(expiresIn: 300)

    _ = await sut.currentOrLoad {
      await counter.increment()
      return loaded
    }
    let second = await sut.currentOrLoad {
      await counter.increment()
      return other
    }

    #expect(second == loaded)
    #expect(await counter.invocations == 1)
  }

  /// The acceptance-criteria scenario: four concurrent callers hit the
  /// service on a cold cache. The single-flight contract means only one of
  /// them runs the loader, the rest await the same result.
  @Test func currentOrLoad_singleFlights_concurrentColdCacheLoaders() async {
    let sut = SessionCache(leadTime: 60, now: fixedClock)
    let loaded = makeSession(expiresIn: 300)
    let counter = CallCounter()

    async let result0 = sut.currentOrLoad {
      try? await Task.sleep(nanoseconds: 10_000_000)
      await counter.increment()
      return loaded
    }
    async let result1 = sut.currentOrLoad {
      try? await Task.sleep(nanoseconds: 10_000_000)
      await counter.increment()
      return loaded
    }
    async let result2 = sut.currentOrLoad {
      try? await Task.sleep(nanoseconds: 10_000_000)
      await counter.increment()
      return loaded
    }
    async let result3 = sut.currentOrLoad {
      try? await Task.sleep(nanoseconds: 10_000_000)
      await counter.increment()
      return loaded
    }
    let results = await [result0, result1, result2, result3]

    #expect(results.allSatisfy { $0 == loaded })
    #expect(await counter.invocations == 1)
  }

  // MARK: - clear()

  @Test func clear_dropsCachedSession() async {
    let sut = SessionCache(leadTime: 60, now: fixedClock)
    await sut.store(makeSession(expiresIn: 300))

    await sut.clear()

    #expect(await sut.current() == nil)
  }

  @Test func clear_forcesNextCurrentOrLoad_toInvokeLoader() async {
    let sut = SessionCache(leadTime: 60, now: fixedClock)
    let stored = makeSession(expiresIn: 300)
    await sut.store(stored)
    let counter = CallCounter()

    await sut.clear()
    let reloaded = makeSession(expiresIn: 600)
    let result = await sut.currentOrLoad {
      await counter.increment()
      return reloaded
    }

    #expect(result == reloaded)
    #expect(await counter.invocations == 1)
  }
}

private actor CallCounter {
  private(set) var invocations = 0
  func increment() { invocations += 1 }
}
