import Foundation
import TownCrierDomain

/// Sort modes for the Applications screen, persisted under
/// `applicationsListSort` in `UserDefaults`. Default: ``recentActivity``.
public enum ApplicationsSort: String, CaseIterable, Sendable {
  /// Default mode: the server orders by
  /// `GREATEST(start_date, unread.created_at) DESC` so newly-decided rows
  /// surface alongside newly-received ones.
  case recentActivity = "recent-activity"
  case newest = "newest"
  case oldest = "oldest"
  /// Groups rows by PlanIt's `app_state`.
  case status = "status"
  /// Nearest-first; only offered when a zone is active.
  case distance = "distance"

  /// The server-side sort order the API pages in; non-`nil` for every case.
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
