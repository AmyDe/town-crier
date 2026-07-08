import Foundation

/// Single live source of truth for the user's appearance preference
/// (System/Light/Dark/OLED Dark), persisted to UserDefaults under the same
/// `"appearanceMode"` key `SettingsViewModel` has always used (GH#878).
///
/// Owned once at the composition root (`TownCrierApp.swift`) and injected
/// into both `SettingsViewModel` (Settings picker) and the anonymous
/// welcome screen's appearance control (`WelcomeViewModel`), so a change
/// from either surface live-updates the root `.preferredColorScheme`
/// immediately. A second object merely writing the same UserDefaults key
/// would persist correctly but would NOT live-update the already-published
/// scheme until relaunch — this type exists so there is exactly one
/// `@Published` instance for every consumer to share.
@MainActor
public final class AppearanceStore: ObservableObject {
  public static let appearanceModeKey = "appearanceMode"

  @Published public var appearanceMode: AppearanceMode {
    didSet {
      defaults.set(appearanceMode.rawValue, forKey: Self.appearanceModeKey)
    }
  }

  private let defaults: UserDefaults

  public init(defaults: UserDefaults = .standard) {
    self.defaults = defaults
    let storedRaw = defaults.string(forKey: Self.appearanceModeKey) ?? ""
    self.appearanceMode = AppearanceMode(rawValue: storedRaw) ?? .system
  }
}
