import SwiftUI

/// A compact badge indicating that a feature requires a subscription upgrade.
///
/// Amber outline only, never filled: filled amber is reserved for the paywall CTA button.
///
/// Usage:
/// ```swift
/// if viewModel.showUpgradeBadge {
///     UpgradeBadgeView()
/// }
/// ```
public struct UpgradeBadgeView: View {
  private let label: String

  public init(label: String = "Upgrade") {
    self.label = label
  }

  public var body: some View {
    HStack(spacing: TCSpacing.extraSmall) {
      Image(systemName: "arrow.up.circle.fill")
      Text(label)
        .textCase(.uppercase)
        .kerning(0.6)
    }
    .font(TCTypography.captionEmphasis)
    .foregroundStyle(Color.tcAmber)
    .padding(.horizontal, TCSpacing.small)
    .padding(.vertical, TCSpacing.extraSmall)
    .overlay(
      RoundedRectangle(cornerRadius: TCCornerRadius.small)
        .stroke(Color.tcAmber, lineWidth: 1.5)
    )
  }
}
