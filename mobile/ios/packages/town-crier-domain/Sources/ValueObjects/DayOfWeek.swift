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

  /// Human-readable name for display in UI (e.g. picker labels). Currently
  /// equal to `rawValue` because the API uses English day names; kept as a
  /// distinct property so the UI doesn't depend on the wire format.
  public var displayName: String {
    rawValue
  }
}
