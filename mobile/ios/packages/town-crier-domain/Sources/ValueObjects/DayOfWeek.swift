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
}
