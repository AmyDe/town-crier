/// The server-supported sort orders for the public, unauthenticated
/// `GET /v1/applications/near-point` endpoint. Raw values
/// are the `?sort=` query vocabulary the API accepts.
public enum NearbyApplicationSortOrder: String, Sendable, CaseIterable {
  /// Nearest-first via the KNN `<->` operator; the server's default.
  case distance
  /// `GREATEST(decided_date, start_date) DESC NULLS LAST`, most-recently-
  /// updated first. Sorted server-side because the list holds a single
  /// bounded page, so a client-side re-sort would swap "most recent N" for
  /// "nearest N by date".
  case recent
}
