import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("SettingsViewModel -- Appearance Mode")
@MainActor
struct SettingsAppearanceModeTests {
  private func makeSUT(
    appearanceMode: AppearanceMode? = nil
  ) -> SettingsViewModel {
    let defaults = UserDefaults(suiteName: "AppearanceTests.\(UUID().uuidString)")
    if let appearanceMode {
      defaults?.set(appearanceMode.rawValue, forKey: SettingsViewModel.appearanceModeKey)
    }
    let authSpy = SpyAuthenticationService()
    let subscriptionSpy = SpySubscriptionService()
    let profileSpy = SpyUserProfileRepository()
    let versionProvider = SpyAppVersionProvider()
    let notificationSpy = SpyNotificationService()
    return SettingsViewModel(
      authService: authSpy,
      subscriptionService: subscriptionSpy,
      userProfileRepository: profileSpy,
      appVersionProvider: versionProvider,
      notificationService: notificationSpy,
      defaults: defaults ?? .standard
    )
  }

  @Test func appearanceMode_defaultsToSystem() {
    let sut = makeSUT()

    #expect(sut.appearanceMode == .system)
  }

  @Test func appearanceMode_canBeSetToDark() {
    let sut = makeSUT()

    sut.appearanceMode = .dark

    #expect(sut.appearanceMode == .dark)
  }

  @Test func appearanceMode_canBeSetToOledDark() {
    let sut = makeSUT()

    sut.appearanceMode = .oledDark

    #expect(sut.appearanceMode == .oledDark)
  }

  @Test func appearanceMode_canBeSetToLight() {
    let sut = makeSUT()

    sut.appearanceMode = .light

    #expect(sut.appearanceMode == .light)
  }

  @Test func appearanceMode_persistsToUserDefaults() {
    let defaults = UserDefaults(suiteName: "PersistTest.\(UUID().uuidString)")
    let authSpy = SpyAuthenticationService()
    let subscriptionSpy = SpySubscriptionService()
    let profileSpy = SpyUserProfileRepository()
    let versionProvider = SpyAppVersionProvider()
    let notificationSpy = SpyNotificationService()
    let sut = SettingsViewModel(
      authService: authSpy,
      subscriptionService: subscriptionSpy,
      userProfileRepository: profileSpy,
      appVersionProvider: versionProvider,
      notificationService: notificationSpy,
      defaults: defaults ?? .standard
    )

    sut.appearanceMode = .oledDark

    let stored = defaults?.string(forKey: SettingsViewModel.appearanceModeKey)
    #expect(stored == "oledDark")
  }

  @Test func appearanceMode_restoresFromUserDefaults() {
    let sut = makeSUT(appearanceMode: .dark)

    #expect(sut.appearanceMode == .dark)
  }
}
