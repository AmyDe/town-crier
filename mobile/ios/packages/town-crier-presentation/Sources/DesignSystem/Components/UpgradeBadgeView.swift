import SwiftUI

/// A compact badge indicating that a feature requires a subscription upgrade.
///
/// Public Notice (GH#857): an upsell surface, styled with `tcAmber` — an
/// uppercase kerned label, 1.5pt amber outline, no fill. `tcAmber` is the
/// only accent this badge carries; it is never a filled container (amber
/// rationing rule — filled amber is reserved for the CTA button on the
/// paywall entry points this badge routes to).
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
