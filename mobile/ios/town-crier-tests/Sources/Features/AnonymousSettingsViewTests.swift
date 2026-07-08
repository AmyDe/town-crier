import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// GH#879 Phase 3: the anonymous Settings tab. Exposes exactly:
/// Create-account, Appearance, Data Attribution, Legal, About (Rate the App
/// + Version). MUST NOT render: account info, notification preferences,
/// subscription, export-your-data, sign out, delete account — enforced
/// structurally by `AnonymousSettingsView.init` and `AnonymousSettingsViewModel`
/// having no parameters/properties for any of those (see
/// `AnonymousSettingsViewModelTests.viewModel_exposesNoAccountBoundStoredProperties`).
@Suite("AnonymousSettingsView")
@MainActor
struct AnonymousSettingsViewTests {
  private func makeViewModel() -> AnonymousSettingsViewModel {
    // swiftlint:disable:next force_unwrapping
    let store = AppearanceStore(defaults: UserDefaults(suiteName: UUID().uuidString)!)
    return AnonymousSettingsViewModel(
      appearanceStore: store, appVersionProvider: SpyAppVersionProvider())
  }

  @Test func body_renders() {
    let sut = AnonymousSettingsView(
      viewModel: makeViewModel(),
      onCreateAccount: {},
      onSignIn: {},
      onPrivacyPolicy: {},
      onTermsOfService: {},
      onRateApp: {}
    )

    _ = sut.body
  }

  @Test func createAccountTap_invokesOnCreateAccount() {
    var invoked = false
    let sut = AnonymousSettingsView(
      viewModel: makeViewModel(),
      onCreateAccount: { invoked = true },
      onSignIn: {},
      onPrivacyPolicy: {},
      onTermsOfService: {},
      onRateApp: {}
    )

    sut.requestCreateAccount()

    #expect(invoked)
  }

  @Test func signInTap_invokesOnSignIn() {
    var invoked = false
    let sut = AnonymousSettingsView(
      viewModel: makeViewModel(),
      onCreateAccount: {},
      onSignIn: { invoked = true },
      onPrivacyPolicy: {},
      onTermsOfService: {},
      onRateApp: {}
    )

    sut.requestSignIn()

    #expect(invoked)
  }

  @Test func privacyPolicyTap_invokesOnPrivacyPolicy() {
    var invoked = false
    let sut = AnonymousSettingsView(
      viewModel: makeViewModel(),
      onCreateAccount: {},
      onSignIn: {},
      onPrivacyPolicy: { invoked = true },
      onTermsOfService: {},
      onRateApp: {}
    )

    sut.requestPrivacyPolicy()

    #expect(invoked)
  }

  @Test func termsOfServiceTap_invokesOnTermsOfService() {
    var invoked = false
    let sut = AnonymousSettingsView(
      viewModel: makeViewModel(),
      onCreateAccount: {},
      onSignIn: {},
      onPrivacyPolicy: {},
      onTermsOfService: { invoked = true },
      onRateApp: {}
    )

    sut.requestTermsOfService()

    #expect(invoked)
  }

  @Test func rateAppTap_invokesOnRateApp() {
    var invoked = false
    let sut = AnonymousSettingsView(
      viewModel: makeViewModel(),
      onCreateAccount: {},
      onSignIn: {},
      onPrivacyPolicy: {},
      onTermsOfService: {},
      onRateApp: { invoked = true }
    )

    sut.requestRateApp()

    #expect(invoked)
  }
}
