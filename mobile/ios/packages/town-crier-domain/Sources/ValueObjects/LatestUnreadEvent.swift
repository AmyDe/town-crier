import Foundation

/// Per-application unread-notification descriptor surfaced on each row of the
/// applications-by-zone result. Drives the saturated/muted styling of the row's
/// `ApplicationStatusPill` and the `recent-activity` sort on the Applications
/// screen. `nil` (or absent) when no notification exists strictly after the
/// user's `lastReadAt` watermark for this row, or the user has no watermark
/// document yet (first-touch path; clients seed via
/// `GET /v1/me/notification-state`).
///
/// `type` mirrors the API's `NotificationEventType` discriminator, currently
/// `"NewApplication"` or `"DecisionUpdate"`. Carried as a string rather than
/// an enum so unrecognised future values do not cause decoding failures.
/// `decision` is the raw PlanIt `app_state` (e.g. `"Permitted"`) for
/// `DecisionUpdate` events; `nil` otherwise. `createdAt` is the instant the
/// notification was raised — used by both the `recent-activity` sort and the
/// push-tap watermark-advance path (tc-1nsa.9).
///
/// Spec: `docs/specs/notifications-unread-watermark.md#api-augment-applications`.
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
