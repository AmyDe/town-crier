import SwiftUI

/// Typography tokens following the Town Crier design language.
/// All tokens use Dynamic Type styles to respect accessibility settings.
///
/// GH#912 Phase 5: standardises on the system sans everywhere, dropping the
/// Fraunces display-serif treatment Public Notice (GH#857) introduced —
/// tester feedback found the clarity/consistency of one typeface worth
/// losing a bit of the serif's "old-timey" character (owner-approved
/// 2026-07-10). The display roles keep their Fraunces-era base sizes and
/// `.semibold` weight (34pt `.largeTitle`, 22pt `.title2`, 17pt `.headline`)
/// by building on the underlying Dynamic Type text style with a weight
/// override, so scaling stays fully native — every role keeps `relativeTo:`
/// or a bare Dynamic Type style, never a fixed point size,
/// so Dynamic Type scaling is never broken. `mono`/`monoEmphasis` style
/// planning references, dates, and distances and are unaffected.
public enum TCTypography {
  public static let displayLarge: Font = .system(.largeTitle).weight(.semibold)
  public static let displaySmall: Font = .system(.title2).weight(.semibold)
  public static let headline: Font = .system(.headline).weight(.semibold)
  public static let body: Font = .system(.body)
  public static let bodyEmphasis: Font = .system(.body, weight: .semibold)
  public static let caption: Font = .system(.caption)
  public static let captionEmphasis: Font = .system(.caption, weight: .medium)

  /// Monospaced metadata role — planning references, dates, distances.
  public static let mono: Font = .system(.caption, design: .monospaced)

  /// Monospaced metadata role, medium weight — the emphasised counterpart
  /// of ``mono``, e.g. a card's leading reference in a mono header strip.
  public static let monoEmphasis: Font = .system(.caption, design: .monospaced).weight(.medium)
}
