import SwiftUI

/// Typography tokens following the Town Crier design language.
/// All tokens use Dynamic Type styles to respect accessibility settings.
///
/// Public Notice (GH#857) moves the display roles to Fraunces — the static
/// Regular/SemiBold instances bundled under `DesignSystem/Resources/Fonts`
/// and registered at process startup by ``FontRegistrar``. Every role keeps
/// `relativeTo:` (or a bare Dynamic Type style) so Dynamic Type scaling is
/// never broken by the serif switch — that invariant is non-negotiable.
/// `body`/`bodyEmphasis`/`caption`/`captionEmphasis` stay system sans. The
/// new `mono`/`monoEmphasis` roles style planning references, dates, and
/// distances.
public enum TCTypography {
  public static let displayLarge: Font =
    .custom("Fraunces", size: 34, relativeTo: .largeTitle).weight(.semibold)
  public static let displaySmall: Font =
    .custom("Fraunces", size: 22, relativeTo: .title2).weight(.semibold)
  public static let headline: Font =
    .custom("Fraunces", size: 17, relativeTo: .headline).weight(.semibold)
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
