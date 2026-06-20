import Foundation

/// Parses timestamp strings emitted by the Go backend's `platform.DotNetTime`
/// format (`2006-01-02T15:04:05.9999999-07:00`).
///
/// Fractional seconds appear on the wire **only** when the sub-second part is
/// non-zero, otherwise they are omitted entirely:
///
/// - `2026-07-20T14:23:45.6789123+00:00` — fractional (the common case, since
///   expiries derive from `time.Now()`).
/// - `2026-06-12T09:30:00+00:00` — whole second, no fractional part.
/// - `2026-06-12T09:30:00Z` — `Z` zero-offset spelling.
///
/// A single `ISO8601DateFormatter` cannot cover all three: `.withFractionalSeconds`
/// rejects whole-second strings, and the default `.withInternetDateTime` rejects
/// fractional ones. So we try fractional first, then fall back to plain. Both
/// option sets include `.withTimeZone`, which accepts the `Z` and `+hh:mm` forms.
///
/// Two `ISO8601DateFormatter` instances are cached. They are configured once and
/// never mutated afterwards, so reusing them avoids per-call allocation. The type
/// is not `Sendable`, but `ISO8601DateFormatter` is documented as safe for
/// concurrent use of its parsing methods once configured, and `formatOptions` is
/// never touched after init here — hence `nonisolated(unsafe)`.
enum DotNetTimeParser {
  private nonisolated(unsafe) static let fractional: ISO8601DateFormatter = {
    let formatter = ISO8601DateFormatter()
    formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
    return formatter
  }()

  private nonisolated(unsafe) static let plain: ISO8601DateFormatter = {
    let formatter = ISO8601DateFormatter()
    formatter.formatOptions = [.withInternetDateTime]
    return formatter
  }()

  /// Returns the parsed `Date`, or `nil` only if BOTH the fractional and the
  /// plain parse fail (i.e. genuinely unparseable input).
  static func date(from string: String) -> Date? {
    fractional.date(from: string) ?? plain.date(from: string)
  }
}
