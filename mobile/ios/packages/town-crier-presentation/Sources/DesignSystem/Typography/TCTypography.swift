import SwiftUI

/// Typography tokens following the Town Crier design language.
/// All tokens use Dynamic Type styles to respect accessibility settings.
public enum TCTypography {
    public static let displayLarge: Font = .system(.largeTitle, weight: .bold)
    public static let displaySmall: Font = .system(.title2, weight: .semibold)
    public static let headline: Font = .system(.headline, weight: .semibold)
    public static let body: Font = .system(.body)
    public static let bodyEmphasis: Font = .system(.body, weight: .semibold)
    public static let caption: Font = .system(.caption)
    public static let captionEmphasis: Font = .system(.caption, weight: .medium)
}
