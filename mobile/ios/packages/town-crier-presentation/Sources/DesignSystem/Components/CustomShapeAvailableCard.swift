import SwiftUI

/// In-context callout shown on the onboarding radius step once the user's
/// tier already allows drawing a custom-shape (polygon) zone (GH#1031,
/// tc-7se1w.4). Replaces a plain "Draw a custom shape instead" text link
/// that carried no explanation of what the capability actually does — a
/// paying user deserves the same clear introduction to the feature that
/// ``CustomShapeUpsellCard`` gives a Free-tier user pre-purchase, just
/// without the paywall framing. Shares that card's layout (the same
/// ``CustomShapeUpsellGraphic``, title, and caption) so the two read as one
/// consistent component with different copy for different entitlement
/// states, not two unrelated pieces of UI.
public struct CustomShapeAvailableCard: View {
  /// User-facing copy, kept in one place so it can be unit-tested and reused.
  enum Copy {
    static let title = "Draw a custom shape"
    static let benefits = "Trace the exact area you want to watch, instead of a circle."
    static let accessibilityLabel = "Draw a custom shape. \(benefits)"
    static let accessibilityHint = "Opens the shape drawing tool"
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
