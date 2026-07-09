import SwiftUI

/// Richer inline upsell card shown beneath a free-tier user's single watch zone,
/// filling the space they would otherwise see empty once they hit their one-zone
/// limit (tc-t8hc).
///
/// It sells the whole plan, not just one feature: bigger zones, more than one
/// zone, and instant alerts by push and email (the free tier gets a weekly digest
/// only). Tapping "View Plans" opens the subscription paywall via the host's
/// `onViewPlans` closure, which the list wires to
/// ``WatchZoneListViewModel/viewPlans()``.
///
/// Visibility is gated by ``WatchZoneListViewModel/showsFreeTierUpsell`` so paid
/// users (including a Personal user at their finite 3-zone cap) and below-cap free
/// users never see it. Copy is shared verbatim with ``UnlockLargerZonesChip``
/// (tc-42gc) so both surfaces speak in one voice.
///
/// Styling follows the Public Notice upsell-surface language (GH#857): an
/// elevated surface with a 1.5pt amber border and a brass small-caps
/// eyebrow. The "View Plans" CTA button is the only filled-amber container
/// on the card — the card itself stays bordered, never filled (amber
/// rationing rule, same as web).
public struct WatchZoneInlineUpsellCard: View {
  /// User-facing copy, kept in one place so it can be unit-tested and reused.
  enum Copy {
    static let eyebrow = "Upgrade"
    static let title = "Do more with a plan"
    static let biggerZones = "Bigger watch zones, up to 10 km"
    static let moreThanOneZone = "More than one watch zone"
    static let instantAlerts = "Instant alerts by push and email"
    static let freeClarifier = "Free gives you a weekly digest."
    static let viewPlans = "View Plans"
    static let accessibilityLabel =
      "Do more with a plan. Bigger watch zones, up to 10 kilometres. "
      + "More than one watch zone. Instant alerts by push and email. "
      + "Free gives you a weekly digest."
    static let accessibilityHint = "Opens subscription plans"
  }

  private let onViewPlans: () -> Void

  public init(onViewPlans: @escaping () -> Void) {
    self.onViewPlans = onViewPlans
  }

  public var body: some View {
    VStack(alignment: .leading, spacing: TCSpacing.medium) {
      VStack(alignment: .leading, spacing: TCSpacing.small) {
        // Brass small-caps eyebrow (GH#857) — the card's border carries the
        // amber accent; the CTA button below stays the only filled-amber
        // surface (amber-rationing rule).
        Text(Copy.eyebrow)
          .font(TCTypography.captionEmphasis)
          .textCase(.uppercase)
          .kerning(1.2)
          .foregroundStyle(Color.tcAmber)

        Text(Copy.title)
          .font(TCTypography.headline)
          .foregroundStyle(Color.tcTextPrimary)

        VStack(alignment: .leading, spacing: TCSpacing.small) {
          benefitRow(icon: "arrow.up.left.and.arrow.down.right", text: Copy.biggerZones)
          benefitRow(icon: "square.on.square", text: Copy.moreThanOneZone)
          benefitRow(icon: "bell.badge", text: Copy.instantAlerts)
        }

        Text(Copy.freeClarifier)
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcTextSecondary)
      }
      .accessibilityElement(children: .combine)
      .accessibilityLabel(Copy.accessibilityLabel)

      PrimaryButton(Copy.viewPlans, action: onViewPlans)
        .accessibilityHint(Copy.accessibilityHint)
    }
    .padding(TCSpacing.medium)
    .frame(maxWidth: .infinity, alignment: .leading)
    .background(Color.tcSurfaceElevated)
    .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
    .overlay(
      // Amber 1.5pt border (GH#857) — an upsell surface, not a status stamp,
      // but the same no-fill "outline reads as the accent" language.
      RoundedRectangle(cornerRadius: TCCornerRadius.medium)
        .strokeBorder(Color.tcAmber, lineWidth: 1.5)
    )
  }

  private func benefitRow(icon: String, text: String) -> some View {
    HStack(alignment: .firstTextBaseline, spacing: TCSpacing.small) {
      Image(systemName: icon)
        .font(TCTypography.body)
        .foregroundStyle(Color.tcAmber)
        .accessibilityHidden(true)
      Text(text)
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextPrimary)
        .fixedSize(horizontal: false, vertical: true)
    }
  }

  // MARK: - Test Helpers

  /// Simulates tapping "View Plans" for unit testing.
  func simulateViewPlansTap() {
    onViewPlans()
  }
}
