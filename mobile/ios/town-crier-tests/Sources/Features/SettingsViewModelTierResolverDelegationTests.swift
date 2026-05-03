import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Verifies `SettingsViewModel` delegates tier resolution to the shared
/// `SubscriptionTierResolver` (tc-exg6.1), so its path cannot drift from
/// `AppCoordinator` again (third recurrence after tc-aza5).
@Suite("SettingsViewModel — Shared tier resolver delegation")
@MainActor
struct SettingsViewModelTierResolverDelegationTests {
  private func makeSUT(
    session: AuthSession? = .valid,
    tierResolver: SubscriptionTierResolving
  ) -> SettingsViewModel {
    let authSpy = SpyAuthenticationService()
    authSpy.currentSessionResult = session
    let subscriptionSpy = SpySubscriptionService()
    let profileSpy = SpyUserProfileRepository()
    let versionProvider = SpyAppVersionProvider()
    let notificationSpy = SpyNotificationService()
    let defaults = UserDefaults(suiteName: "SettingsVMResolverTests.\(UUID().uuidString)")
    return SettingsViewModel(
      authService: authSpy,
      subscriptionService: subscriptionSpy,
      userProfileRepository: profileSpy,
      tierResolver: tierResolver,
      appVersionProvider: versionProvider,
      notificationService: notificationSpy,
      defaults: defaults ?? .standard
    )
  }

  @Test
  func load_delegatesToSharedResolver_passingJwtAndPreviousTier() async throws {
    let fakeResolver = FakeSubscriptionTierResolver()
    fakeResolver.resolveResult = (.pro, false)
    let sut = makeSUT(session: .pro, tierResolver: fakeResolver)

    await sut.load()

    let call = try #require(fakeResolver.resolveCalls.first)
    #expect(call.jwtTier == .pro)
    // First call: previousTier defaults to .free (no cached tier yet).
    #expect(call.previousTier == .free)
    #expect(sut.subscriptionTier == .pro)
  }

  @Test
  func load_passesPreviouslyResolvedTier_asPreviousTierOnSecondCall() async {
    // Once the resolver has returned .pro, a subsequent load() must pass
    // .pro as previousTier so the no-downgrade fallback can preserve it.
    let fakeResolver = FakeSubscriptionTierResolver()
    fakeResolver.resolveResult = (.pro, false)
    let sut = makeSUT(session: .valid, tierResolver: fakeResolver)

    await sut.load()
    await sut.load()

    #expect(fakeResolver.resolveCalls.count == 2)
    #expect(fakeResolver.resolveCalls[1].previousTier == .pro)
  }

  @Test
  func load_passesUserSubFromSession() async throws {
    let fakeResolver = FakeSubscriptionTierResolver()
    let sut = makeSUT(session: .valid, tierResolver: fakeResolver)

    await sut.load()

    let call = try #require(fakeResolver.resolveCalls.first)
    #expect(call.userSub == "auth0|user-001")
  }

  @Test
  func load_propagatesIsTrialPeriodFromResolver() async {
    let fakeResolver = FakeSubscriptionTierResolver()
    fakeResolver.resolveResult = (.personal, true)
    let sut = makeSUT(session: .valid, tierResolver: fakeResolver)

    await sut.load()

    #expect(sut.subscriptionTier == .personal)
    #expect(sut.isTrialPeriod)
  }
}
