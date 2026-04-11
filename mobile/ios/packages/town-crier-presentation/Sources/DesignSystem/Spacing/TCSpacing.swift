import Foundation

/// Spacing tokens based on a 4pt grid.
public enum TCSpacing {
  /// 4pt — Tight gaps: icon-to-label, inline elements.
  public static let extraSmall: CGFloat = 4

  /// 8pt — Compact padding: within dense components.
  public static let small: CGFloat = 8

  /// 16pt — Standard padding: card insets, list row padding.
  public static let medium: CGFloat = 16

  /// 24pt — Section gaps: between card groups.
  public static let large: CGFloat = 24

  /// 32pt — Major sections: screen-level vertical rhythm.
  public static let extraLarge: CGFloat = 32

  /// 48pt — Hero spacing: top of screen breathing room.
  public static let extraExtraLarge: CGFloat = 48
}
