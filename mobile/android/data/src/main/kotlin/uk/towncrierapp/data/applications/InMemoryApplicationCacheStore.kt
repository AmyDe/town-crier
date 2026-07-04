package uk.towncrierapp.data.applications

import uk.towncrierapp.domain.applications.ApplicationCacheStore
import uk.towncrierapp.domain.applications.CachedApplicationPage
import uk.towncrierapp.domain.watchzones.WatchZoneId
import java.util.concurrent.ConcurrentHashMap

/**
 * In-memory, process-lifetime [ApplicationCacheStore] — no persistence, no
 * TTL logic of its own (that policy lives entirely in
 * [uk.towncrierapp.domain.applications.OfflineAwareRepository], which is what
 * lets this be a plain keyed map). A `ConcurrentHashMap` is enough
 * concurrency safety for a handful of zone-keyed reads/writes; no explicit
 * locking is warranted. Port of iOS `InMemoryApplicationCacheStore` (GH#775).
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
