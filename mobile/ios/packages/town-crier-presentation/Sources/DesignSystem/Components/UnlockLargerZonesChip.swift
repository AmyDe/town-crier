import SwiftUI

/// In-context upsell affordance surfaced beneath the radius slider when the
/// user's tier caps their watch zones below the full set of paid benefits
/// (tc-w3cb.3, tc-42gc).
///
/// This is the "safe fallback" interaction from the design: a capped slider plus
/// an explicit tappable chip, rather than a drag-past-the-cap gesture. Tapping it
/// opens the subscription paywall; on a successful upgrade the radius range
/// unlocks live. The richer "locked region on the track" gesture is an open
/// question to be prototyped in TestFlight before release.
///
/// The copy sells the whole upgrade, not just a bigger radius: bigger zones, more
/// than one zone, and instant alerts by push and email (the free tier gets a
/// weekly digest only). Uses the brand amber at 15% opacity so it reads as an
/// invitation, not an error — mirrors ``LargeRadiusWarningView``.
public struct UnlockLargerZonesChip: View {
  /// User-facing copy, kept in one place so it can be unit-tested and reused.
  enum Copy {
    static let title = "Do more with a plan"
    static let benefits =
      "Bigger watch zones up to 10 km, more than one zone, and instant alerts by "
      + "push and email. Free gives you a weekly digest."
    static let accessibilityLabel =
      "Do more with a plan. Bigger watch zones up to 10 kilometres, more than one "
      + "watch zone, and instant alerts by push and email. Free gives you a weekly digest."
    static let accessibilityHint = "Opens subscription plans"
  }

  private let action: () -> Void

  public init(action: @escaping () -> Void) {
    self.action = action
  }

  public var body: some View {
    Button(action: action) {
      HStack(alignment: .top, spacing: TCSpacing.small) {
        Image(systemName: "lock.fill")
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcAmber)
          .accessibilityHidden(true)

        VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
          Text(Copy.title)
            .font(TCTypography.bodyEmphasis)
            .foregroundStyle(Color.tcTextPrimary)

          Text(Copy.benefits)
            .font(TCTypography.caption)
            .foregroundStyle(Color.tcTextSecondary)
            .fixedSize(horizontal: false, vertical: true)
        }

        Spacer(minLength: 0)

        Image(systemName: "chevron.right")
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcTextSecondary)
          .accessibilityHidden(true)
      }
      .padding(.horizontal, TCSpacing.medium)
      .padding(.vertical, TCSpacing.small)
      .frame(maxWidth: .infinity, alignment: .leading)
      .frame(minHeight: 44)
      .background(Color.tcAmberMuted)
      .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.small))
    }
    .buttonStyle(.plain)
    .accessibilityLabel(Copy.accessibilityLabel)
    .accessibilityHint(Copy.accessibilityHint)
  }
}
