package uk.towncrierapp.data.profile

import uk.towncrierapp.data.api.ApiClient
import uk.towncrierapp.data.api.FakeHttpTransport
import uk.towncrierapp.domain.auth.FakeAuthenticationService
import uk.towncrierapp.domain.profile.ServerProfile
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/** `POST /v1/me` (no body, idempotent) — ensures the server profile and returns it (epic #770). */
class ApiUserProfileRepositoryTest {
    @Test
    fun `ensureProfile POSTs to v1 me with no request body and parses the response`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, """{"userId":"auth0|1","pushEnabled":true,"tier":"Personal"}""")
            val apiClient = ApiClient("https://api-dev.towncrierapp.uk", transport, FakeAuthenticationService())
            val sut = ApiUserProfileRepository(apiClient)

            val profile = sut.ensureProfile()

            assertEquals(ServerProfile("auth0|1", pushEnabled = true, tier = SubscriptionTier.PERSONAL), profile)
            val request = transport.requests.single()
            assertEquals("POST", request.method)
            assertEquals("/v1/me", request.url.encodedPath)
            assertNull(request.header("Content-Type"))
        }

    @Test
    fun `an unrecognised tier string falls back to Free rather than crashing`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, """{"userId":"auth0|1","pushEnabled":false,"tier":"Enterprise"}""")
            val apiClient = ApiClient("https://api-dev.towncrierapp.uk", transport, FakeAuthenticationService())
            val sut = ApiUserProfileRepository(apiClient)

            val profile = sut.ensureProfile()

            assertEquals(SubscriptionTier.FREE, profile.tier)
        }
}
