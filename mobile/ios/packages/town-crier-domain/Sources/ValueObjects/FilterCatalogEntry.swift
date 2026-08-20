/// One entry in the pre-canned watch-zone filter catalog, fetched from
/// `GET /v1/watch-zones/filter-catalog` (GH#1104, bead tc-m8j90.2).
///
/// Replaces the closed `WatchZoneFilterKey` enum (GH#1098/tc-hogkn): the
/// catalog now lives server-side only (`api-go/internal/watchzones/filter.go`)
/// and is fetched at runtime, so adding, renaming, or retiring a filter is a
/// Go-only deploy -- no iOS release needed to add an enum `case`. See
/// ``WatchZone/filterKey`` for the opaque `String?` this catalog resolves
/// display copy for, and ``WatchZoneRepository/filterCatalog()`` for the
/// fetch.
public struct FilterCatalogEntry: Equatable, Hashable, Sendable {
  /// The opaque filter key, round-tripped verbatim to/from the API as
  /// `WatchZone.filterKey`'s wire value. A key with no matching catalog
  /// entry (fetch pending, fetch failed, or a genuinely newer key an older
  /// client build predates) is never treated as invalid client-side --
  /// display falls back gracefully instead (tc-8z9ri).
  public let key: String

  /// The picker row's primary label.
  public let displayName: String

  /// The picker row's secondary explanation.
  public let description: String

  public init(key: String, displayName: String, description: String) {
    self.key = key
    self.displayName = displayName
    self.description = description
  }
}
