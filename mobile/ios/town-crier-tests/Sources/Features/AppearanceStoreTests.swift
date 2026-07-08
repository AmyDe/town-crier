import Foundation
import Testing

@testable import TownCrierPresentation

/// GH#878: single live source of truth for the app's appearance preference,
/// shared between the Settings picker and the anonymous welcome screen's
/// appearance control. Persists to the same `"appearanceMode"` UserDefaults
/// key `SettingsViewModel` has always used (mirrors
/// `SettingsAppearanceModeTests`' persistence-test conventions).
@Suite("AppearanceStore")
@MainActor
struct AppearanceStoreTests {
  private func makeDefaults() -> UserDefaults {
    // swiftlint:disable:next force_unwrapping
    UserDefaults(suiteName: UUID().uuidString)!
  }

  @Test func appearanceMode_defaultsToSystem_whenNothingStored() {
    let sut = AppearanceStore(defaults: makeDefaults())

    #expect(sut.appearanceMode == .system)
  }

  @Test func appearanceMode_defaultsToSystem_forUnknownStoredRawValue() {
    let defaults = makeDefaults()
    defaults.set("not-a-real-mode", forKey: AppearanceStore.appearanceModeKey)

    let sut = AppearanceStore(defaults: defaults)

    #expect(sut.appearanceMode == .system)
  }

  @Test func settingAppearanceMode_persistsRawValueToUserDefaults() {
    let defaults = makeDefaults()
    let sut = AppearanceStore(defaults: defaults)

    sut.appearanceMode = .oledDark

    #expect(defaults.string(forKey: AppearanceStore.appearanceModeKey) == "oledDark")
  }

  @Test func freshInstance_roundTripsPersistedValue() {
    let defaults = makeDefaults()
    let first = AppearanceStore(defaults: defaults)
    first.appearanceMode = .dark

    let second = AppearanceStore(defaults: defaults)

    #expect(second.appearanceMode == .dark)
  }
}
