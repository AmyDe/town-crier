import SwiftUI

/// Shared row/section styling helpers for Settings-style `List` screens.
/// Extracted (GH#879 Phase 3) so `SettingsView` (authenticated) and
/// `AnonymousSettingsView` render identical row chrome without duplicating
/// the same private helper methods across both files.
/// `@MainActor` because the CI toolchain treats `Spacer.init(minLength:)` as
/// main-actor-isolated; every call site is a `View.body`, so this adds no
/// constraint in practice.
@MainActor
enum SettingsRowStyling {
  /// A label-value row: primary-styled label on the left, secondary-styled
  /// value on the right.
  static func settingRow(label: String, value: String) -> some View {
    HStack {
      settingLabel(label)
      Spacer()
      settingValue(value)
    }
  }

  /// Body text styled as a setting label (primary foreground).
  static func settingLabel(_ text: String) -> some View {
    Text(text)
      .font(TCTypography.body)
      .foregroundStyle(Color.tcTextPrimary)
  }

  /// Body text styled as a setting value (secondary foreground).
  static func settingValue(_ text: String) -> some View {
    Text(text)
      .font(TCTypography.body)
      .foregroundStyle(Color.tcTextSecondary)
  }

  /// Caption text styled for metadata (secondary foreground).
  static func settingCaption(_ text: String) -> some View {
    Text(text)
      .font(TCTypography.caption)
      .foregroundStyle(Color.tcTextSecondary)
  }

  /// A tappable row with a label, SF Symbol icon, and trailing chevron
  /// disclosure indicator.
  static func navigationRow(
    _ title: String, systemImage: String, action: @escaping () -> Void
  ) -> some View {
    Button(action: action) {
      HStack {
        Label(title, systemImage: systemImage)
          .font(TCTypography.body)
          .foregroundStyle(Color.tcTextPrimary)
        Spacer()
        settingChevron
      }
    }
  }

  /// Trailing chevron indicator for navigation rows.
  static var settingChevron: some View {
    Image(systemName: "chevron.right")
      .font(TCTypography.caption)
      .foregroundStyle(Color.tcTextTertiary)
  }
}
