import Foundation

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
}
