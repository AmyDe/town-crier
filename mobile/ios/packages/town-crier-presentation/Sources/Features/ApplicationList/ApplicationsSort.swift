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
  /// Default mode — the server orders by
  /// `GREATEST(start_date, unread.created_at) DESC` via the notification join
  /// (GH#682 slice 3, #692) so newly-decided rows surface alongside
  /// newly-received ones. Server-driven and paged like every other sort.
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

  /// The server-side sort order the API pages in. Every UI sort now maps to a
  /// server order: distance/newest/oldest (GH#682 slice 1), status (slice 2),
  /// and recent-activity (slice 3, #692). The client never sorts locally, so
  /// this is non-`nil` for all cases. It stays `Optional` only so the paged
  /// infinite-scroll plumbing in `+Pagination` can remain generic over a future
  /// client-only sort, should one ever be added.
  public var serverOrder: ApplicationSortOrder? {
    switch self {
    case .distance:
      return .distance
    case .newest:
      return .newest
    case .oldest:
      return .oldest
    case .status:
      return .status
    case .recentActivity:
      return .recentActivity
    }
  }

  /// Whether the server owns the ordering (and therefore drives the paged
  /// infinite-scroll path) for this sort. True for every sort since slice 3.
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
