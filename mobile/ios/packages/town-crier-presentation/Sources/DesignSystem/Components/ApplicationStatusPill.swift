import SwiftUI
import TownCrierDomain

/// A capsule pill rendering a planning application's status with optional
/// muted styling for "already-read" rows.
///
/// The pill follows the design language's status-badge specification (capsule
/// shape, status colour at 15% opacity background, paired SF Symbol icon for
/// colour-blind accessibility) and delegates all label / icon / colour
/// decisions to ``ApplicationStatus`` display extensions so there is a single
/// source of truth for the UK planning vocabulary.
///
/// The `isMuted` knob desaturates the pill to communicate that the row's
/// latest unread event has already been read by the user. When `isMuted` is
/// `true` the pill drops to neutral text colours and a translucent surface
/// background; when `false` the pill renders at full saturation. The
/// `latestUnreadEvent`-driven wiring is added by the Applications-screen
/// unread-UI bead (tc-1nsa.8) — this component just exposes the knob.
struct ApplicationStatusPill: View {
  let status: ApplicationStatus
  let isMuted: Bool

  init(status: ApplicationStatus, isMuted: Bool = false) {
    self.status = status
    self.isMuted = isMuted
  }

  /// The user-facing label rendered in the pill. Mirrors
  /// ``ApplicationStatus.displayLabel`` so tests can assert vocabulary
  /// without scraping the SwiftUI render tree.
  var label: String { status.displayLabel }

  /// The SF Symbol name rendered in the pill. Mirrors
  /// ``ApplicationStatus.displayIcon``.
  var iconName: String { status.displayIcon }

  var body: some View {
    HStack(spacing: TCSpacing.extraSmall) {
      Image(systemName: iconName)
      Text(label)
    }
    .font(TCTypography.captionEmphasis)
    .foregroundStyle(foreground)
    .padding(.horizontal, TCSpacing.small)
    .padding(.vertical, TCSpacing.extraSmall)
    .background(background)
    .clipShape(Capsule())
    .accessibilityElement(children: .combine)
    .accessibilityLabel("Status: \(label)")
  }

  // MARK: - Styling

  private var foreground: Color {
    isMuted ? .tcTextSecondary : status.displayColor
  }

  private var background: Color {
    isMuted ? .tcSurface : status.displayColor.opacity(0.15)
  }
}
