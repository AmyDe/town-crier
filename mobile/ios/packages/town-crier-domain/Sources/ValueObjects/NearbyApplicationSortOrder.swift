/// The server-supported sort orders for the public, unauthenticated
/// `GET /v1/applications/near-point` endpoint (GH#912 Phase 2/3). Raw values
/// are the `?sort=` query vocabulary the API accepts.
public enum NearbyApplicationSortOrder: String, Sendable, CaseIterable {
  /// Nearest-first via the KNN `<->` operator. The server's own default —
  /// the anonymous map relies on this staying byte-for-byte unchanged
  /// (GH#912 settled decision #5), so it keeps requesting this explicitly
  /// via ``AnonymousApplicationsRepository``'s no-sort convenience overload.
  case distance
  /// `GREATEST(decided_date, start_date) DESC NULLS LAST`, most-recently-
  /// updated first. The anonymous list's default (GH#912 settled decision
  /// #3) — deliberately server-side: the list only ever holds a single
  /// bounded page, so a client-side re-sort would silently swap "most
  /// recent N" for "nearest N by date".
  case recent
}
