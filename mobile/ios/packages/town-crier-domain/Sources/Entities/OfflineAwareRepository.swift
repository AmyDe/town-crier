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

    public func fetchApplications(for authority: LocalAuthority) async throws -> CacheEntry<[PlanningApplication]> {
        // Check cache first
        let cached = await cache.retrieve(for: authority)

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
            let applications = try await remote.fetchApplications(for: authority)
            let entry = CacheEntry(data: applications, fetchedAt: Date())
            await cache.store(entry, for: authority)
            return entry
        } catch {
            // Remote failed — fall back to cache if available
            if let cached {
                return cached
            }
            throw error
        }
    }
}
