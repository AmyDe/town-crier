import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("SettingsViewModel")
@MainActor
struct SettingsViewModelTests {
  private func makeSUT(
    session: AuthSession? = .valid,
    entitlement: SubscriptionEntitlement? = nil,
    serverProfile: Result<ServerProfile?, Error> = .success(nil),
    version: String = "1.0.0",
    buildNumber: String = "42"
  ) -> (
    SettingsViewModel, SpyAuthenticationService, SpySubscriptionService,
    SpyUserProfileRepository, SpyAppVersionProvider, SpyNotificationService
  ) {
    let authSpy = SpyAuthenticationService()
    authSpy.currentSessionResult = session
    let subscriptionSpy = SpySubscriptionService()
    subscriptionSpy.currentEntitlementResult = entitlement
    let profileSpy = SpyUserProfileRepository()
    profileSpy.fetchResult = serverProfile
    let versionProvider = SpyAppVersionProvider()
    versionProvider.version = version
    versionProvider.buildNumber = buildNumber
    let notificationSpy = SpyNotificationService()
    let defaults = UserDefaults(suiteName: "SettingsVMTests.\(UUID().uuidString)")
    let vm = SettingsViewModel(
      authService: authSpy,
      subscriptionService: subscriptionSpy,
      userProfileRepository: profileSpy,
      appVersionProvider: versionProvider,
      notificationService: notificationSpy,
      defaults: defaults ?? .standard
    )
    return (vm, authSpy, subscriptionSpy, profileSpy, versionProvider, notificationSpy)
  }

  // MARK: - Loading

  @Test func load_populatesUserEmailFromSession() async {
    let (sut, _, _, _, _, _) = makeSUT(session: .valid)

    await sut.load()

    #expect(sut.userEmail == "test@example.com")
  }

  @Test func load_populatesUserNameFromSession() async {
    let (sut, _, _, _, _, _) = makeSUT(session: .valid)

    await sut.load()

    #expect(sut.userName == "Test User")
  }

  @Test func load_populatesAuthMethodFromSession() async {
    let (sut, _, _, _, _, _) = makeSUT(session: .valid)

    await sut.load()

    #expect(sut.authMethod == .emailPassword)
  }

  @Test func load_noSession_leavesUserFieldsNil() async {
    let (sut, _, _, _, _, _) = makeSUT(session: nil)

    await sut.load()

    #expect(sut.userEmail == nil)
    #expect(sut.userName == nil)
    #expect(sut.authMethod == nil)
  }

  @Test func load_populatesSubscriptionTier() async {
    let (sut, _, _, _, _, _) = makeSUT(entitlement: .personalActive)

    await sut.load()

    #expect(sut.subscriptionTier == .personal)
  }

  @Test func load_noEntitlement_showsFreeTier() async {
    let (sut, _, _, _, _, _) = makeSUT(entitlement: nil)

    await sut.load()

    #expect(sut.subscriptionTier == .free)
  }

  @Test func load_trialEntitlement_showsTrialFlag() async {
    let (sut, _, _, _, _, _) = makeSUT(entitlement: .personalTrial)

    await sut.load()

    #expect(sut.isTrialPeriod)
  }

  // MARK: - Server Profile Tier (source of truth)

  @Test func load_usesServerProfileTierAsSourceOfTruth() async {
    let (sut, _, _, _, _, _) = makeSUT(
      serverProfile: .success(.proUser)
    )

    await sut.load()

    #expect(sut.subscriptionTier == .pro)
  }

  @Test func load_serverProTier_overridesStoreKitFree() async {
    // User bought Pro on web — StoreKit has no record, but server knows
    let (sut, _, _, _, _, _) = makeSUT(
      entitlement: nil,
      serverProfile: .success(.proUser)
    )

    await sut.load()

    #expect(sut.subscriptionTier == .pro)
  }

  @Test func load_picksHigherTier_whenStoreKitAndServerDisagree() async {
    // StoreKit says personal, server says pro — take the higher
    let (sut, _, _, _, _, _) = makeSUT(
      entitlement: .personalActive,
      serverProfile: .success(.proUser)
    )

    await sut.load()

    #expect(sut.subscriptionTier == .pro)
  }

