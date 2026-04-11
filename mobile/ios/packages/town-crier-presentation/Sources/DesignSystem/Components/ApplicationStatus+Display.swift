import SwiftUI
import TownCrierDomain

extension ApplicationStatus {

  /// The user-facing label for this status (e.g. "Pending", "Approved").
  public var displayLabel: String {
    switch self {
    case .underReview:
      "Pending"
    case .approved:
      "Approved"
    case .refused:
      "Refused"
    case .withdrawn:
      "Withdrawn"
    case .appealed:
      "Appealed"
    case .unknown:
      "Unknown"
    }
  }

  /// The SF Symbol name representing this status.
  public var displayIcon: String {
    switch self {
    case .underReview:
      "clock"
    case .approved:
      "checkmark.circle"
    case .refused:
      "xmark.circle"
    case .withdrawn:
      "arrow.uturn.backward.circle"
    case .appealed:
      "exclamationmark.triangle"
    case .unknown:
      "questionmark.circle"
    }
  }

  /// The design-system color associated with this status.
  public var displayColor: Color {
    switch self {
    case .underReview:
      .tcStatusPending
    case .approved:
      .tcStatusApproved
    case .refused:
      .tcStatusRefused
    case .withdrawn:
      .tcStatusWithdrawn
    case .appealed:
      .tcStatusAppealed
    case .unknown:
      .tcTextTertiary
    }
  }
}
