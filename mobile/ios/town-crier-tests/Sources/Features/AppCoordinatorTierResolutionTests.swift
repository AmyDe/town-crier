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
    serverProfileError: Error? = nil
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
      versionConfigService: SpyVersionConfigService()
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
}
