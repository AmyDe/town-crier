/// Port for fetching paginated notification history.
///
/// Available to all tiers -- no entitlement gating.
public protocol NotificationRepository: Sendable {
  func fetch(page: Int, pageSize: Int) async throws -> NotificationPage
}
