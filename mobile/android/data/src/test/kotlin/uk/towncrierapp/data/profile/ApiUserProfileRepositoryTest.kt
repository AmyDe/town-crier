package uk.towncrierapp.data.profile

import kotlinx.coroutines.test.runTest
import okhttp3.Request
import okio.Buffer
import org.junit.jupiter.api.Test
import uk.towncrierapp.data.api.ApiClient
import uk.towncrierapp.data.api.FakeHttpTransport
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.auth.FakeAuthenticationService
import uk.towncrierapp.domain.profile.DigestDay
import uk.towncrierapp.domain.profile.ServerProfile
import uk.towncrierapp.domain.profile.UserPreferences
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import kotlin.test.assertContentEquals
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** Reads a request's JSON body back out for assertions. */
private fun Request.bodyAsString(): String {
    val buffer = Buffer()
    body?.writeTo(buffer)
    return buffer.readUtf8()
}

/** `POST/GET/PATCH/DELETE /v1/me` + `GET /v1/me/data` (epic #770 / #778). */
class ApiUserProfileRepositoryTest {
    private fun makeSut(transport: FakeHttpTransport = FakeHttpTransport()): ApiUserProfileRepository {
        val apiClient = ApiClient("https://api-dev.towncrierapp.uk", transport, FakeAuthenticationService())
        return ApiUserProfileRepository(apiClient)
    }

    @Test
    fun `ensureProfile POSTs to v1 me with no request body and parses the response`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, """{"userId":"auth0|1","pushEnabled":true,"tier":"Personal"}""")
            val sut = makeSut(transport)

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
            val sut = makeSut(transport)

            val profile = sut.ensureProfile()

            assertEquals(SubscriptionTier.FREE, profile.tier)
        }

    @Test
    fun `ensureProfile response with no preference fields defaults them to the server's DefaultPreferences`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, """{"userId":"auth0|1","pushEnabled":true,"tier":"Free"}""")
            val sut = makeSut(transport)

            val profile = sut.ensureProfile()

            assertEquals(DigestDay.MONDAY, profile.digestDay)
            assertTrue(profile.emailDigestEnabled)
            assertTrue(profile.savedDecisionPush)
            assertTrue(profile.savedDecisionEmail)
        }

    @Test
    fun `fetchProfile GETs v1 me and parses the full field set`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(
                200,
                """{"userId":"auth0|1","pushEnabled":true,"digestDay":"Friday",""" +
                    """"emailDigestEnabled":false,"savedDecisionPush":false,"savedDecisionEmail":true,"tier":"Pro"}""",
            )
            val sut = makeSut(transport)

            val profile = sut.fetchProfile()

            assertEquals(
                ServerProfile(
                    userId = "auth0|1",
                    pushEnabled = true,
                    tier = SubscriptionTier.PRO,
                    digestDay = DigestDay.FRIDAY,
                    emailDigestEnabled = false,
                    savedDecisionPush = false,
                    savedDecisionEmail = true,
                ),
                profile,
            )
            val request = transport.requests.single()
            assertEquals("GET", request.method)
            assertEquals("/v1/me", request.url.encodedPath)
        }

    @Test
    fun `fetchProfile returns null on a 404 (no profile yet)`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(404)
            val sut = makeSut(transport)

            assertEquals(null, sut.fetchProfile())
        }

    @Test
    fun `fetchProfile propagates other failures`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(500, "boom")
            val sut = makeSut(transport)

            assertFailsWith<DomainError.ServerError> { sut.fetchProfile() }
        }

    @Test
    fun `updatePreferences PATCHes v1 me with the full five-field body`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(
                200,
                """{"userId":"auth0|1","pushEnabled":true,"digestDay":"Saturday",""" +
                    """"emailDigestEnabled":true,"savedDecisionPush":true,"savedDecisionEmail":false,"tier":"Personal"}""",
            )
            val sut = makeSut(transport)

            val updated =
                sut.updatePreferences(
                    UserPreferences(
                        pushEnabled = true,
                        digestDay = DigestDay.SATURDAY,
                        emailDigestEnabled = true,
                        savedDecisionPush = true,
                        savedDecisionEmail = false,
                    ),
                )

            assertEquals(DigestDay.SATURDAY, updated.digestDay)
            assertEquals(false, updated.savedDecisionEmail)
            val request = transport.requests.single()
            assertEquals("PATCH", request.method)
            assertEquals("/v1/me", request.url.encodedPath)
            val body = request.bodyAsString()
            assertTrue(body.contains(""""pushEnabled":true"""))
            assertTrue(body.contains(""""digestDay":"Saturday""""))
            assertTrue(body.contains(""""emailDigestEnabled":true"""))
            assertTrue(body.contains(""""savedDecisionPush":true"""))
            assertTrue(body.contains(""""savedDecisionEmail":false"""))
        }

    @Test
    fun `deleteAccount DELETEs v1 me`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(204)
            val sut = makeSut(transport)

            sut.deleteAccount()

            val request = transport.requests.single()
            assertEquals("DELETE", request.method)
            assertEquals("/v1/me", request.url.encodedPath)
        }

    @Test
    fun `deleteAccount propagates a server failure so the caller keeps the session`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(500, "boom")
            val sut = makeSut(transport)

            assertFailsWith<DomainError.ServerError> { sut.deleteAccount() }
        }

    @Test
    fun `exportData GETs v1 me data and returns the server bytes byte-for-byte unmodified`() =
        runTest {
            val transport = FakeHttpTransport()
            val scriptedBody = """{"profile":{"userId":"auth0|1"},"watchZones":[]}"""
            transport.enqueueResponse(200, scriptedBody)
            val sut = makeSut(transport)

            val bytes = sut.exportData()

            assertContentEquals(scriptedBody.toByteArray(Charsets.UTF_8), bytes)
            val request = transport.requests.single()
            assertEquals("GET", request.method)
            assertEquals("/v1/me/data", request.url.encodedPath)
        }
}
