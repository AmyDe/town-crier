import SwiftUI

/// Callout warning that a watch zone with a large radius may generate a high
/// volume of notifications — surfaced on both the onboarding radius picker and
/// the watch-zone editor (tc-1zb7).
///
/// Tone matches existing onboarding copy: friendly, neighbourly, concrete. The
/// callout uses the brand amber at 15% opacity so it reads as a heads-up, not
/// an error — the radius itself is still allowed.
public struct LargeRadiusWarningView: View {
  /// Radius (in metres) at or above which the warning is shown. The bead
  /// recommends "100–500 m in cities, under 2 km elsewhere" — 2 km is the
  /// upper edge of the recommended small-zone range, so any selection at or
  /// above it warrants the heads-up.
  public static let thresholdMetres: Double = 2000

  public init() {}

  public var body: some View {
    HStack(alignment: .top, spacing: TCSpacing.small) {
      Image(systemName: "exclamationmark.triangle.fill")
        .font(.system(.body))
        .foregroundStyle(Color.tcAmber)
        .accessibilityHidden(true)

      VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
        Text("Heads up — large zones get noisy")
          .font(TCTypography.bodyEmphasis)
          .foregroundStyle(Color.tcTextPrimary)

        Text(
          """
          A wide watch zone can produce hundreds of notifications a day, \
          especially in cities. We recommend 100–500 m in built-up areas, \
          and under 2 km everywhere else.
          """
        )
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextSecondary)
        .fixedSize(horizontal: false, vertical: true)
      }
    }
    .padding(TCSpacing.medium)
    .frame(maxWidth: .infinity, alignment: .leading)
    .background(Color.tcAmberMuted)
    .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
    .accessibilityElement(children: .combine)
    .accessibilityLabel(
      "Heads up. Large watch zones can produce hundreds of notifications a day, "
        + "especially in cities. We recommend 100 to 500 metres in built-up areas, "
        + "and under 2 kilometres elsewhere."
    )
  }
}
