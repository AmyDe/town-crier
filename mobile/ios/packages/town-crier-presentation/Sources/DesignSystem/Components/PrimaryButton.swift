import SwiftUI

/// Primary action button following the Town Crier design language.
///
/// Renders a full-width, amber-tinted bordered-prominent button with
/// `tcTextOnAccent` foreground, `bodyEmphasis` font, 44pt minimum height,
/// and medium corner radius. Supports an optional loading state that
/// replaces the label with a progress spinner.
///
/// Usage with a title string:
/// ```swift
/// PrimaryButton("Subscribe") {
///     await viewModel.subscribe()
/// }
/// ```
///
/// Usage with a custom label:
/// ```swift
/// PrimaryButton(action: { showSafari = true }) {
///     HStack {
///         Image(systemName: "safari")
///         Text("View on Council Portal")
///     }
/// }
/// ```
public struct PrimaryButton<Label: View>: View {
    private let action: () -> Void
    private let isLoading: Bool
    private let isDisabled: Bool
    private let label: () -> Label

    public init(
        isLoading: Bool = false,
        isDisabled: Bool = false,
        action: @escaping () -> Void,
        label: @escaping () -> Label
    ) {
        self.action = action
        self.isLoading = isLoading
        self.isDisabled = isDisabled
        self.label = label
    }

    public var body: some View {
        Button(action: action) {
            Group {
                if isLoading {
                    ProgressView()
                        .tint(Color.tcTextOnAccent)
                } else {
                    label()
                        .font(TCTypography.bodyEmphasis)
                }
            }
            .frame(maxWidth: .infinity)
            .frame(minHeight: 44)
        }
        .buttonStyle(.borderedProminent)
        .tint(Color.tcAmber)
        .foregroundStyle(Color.tcTextOnAccent)
        .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
        .disabled(isLoading || isDisabled)
    }
}

// MARK: - Convenience initializer for text-only labels

extension PrimaryButton where Label == Text {
    public init(
        _ title: String,
        isLoading: Bool = false,
        isDisabled: Bool = false,
        action: @escaping () -> Void
    ) {
        self.action = action
        self.isLoading = isLoading
        self.isDisabled = isDisabled
        self.label = { Text(title) }
    }
}
