import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Tests covering the saved-application decision-notification preferences
/// surfaced in `SettingsViewModel`. Split out from `SettingsViewModelTests`
/// to keep both files within SwiftLint's 400-line file-length limit.
@Suite("SettingsViewModel — Saved-Decision Preferences")
@MainActor
struct SettingsViewModelSavedDecisionTests {
  private func makeSUT(
    serverProfile: Result<ServerProfile, Error> = .success(.freeUser)
  ) -> (SettingsViewModel, SpyUserProfileRepository) {
    let authSpy = SpyAuthenticationService()
    authSpy.currentSessionResult = .valid
    let subscriptionSpy = SpySubscriptionService()
    let profileSpy = SpyUserProfileRepository()
    profileSpy.createResult = serverProfile
    let versionProvider = SpyAppVersionProvider()
    let notificationSpy = SpyNotificationService()
    let defaults = UserDefaults(suiteName: "SettingsVMSavedTests.\(UUID().uuidString)")
    let vm = SettingsViewModel(
      authService: authSpy,
      subscriptionService: subscriptionSpy,
      userProfileRepository: profileSpy,
      appVersionProvider: versionProvider,
      notificationService: notificationSpy,
      defaults: defaults ?? .standard
    )
    return (vm, profileSpy)
  }

  @Test func load_populatesSavedDecisionFlagsFromServerProfile() async {
    let profile = ServerProfile(
      userId: "auth0|user-001",
      tier: .personal,
      pushEnabled: true,
      digestDay: .monday,
      emailDigestEnabled: true,
      savedDecisionPush: false,
      savedDecisionEmail: true
    )
    let (sut, _) = makeSUT(serverProfile: .success(profile))

    await sut.load()

    #expect(sut.savedDecisionPush == false)
    #expect(sut.savedDecisionEmail == true)
  }

  @Test func load_savedDecisionFlagsDefaultToTrue_whenServerProfileMissing() async {
    let (sut, _) = makeSUT(
      serverProfile: .failure(DomainError.networkUnavailable)
    )

    await sut.load()

    #expect(sut.savedDecisionPush == true)
    #expect(sut.savedDecisionEmail == true)
  }

  @Test func setSavedDecisionPush_persistsViaRepositoryUpdate() async {
    let initialProfile = ServerProfile(
      userId: "auth0|user-001",
      tier: .personal,
      pushEnabled: true,
      digestDay: .friday,
      emailDigestEnabled: true,
      savedDecisionPush: true,
      savedDecisionEmail: true
    )
    let updatedProfile = ServerProfile(
      userId: "auth0|user-001",
      tier: .personal,
      pushEnabled: true,
      digestDay: .friday,
      emailDigestEnabled: true,
      savedDecisionPush: false,
      savedDecisionEmail: true
    )
    let (sut, profileSpy) = makeSUT(serverProfile: .success(initialProfile))
    profileSpy.updateResult = .success(updatedProfile)
    await sut.load()

    await sut.setSavedDecisionPush(false)

    #expect(profileSpy.updateCalls.count == 1)
    let call = profileSpy.updateCalls[0]
    #expect(call.savedDecisionPush == false)
    #expect(call.savedDecisionEmail == true)
    #expect(call.pushEnabled == true)
    #expect(call.digestDay == .friday)
    #expect(call.emailDigestEnabled == true)
    #expect(sut.savedDecisionPush == false)
  }

  @Test func setSavedDecisionEmail_persistsViaRepositoryUpdate() async {
    let initialProfile = ServerProfile(
      userId: "auth0|user-001",
      tier: .personal,
      pushEnabled: false,
      digestDay: .wednesday,
      emailDigestEnabled: false,
      savedDecisionPush: true,
      savedDecisionEmail: true
    )
    let updatedProfile = ServerProfile(
      userId: "auth0|user-001",
      tier: .personal,
      pushEnabled: false,
      digestDay: .wednesday,
      emailDigestEnabled: false,
      savedDecisionPush: true,
      savedDecisionEmail: false
    )
    let (sut, profileSpy) = makeSUT(serverProfile: .success(initialProfile))
    profileSpy.updateResult = .success(updatedProfile)
    await sut.load()

    await sut.setSavedDecisionEmail(false)

    #expect(profileSpy.updateCalls.count == 1)
    let call = profileSpy.updateCalls[0]
    #expect(call.savedDecisionEmail == false)
    #expect(call.savedDecisionPush == true)
    #expect(call.pushEnabled == false)
    #expect(call.digestDay == .wednesday)
    #expect(call.emailDigestEnabled == false)
    #expect(sut.savedDecisionEmail == false)
  }

  @Test func setSavedDecisionPush_setsErrorAndRollsBackOnFailure() async {
    let initialProfile = ServerProfile(
      userId: "auth0|user-001",
      tier: .free,
      pushEnabled: true,
      digestDay: .monday,
      emailDigestEnabled: true,
      savedDecisionPush: true,
      savedDecisionEmail: true
    )
    let (sut, profileSpy) = makeSUT(serverProfile: .success(initialProfile))
    profileSpy.updateResult = .failure(DomainError.networkUnavailable)
    await sut.load()

    await sut.setSavedDecisionPush(false)

    #expect(sut.error == .networkUnavailable)
    #expect(sut.savedDecisionPush == true)
  }
}