  @Test func load_picksHigherTier_whenStoreKitHigherThanServer() async {
    // StoreKit says pro (just purchased), server hasn't synced yet — take StoreKit
    let (sut, _, _, _, _, _) = makeSUT(
      entitlement: .proActive,
      serverProfile: .success(.freeUser)
    )

    await sut.load()

    #expect(sut.subscriptionTier == .pro)
  }

  @Test func load_fallsBackToStoreKit_whenServerProfileFetchFails() async {
    let (sut, _, _, _, _, _) = makeSUT(
      entitlement: .personalActive,
      serverProfile: .failure(DomainError.networkUnavailable)
    )

    await sut.load()

    #expect(sut.subscriptionTier == .personal)
  }

  @Test func load_usesJWTTier_whenServerAndStoreKitBothUnavailable() async {
    let (sut, _, _, _, _, _) = makeSUT(
      session: .pro,
      entitlement: nil,
      serverProfile: .failure(DomainError.networkUnavailable)
    )

    await sut.load()

    #expect(sut.subscriptionTier == .pro)
  }

  @Test func load_usesJWTTier_whenServerFailsAndStoreKitNil() async {
    let (sut, _, _, _, _, _) = makeSUT(
      session: .personal,
      entitlement: nil,
      serverProfile: .failure(DomainError.networkUnavailable)
    )

    await sut.load()

    #expect(sut.subscriptionTier == .personal)
  }

  @Test func load_picksHighestAcrossAllThreeSources() async {
    // JWT says pro, server says free, StoreKit says personal — take pro
    let (sut, _, _, _, _, _) = makeSUT(
      session: .pro,
      entitlement: .personalActive,
      serverProfile: .success(.freeUser)
    )

    await sut.load()

    #expect(sut.subscriptionTier == .pro)
  }

  @Test func load_noTrialFlag_whenJWTTierHigherThanStoreKitTrial() async {
    // JWT says pro, StoreKit says personal trial — trial is not meaningful
    // because the user's actual tier (pro) is above the trial tier
    let (sut, _, _, _, _, _) = makeSUT(
      session: .pro,
      entitlement: .personalTrial,
      serverProfile: .failure(DomainError.networkUnavailable)
    )

    await sut.load()

    #expect(sut.subscriptionTier == .pro)
    #expect(!sut.isTrialPeriod)
  }

  @Test func load_defaultsToFree_whenAllSourcesReturnFree() async {
    let (sut, _, _, _, _, _) = makeSUT(
      session: .valid,
      entitlement: nil,
      serverProfile: .failure(DomainError.networkUnavailable)
    )

    await sut.load()

    #expect(sut.subscriptionTier == .free)
  }

  @Test func load_fetchesServerProfile() async {
    let (sut, _, _, profileSpy, _, _) = makeSUT(
      serverProfile: .success(.proUser)
    )

    await sut.load()

    #expect(profileSpy.fetchCallCount == 1)
  }

  @Test func load_serverProfileNil_usesStoreKitTier() async {
    let (sut, _, _, _, _, _) = makeSUT(
      entitlement: .personalActive,
      serverProfile: .success(nil)
    )

    await sut.load()

    #expect(sut.subscriptionTier == .personal)
  }

  @Test func load_serverProfileReturnsFreeTier_storeKitNil_showsFree() async {
    let (sut, _, _, _, _, _) = makeSUT(
      entitlement: nil,
      serverProfile: .success(.freeUser)
    )

    await sut.load()

    #expect(sut.subscriptionTier == .free)
  }

  // MARK: - App Version

  @Test func appVersion_returnsVersionFromProvider() {
    let (sut, _, _, _, _, _) = makeSUT(version: "2.1.0", buildNumber: "99")

    #expect(sut.appVersion == "2.1.0 (99)")
  }

  // MARK: - Logout

  @Test func logout_callsAuthService() async {
    let (sut, authSpy, _, _, _, _) = makeSUT()

    await sut.logout()

    #expect(authSpy.logoutCallCount == 1)
  }

