import SwiftUI
import TownCrierDomain

/// A compact capsule badge showing the icon, label, and color for a planning application status.
///
/// Uses the design system's `tcCaptionEmphasis` typography and the semantic `tcStatus*` colors
/// with a 15% opacity background, following the design language specification for status badges.
struct StatusBadgeView: View {
  let status: ApplicationStatus

  var body: some View {
    HStack(spacing: TCSpacing.extraSmall) {
      Image(systemName: status.displayIcon)
      Text(status.displayLabel)
    }
    .font(TCTypography.captionEmphasis)
    .foregroundStyle(status.displayColor)
    .padding(.horizontal, TCSpacing.small)
    .padding(.vertical, TCSpacing.extraSmall)
    .background(status.displayColor.opacity(0.15))
    .clipShape(Capsule())
  }
}
