import Foundation

/// The pre-canned watch-zone filter catalog fetch, memoization, selection,
/// and display-name resolution (GH#1104, bead tc-m8j90.2). Split out of
/// `WatchZoneEditorViewModel` to keep that file under SwiftLint's
/// `file_length` ceiling, mirroring the existing split into
/// `ApplicationListViewModel+GlobalUnread.swift`.
extension WatchZoneEditorViewModel {
  /// Sets or clears the selected filter. Clearing (`nil`) is always allowed
  /// regardless of tier, matching the server's "clearing never requires an
  /// entitlement check". Setting a non-nil value while ineligible
  /// (``WatchZoneEditorViewModel/canSetWatchZoneFilter`` is `false`) is a
  /// silent no-op, matching ``WatchZoneEditorViewModel/selectShapeMode(_:)``'s
  /// defence-in-depth pattern.
  public func selectFilterKey(_ key: String?) {
    guard key == nil || canSetWatchZoneFilter else { return }
    selectedFilterKey = key
  }

  /// Fetches ``WatchZoneEditorViewModel/filterCatalog`` from the repository,
  /// memoized for this editor session -- a repeat call after a successful
  /// fetch is a no-op, so re-opening the filter picker within the same
  /// session never re-fetches. A call while a fetch is already in flight is
  /// also a no-op rather than firing a second concurrent request. On
  /// failure, ``WatchZoneEditorViewModel/filterCatalogLoadFailed`` is set so
  /// the picker can offer a retry (calling this method again);
  /// ``WatchZoneEditorViewModel/selectedFilterKey`` is never touched by this
  /// method, on success or failure -- a display-layer hiccup must never lose
  /// or clear an already-stored filter (tc-8z9ri).
  public func loadFilterCatalogIfNeeded() async {
    guard !hasLoadedFilterCatalog, !isLoadingFilterCatalog else { return }
    isLoadingFilterCatalog = true
    filterCatalogLoadFailed = false
    do {
      filterCatalog = try await repository.filterCatalog()
      hasLoadedFilterCatalog = true
    } catch {
      filterCatalogLoadFailed = true
    }
    isLoadingFilterCatalog = false
  }

  /// The generic fallback shown for a key that isn't in
  /// ``WatchZoneEditorViewModel/filterCatalog`` -- the catalog fetch is
  /// pending or failed, or the key is a genuinely newer filter this build's
  /// fetched catalog doesn't (yet) contain. Never "None": a set filter must
  /// always read as set (tc-8z9ri).
  private static var fallbackFilterDisplayName: String { "Filter applied" }

  /// Resolves `key` to its catalog display name, falling back to
  /// ``fallbackFilterDisplayName`` when `key` isn't present in
  /// ``WatchZoneEditorViewModel/filterCatalog``. Returns `nil` only when
  /// `key` itself is `nil` (unfiltered) -- a non-nil key always resolves to
  /// some non-nil label, so a set filter never silently reads as unset.
  public func filterDisplayName(for key: String?) -> String? {
    guard let key else { return nil }
    return filterCatalog.first { $0.key == key }?.displayName ?? Self.fallbackFilterDisplayName
  }
}
