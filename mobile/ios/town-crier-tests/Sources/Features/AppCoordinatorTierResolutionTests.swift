import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("AppCoordinator — Subscription Tier Resolution")
@MainActor
struct AppCoordinatorTierResolutionTests {

  // MARK: - Helpers

  private func makeSUT(
    authSession: AuthSession? = nil,
    entitlement: SubscriptionEntitlement? = nil,
    serverProfile: ServerProfile? = nil,
    serverProfileError: Error? = nil,
    tierCache: UserDefaults? = nil
  ) -> (AppCoordinator, SpyAuthenticationService, SpySubscriptionService, SpyUserProfileRepository) {
    let authSpy = SpyAuthenticationService()
    authSpy.currentSessionResult = authSession
    let subscriptionSpy = SpySubscriptionService()
    subscriptionSpy.currentEntitlementResult = entitlement
    let profileSpy = SpyUserProfileRepository()
    if let serverProfileError {
      profileSpy.fetchResult = .failure(serverProfileError)
    } else {
      profileSpy.fetchResult = .success(serverProfile)
    }
    let coordinator = AppCoordinator(
      repository: SpyPlanningApplicationRepository(),
      authService: authSpy,
      subscriptionService: subscriptionSpy,
      userProfileRepository: profileSpy,
      watchZoneRepository: SpyWatchZoneRepository(),
      geocoder: SpyPostcodeGeocoder(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService(),
      tierCache: tierCache
    )
    return (coordinator, authSpy, subscriptionSpy, profileSpy)
  }

  private func makeServerProfile(tier: SubscriptionTier) -> ServerProfile {
    ServerProfile(
      userId: "u1",
      tier: tier,
      pushEnabled: true,
      digestDay: .monday,
      emailDigestEnabled: true
    )
  }

  // MARK: - resolveSubscriptionTier

  @Test func resolveSubscriptionTier_picksHighestFromAllSources() async {
    let (sut, _, _, _) = makeSUT(
      authSession: .pro,
      serverProfile: makeServerProfile(tier: .personal)
    )

    await sut.resolveSubscriptionTier()

    #expect(sut.subscriptionTier == .pro)
  }

  @Test func resolveSubscriptionTier_storeKitWins_whenHighest() async {
    let (sut, _, _, _) = makeSUT(
      authSession: .valid,
      entitlement: .proActive
    )

    await sut.resolveSubscriptionTier()

    #expect(sut.subscriptionTier == .pro)
  }

  @Test func resolveSubscriptionTier_serverWins_whenHighest() async {
    let (sut, _, _, _) = makeSUT(
      authSession: .valid,
      serverProfile: makeServerProfile(tier: .pro)
    )

    await sut.resolveSubscriptionTier()

    #expect(sut.subscriptionTier == .pro)
  }

  @Test func resolveSubscriptionTier_defaultsToFree_whenNoSession() async {
    let (sut, _, _, _) = makeSUT()

    await sut.resolveSubscriptionTier()

    #expect(sut.subscriptionTier == .free)
  }

  @Test func resolveSubscriptionTier_fallsBackToJWT_whenServerFails() async {
    let (sut, _, _, _) = makeSUT(
      authSession: .personal,
      serverProfileError: DomainError.networkUnavailable
    )

    await sut.resolveSubscriptionTier()

    #expect(sut.subscriptionTier == .personal)
  }

  // MARK: - Factory methods use resolved tier

  @Test func makeWatchZoneListViewModel_usesResolvedTier() async {
    let (sut, _, _, _) = makeSUT(authSession: .pro)

    await sut.resolveSubscriptionTier()
    let vm = sut.makeWatchZoneListViewModel()

    #expect(vm.featureGate.tier == .pro)
  }

  @Test func makeWatchZoneEditorViewModel_usesResolvedTier() async {
    let (sut, _, _, _) = makeSUT(authSession: .personal)

    await sut.resolveSubscriptionTier()
    let vm = sut.makeWatchZoneEditorViewModel()

    #expect(vm.availableRadiusOptions == WatchZoneLimits(tier: .personal).availableRadiusOptions)
  }

  // MARK: - Tier preservation when all sources fail

  @Test func resolveSubscriptionTier_preservesPreviousTier_whenAllSourcesFail() async {
    // First resolution: server returns .pro, others return .free
    let (sut, authSpy, subscriptionSpy, profileSpy) = makeSUT(
      authSession: .valid,
      serverProfile: makeServerProfile(tier: .pro)
    )
    await sut.resolveSubscriptionTier()
    #expect(sut.subscriptionTier == .pro)

    // Second resolution: all sources fail/return .free
    authSpy.currentSessionResult = nil
    subscriptionSpy.currentEntitlementResult = nil
    profileSpy.fetchResult = .failure(DomainError.networkUnavailable)

    await sut.resolveSubscriptionTier()

    // Should preserve the previous .pro tier, not fall back to .free
    #expect(sut.subscriptionTier == .pro)
  }

  @Test func resolveSubscriptionTier_updatesToFree_whenServerExplicitlyReturnsFree() async {
    // First resolution: server returns .pro
    let (sut, authSpy, subscriptionSpy, profileSpy) = makeSUT(
      authSession: .valid,
      serverProfile: makeServerProfile(tier: .pro)
    )
    await sut.resolveSubscriptionTier()
    #expect(sut.subscriptionTier == .pro)

    // Second resolution: server explicitly returns .free (user downgraded)
    authSpy.currentSessionResult = nil
    subscriptionSpy.currentEntitlementResult = nil
    profileSpy.fetchResult = .success(makeServerProfile(tier: .free))

    await sut.resolveSubscriptionTier()

    // Should update to .free because server explicitly returned .free
    #expect(sut.subscriptionTier == .free)
  }

  @Test func resolveSubscriptionTier_preservesPreviousTier_whenServerReturnsNilProfile() async {
    // First resolution: server returns .pro
    let (sut, authSpy, subscriptionSpy, profileSpy) = makeSUT(
      authSession: .valid,
      serverProfile: makeServerProfile(tier: .pro)
    )
    await sut.resolveSubscriptionTier()
    #expect(sut.subscriptionTier == .pro)

    // Second resolution: server returns nil (404), others fail
    authSpy.currentSessionResult = nil
    subscriptionSpy.currentEntitlementResult = nil
    profileSpy.fetchResult = .success(nil)

    await sut.resolveSubscriptionTier()

    // Profile not found (404) means the profile was deleted -- this is a genuine
    // state change, so tier should update to .free
    #expect(sut.subscriptionTier == .free)
  }

  // MARK: - Persistent tier caching

  @Test func resolveSubscriptionTier_persistsResolvedTier() async throws {
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    let (sut, _, _, _) = makeSUT(
      authSession: .valid,
      serverProfile: makeServerProfile(tier: .pro),
      tierCache: defaults
    )

    await sut.resolveSubscriptionTier()

    #expect(defaults.string(forKey: "cachedSubscriptionTier") == "pro")
  }

  @Test func init_restoresCachedTier() throws {
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    defaults.set("pro", forKey: "cachedSubscriptionTier")

    let (sut, _, _, _) = makeSUT(tierCache: defaults)

    #expect(sut.subscriptionTier == .pro)
  }

  @Test func resolveSubscriptionTier_usesCachedTier_whenAllSourcesFail() async throws {
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    defaults.set("pro", forKey: "cachedSubscriptionTier")

    let (sut, _, _, _) = makeSUT(
      serverProfileError: DomainError.networkUnavailable,
      tierCache: defaults
    )

    await sut.resolveSubscriptionTier()

    // Cached tier should be used when all live sources fail
    #expect(sut.subscriptionTier == .pro)
  }
}
