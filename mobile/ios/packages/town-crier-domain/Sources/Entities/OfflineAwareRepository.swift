import Foundation

/// A repository decorator that caches remote results and serves cached data when offline.
public final class OfflineAwareRepository: Sendable {
  private let remote: PlanningApplicationRepository
  private let cache: ApplicationCacheStore
  private let connectivity: ConnectivityMonitor

  public init(
    remote: PlanningApplicationRepository,
    cache: ApplicationCacheStore,
    connectivity: ConnectivityMonitor
  ) {
    self.remote = remote
    self.cache = cache
    self.connectivity = connectivity
  }

  public func fetchApplications(for zone: WatchZone) async throws -> CacheEntry<
    [PlanningApplication]
  > {
    // Check cache first
    let cached = await cache.retrieve(for: zone)

    // If we have a fresh cache hit, return it without a network call
    if let cached, cached.isFresh() {
      return cached
    }

    // If offline, return whatever cache we have or throw
    guard connectivity.isConnected else {
      if let cached {
        return cached
      }
      throw DomainError.networkUnavailable
    }

    // Online — try remote
    do {
      let applications = try await remote.fetchApplications(for: zone)
      let entry = CacheEntry(data: applications, fetchedAt: Date())
      await cache.store(entry, for: zone)
      return entry
    } catch {
      // Remote failed — fall back to cache if available
      if let cached {
        return cached
      }
      throw error
    }
  }

  /// Fetches a single server-sorted, server-paged page straight from the remote.
  ///
  /// Pagination deliberately bypasses the offline cache: the cache is keyed
  /// per-zone on the full first-page set, not on cursor-addressed pages, so the
  /// list's infinite-scroll pages always hit the network (GH#682 slice 1). The
  /// param-less ``fetchApplications(for:)`` path keeps its cache behaviour.
  public func fetchApplicationsPage(
    for zone: WatchZone,
    sort: ApplicationSortOrder,
    filter: ApplicationFilter,
    cursor: String?,
    limit: Int
  ) async throws -> ApplicationPage {
    try await remote.fetchApplicationsPage(
      for: zone, sort: sort, filter: filter, cursor: cursor, limit: limit)
  }

  /// Invalidates the cached applications for the given zone id.
  ///
  /// Callers should invoke this after a watch-zone edit changes the zone's
  /// geometry (radius/centre), so a subsequent `fetchApplications(for:)`
  /// re-queries the server and stores fresh results that reflect the new
  /// shape. Without invalidation, a TTL-fresh cache hit could serve
  /// applications matching the previous geometry for up to the cache TTL.
  public func invalidateCache(for zoneId: WatchZoneId) async {
    await cache.invalidate(for: zoneId)
  }

  /// Invalidates every cached zone.
  ///
  /// Callers should invoke this after a global state mutation that affects
  /// every zone's view of applications — e.g. mark-all-read, which clears
  /// each row's `latestUnreadEvent`. Without invalidation, a TTL-fresh
  /// per-zone cache hit would keep serving the old unread flags for up to
  /// the cache TTL, leaving the `Unread (N)` chip stuck on the prior count
  /// (tc-e3bu).
  public func invalidateAllCaches() async {
    await cache.invalidateAll()
  }
}
