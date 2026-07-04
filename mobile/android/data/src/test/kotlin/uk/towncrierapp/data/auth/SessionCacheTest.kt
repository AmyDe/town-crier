package uk.towncrierapp.data.auth

import uk.towncrierapp.domain.auth.anAuthSession
import java.time.Clock
import java.time.Instant
import java.time.ZoneOffset
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.async
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/**
 * In-memory session cache (iOS tc-3d7b): a hit while the access token has
 * >= [SessionCache]'s lead time to expiry avoids a `SecureCredentialsManager`
 * read on every call, and concurrent cold reads single-flight to one loader
 * invocation rather than each hitting the credentials store.
 */
class SessionCacheTest {
    private val expiresAt = Instant.parse("2026-07-20T15:00:00Z")

    private fun clockAt(instant: Instant): Clock = Clock.fixed(instant, ZoneOffset.UTC)

    @Test
    fun `current is null when nothing has been cached yet`() =
        runTest {
            val cache = SessionCache(scope = backgroundScope)

            assertNull(cache.current(clockAt(expiresAt.minusSeconds(120))))
        }

    @Test
    fun `current returns the cached session while the access token has at least the lead time left`() =
        runTest {
            val cache = SessionCache(scope = backgroundScope, leadTimeSeconds = 60)
            val session = anAuthSession(expiresAt = expiresAt)
            cache.store(session)

            val result = cache.current(clockAt(expiresAt.minusSeconds(61)))

            assertEquals(session, result)
        }

    @Test
    fun `current is null once the access token is within the lead time of expiry`() =
        runTest {
            val cache = SessionCache(scope = backgroundScope, leadTimeSeconds = 60)
            cache.store(anAuthSession(expiresAt = expiresAt))

            assertNull(cache.current(clockAt(expiresAt.minusSeconds(59))))
            assertNull(cache.current(clockAt(expiresAt)))
        }

    @Test
    fun `currentOrLoad returns the cached session without invoking the loader when the cache is valid`() =
        runTest {
            val cache = SessionCache(scope = backgroundScope, leadTimeSeconds = 60)
            val session = anAuthSession(expiresAt = expiresAt)
            cache.store(session)
            var loaderCalls = 0

            val result = cache.currentOrLoad(clockAt(expiresAt.minusSeconds(120))) { loaderCalls++; anAuthSession() }

            assertEquals(session, result)
            assertEquals(0, loaderCalls)
        }

    @Test
    fun `currentOrLoad invokes the loader and caches the result on a cold cache`() =
        runTest {
            val cache = SessionCache(scope = backgroundScope)
            val loaded = anAuthSession(accessToken = "freshly-loaded")

            val result = cache.currentOrLoad(clockAt(expiresAt.minusSeconds(120))) { loaded }

            assertEquals(loaded, result)
            assertEquals(loaded, cache.current(clockAt(expiresAt.minusSeconds(120))))
        }

    @Test
    fun `concurrent cold callers single-flight to exactly one loader invocation`() =
        runTest {
            val cache = SessionCache(scope = backgroundScope)
            var loaderCalls = 0
            val gate = CompletableDeferred<Unit>()
            val loader: suspend () -> uk.towncrierapp.domain.auth.AuthSession? = {
                loaderCalls++
                gate.await()
                anAuthSession()
            }
            val clock = clockAt(expiresAt.minusSeconds(120))

            val first = async { cache.currentOrLoad(clock, loader) }
            val second = async { cache.currentOrLoad(clock, loader) }
            advanceUntilIdle()
            assertEquals(1, loaderCalls)

            gate.complete(Unit)
            assertEquals(first.await(), second.await())
        }

    @Test
    fun `clear drops the cached session`() =
        runTest {
            val cache = SessionCache(scope = backgroundScope)
            cache.store(anAuthSession(expiresAt = expiresAt))

            cache.clear()

            assertNull(cache.current(clockAt(expiresAt.minusSeconds(120))))
        }
}
