import SwiftUI

// The system's disabled prominent button is unreadable on a light form row.
struct WebhookSaveButtonStyle: ButtonStyle {
  @Environment(\.isEnabled) private var isEnabled

  func makeBody(configuration: Configuration) -> some View {
    configuration.label
      .font(TCTypography.bodyEmphasis)
      .foregroundStyle(isEnabled ? Color.tcTextOnAccent : Color.tcTextSecondary)
      .frame(maxWidth: .infinity, minHeight: 44)
      .background(
        isEnabled ? Color.tcAmber : Color.tcAmberMuted,
        in: Capsule()
      )
      .opacity(configuration.isPressed ? 0.8 : 1)
  }
}
