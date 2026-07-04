package uk.towncrierapp.data.applications

import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import uk.towncrierapp.domain.applications.ApplicationPage
import uk.towncrierapp.domain.applications.CachedApplicationPage
import uk.towncrierapp.domain.watchzones.WatchZoneId
import java.time.Instant
import kotlin.test.assertEquals
import kotlin.test.assertNull

/** Plain keyed storage, no TTL logic of its own (see `OfflineAwareRepository` for that policy). Port of iOS `InMemoryApplicationCacheStore` (GH#775). */
class InMemoryApplicationCacheStoreTest {
    @Test
    fun `get returns null when nothing has been cached for a zone`() =
        runTest {
            val sut = InMemoryApplicationCacheStore()

            assertNull(sut.get(WatchZoneId("wz-1")))
        }

    @Test
    fun `put then get round-trips the cached entry`() =
        runTest {
            val sut = InMemoryApplicationCacheStore()
            val entry = CachedApplicationPage(ApplicationPage(emptyList()), Instant.parse("2026-01-01T00:00:00Z"))

            sut.put(WatchZoneId("wz-1"), entry)

            assertEquals(entry, sut.get(WatchZoneId("wz-1")))
        }

    @Test
    fun `invalidate removes only the entry for that zone`() =
        runTest {
            val sut = InMemoryApplicationCacheStore()
            val entry = CachedApplicationPage(ApplicationPage(emptyList()), Instant.parse("2026-01-01T00:00:00Z"))
            sut.put(WatchZoneId("wz-1"), entry)
            sut.put(WatchZoneId("wz-2"), entry)

            sut.invalidate(WatchZoneId("wz-1"))

            assertNull(sut.get(WatchZoneId("wz-1")))
            assertEquals(entry, sut.get(WatchZoneId("wz-2")))
        }

    @Test
    fun `invalidateAll clears every zone's entry`() =
        runTest {
            val sut = InMemoryApplicationCacheStore()
            val entry = CachedApplicationPage(ApplicationPage(emptyList()), Instant.parse("2026-01-01T00:00:00Z"))
            sut.put(WatchZoneId("wz-1"), entry)
            sut.put(WatchZoneId("wz-2"), entry)

            sut.invalidateAll()

            assertNull(sut.get(WatchZoneId("wz-1")))
            assertNull(sut.get(WatchZoneId("wz-2")))
        }
}
