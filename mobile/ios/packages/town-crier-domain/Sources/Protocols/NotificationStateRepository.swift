import Foundation

/// Port for the server-side notification read state.
///
/// A notification is unread iff its server-side `read_at IS NULL`. Read state
/// clears per application (opening it) or wholesale (mark-all-read); the
/// retired scroll-to-clear watermark no longer applies. See ADR 0035
/// (`docs/adr/0035-per-application-notification-read-state.md`).
///
/// - `GET /v1/me/notification-state` returns the current `NotificationState`.
/// - `POST /v1/me/notification-state/mark-all-read` clears every unread
///   notification; subsequent fetches report zero unread.
/// - `POST /v1/me/applications/mark-read` clears one application's unread
///   notifications.
public protocol NotificationStateRepository: Sendable {
  /// Returns the user's current read-state snapshot and unread count.
  func fetchState() async throws -> NotificationState

  /// Clears every unread notification for the current user.
  func markAllRead() async throws

  /// Marks a single application's notifications read for the current user.
  /// Scoped by the composite `(applicationUid, authorityId)` — a PlanIt
  /// reference is unique only within a council, so both fields are required.
  /// Idempotent and fire-and-forget: a later ``fetchState()`` reconciles any
  /// drift.
  func markApplicationRead(applicationUid: String, authorityId: Int) async throws
}
