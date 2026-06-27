import Foundation
import TownCrierDomain

/// Sort modes for the Applications screen — persisted under
/// `applicationsListSort` in `UserDefaults` so the user's choice survives
/// app launches. Default: ``recentActivity``.
///
/// Mirrors the web `ApplicationsSort` type from `useApplications.ts` so the
/// two clients stay behaviourally identical (tc-1nsa.11, tc-mso6).
///
/// Spec: `docs/specs/notifications-unread-watermark.md`, Pre-Resolved
/// Decisions #9 (default sort) and #10 (4 client-side options persisted).
public enum ApplicationsSort: String, CaseIterable, Sendable {
  /// Default mode — orders by `max(receivedDate, latestUnreadEvent.createdAt)`
  /// descending so newly-decided rows surface alongside newly-received ones.
  case recentActivity = "recent-activity"
  /// Receive-date descending (the previous default before unread tracking).
  case newest = "newest"
  /// Receive-date ascending.
  case oldest = "oldest"
  /// Lexicographic sort by `ApplicationStatus.rawValue` — groups rows by
  /// PlanIt's `app_state` vocabulary.
  case status = "status"
  /// Haversine distance from the active watch zone's centre to each
  /// application's `location`, ascending. Apps without a location sort
  /// last. Only meaningful when a single zone is selected — the picker
  /// hides this option in multi-zone "all" views (tc-mso6).
  case distance = "distance"

  /// The server-side sort order the API can page in, or `nil` for the
  /// client-only sorts (recent-activity, status) which this slice keeps
  /// computing locally over the fetched page (GH#682 slice 1). Distance, newest,
  /// and oldest are server-driven and paged via infinite scroll.
  public var serverOrder: ApplicationSortOrder? {
    switch self {
    case .distance:
      return .distance
    case .newest:
      return .newest
    case .oldest:
      return .oldest
    case .recentActivity, .status:
      return nil
    }
  }

  /// Whether the server owns the ordering (and therefore drives the paged
  /// infinite-scroll path) for this sort.
  public var isServerSorted: Bool {
    serverOrder != nil
  }

  /// User-facing label rendered in the sort menu.
  public var displayLabel: String {
    switch self {
    case .recentActivity:
      return "Recent activity"
    case .newest:
      return "Newest"
    case .oldest:
      return "Oldest"
    case .status:
      return "Status"
    case .distance:
      return "Distance"
    }
  }
}
