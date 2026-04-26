import SwiftUI

// MARK: - Brand

extension Color {
  /// Primary accent. Warm gold evoking town crier bells.
  public static let tcAmber = Color.themed(light: 0xD4910A, dark: 0xE9A620, oled: 0xE9A620)

  /// Lower-emphasis accent for secondary buttons and tags.
  public static let tcAmberMuted = tcAmber.opacity(0.15)
}

// MARK: - Surfaces

extension Color {
  /// Page-level background.
  public static let tcBackground = Color.themed(light: 0xFAF8F5, dark: 0x1A1A1E, oled: 0x000000)

  /// Card and component background.
  public static let tcSurface = Color.themed(light: 0xFFFFFF, dark: 0x242428, oled: 0x0A0A0A)

  /// Modals, bottom sheets, popovers.
  public static let tcSurfaceElevated = Color.themed(
    light: 0xFFFFFF, dark: 0x2E2E33, oled: 0x161616)
}

// MARK: - Text

extension Color {
  /// Body text and headings.
  public static let tcTextPrimary = Color.themed(light: 0x1C1917, dark: 0xF1EFE9, oled: 0xF1EFE9)

  /// Captions, metadata, timestamps.
  public static let tcTextSecondary = Color.themed(light: 0x6B6560, dark: 0x9B9590, oled: 0x9B9590)

  /// Placeholder text, disabled labels.
  public static let tcTextTertiary = Color.themed(light: 0xA39E98, dark: 0x5C5852, oled: 0x5C5852)

  /// Text rendered on tcAmber backgrounds.
  public static let tcTextOnAccent = Color.themed(light: 0xFFFFFF, dark: 0x1C1917, oled: 0x1C1917)
}

// MARK: - Status

extension Color {
  /// Application granted (PlanIt `Permitted`).
  public static let tcStatusPermitted = Color.themed(
    light: 0x1A7D37, dark: 0x34C759, oled: 0x34C759)

  /// Application granted with conditions (PlanIt `Conditions`).
  /// Amber/orange — granted but constrained, distinct from outright approval.
  public static let tcStatusConditions = Color.themed(
    light: 0xB85C00, dark: 0xFF9F0A, oled: 0xFF9F0A)

  /// Application refused (PlanIt `Rejected`).
  public static let tcStatusRejected = Color.themed(
    light: 0xC42B2B, dark: 0xFF453A, oled: 0xFF453A)

  /// Awaiting decision / under review.
  public static let tcStatusPending = Color.themed(light: 0xC27A0A, dark: 0xFFB340, oled: 0xFFB340)

  /// Application withdrawn.
  public static let tcStatusWithdrawn = Color.themed(
    light: 0x7A7570, dark: 0x8E8A85, oled: 0x8E8A85)

  /// Decision under appeal.
  public static let tcStatusAppealed = Color.themed(light: 0x7C3AED, dark: 0xA78BFA, oled: 0xA78BFA)
}

// MARK: - Utility

extension Color {
  /// Subtle dividers and card outlines.
  public static let tcBorder = Color.themed(light: 0xE8E4DF, dark: 0x3A3A3F, oled: 0x1E1E22)

  /// Focus rings and active input borders.
  public static let tcBorderFocused = tcAmber

  /// Semi-transparent scrim behind modals.
  public static let tcOverlay = Color.black.opacity(0.4)
}
