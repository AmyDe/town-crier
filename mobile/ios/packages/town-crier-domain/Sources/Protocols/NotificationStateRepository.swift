import Foundation

/// Port for the server-side notification read-state watermark.
///
/// Backed by three endpoints (see spec
/// `docs/specs/notifications-unread-watermark.md#api-surface`):
/// - `GET /v1/me/notification-state` returns the current `NotificationState`.
/// - `POST /v1/me/notification-state/mark-all-read` stamps the watermark to
///   the server's "now"; subsequent fetches report zero unread.
/// - `POST /v1/me/notification-state/advance` moves the watermark to the
///   supplied `asOf` instant. The server enforces monotonicity, so passing a
///   stale instant is a no-op rather than an error — callers can fire and
///   forget without checking the existing watermark first.
public protocol NotificationStateRepository: Sendable {
  /// Returns the user's current watermark and unread count.
  func fetchState() async throws -> NotificationState

  /// Stamps the watermark to the server's current instant.
  func markAllRead() async throws

  /// Advances the watermark forward to `asOf`. No-op if `asOf` is at or
  /// before the existing watermark (server-enforced monotonicity).
  func advance(asOf: Date) async throws
}
