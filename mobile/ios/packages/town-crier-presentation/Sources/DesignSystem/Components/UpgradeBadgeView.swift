import SwiftUI

/// A compact capsule badge indicating that a feature requires a subscription upgrade.
///
/// Follows the design language specification for status badges: capsule shape with
/// `tcCaptionEmphasis` typography and a 15% opacity background. Uses `tcAmber` as
/// the accent color to match the brand's upgrade/upsell pattern.
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
    }
    .font(TCTypography.captionEmphasis)
    .foregroundStyle(Color.tcAmber)
    .padding(.horizontal, TCSpacing.small)
    .padding(.vertical, TCSpacing.extraSmall)
    .background(Color.tcAmber.opacity(0.15))
    .clipShape(Capsule())
  }
}