  @Test func logout_clearsSession() async {
    let (sut, _, _, _, _, _) = makeSUT()
    await sut.load()
    #expect(sut.userEmail != nil)

    await sut.logout()

    #expect(sut.userEmail == nil)
  }

  @Test func logout_notifiesCoordinator() async {
    var logoutCalled = false
    let (sut, _, _, _, _, _) = makeSUT()
    sut.onLogout = { logoutCalled = true }

    await sut.logout()

    #expect(logoutCalled)
  }

  @Test func logout_setsErrorOnFailure() async {
    let (sut, authSpy, _, _, _, _) = makeSUT()
    authSpy.logoutResult = .failure(DomainError.logoutFailed("network"))

    await sut.logout()

    #expect(sut.error == .logoutFailed("network"))
  }

  // MARK: - Account Deletion

  @Test func deleteAccount_requiresConfirmation() async {
    let (sut, authSpy, _, _, _, _) = makeSUT()

    #expect(!sut.isShowingDeleteConfirmation)

    sut.requestAccountDeletion()

    #expect(sut.isShowingDeleteConfirmation)
    #expect(authSpy.deleteAccountCallCount == 0)
  }

  @Test func confirmDeleteAccount_callsAuthService() async {
    let (sut, authSpy, _, _, _, _) = makeSUT()
    sut.requestAccountDeletion()

    await sut.confirmDeleteAccount()

    #expect(authSpy.deleteAccountCallCount == 1)
  }

  @Test func confirmDeleteAccount_clearsSessionAndNotifies() async {
    var logoutCalled = false
    let (sut, _, _, _, _, _) = makeSUT()
    sut.onLogout = { logoutCalled = true }
    await sut.load()
    sut.requestAccountDeletion()

    await sut.confirmDeleteAccount()

    #expect(sut.userEmail == nil)
    #expect(logoutCalled)
  }

  @Test func confirmDeleteAccount_setsErrorOnFailure() async {
    let (sut, authSpy, _, _, _, _) = makeSUT()
    authSpy.deleteAccountResult = .failure(DomainError.unexpected("deletion failed"))
    sut.requestAccountDeletion()

    await sut.confirmDeleteAccount()

    #expect(sut.error == .unexpected("deletion failed"))
  }

  @Test func cancelDeletion_dismissesConfirmation() {
    let (sut, _, _, _, _, _) = makeSUT()
    sut.requestAccountDeletion()
    #expect(sut.isShowingDeleteConfirmation)

    sut.cancelDeletion()

    #expect(!sut.isShowingDeleteConfirmation)
  }

  // MARK: - Device Token Removal on Logout

  @Test func logout_callsRemoveDeviceToken() async {
    let (sut, _, _, _, _, notificationSpy) = makeSUT()

    await sut.logout()

    #expect(notificationSpy.removeDeviceTokenCallCount == 1)
  }

  @Test func logout_succeedsWhenDeviceTokenRemovalFails() async {
    var logoutCalled = false
    let (sut, _, _, _, _, notificationSpy) = makeSUT()
    notificationSpy.removeDeviceTokenResult = .failure(DomainError.networkUnavailable)
    sut.onLogout = { logoutCalled = true }

    await sut.logout()

    #expect(logoutCalled)
    #expect(sut.error == nil)
  }

  @Test func confirmDeleteAccount_callsRemoveDeviceToken() async {
    let (sut, _, _, _, _, notificationSpy) = makeSUT()
    sut.requestAccountDeletion()

    await sut.confirmDeleteAccount()

    #expect(notificationSpy.removeDeviceTokenCallCount == 1)
  }

  @Test func attributionItems_containsExpectedSources() {
    let (sut, _, _, _, _, _) = makeSUT()
    let items = sut.attributionItems
    #expect(items.count == 4)
    #expect(items.contains { $0.name == "PlanIt" })
    #expect(items.contains { $0.name == "Crown Copyright" })
    #expect(items.contains { $0.name == "Ordnance Survey" })
    #expect(items.contains { $0.name == "Apple Maps" })
    #expect(!items.contains { $0.name == "OpenStreetMap" })
  }
}
