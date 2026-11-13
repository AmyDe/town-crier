import Foundation

/// Resolves which hex value to use based on the current appearance mode.
public enum ThemeColorResolver {
  static let appearanceModeKey = "appearanceMode"

  /// Resolves the correct hex value for the given theme state.
  public static func resolveHex(
    light: UInt32,
    dark: UInt32,
    oled: UInt32,
    isDarkMode: Bool,
    isOledEnabled: Bool
  ) -> UInt32 {
    guard isDarkMode else {
      return light
    }
    return isOledEnabled ? oled : dark
  }

  static func isOledEnabled(defaults: UserDefaults = .standard) -> Bool {
    let raw = defaults.string(forKey: appearanceModeKey) ?? ""
    return AppearanceMode(rawValue: raw) == .oledDark
  }
}
