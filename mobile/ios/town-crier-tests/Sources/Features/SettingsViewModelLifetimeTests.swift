import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("SettingsViewModel — lifetime entitlement")
@MainActor
struct SettingsViewModelLifetimeTests {
  private func makeSUT(
    entitlement: SubscriptionEntitlement? = nil,
    serverProfile: Result<ServerProfile, Error> = .success(.freeUser)
  ) -> SettingsViewModel {
    let authSpy = SpyAuthenticationService()
    authSpy.currentSessionResult = .valid
    let subscriptionSpy = SpySubscriptionService()
    subscriptionSpy.currentEntitlementResult = entitlement
    let profileSpy = SpyUserProfileRepository()
    profileSpy.createResult = serverProfile
    let defaults = UserDefaults(suiteName: "SettingsVMLifetimeTests.\(UUID().uuidString)")
    return SettingsViewModel(
      authService: authSpy,
      subscriptionService: subscriptionSpy,
      userProfileRepository: profileSpy,
      appVersionProvider: SpyAppVersionProvider(),
      notificationService: SpyNotificationService(),
      defaults: defaults ?? .standard
    )
  }

  @Test func init_isNotLifetime() {
    #expect(!makeSUT().isLifetime)
  }

  @Test func load_lifetimeEntitlement_showsLifetimeFlag() async {
    let sut = makeSUT(entitlement: .proLifetime, serverProfile: .success(.proUser))

    await sut.load()

    #expect(sut.subscriptionTier == .pro)
    #expect(sut.isLifetime)
  }

  @Test func load_subscriptionEntitlement_hidesLifetimeFlag() async {
    let sut = makeSUT(entitlement: .proMonthlyActive, serverProfile: .success(.proUser))

    await sut.load()

    #expect(!sut.isLifetime)
  }

  @Test func logout_resetsLifetimeFlag() async {
    let sut = makeSUT(entitlement: .proLifetime, serverProfile: .success(.proUser))
    await sut.load()
    #expect(sut.isLifetime)

    await sut.logout()

    #expect(!sut.isLifetime)
  }
}
