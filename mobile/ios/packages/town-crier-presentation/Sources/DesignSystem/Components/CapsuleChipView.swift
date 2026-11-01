import SwiftUI

/// A compact, tappable capsule chip used in horizontal chip rows.
///
/// Purely presentational: fill, text colour and border reflect `isSelected`,
/// taps forward to `onTap`, and the caller owns all selection logic.
///
/// Usage:
/// ```swift
/// CapsuleChipView(
///     label: "Pending",
///     isSelected: viewModel.selectedStatusFilter == .undecided
/// ) {
///     viewModel.selectedStatusFilter = .undecided
/// }
/// ```
public struct CapsuleChipView: View {
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
