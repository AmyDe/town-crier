import SwiftUI

extension Color {
    /// Creates a theme-aware color from hex values for light, dark, and OLED dark modes.
    ///
    /// The OLED dark variant activates when the system is in dark mode and
    /// the user has enabled "True Black" via `@AppStorage("oledDarkEnabled")`.
    static func themed(light: UInt32, dark: UInt32, oled: UInt32) -> Color {
        // For the scaffolding phase, return a static color based on the light hex.
        // Full theme resolution (including OLED toggle) will be wired once
        // the settings infrastructure is in place.
        Color(hex: light)
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
