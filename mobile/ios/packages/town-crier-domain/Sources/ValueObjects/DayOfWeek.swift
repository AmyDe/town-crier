import Foundation

/// Day of the week matching the API's `DayOfWeek` enum (System.DayOfWeek).
public enum DayOfWeek: String, CaseIterable, Codable, Equatable, Hashable, Sendable {
  case sunday = "Sunday"
  case monday = "Monday"
  case tuesday = "Tuesday"
  case wednesday = "Wednesday"
  case thursday = "Thursday"
  case friday = "Friday"
  case saturday = "Saturday"

  /// Display order for UK-facing pickers: the week starts on Monday.
  /// This is presentation-only — `allCases` and `rawValue` keep the API/wire
  /// order (Sunday-first, mirroring System.DayOfWeek) untouched.
  public static let weekOrderUK: [DayOfWeek] = [
    .monday, .tuesday, .wednesday, .thursday, .friday, .saturday, .sunday,
  ]

  /// Human-readable name for display in UI (e.g. picker labels). Currently
  /// equal to `rawValue` because the API uses English day names; kept as a
  /// distinct property so the UI doesn't depend on the wire format.
  public var displayName: String {
    rawValue
  }
}
