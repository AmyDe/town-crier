import SwiftUI

/// Primary call-to-action button following the Town Crier design language.
///
/// Applies `borderedProminent` style with `tcAmber` tint, `tcTextOnAccent` foreground,
/// `bodyEmphasis` font, full-width layout, 44pt minimum height, and medium corner radius.
///
/// Usage:
/// ```swift
/// // Simple text label
/// PrimaryButton("Subscribe") {
///     await viewModel.subscribe()
/// }
///
/// // Custom label with icon
/// PrimaryButton {
///     viewModel.openPortal()
/// } label: {
///     HStack {
///         Image(systemName: "safari")
///         Text("View on Council Portal")
///     }
/// }
/// ```
public struct PrimaryButton<Label: View>: View {
  private let action: () -> Void
  private let label: Label

  public init(action: @escaping () -> Void, @ViewBuilder label: () -> Label) {
    self.action = action
    self.label = label()
  }

  public var body: some View {
    Button(action: action) {
      label
        .font(TCTypography.bodyEmphasis)
        .frame(maxWidth: .infinity)
        .frame(height: 44)
    }
    .buttonStyle(.borderedProminent)
    .tint(Color.tcAmber)
    .foregroundStyle(Color.tcTextOnAccent)
    .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
  }
}

extension PrimaryButton where Label == Text {
  /// Convenience initializer for simple text-only buttons.
  public init(_ title: String, action: @escaping () -> Void) {
    self.action = action
    self.label = Text(title)
  }
}
