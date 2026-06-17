import SwiftUI

/// A compact, tappable capsule chip used in horizontal filter rows.
///
/// Purely presentational: it renders a label in a capsule whose fill, text
/// colour, and border reflect the `isSelected` flag, and forwards taps to
/// `onTap`. The caller owns all selection logic — `FilterChipView` never
/// inspects a view model — so the same chip can express different selection
/// rules at each call site (e.g. the application list ANDs in an
/// "unread filter inactive" guard) without changing its appearance.
///
/// Follows the design language: `tcCaptionEmphasis` typography, `tcAmber`
/// for the selected fill, `tcSurface` for the unselected fill, and a
/// `tcBorder` outline that hides when selected.
///
/// Usage:
/// ```swift
/// FilterChipView(
///     label: "Pending",
///     isSelected: viewModel.selectedStatusFilter == .undecided
/// ) {
///     viewModel.selectedStatusFilter = .undecided
/// }
/// ```
public struct FilterChipView: View {
  private let label: String
  private let isSelected: Bool
  let onTap: () -> Void

  public init(
    label: String,
    isSelected: Bool,
    onTap: @escaping () -> Void
  ) {
    self.label = label
    self.isSelected = isSelected
    self.onTap = onTap
  }

  public var body: some View {
    Text(label)
      .font(TCTypography.captionEmphasis)
      .foregroundStyle(isSelected ? Color.tcTextOnAccent : Color.tcTextPrimary)
      .padding(.horizontal, TCSpacing.small)
      .padding(.vertical, TCSpacing.extraSmall)
      .background(isSelected ? Color.tcAmber : Color.tcSurface)
      .clipShape(Capsule())
      .overlay(
        Capsule()
          .stroke(Color.tcBorder, lineWidth: isSelected ? 0 : 1)
      )
      .contentShape(Capsule())
      .onTapGesture {
        onTap()
      }
  }
}
