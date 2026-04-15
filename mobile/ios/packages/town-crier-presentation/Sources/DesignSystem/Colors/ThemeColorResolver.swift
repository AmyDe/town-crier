import Foundation

/// Resolves which hex value to use based on the current appearance mode.
///
/// This is extracted as a pure function so it can be unit tested without
/// UIKit trait collection dependencies.
public enum ThemeColorResolver {
  /// The UserDefaults key for the OLED dark mode preference.
  /// Matches the key used by `AppearanceMode.oledDark` in `SettingsViewModel`.
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

  /// Returns `true` when OLED dark mode is active — i.e. the user has
  /// selected the `.oledDark` appearance mode.
  static func isOledEnabled(defaults: UserDefaults = .standard) -> Bool {
    let raw = defaults.string(forKey: appearanceModeKey) ?? ""
    return AppearanceMode(rawValue: raw) == .oledDark
  }
}
