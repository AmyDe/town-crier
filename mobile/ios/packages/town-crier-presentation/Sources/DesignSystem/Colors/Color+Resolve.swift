import SwiftUI
#if canImport(UIKit)
  import UIKit
#endif

extension Color {
  /// Creates a theme-aware color from hex values for light, dark, and OLED dark modes.
  ///
  /// Uses `UIColor(dynamicProvider:)` so the color resolves at render time
  /// based on the current `UITraitCollection` and the user's OLED preference
  /// stored in `UserDefaults`.
  static func themed(light: UInt32, dark: UInt32, oled: UInt32) -> Color {
    #if canImport(UIKit)
      Color(uiColor: UIColor { traitCollection in
        let isDark = traitCollection.userInterfaceStyle == .dark
        let hex = ThemeColorResolver.resolveHex(
          light: light,
          dark: dark,
          oled: oled,
          isDarkMode: isDark,
          isOledEnabled: isDark && ThemeColorResolver.isOledEnabled()
        )
        return UIColor(
          red: CGFloat((hex >> 16) & 0xFF) / 255.0,
          green: CGFloat((hex >> 8) & 0xFF) / 255.0,
          blue: CGFloat(hex & 0xFF) / 255.0,
          alpha: 1.0
        )
      })
    #else
      Color(hex: light)
    #endif
  }
}

extension Color {
  /// Initializes a Color from a hex value (e.g., 0xD4910A).
  init(hex: UInt32) {
    let red = Double((hex >> 16) & 0xFF) / 255.0
    let green = Double((hex >> 8) & 0xFF) / 255.0
    let blue = Double(hex & 0xFF) / 255.0
    self.init(red: red, green: green, blue: blue)
  }
}
