import Foundation

/// Single live source of truth for the user's appearance preference
/// (System/Light/Dark/OLED Dark), persisted to UserDefaults under the
/// `"appearanceMode"` key.
///
/// Share one instance (owned at the composition root): a second object writing
/// the same UserDefaults key would not live-update the published scheme until
/// relaunch.
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
