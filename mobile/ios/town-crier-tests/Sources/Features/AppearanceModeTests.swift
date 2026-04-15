import SwiftUI
import Testing

@testable import TownCrierPresentation

@Suite("AppearanceMode")
struct AppearanceModeTests {

  @Test func system_isDefaultRawValue() {
    let mode = AppearanceMode(rawValue: "system")

    #expect(mode == .system)
  }

  @Test func allCases_haveExpectedRawValues() {
    #expect(AppearanceMode.system.rawValue == "system")
    #expect(AppearanceMode.light.rawValue == "light")
    #expect(AppearanceMode.dark.rawValue == "dark")
    #expect(AppearanceMode.oledDark.rawValue == "oledDark")
  }

  @Test func allCases_hasAllFourModes() {
    #expect(AppearanceMode.allCases.count == 4)
  }

  @Test func displayName_returnsHumanReadableLabel() {
    #expect(AppearanceMode.system.displayName == "System")
    #expect(AppearanceMode.light.displayName == "Light")
    #expect(AppearanceMode.dark.displayName == "Dark")
    #expect(AppearanceMode.oledDark.displayName == "OLED Dark")
  }

  // MARK: - preferredColorScheme

  @Test func preferredColorScheme_system_returnsNil() {
    #expect(AppearanceMode.system.preferredColorScheme == nil)
  }

  @Test func preferredColorScheme_light_returnsLight() {
    #expect(AppearanceMode.light.preferredColorScheme == .light)
  }

  @Test func preferredColorScheme_dark_returnsDark() {
    #expect(AppearanceMode.dark.preferredColorScheme == .dark)
  }

  @Test func preferredColorScheme_oledDark_returnsDark() {
    // OLED dark is a sub-mode of dark — the system color scheme is dark
    #expect(AppearanceMode.oledDark.preferredColorScheme == .dark)
  }
}
