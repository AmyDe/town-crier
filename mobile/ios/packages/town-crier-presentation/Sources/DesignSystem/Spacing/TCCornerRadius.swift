import Foundation

/// Corner radius tokens for the Town Crier design language.
///
/// Public Notice (GH#857) sharpens the previous 8/12/16 rounded-shape scale
/// to 3/6/12 — crisper, closer to a printed notice than a rounded app icon.
/// `Capsule()` stays reserved for filter-chip components
/// (``CapsuleChipView``); status indicators moved to the stamp treatment
/// (see ``StatusBadgeView``, ``ApplicationStatusPill``).
public enum TCCornerRadius {
  /// 3pt — Small elements: badges, chips, input fields.
  public static let small: CGFloat = 3

  /// 6pt — Cards, buttons, list groupings.
  public static let medium: CGFloat = 6

  /// 12pt — Bottom sheets, modals, large cards.
  public static let large: CGFloat = 12
}
