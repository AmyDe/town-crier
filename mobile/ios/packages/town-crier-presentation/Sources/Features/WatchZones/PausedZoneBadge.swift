import SwiftUI

/// A compact "Paused" indicator shown on a watch zone row that currently
/// exceeds the user's tier quota and has stopped generating notifications
/// (GH#889 P1/P2 — a subscription downgrade left the user over quota; oldest
/// zones stay active, newest are paused).
///
/// The badge doubles as its own upgrade affordance: tapping it opens the
/// subscription paywall via the host's `onUpgrade` closure, which
/// ``WatchZoneListView`` wires to ``WatchZoneListViewModel/viewPlans()`` —
/// the same routing method used by the toolbar upgrade badge and the
/// free-tier inline upsell card, so every "upgrade" entry point on the Watch
/// Zones screen converges on one paywall presentation path.
///
/// Follows the design language's stamp treatment (GH#857): uppercase kerned
/// label, 1.5pt outline, no fill, paired icon — using `tcStatusWithdrawn`,
/// the same "no longer active" grey used for withdrawn applications, since a
/// paused zone is, in the same sense, currently dormant. The zone itself is
/// never deleted, edited, or hidden while paused; only new notifications
/// stop. The amber "upgrade" glyph a paused stamp previously carried is
/// dropped deliberately (amber-rationing rule) — upgrade emphasis belongs on
/// the paywall screen this badge opens, not inside the stamp itself.
struct PausedZoneBadge: View {
  /// User-facing copy, kept in one place so it can be unit-tested directly.
  enum Copy {
    static let label = "Paused"
    static let accessibilityLabel =
      "Paused. This zone exceeds your plan's limit and won't generate new notifications."
    static let accessibilityHint = "Opens subscription plans"
  }

  private let onUpgrade: () -> Void

  init(onUpgrade: @escaping () -> Void) {
    self.onUpgrade = onUpgrade
  }

  var body: some View {
    Button(action: onUpgrade) {
      HStack(spacing: TCSpacing.extraSmall) {
        Image(systemName: "pause.circle")
        Text(Copy.label)
          .textCase(.uppercase)
          .kerning(0.6)
      }
      .font(TCTypography.captionEmphasis)
      .foregroundStyle(Color.tcStatusWithdrawn)
      .padding(.horizontal, TCSpacing.small)
      .padding(.vertical, TCSpacing.extraSmall)
      .overlay(
        RoundedRectangle(cornerRadius: TCCornerRadius.small)
          .stroke(Color.tcStatusWithdrawn, lineWidth: 1.5)
      )
    }
    .buttonStyle(.plain)
    .accessibilityLabel(Copy.accessibilityLabel)
    .accessibilityHint(Copy.accessibilityHint)
  }

  // MARK: - Test Helpers

  /// Simulates tapping the badge for unit testing.
  func simulateUpgradeTap() {
    onUpgrade()
  }
}
