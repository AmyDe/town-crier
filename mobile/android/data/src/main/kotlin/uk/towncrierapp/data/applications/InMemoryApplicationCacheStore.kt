package uk.towncrierapp.data.applications

import uk.towncrierapp.domain.applications.ApplicationCacheStore
import uk.towncrierapp.domain.applications.CachedApplicationPage
import uk.towncrierapp.domain.watchzones.WatchZoneId
import java.util.concurrent.ConcurrentHashMap

/**
 * In-memory, process-lifetime [ApplicationCacheStore]. TTL policy lives in
 * [uk.towncrierapp.domain.applications.OfflineAwareRepository].
 */
public class InMemoryApplicationCacheStore : ApplicationCacheStore {
    private val entries = ConcurrentHashMap<WatchZoneId, CachedApplicationPage>()

    override suspend fun get(zoneId: WatchZoneId): CachedApplicationPage? = entries[zoneId]

    override suspend fun put(
        zoneId: WatchZoneId,
        entry: CachedApplicationPage,
    ) {
        entries[zoneId] = entry
    }

    override suspend fun invalidate(zoneId: WatchZoneId) {
        entries.remove(zoneId)
    }

    override suspend fun invalidateAll() {
        entries.clear()
    }
}
