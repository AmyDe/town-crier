import SwiftUI

extension View {
  /// Adds a Settings entry-point gear icon to the trailing edge of the
  /// navigation bar. Call once on every tab so users always have a uniform
  /// way back to Settings.
  ///
  /// Centralized here so the gear-icon styling, accessibility label, and tap
  /// affordance stay consistent across tabs. Tapping the icon invokes
  /// `action` — typically `coordinator.showSettings()`.
  ///
  /// Usage:
  /// ```swift
  /// NavigationStack { ... }
  ///   .settingsToolbar { coordinator.showSettings() }
  /// ```
  public func settingsToolbar(action: @escaping () -> Void) -> some View {
    toolbar {
      ToolbarItem(placement: settingsToolbarPlacement) {
        Button(action: action) {
          Image(systemName: "gearshape")
            .foregroundStyle(Color.tcTextPrimary)
        }
        .accessibilityLabel("Settings")
      }
    }
  }

  private var settingsToolbarPlacement: ToolbarItemPlacement {
    #if os(iOS)
      return .topBarTrailing
    #else
      return .automatic
    #endif
  }
}
