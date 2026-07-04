package uk.towncrierapp.data.applications

import kotlinx.coroutines.test.runTest
import okhttp3.Request
import okio.Buffer
import org.junit.jupiter.api.Test
import uk.towncrierapp.data.api.ApiClient
import uk.towncrierapp.data.api.FakeHttpTransport
import uk.towncrierapp.domain.applications.PlanningApplicationId
import uk.towncrierapp.domain.auth.FakeAuthenticationService
import kotlin.test.assertEquals
import kotlin.test.assertTrue

private fun Request.bodyAsString(): String {
    val buffer = Buffer()
    body?.writeTo(buffer)
    return buffer.readUtf8()
}

/**
 * `GET /v1/me/notification-state`, `POST /v1/me/applications/mark-read`
 * (⚠️ the `applicationUid` wire key holds the case NAME, i.e. [PlanningApplicationId.name],
 * not the uid — ported as-is, not "fixed"), and
 * `POST /v1/me/notification-state/mark-all-read`. Port of the iOS
 * `APINotificationStateRepositoryTests` suite (GH#775). The legacy `/advance`
 * endpoint is deliberately not implemented.
 */
class ApiNotificationStateRepositoryTest {
    private fun makeSut(transport: FakeHttpTransport = FakeHttpTransport()): ApiNotificationStateRepository {
        val apiClient = ApiClient("https://api-dev.towncrierapp.uk", transport, FakeAuthenticationService())
        return ApiNotificationStateRepository(apiClient)
    }

    @Test
    fun `state GETs and decodes lastReadAt, version and totalUnreadCount`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, """{"lastReadAt":"2026-01-01T00:00:00Z","version":3,"totalUnreadCount":7}""")
            val sut = makeSut(transport)

            val state = sut.state()

            assertEquals(
                "/v1/me/notification-state",
                transport.requests
                    .single()
                    .url.encodedPath,
            )
            assertEquals(3, state.version)
            assertEquals(7, state.totalUnreadCount)
        }

    @Test
    fun `markRead POSTs applicationUid as the case name (not the uid) and authorityId as an int`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(204)
            val sut = makeSut(transport)

            sut.markRead(listOf(PlanningApplicationId(authority = "42", name = "24/0001")))

            val request = transport.requests.single()
            assertEquals("POST", request.method)
            assertEquals("/v1/me/applications/mark-read", request.url.encodedPath)
            assertEquals(
                """{"applications":[{"applicationUid":"24/0001","authorityId":42}]}""",
                request.bodyAsString(),
            )
        }

    @Test
    fun `markRead batches more than 500 ids into multiple requests`() =
        runTest {
            val transport = FakeHttpTransport()
            repeat(2) { transport.enqueueResponse(204) }
            val sut = makeSut(transport)
            val ids = (1..501).map { PlanningApplicationId(authority = "42", name = "24/$it") }

            sut.markRead(ids)

            assertEquals(2, transport.requests.size)
            assertTrue(transport.requests[0].bodyAsString().contains(""""authorityId":42"""))
        }

    @Test
    fun `markAllRead POSTs with no body`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(204)
            val sut = makeSut(transport)

            sut.markAllRead()

            val request = transport.requests.single()
            assertEquals("POST", request.method)
            assertEquals("/v1/me/notification-state/mark-all-read", request.url.encodedPath)
        }
}
