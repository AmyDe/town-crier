import SwiftUI
import TownCrierDomain

/// Read/unread state is signalled by the accent dot on ``ApplicationListRow``, not
/// here, so the pill renders from `status` alone.
struct ApplicationStatusPill: View {
  let status: ApplicationStatus

  var label: String { status.displayLabel }

  var iconName: String { status.displayIcon }

  var body: some View {
    HStack(spacing: TCSpacing.extraSmall) {
      Image(systemName: iconName)
      Text(label)
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
    .accessibilityElement(children: .combine)
    .accessibilityLabel("Status: \(label)")
  }
}
