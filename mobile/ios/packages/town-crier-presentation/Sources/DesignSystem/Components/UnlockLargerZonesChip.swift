import SwiftUI

/// In-context upsell affordance surfaced beneath the radius slider when the
/// user's tier caps their watch-zone radius below the 10 km maximum (tc-w3cb.3).
///
/// This is the "safe fallback" interaction from the design: a capped slider plus
/// an explicit tappable chip, rather than a drag-past-the-cap gesture. Tapping it
/// opens the subscription paywall; on a successful upgrade the radius range
/// unlocks live. The richer "locked region on the track" gesture is an open
/// question to be prototyped in TestFlight before release.
///
/// Uses the brand amber at 15% opacity so it reads as an invitation, not an
/// error — mirrors ``LargeRadiusWarningView``.
public struct UnlockLargerZonesChip: View {
  private let action: () -> Void

  public init(action: @escaping () -> Void) {
    self.action = action
  }

  public var body: some View {
    Button(action: action) {
      HStack(spacing: TCSpacing.small) {
        Image(systemName: "lock.fill")
          .font(.system(.caption))
          .foregroundStyle(Color.tcAmber)
          .accessibilityHidden(true)

        Text("Unlock zones up to 10 km")
          .font(TCTypography.bodyEmphasis)
          .foregroundStyle(Color.tcTextPrimary)

        Spacer(minLength: 0)

        Image(systemName: "chevron.right")
          .font(.system(.caption))
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
    .accessibilityLabel("Unlock larger watch zones, up to 10 kilometres")
    .accessibilityHint("Opens subscription plans")
  }
}
