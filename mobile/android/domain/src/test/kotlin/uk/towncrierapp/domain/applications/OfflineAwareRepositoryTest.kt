package uk.towncrierapp.domain.applications

import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.watchzones.WatchZoneId
import java.time.Clock
import java.time.Duration
import java.time.Instant
import java.time.ZoneOffset
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertIs
import kotlin.test.assertTrue

/**
 * The 900s TTL cache decorator over [PlanningApplicationRepository]'s
 * first-page (param-less) fetch, keyed by zone id only. Port of iOS
 * `OfflineAwareRepository` (GH#775).
 */
class OfflineAwareRepositoryTest {
    private val zoneId = WatchZoneId("wz-1")
    private val fixedNow = Instant.parse("2026-01-01T12:00:00Z")

    private fun clockAt(instant: Instant): Clock = Clock.fixed(instant, ZoneOffset.UTC)

    @Test
    fun `a fresh cache entry is served without calling the remote repository`() =
        runTest {
            val remote = FakePlanningApplicationRepository()
            val cache = FakeApplicationCacheStore()
            val cachedPage = anApplicationPage()
            cache.entries[zoneId] = CachedApplicationPage(cachedPage, cachedAt = fixedNow.minusSeconds(60))
            val sut = OfflineAwareRepository(remote, cache, clockAt(fixedNow))

            val result = sut.applications(zoneId, ApplicationSortOrder.RECENT_ACTIVITY)

            assertEquals(cachedPage, result)
            assertTrue(remote.applicationsCalls.isEmpty())
        }

    @Test
    fun `a cache entry exactly at the 900s TTL boundary is treated as stale`() =
        runTest {
            val remote = FakePlanningApplicationRepository().apply { applicationsResult = anApplicationPage() }
            val cache = FakeApplicationCacheStore()
            cache.entries[zoneId] =
                CachedApplicationPage(anApplicationPage(applications = emptyList()), cachedAt = fixedNow.minusSeconds(900))
            val sut = OfflineAwareRepository(remote, cache, clockAt(fixedNow))

            sut.applications(zoneId, ApplicationSortOrder.RECENT_ACTIVITY)

            assertEquals(1, remote.applicationsCalls.size)
        }

    @Test
    fun `a stale cache entry is refreshed from remote and re-cached on success`() =
        runTest {
            val freshPage = anApplicationPage()
            val remote = FakePlanningApplicationRepository().apply { applicationsResult = freshPage }
            val cache = FakeApplicationCacheStore()
            cache.entries[zoneId] =
                CachedApplicationPage(anApplicationPage(applications = emptyList()), cachedAt = fixedNow.minusSeconds(901))
            val sut = OfflineAwareRepository(remote, cache, clockAt(fixedNow))

            val result = sut.applications(zoneId, ApplicationSortOrder.RECENT_ACTIVITY)

            assertEquals(freshPage, result)
            assertEquals(freshPage, cache.entries.getValue(zoneId).page)
            assertEquals(fixedNow, cache.entries.getValue(zoneId).cachedAt)
        }

    @Test
    fun `a stale cache entry is served when the remote fetch fails offline (stale-offline)`() =
        runTest {
            val stalePage = anApplicationPage()
            val remote = FakePlanningApplicationRepository().apply { applicationsFailWith = DomainError.NetworkUnavailable }
            val cache = FakeApplicationCacheStore()
            cache.entries[zoneId] = CachedApplicationPage(stalePage, cachedAt = fixedNow.minusSeconds(901))
            val sut = OfflineAwareRepository(remote, cache, clockAt(fixedNow))

            val result = sut.applications(zoneId, ApplicationSortOrder.RECENT_ACTIVITY)

            assertEquals(stalePage, result)
        }

    @Test
    fun `no cache entry and an offline remote failure throws NetworkUnavailable (no-cache-offline)`() =
        runTest {
            val remote = FakePlanningApplicationRepository().apply { applicationsFailWith = DomainError.NetworkUnavailable }
            val cache = FakeApplicationCacheStore()
            val sut = OfflineAwareRepository(remote, cache, clockAt(fixedNow))

            assertIs<DomainError.NetworkUnavailable>(
                assertFailsWith<DomainError> { sut.applications(zoneId, ApplicationSortOrder.RECENT_ACTIVITY) },
            )
        }

    @Test
    fun `a cached entry is served when an online (non-network) remote failure occurs`() =
        runTest {
            val cachedPage = anApplicationPage()
            val remote = FakePlanningApplicationRepository().apply { applicationsFailWith = DomainError.ServerError(500, "boom") }
            val cache = FakeApplicationCacheStore()
            cache.entries[zoneId] = CachedApplicationPage(cachedPage, cachedAt = fixedNow.minusSeconds(901))
            val sut = OfflineAwareRepository(remote, cache, clockAt(fixedNow))

            val result = sut.applications(zoneId, ApplicationSortOrder.RECENT_ACTIVITY)

            assertEquals(cachedPage, result)
        }

    @Test
    fun `no cache entry and a non-network remote failure propagates the original error`() =
        runTest {
            val remote = FakePlanningApplicationRepository().apply { applicationsFailWith = DomainError.ServerError(500, "boom") }
            val cache = FakeApplicationCacheStore()
            val sut = OfflineAwareRepository(remote, cache, clockAt(fixedNow))

            val error =
                assertIs<DomainError.ServerError>(
                    assertFailsWith<DomainError> { sut.applications(zoneId, ApplicationSortOrder.RECENT_ACTIVITY) },
                )
            assertEquals(500, error.status)
        }

    @Test
    fun `a paged fetch (non-null cursor) bypasses the cache entirely`() =
        runTest {
            val remote = FakePlanningApplicationRepository().apply { applicationsResult = anApplicationPage() }
            val cache = FakeApplicationCacheStore()
            cache.entries[zoneId] = CachedApplicationPage(anApplicationPage(applications = emptyList()), cachedAt = fixedNow)
            val sut = OfflineAwareRepository(remote, cache, clockAt(fixedNow))

            sut.applications(zoneId, ApplicationSortOrder.RECENT_ACTIVITY, cursor = "next-page-cursor")

            assertEquals(1, remote.applicationsCalls.size)
            assertEquals("next-page-cursor", remote.applicationsCalls.single().cursor)
            // The cache is untouched by a paged fetch — still holds only the original first-page entry.
            assertEquals(0, cache.entries.getValue(zoneId).page.applications.size)
        }

    @Test
    fun `detail delegates straight to the remote repository, uncached`() =
        runTest {
            val detail = aPlanningApplication()
            val remote = FakePlanningApplicationRepository().apply { detailResult = detail }
            val cache = FakeApplicationCacheStore()
            val sut = OfflineAwareRepository(remote, cache, clockAt(fixedNow))

            val result = sut.detail("42", "24/0001")

            assertEquals(detail, result)
            assertEquals("42" to "24/0001", remote.detailCalls.single())
        }

    @Test
    fun `the TTL constant is 900 seconds`() {
        assertEquals(Duration.ofSeconds(900), OfflineAwareRepository.TTL)
    }
}
