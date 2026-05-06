import SwiftUI
import TownCrierDomain

/// A capsule pill rendering a planning application's status.
///
/// The pill follows the design language's status-badge specification (capsule
/// shape, status colour at 15% opacity background, paired SF Symbol icon for
/// colour-blind accessibility) and delegates all label / icon / colour
/// decisions to ``ApplicationStatus`` display extensions so there is a single
/// source of truth for the UK planning vocabulary.
///
/// Read/unread state is signalled by the leading accent dot on
/// ``ApplicationListRow``, not by mutating the pill's saturation. Keeping the
/// pill stateless makes its rendering deterministic from `status` alone.
struct ApplicationStatusPill: View {
  let status: ApplicationStatus

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
    .foregroundStyle(status.displayColor)
    .padding(.horizontal, TCSpacing.small)
    .padding(.vertical, TCSpacing.extraSmall)
    .background(status.displayColor.opacity(0.15))
    .clipShape(Capsule())
    .accessibilityElement(children: .combine)
    .accessibilityLabel("Status: \(label)")
  }
}
