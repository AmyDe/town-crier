import SwiftUI
import TownCrierDomain

/// A compact "stamp" badge showing the icon, label, and color for a planning
/// application status.
///
/// Public Notice (GH#857): status indicators read as an official stamp
/// rather than a filled pill — uppercase kerned text, a 1.5pt outline in
/// `status.displayColor`, and no fill. The SF Symbol icon is always paired
/// with the text label (never colour alone) for colour-blind accessibility.
struct StatusBadgeView: View {
  let status: ApplicationStatus

  var body: some View {
    HStack(spacing: TCSpacing.extraSmall) {
      Image(systemName: status.displayIcon)
      Text(status.displayLabel)
        .textCase(.uppercase)
        .kerning(0.6)
    }
    .font(TCTypography.captionEmphasis)
    .foregroundStyle(status.displayColor)
    .padding(.horizontal, TCSpacing.small)
    .padding(.vertical, TCSpacing.extraSmall)
    .overlay(
      RoundedRectangle(cornerRadius: TCCornerRadius.small)
        .stroke(status.displayColor, lineWidth: 1.5)
    )
  }
}
