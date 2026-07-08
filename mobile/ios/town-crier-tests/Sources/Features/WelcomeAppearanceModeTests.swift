import Combine
import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// GH#878: appearance control on the anonymous welcome screen. Mirrors
/// `SettingsAppearanceModeTests`' persistence-test conventions (injected
/// `UserDefaults(suiteName:)`), but the crux of this suite is the
/// single-source-of-truth guarantee — a mode picked from the welcome screen
/// must be exactly what `SettingsViewModel` exposes afterwards, with no
/// separate copy of the state to diverge.
@Suite("WelcomeViewModel -- Appearance Mode")
@MainActor
struct WelcomeAppearanceModeTests {
  private func makeDefaults() -> UserDefaults {
    // swiftlint:disable:next force_unwrapping
    UserDefaults(suiteName: UUID().uuidString)!
  }

  private func makeSettingsViewModel(
    defaults: UserDefaults,
    appearanceStore: AppearanceStore
  ) -> SettingsViewModel {
    SettingsViewModel(
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      appVersionProvider: SpyAppVersionProvider(),
      notificationService: SpyNotificationService(),
      defaults: defaults,
      appearanceStore: appearanceStore
    )
  }

  @Test func appearanceMode_defaultsToStoreValue() {
    let store = AppearanceStore(defaults: makeDefaults())
    let sut = WelcomeViewModel(appearanceStore: store)

    #expect(sut.appearanceMode == .system)
  }

  @Test func selectAppearanceMode_updatesExposedAppearanceMode() {
    let store = AppearanceStore(defaults: makeDefaults())
    let sut = WelcomeViewModel(appearanceStore: store)

    sut.selectAppearanceMode(.dark)

    #expect(sut.appearanceMode == .dark)
  }

  @Test func selectAppearanceMode_persistsRawValueToUserDefaults() {
    let defaults = makeDefaults()
    let store = AppearanceStore(defaults: defaults)
    let sut = WelcomeViewModel(appearanceStore: store)

    sut.selectAppearanceMode(.oledDark)

    #expect(defaults.string(forKey: AppearanceStore.appearanceModeKey) == "oledDark")
  }

  @Test func selectAppearanceMode_updatesTheSharedStoreDirectly() {
    let store = AppearanceStore(defaults: makeDefaults())
    let sut = WelcomeViewModel(appearanceStore: store)

    sut.selectAppearanceMode(.light)

    #expect(store.appearanceMode == .light)
  }

  // MARK: - One source of truth with Settings (no divergence)

  @Test func selectAppearanceMode_isReflectedBySettingsViewModel_sharingSameStore() {
    let defaults = makeDefaults()
    let store = AppearanceStore(defaults: defaults)
    let welcomeVM = WelcomeViewModel(appearanceStore: store)
    let settingsVM = makeSettingsViewModel(defaults: defaults, appearanceStore: store)

    welcomeVM.selectAppearanceMode(.oledDark)

    #expect(settingsVM.appearanceMode == .oledDark)
  }

  @Test func settingsViewModelChange_isReflectedByWelcomeViewModel_sharingSameStore() {
    let defaults = makeDefaults()
    let store = AppearanceStore(defaults: defaults)
    let welcomeVM = WelcomeViewModel(appearanceStore: store)
    let settingsVM = makeSettingsViewModel(defaults: defaults, appearanceStore: store)

    settingsVM.appearanceMode = .dark

    #expect(welcomeVM.appearanceMode == .dark)
  }

  // MARK: - Live re-render when the store changes from elsewhere (defect repro)

  /// GH#878 follow-up: live-simulator verification found reopening the
  /// welcome Menu after picking Dark/Light still showed "System" checked —
  /// the underlying store value was correct, but `WelcomeViewModel` never
  /// forwarded the store's change into its own `objectWillChange`, so
  /// SwiftUI never rebuilt the Menu's Picker. `appearanceMode` being a live
  /// read-through (already covered above) is necessary but not sufficient —
  /// this test targets the missing notification specifically, by mutating
  /// the store WITHOUT going through `sut.selectAppearanceMode(_:)`.
  @Test func externalStoreMutation_notifiesWelcomeViewModelObjectWillChange() {
    let store = AppearanceStore(defaults: makeDefaults())
    let sut = WelcomeViewModel(appearanceStore: store)
    var notified = false
    let subscription = sut.objectWillChange.sink { _ in notified = true }

    withExtendedLifetime(subscription) {
      store.appearanceMode = .dark
    }

    #expect(notified)
  }

  // MARK: - Existing WelcomeViewModel callback behaviour is untouched

  @Test func missingAppearanceStore_stillConstructsAValidViewModel() {
    // No injected store — falls back to `AppearanceStore()` (real
    // `UserDefaults.standard`), mirroring `SettingsViewModel`'s own
    // `.standard` fallback. Deliberately does not assert a specific mode:
    // `.standard` is shared, real device/host state, not a value this test
    // owns (see `AppearanceStoreTests` for the isolated-defaults coverage of
    // the actual `.system` fallback behaviour).
    let sut = WelcomeViewModel()

    _ = sut.appearanceMode
  }
}
