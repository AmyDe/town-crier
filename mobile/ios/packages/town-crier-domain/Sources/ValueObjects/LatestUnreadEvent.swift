import Foundation

/// Per-application unread-notification descriptor surfaced on each row of the
/// applications-by-zone result. Drives the saturated/muted styling of the row's
/// `ApplicationStatusPill` and the `recent-activity` sort on the Applications
/// screen. `nil` (or absent) when the application has no unread notification
/// for the user (server-side `read_at IS NULL`).
///
/// `type` mirrors the API's `NotificationEventType` discriminator, currently
/// `"NewApplication"` or `"DecisionUpdate"`. Carried as a string rather than
/// an enum so unrecognised future values do not cause decoding failures.
/// `decision` is the raw PlanIt `app_state` (e.g. `"Permitted"`) for
/// `DecisionUpdate` events; `nil` otherwise. `createdAt` is the instant the
/// notification was raised — used by the `recent-activity` sort.
///
/// See ADR 0035 (`docs/adr/0035-per-application-notification-read-state.md`).
public struct LatestUnreadEvent: Equatable, Sendable {
  public let type: String
  public let decision: String?
  public let createdAt: Date

  public init(type: String, decision: String?, createdAt: Date) {
    self.type = type
    self.decision = decision
    self.createdAt = createdAt
  }
}
