import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Tests for the UK GDPR Art. 17 account erasure flow in `SettingsViewModel`.
///
/// The correct sequence is:
/// 1. `UserProfileRepository.delete()` -> DELETE /v1/me (server-side cascade)
/// 2. Only on success, `AuthenticationService.deleteAccount()` clears the
///    local session + keychain.
/// 3. On server failure, local credentials MUST be preserved so the user can
///    retry — otherwise their server data is orphaned.
@Suite("SettingsViewModel account deletion")
@MainActor
struct SettingsViewModelAccountDeletionTests {
  private func makeSUT(
    session: AuthSession? = .valid
  ) -> (
    SettingsViewModel, SpyAuthenticationService, SpyUserProfileRepository,
    SpyNotificationService
  ) {
    let authSpy = SpyAuthenticationService()
    authSpy.currentSessionResult = session
    let subscriptionSpy = SpySubscriptionService()
    let profileSpy = SpyUserProfileRepository()
    let versionProvider = SpyAppVersionProvider()
    let notificationSpy = SpyNotificationService()
    let defaults = UserDefaults(suiteName: "SettingsVMDeleteTests.\(UUID().uuidString)")
    let vm = SettingsViewModel(
      authService: authSpy,
      subscriptionService: subscriptionSpy,
      userProfileRepository: profileSpy,
      appVersionProvider: versionProvider,
      notificationService: notificationSpy,
      defaults: defaults ?? .standard
    )
    return (vm, authSpy, profileSpy, notificationSpy)
  }

  @Test func confirmDeleteAccount_callsRepositoryDeleteBeforeAuthDeleteAccount() async {
    let (sut, authSpy, profileSpy, _) = makeSUT()
    var authDeleteCountAtRepositoryCall: Int?
    profileSpy.onDelete = { [authSpy] in
      authDeleteCountAtRepositoryCall = authSpy.deleteAccountCallCount
    }
    sut.requestAccountDeletion()

    await sut.confirmDeleteAccount()

    #expect(profileSpy.deleteCallCount == 1)
    #expect(authSpy.deleteAccountCallCount == 1)
    #expect(
      authDeleteCountAtRepositoryCall == 0,
      "repository.delete() must run before auth.deleteAccount()"
    )
  }

  @Test func confirmDeleteAccount_serverDeleteFails_doesNotCallAuthDeleteAccount() async {
    let (sut, authSpy, profileSpy, _) = makeSUT()
    profileSpy.deleteResult = .failure(DomainError.networkUnavailable)
    sut.requestAccountDeletion()

    await sut.confirmDeleteAccount()

    #expect(profileSpy.deleteCallCount == 1)
    #expect(
      authSpy.deleteAccountCallCount == 0,
      "local credentials must not be cleared when server deletion fails"
    )
  }

  @Test func confirmDeleteAccount_serverDeleteFails_preservesSessionAndDoesNotNotify() async {
    var logoutCalled = false
    let (sut, _, profileSpy, _) = makeSUT()
    sut.onLogout = { logoutCalled = true }
    await sut.load()
    #expect(sut.userEmail != nil)
    profileSpy.deleteResult = .failure(DomainError.networkUnavailable)
    sut.requestAccountDeletion()

    await sut.confirmDeleteAccount()

    #expect(sut.userEmail != nil, "session must remain so the user can retry")
    #expect(!logoutCalled, "coordinator must not be notified when server delete fails")
  }

  @Test func confirmDeleteAccount_serverDeleteFails_setsError() async {
    let (sut, _, profileSpy, _) = makeSUT()
    profileSpy.deleteResult = .failure(DomainError.networkUnavailable)
    sut.requestAccountDeletion()

    await sut.confirmDeleteAccount()

    #expect(sut.error == .networkUnavailable)
  }
}
