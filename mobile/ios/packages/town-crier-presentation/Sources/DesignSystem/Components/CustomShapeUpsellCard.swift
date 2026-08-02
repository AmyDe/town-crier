import SwiftUI

/// In-context upsell shown on the onboarding radius step for a Free-tier
/// user, explaining that a paid tier can draw a custom-shape zone instead of
/// a circle (GH#1031, tc-6he3x.10). Wraps ``CustomShapeUpsellGraphic`` with
/// title copy and a tappable action, mirroring ``UnlockLargerZonesChip``'s
/// "safe fallback" pattern (tap to open the paywall) but for the
/// custom-shape entitlement rather than a larger radius — the two upsells
/// are distinct and may both show together for a Free-tier user.
public struct CustomShapeUpsellCard: View {
  /// User-facing copy, kept in one place so it can be unit-tested and reused.
  enum Copy {
    static let title = "Draw your own shape"
    static let benefits =
      "Free gives you a circle. Personal and Pro let you draw the exact area you care about."
    static let accessibilityLabel =
      "Draw your own shape. Free gives you a circle. Personal and Pro let you draw "
      + "the exact area you care about."
    static let accessibilityHint = "Opens subscription plans"
  }

  private let action: () -> Void

  public init(action: @escaping () -> Void) {
    self.action = action
  }

  public var body: some View {
    Button(action: action) {
      VStack(spacing: TCSpacing.small) {
        CustomShapeUpsellGraphic()

        Text(Copy.title)
          .font(TCTypography.bodyEmphasis)
          .foregroundStyle(Color.tcTextPrimary)

        Text(Copy.benefits)
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcTextSecondary)
          .multilineTextAlignment(.center)
          .fixedSize(horizontal: false, vertical: true)
      }
      .padding(TCSpacing.medium)
      .frame(maxWidth: .infinity)
      .background(Color.tcAmberMuted)
      .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
    }
    .buttonStyle(.plain)
    .accessibilityLabel(Copy.accessibilityLabel)
    .accessibilityHint(Copy.accessibilityHint)
  }
}
