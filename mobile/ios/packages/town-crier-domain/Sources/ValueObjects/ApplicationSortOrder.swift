/// The server-supported sort orders for the paged watch-zone applications
/// endpoint. Raw values are the `?sort=` query vocabulary the API accepts.
public enum ApplicationSortOrder: String, Sendable, CaseIterable {
  /// Nearest-first via the KNN `<->` operator. The server default and cheapest
  /// plan; matches the param-less legacy behaviour.
  case distance
  /// `start_date` descending, NULLS LAST, with a stable unique tiebreak.
  case newest
  /// `start_date` ascending, NULLS LAST, with a stable unique tiebreak.
  case oldest
  /// `app_state` ascending, NULLS LAST, then `start_date` descending, with a
  /// stable unique tiebreak. The server owns this ordering —
  /// the client must not re-sort the fetched pages by `app_state` locally.
  case status
  /// `GREATEST(start_date, unread.created_at) DESC NULLS LAST`, with a stable
  /// unique tiebreak. The server joins the caller's
  /// unread notifications and owns the ordering — the client must not re-derive
  /// `max(startDate, latestUnreadEvent.createdAt)` over the fetched pages.
  case recentActivity = "recent-activity"
}
