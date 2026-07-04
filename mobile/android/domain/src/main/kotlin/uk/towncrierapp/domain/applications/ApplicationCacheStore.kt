package uk.towncrierapp.domain.applications

import uk.towncrierapp.domain.watchzones.WatchZoneId
import java.time.Instant

/** An [ApplicationPage] snapshot plus the instant it was cached — the freshness check lives in [OfflineAwareRepository], not here. */
public data class CachedApplicationPage(
    public val page: ApplicationPage,
    public val cachedAt: Instant,
)

/**
 * Pure keyed storage for [OfflineAwareRepository]'s zone-scoped first-page
 * snapshots — no TTL/freshness logic of its own (that policy lives in the
 * decorator, which is what makes "stale but still servable offline" and
 * "fresh enough to skip the network" two different questions the store
 * doesn't need to answer). [invalidate]/[invalidateAll] are the explicit hooks
 * a watch-zone edit (tc-cnme) or a mark-all-read fires. Port of iOS
 * `ApplicationCacheStore`.
 */
public interface ApplicationCacheStore {
    public suspend fun get(zoneId: WatchZoneId): CachedApplicationPage?

    public suspend fun put(
        zoneId: WatchZoneId,
        entry: CachedApplicationPage,
    )

    public suspend fun invalidate(zoneId: WatchZoneId)

    public suspend fun invalidateAll()
}
