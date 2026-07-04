package uk.towncrierapp.domain.applications

import uk.towncrierapp.domain.watchzones.WatchZoneId

/** Hand-written fake for [ApplicationCacheStore] — state-based, per testing.md conventions. */
public class FakeApplicationCacheStore : ApplicationCacheStore {
    public val entries: MutableMap<WatchZoneId, CachedApplicationPage> = mutableMapOf()
    public val invalidateCalls: MutableList<WatchZoneId> = mutableListOf()
    public var invalidateAllCallCount: Int = 0

    override suspend fun get(zoneId: WatchZoneId): CachedApplicationPage? = entries[zoneId]

    override suspend fun put(
        zoneId: WatchZoneId,
        entry: CachedApplicationPage,
    ) {
        entries[zoneId] = entry
    }

    override suspend fun invalidate(zoneId: WatchZoneId) {
        invalidateCalls += zoneId
        entries.remove(zoneId)
    }

    override suspend fun invalidateAll() {
        invalidateAllCallCount++
        entries.clear()
    }
}
