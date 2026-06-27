import Foundation
import TownCrierDomain

/// Infinite-scroll pagination for the watch-zone list (GH#682). Split out of
/// `ApplicationListViewModel` to keep that file under SwiftLint's `file_length`
/// ceiling. Every UI sort is server-driven now — distance/newest/oldest
/// (slice 1), status (slice 2) and recent-activity (slice 3, #692) — so the
/// server owns the ordering and the list follows `X-Next-Cursor` to the end for
/// all of them. The `serverOrder == nil` fallback below is dormant defensive
/// plumbing kept generic should a future client-only sort ever be added.
extension ApplicationListViewModel {

  /// Loads the active zone's first page in the server-ordered, cursor-paged way:
  /// fetch the first page for the current sort and capture its next-page cursor.
  /// The cursor resets here, so pagination always restarts cleanly on a fresh
  /// load or sort change.
  func loadActiveZone(_ zone: WatchZone) async throws {
    if let order = sort.serverOrder {
      let page = try await fetchPage(for: zone, sort: order, cursor: nil)
      applications = page.applications
      nextCursor = page.nextCursor
    } else {
      applications = try await fetchApplications(for: zone)
      nextCursor = nil
    }
    loadedSort = sort
    loadedFilter = activeFilter
  }

  /// Fetches and appends the next server page when one exists. No-op unless the
  /// active sort is server-driven, a cursor is held, and no append is already in
  /// flight. Following `X-Next-Cursor` until it is absent walks the whole set;
  /// the last page (no cursor) ends the loop. A fetch error surfaces to `error`.
  public func loadNextPage() async {
    guard let order = sort.serverOrder,
      let cursor = nextCursor,
      !isPageLoadInFlight,
      let activeZone = selectedZone ?? zone
    else { return }
    isPageLoadInFlight = true
    defer { isPageLoadInFlight = false }
    do {
      let page = try await fetchPage(for: activeZone, sort: order, cursor: cursor)
      appendPage(page)
    } catch {
      handleError(error)
    }
  }

  /// Called by the list as each row appears. Kicks off the next-page fetch when
  /// more pages remain (a cursor is held) and the appearing row is within
  /// ``prefetchThreshold`` of the end of the loaded set. Every sort is
  /// server-driven now, so this drives infinite scroll for all of them.
  public func onRowAppear(_ application: PlanningApplication) async {
    guard sort.isServerSorted, nextCursor != nil, !isPageLoadInFlight else { return }
    guard let index = applications.firstIndex(where: { $0.id == application.id }) else { return }
    guard index >= applications.count - Self.prefetchThreshold else { return }
    await loadNextPage()
  }

  /// Reacts to a sort change from the toolbar. Every sort is server-driven now
  /// (GH#682 slice 3), so any change to a different sort reloads from page 1 with
  /// a fresh cursor and re-pages in the new order; the cursor and `loadedSort`
  /// reset happen inside the reload. The `!newIsServer && !oldWasServer` branch
  /// is dormant defensive plumbing for a hypothetical future client-only sort.
  public func handleSortChanged() async {
    guard sort != loadedSort else { return }
    let newIsServer = sort.isServerSorted
    let oldWasServer = loadedSort?.isServerSorted ?? false
    if !newIsServer && !oldWasServer {
      loadedSort = sort
      return
    }
    await loadApplications()
  }

  /// Appends a server page, dropping any rows already loaded so a keyset overlap
  /// at a page boundary never duplicates a row. Captures the new cursor.
  private func appendPage(_ page: ApplicationPage) {
    let existingIds = Set(applications.map(\.id))
    applications.append(contentsOf: page.applications.filter { !existingIds.contains($0.id) })
    nextCursor = page.nextCursor
  }

  /// Fetches one page, threading the ViewModel's current ``activeFilter`` into
  /// the request so every page (first and subsequent) carries the same
  /// `?status=`/`?unread=` filter. A filter change resets the cursor and
  /// reloads from page 1 (see ``handleFilterChanged()``), so a cursor is only
  /// ever replayed under the filter it was minted with.
  private func fetchPage(
    for zone: WatchZone,
    sort order: ApplicationSortOrder,
    cursor: String?
  ) async throws -> ApplicationPage {
    if let offlineRepository {
      return try await offlineRepository.fetchApplicationsPage(
        for: zone, sort: order, filter: activeFilter, cursor: cursor, limit: Self.pageSize)
    }
    if let repository {
      return try await repository.fetchApplicationsPage(
        for: zone, sort: order, filter: activeFilter, cursor: cursor, limit: Self.pageSize)
    }
    return ApplicationPage(applications: [], nextCursor: nil)
  }
}
