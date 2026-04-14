import SwiftUI
import Testing

@testable import TownCrierPresentation

@Suite("Color.themed")
struct ColorThemedTests {

  // MARK: - resolveHex helper

  @Test func resolveHex_lightMode_noOled_returnsLightHex() {
    let result = ThemeColorResolver.resolveHex(
      light: 0xFAF8F5,
      dark: 0x1A1A1E,
      oled: 0x000000,
      isDarkMode: false,
      isOledEnabled: false
    )

    #expect(result == 0xFAF8F5)
  }

  @Test func resolveHex_darkMode_noOled_returnsDarkHex() {
    let result = ThemeColorResolver.resolveHex(
      light: 0xFAF8F5,
      dark: 0x1A1A1E,
      oled: 0x000000,
      isDarkMode: true,
      isOledEnabled: false
    )

    #expect(result == 0x1A1A1E)
  }

  @Test func resolveHex_darkMode_oledEnabled_returnsOledHex() {
    let result = ThemeColorResolver.resolveHex(
      light: 0xFAF8F5,
      dark: 0x1A1A1E,
      oled: 0x000000,
      isDarkMode: true,
      isOledEnabled: true
    )

    #expect(result == 0x000000)
  }

  @Test func resolveHex_lightMode_oledEnabled_returnsLightHex() {
    // OLED toggle is meaningless in light mode — should return light
    let result = ThemeColorResolver.resolveHex(
      light: 0xFAF8F5,
      dark: 0x1A1A1E,
      oled: 0x000000,
      isDarkMode: false,
      isOledEnabled: true
    )

    #expect(result == 0xFAF8F5)
  }
}
