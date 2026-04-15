import SwiftUI

/// User's preferred appearance mode, persisted via `@AppStorage`.
public enum AppearanceMode: String, CaseIterable, Sendable {
  case system
  case light
  case dark
  case oledDark

  /// Human-readable label for display in Settings.
  public var displayName: String {
    switch self {
    case .system:
      "System"
    case .light:
      "Light"
    case .dark:
      "Dark"
    case .oledDark:
      "OLED Dark"
    }
  }

  /// The `ColorScheme` override to apply via `.preferredColorScheme()`.
  /// Returns `nil` for `.system` to follow the device setting.
  public var preferredColorScheme: ColorScheme? {
    switch self {
    case .system:
      nil
    case .light:
      .light
    case .dark, .oledDark:
      .dark
    }
  }
}
