import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// GH#879 Phase 3: the anonymous Settings tab's ViewModel — deliberately a
/// distinct, much smaller type than the authenticated `SettingsViewModel`
/// rather than a shared abstraction. It has no `AuthenticationService`,
/// `SubscriptionService`, or `UserProfileRepository` dependency at all, which
/// is itself the structural guarantee that account info, notification
/// preferences, subscription state, data export, sign-out, and account
/// deletion can never appear on this screen — there is no data behind them.
@Suite("AnonymousSettingsViewModel")
@MainActor
struct AnonymousSettingsViewModelTests {
  private func makeSUT(
    version: String = "1.0.0",
    buildNumber: String = "42",
    defaults: UserDefaults? = nil
  ) -> (AnonymousSettingsViewModel, AppearanceStore, SpyAppVersionProvider) {
    // swiftlint:disable:next force_unwrapping
    let store = AppearanceStore(defaults: defaults ?? UserDefaults(suiteName: UUID().uuidString)!)
    let versionProvider = SpyAppVersionProvider()
    versionProvider.version = version
    versionProvider.buildNumber = buildNumber
    let sut = AnonymousSettingsViewModel(
      appearanceStore: store, appVersionProvider: versionProvider)
    return (sut, store, versionProvider)
  }

  // MARK: - Appearance (shared live source, GH#878)

  @Test func appearanceMode_readsThroughToSharedStore() {
    let (sut, store, _) = makeSUT()
    store.appearanceMode = .oledDark

    #expect(sut.appearanceMode == .oledDark)
  }

  @Test func appearanceMode_setter_writesThroughToSharedStore() {
    let (sut, store, _) = makeSUT()

    sut.appearanceMode = .dark

    #expect(store.appearanceMode == .dark)
  }

  // MARK: - App version

  @Test func appVersion_formatsVersionAndBuildNumber() {
    let (sut, _, _) = makeSUT(version: "2.3.0", buildNumber: "77")

    #expect(sut.appVersion == "2.3.0 (77)")
  }

  // MARK: - Data attribution — same content as the authed screen

  @Test func attributionItems_matchesTheSharedStandardSet() {
    let (sut, _, _) = makeSUT()

    #expect(sut.attributionItems == AttributionItem.standard)
  }

  // MARK: - No account-bound state (GH#879 Phase 3 acceptance criteria)

  @Test func viewModel_exposesNoAccountBoundStoredProperties() {
    let (sut, _, _) = makeSUT()
    let forbiddenSubstrings = [
      "userEmail", "userName", "authMethod", "subscriptionTier", "isTrialPeriod",
      "isExporting", "exportFileURL", "exportErrorMessage", "isShowingDeleteConfirmation",
      "onLogout",
    ]

    let labels = Mirror(reflecting: sut).children.compactMap(\.label)

    #expect(
      labels.allSatisfy { label in
        !forbiddenSubstrings.contains { label.localizedCaseInsensitiveContains($0) }
      })
  }
}
