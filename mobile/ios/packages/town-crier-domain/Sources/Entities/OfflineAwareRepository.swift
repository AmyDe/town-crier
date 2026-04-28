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
}
