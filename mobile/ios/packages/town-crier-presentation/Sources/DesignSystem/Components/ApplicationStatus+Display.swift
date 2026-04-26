import SwiftUI
import TownCrierDomain

extension ApplicationStatus {

  /// The user-facing label for this status.
  ///
  /// We translate PlanIt's wire vocabulary into the language UK residents
  /// actually use about planning outcomes (e.g. "Refused" rather than
  /// "Rejected"). Display strings stay local; the wire raw values remain
  /// PlanIt-canonical.
  public var displayLabel: String {
    switch self {
    case .undecided:
      "Pending"
    case .permitted:
      "Granted"
    case .conditions:
      "Granted with conditions"
    case .rejected:
      "Refused"
    case .withdrawn:
      "Withdrawn"
    case .appealed:
      "Appealed"
    case .unresolved:
      "Unresolved"
    case .referred:
      "Referred"
    case .notAvailable:
      "Not Available"
    case .unknown:
      "Unknown"
    }
  }

  /// The SF Symbol name representing this status.
  public var displayIcon: String {
    switch self {
    case .undecided:
      "clock"
    case .permitted:
      "checkmark.circle"
    case .conditions:
      "checkmark.circle.badge.questionmark"
    case .rejected:
      "xmark.circle"
    case .withdrawn:
      "arrow.uturn.backward.circle"
    case .appealed:
      "exclamationmark.triangle"
    case .unresolved:
      "questionmark.circle"
    case .referred:
      "arrow.up.forward.circle"
    case .notAvailable:
      "minus.circle"
    case .unknown:
      "questionmark.circle"
    }
  }

  /// The design-system color associated with this status.
  public var displayColor: Color {
    switch self {
    case .undecided:
      .tcStatusPending
    case .permitted:
      .tcStatusPermitted
    case .conditions:
      .tcStatusConditions
    case .rejected:
      .tcStatusRejected
    case .withdrawn:
      .tcStatusWithdrawn
    case .appealed:
      .tcStatusAppealed
    case .unresolved, .referred, .notAvailable, .unknown:
      .tcTextTertiary
    }
  }
}
