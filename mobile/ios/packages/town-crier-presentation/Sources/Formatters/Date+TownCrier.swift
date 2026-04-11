import Foundation

extension Date {
  /// Shared display formatter: "d MMM yyyy", en_GB locale, UTC timezone.
  private static let displayFormatter: DateFormatter = {
    let formatter = DateFormatter()
    formatter.dateFormat = "d MMM yyyy"
    formatter.locale = Locale(identifier: "en_GB")
    formatter.timeZone = TimeZone(identifier: "UTC")
    return formatter
  }()

  /// Formats the date for user-facing display (e.g. "14 Nov 2023").
  public var formattedForDisplay: String {
    Self.displayFormatter.string(from: self)
  }
}
