import SwiftUI

/// A compact, tappable capsule chip used in horizontal watch-zone picker rows.
///
/// Purely presentational: it renders a label in a capsule whose fill, text
/// colour, and border reflect the `isSelected` flag, and forwards taps to
/// `onTap`. The caller owns all selection logic — `ZoneChipView` never
/// inspects a view model — so each call site can compute selection from its
/// own state (e.g. `zone.id == viewModel.selectedZone?.id`) and run its own
/// async tap action (e.g. `await viewModel.selectZone(zone)`) without
/// changing the chip's appearance.
///
/// Visually identical to `FilterChipView`: `tcCaptionEmphasis` typography,
/// `tcAmber` for the selected fill, `tcSurface` for the unselected fill, and
/// a `tcBorder` outline that hides when selected.
///
/// Usage:
/// ```swift
/// ZoneChipView(
///     label: zone.name,
///     isSelected: zone.id == viewModel.selectedZone?.id
/// ) {
///     Task { await viewModel.selectZone(zone) }
/// }
/// ```
public struct ZoneChipView: View {
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
