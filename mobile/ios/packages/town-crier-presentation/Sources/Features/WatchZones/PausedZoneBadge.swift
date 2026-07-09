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
/// Follows the design language's status-badge shape (capsule, paired icon,
/// 15% opacity background) using `tcStatusWithdrawn` — the same "no longer
/// active" grey used for withdrawn applications — since a paused zone is, in
/// the same sense, currently dormant. The zone itself is never deleted,
/// edited, or hidden while paused; only new notifications stop.
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
        Image(systemName: "pause.circle.fill")
        Text(Copy.label)
        Image(systemName: "arrow.up.circle.fill")
          .foregroundStyle(Color.tcAmber)
      }
      .font(TCTypography.captionEmphasis)
      .foregroundStyle(Color.tcStatusWithdrawn)
      .padding(.horizontal, TCSpacing.small)
      .padding(.vertical, TCSpacing.extraSmall)
      .background(Color.tcStatusWithdrawn.opacity(0.15))
      .clipShape(Capsule())
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
