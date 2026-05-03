import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Tests for the shared `SubscriptionTierResolver` that both `AppCoordinator`
/// and `SettingsViewModel` delegate to. The resolver folds the three tier
/// sources (server profile, StoreKit, JWT claim) into a single answer and
/// guarantees a paying user is never silently downgraded to `.free` due to a
/// transient server failure (bug tc-exg6).
@Suite("SubscriptionTierResolver")
struct SubscriptionTierResolverTests {

  // MARK: - Helpers

  private func makeSUT(
    serverTier: SubscriptionTier? = .free,
    storeKitEntitlement: SubscriptionEntitlement? = nil,
    refreshedSessionTier: SubscriptionTier = .free,
    refreshSessionThrows: Bool = false
  ) -> (SubscriptionTierResolver, SpyAuthenticationService) {
    let authSpy = SpyAuthenticationService()
    let refreshedSession = AuthSession(
      accessToken: "refreshed",
      idToken: "refreshed",
      expiresAt: Date.distantFuture,
      userProfile: .testUser,
      subscriptionTier: refreshedSessionTier
    )
    if refreshSessionThrows {
      authSpy.refreshSessionResult = .failure(DomainError.networkUnavailable)
    } else {
      authSpy.refreshSessionResult = .success(refreshedSession)
    }
    let sut = SubscriptionTierResolver(
      serverFetcher: { serverTier },
      storeKitFetcher: { storeKitEntitlement },
      authService: authSpy
    )
    return (sut, authSpy)
  }

  // MARK: - Basic resolution

  @Test
  func whenServerReturnsPro_andJwtIsFree_resolvesPro() async {
    let (sut, _) = makeSUT(serverTier: .pro)

    let result = await sut.resolve(
      jwtTier: .free,
      previousTier: .free,
      userSub: "auth0|user-001"
    )

    #expect(result.tier == .pro)
    #expect(!result.isTrialPeriod)
  }

  // MARK: - No-downgrade fallback (regression for tc-exg6)

  @Test
  func whenServerFailsAndPreviousTierWasPro_preservesPro() async {
    // Regression: server fetch fails (returns nil), JWT and StoreKit both
    // .free, but previousTier was .pro. The resolver MUST preserve .pro
    // and never silently downgrade the user.
    let (sut, _) = makeSUT(
      serverTier: nil,
      storeKitEntitlement: nil
    )

    let result = await sut.resolve(
      jwtTier: .free,
      previousTier: .pro,
      userSub: "auth0|user-001"
    )

    #expect(result.tier == .pro)
  }

  @Test
  func whenServerFailsAndPreviousTierWasFreeButJwtIsPro_resolvesPro() async {
    // Server fails, previousTier is .free, but JWT claim is .pro — the
    // fallback `max(previousTier, jwtTier)` should still pick the JWT tier.
    let (sut, _) = makeSUT(
      serverTier: nil,
      storeKitEntitlement: nil
    )

    let result = await sut.resolve(
      jwtTier: .pro,
      previousTier: .free,
      userSub: "auth0|user-001"
    )

    #expect(result.tier == .pro)
  }

  // MARK: - Free winner: log + refreshSession retry

  @Test
  func whenAllSourcesReturnFree_resolvesFreeAndLogsNotice() async {
    // Even when all sources genuinely report .free, the resolver returns
    // .free (after a single refreshSession retry attempt) — the log notice
    // is for diagnostic purposes and is verified by behaviour, not via a
    // log spy: we observe that refreshSession() was called exactly once.
    let (sut, authSpy) = makeSUT(
      serverTier: .free,
      storeKitEntitlement: nil,
      refreshedSessionTier: .free
    )

    let result = await sut.resolve(
      jwtTier: .free,
      previousTier: .free,
      userSub: "auth0|user-001"
    )

    #expect(result.tier == .free)
    #expect(authSpy.refreshSessionCallCount == 1)
  }

  @Test
  func whenWinnerIsFree_andRefreshSessionPromotesJwtToPro_secondPassResolvesPro() async {
    // First pass: jwtTier=.free, server=.free, storeKit=nil → winner .free.
    // The resolver calls refreshSession(); the new session's JWT carries
    // .pro. Re-resolving picks that up and returns .pro.
    let (sut, authSpy) = makeSUT(
      serverTier: .free,
      storeKitEntitlement: nil,
      refreshedSessionTier: .pro
    )

    let result = await sut.resolve(
      jwtTier: .free,
      previousTier: .free,
      userSub: "auth0|user-001"
    )

    #expect(result.tier == .pro)
    #expect(authSpy.refreshSessionCallCount == 1)
  }

  @Test
  func whenWinnerIsFreeOnBothPasses_doesNotLoop() async {
    // Guard against infinite recursion: if the second pass also resolves
    // to .free, the resolver must NOT call refreshSession() again.
    let (sut, authSpy) = makeSUT(
      serverTier: .free,
      storeKitEntitlement: nil,
      refreshedSessionTier: .free
    )

    _ = await sut.resolve(
      jwtTier: .free,
      previousTier: .free,
      userSub: "auth0|user-001"
    )

    #expect(authSpy.refreshSessionCallCount == 1)
  }

  @Test
  func whenRefreshSessionThrows_resolverStillReturnsFreeWithoutLooping() async {
    // refreshSession() failures must be tolerated: the resolver returns the
    // first-pass result (.free) without crashing or looping.
    let (sut, authSpy) = makeSUT(
      serverTier: .free,
      storeKitEntitlement: nil,
      refreshSessionThrows: true
    )

    let result = await sut.resolve(
      jwtTier: .free,
      previousTier: .free,
      userSub: "auth0|user-001"
    )

    #expect(result.tier == .free)
    #expect(authSpy.refreshSessionCallCount == 1)
  }

  // MARK: - Trial period

  @Test
  func whenStoreKitTrialIsHighestTier_isTrialPeriodIsTrue() async {
    let (sut, _) = makeSUT(
      serverTier: .free,
      storeKitEntitlement: .personalTrial
    )

    let result = await sut.resolve(
      jwtTier: .free,
      previousTier: .free,
      userSub: nil
    )

    #expect(result.tier == .personal)
    #expect(result.isTrialPeriod)
  }

  @Test
  func whenJwtTierExceedsStoreKitTrial_isTrialPeriodIsFalse() async {
    // JWT says pro, StoreKit says personal trial — the user's actual tier
    // is .pro, so the trial flag is not meaningful.
    let (sut, _) = makeSUT(
      serverTier: .free,
      storeKitEntitlement: .personalTrial
    )

    let result = await sut.resolve(
      jwtTier: .pro,
      previousTier: .free,
      userSub: nil
    )

    #expect(result.tier == .pro)
    #expect(!result.isTrialPeriod)
  }
}
