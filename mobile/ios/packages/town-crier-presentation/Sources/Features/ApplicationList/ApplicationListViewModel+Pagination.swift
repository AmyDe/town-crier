import Foundation
import TownCrierDomain

/// Infinite-scroll pagination for the watch-zone list (GH#682 slice 1). Split
/// out of `ApplicationListViewModel` to keep that file under SwiftLint's
/// `file_length` ceiling. For the three server-supported sorts
/// (distance/newest/oldest) the server owns the ordering and the list follows
/// `X-Next-Cursor` to the end; the client-side sorts (recent-activity/status)
/// keep their param-less single-page path.
extension ApplicationListViewModel {

  /// Loads the active zone's first page via the path appropriate to the current
  /// sort. The three server-supported sorts (distance/newest/oldest) fetch a
  /// server-ordered page and capture the next-page cursor; the client-side sorts
  /// (recent-activity/status) keep the legacy param-less first-page fetch. Either
  /// way the cursor resets, so pagination always restarts cleanly.
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
  /// a server sort is active, more pages remain, and the appearing row is within
  /// ``prefetchThreshold`` of the end of the loaded set. A no-op for client-side
  /// sorts, which hold the whole (first-page) set already.
  public func onRowAppear(_ application: PlanningApplication) async {
    guard sort.isServerSorted, nextCursor != nil, !isPageLoadInFlight else { return }
    guard let index = applications.firstIndex(where: { $0.id == application.id }) else { return }
    guard index >= applications.count - Self.prefetchThreshold else { return }
    await loadNextPage()
  }

  /// Reacts to a sort change from the toolbar. A server sort — or any transition
  /// away from one — reloads from page 1 with a fresh cursor so the list re-pages
  /// in the new order. A switch between the two client-side sorts only changes
  /// the in-memory ordering, so it skips the refetch (unchanged from the
  /// pre-pagination behaviour). The cursor and `loadedSort` reset happen inside
  /// the reload.
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

  private func fetchPage(
    for zone: WatchZone,
    sort order: ApplicationSortOrder,
    cursor: String?
  ) async throws -> ApplicationPage {
    if let offlineRepository {
      return try await offlineRepository.fetchApplicationsPage(
        for: zone, sort: order, cursor: cursor, limit: Self.pageSize)
    }
    if let repository {
      return try await repository.fetchApplicationsPage(
        for: zone, sort: order, cursor: cursor, limit: Self.pageSize)
    }
    return ApplicationPage(applications: [], nextCursor: nil)
  }
}
