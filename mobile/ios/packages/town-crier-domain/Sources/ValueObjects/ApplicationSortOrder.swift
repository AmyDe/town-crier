/// The server-supported sort orders for the paged watch-zone applications
/// endpoint. Raw values are the `?sort=` query vocabulary the API accepts
/// (GH#682 slices 1-2) — `recent-activity` is still computed client-side and is
/// *not* representable here.
public enum ApplicationSortOrder: String, Sendable, CaseIterable {
  /// Nearest-first via the KNN `<->` operator. The server default and cheapest
  /// plan; matches the param-less legacy behaviour.
  case distance
  /// `start_date` descending, NULLS LAST, with a stable unique tiebreak.
  case newest
  /// `start_date` ascending, NULLS LAST, with a stable unique tiebreak.
  case oldest
  /// `app_state` ascending, NULLS LAST, then `start_date` descending, with a
  /// stable unique tiebreak (GH#682 slice 2). The server owns this ordering —
  /// the client must not re-sort the fetched pages by `app_state` locally.
  case status
}
