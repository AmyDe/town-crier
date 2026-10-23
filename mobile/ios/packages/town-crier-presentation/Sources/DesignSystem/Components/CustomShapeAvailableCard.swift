import SwiftUI

/// In-context callout shown on the onboarding radius step once the user's
/// tier already allows drawing a custom-shape (polygon) zone. Shares
/// ``CustomShapeUpsellCard``'s layout (the same ``CustomShapeUpsellGraphic``,
/// title, and caption) with copy for a paying user, without the paywall
/// framing.
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
