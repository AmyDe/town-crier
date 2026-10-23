import SwiftUI

/// Typography tokens following the Town Crier design language.
/// All tokens use Dynamic Type styles to respect accessibility settings.
///
/// Display roles are the system sans at `.semibold` (34pt `.largeTitle`,
/// 22pt `.title2`, 17pt `.headline`), built on the underlying Dynamic Type
/// text style with a weight override, never a fixed point size, so scaling
/// stays native. `mono`/`monoEmphasis` style planning references, dates,
/// and distances.
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
