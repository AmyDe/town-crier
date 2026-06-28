import SwiftUI

/// A compact, tappable capsule chip used in horizontal chip rows.
///
/// Shared, neutral component for any single-line capsule selection chip — both
/// the status-filter rows (e.g. "Pending", "Refused") and the watch-zone picker
/// rows (e.g. a zone name) render with it, so the name describes the shape
/// rather than any one call site's meaning.
///
/// Purely presentational: it renders a label in a capsule whose fill, text
/// colour, and border reflect the `isSelected` flag, and forwards taps to
/// `onTap`. The caller owns all selection logic — `CapsuleChipView` never
/// inspects a view model — so each call site can compute selection from its own
/// state (e.g. `viewModel.selectedStatusFilter == .undecided` or
/// `zone.id == viewModel.selectedZone?.id`) and run its own tap action,
/// synchronous or async (e.g. `Task { await viewModel.selectZone(zone) }`),
/// without changing the chip's appearance.
///
/// Follows the design language: `tcCaptionEmphasis` typography, `tcAmber`
/// for the selected fill, `tcSurface` for the unselected fill, and a
/// `tcBorder` outline that hides when selected.
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
