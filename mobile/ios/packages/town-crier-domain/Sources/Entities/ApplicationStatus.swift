/// The lifecycle status of a planning application.
///
/// Raw values mirror PlanIt's `app_state` wire vocabulary so that decoding
/// can be a direct `ApplicationStatus(rawValue:)` lookup with no string-table
/// indirection.
public enum ApplicationStatus: String, Equatable, Hashable, Sendable {
  case undecided = "Undecided"
  case permitted = "Permitted"
  case conditions = "Conditions"
  case rejected = "Rejected"
  case withdrawn = "Withdrawn"
  case appealed = "Appealed"
  case unresolved = "Unresolved"
  case referred = "Referred"
  case notAvailable = "Not Available"
  case unknown
}
