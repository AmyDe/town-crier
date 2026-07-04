package uk.towncrierapp.data.api

import uk.towncrierapp.domain.auth.FakeAuthenticationService
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.Serializable
import kotlinx.serialization.builtins.ListSerializer
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

@Serializable
private data class PagedRow(val id: String)

/**
 * `requestPaged` surfaces the opaque `X-Next-Cursor` response header
 * alongside the decoded body — used by keyset-paged list endpoints (later
 * issues). `requestBytes` returns the raw response body untouched, for
 * opaque payloads (e.g. the future GDPR export). Port of iOS
 * `URLSessionAPIClientPagedTests`.
 */
class ApiClientPagedTest {
    private val baseUrl = "https://api-dev.towncrierapp.uk"

    private fun makeSut(transport: FakeHttpTransport) = ApiClient(baseUrl, transport, FakeAuthenticationService())

    @Test
    fun `requestPaged surfaces the X-Next-Cursor response header`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, """[{"id":"a"}]""", headers = mapOf("X-Next-Cursor" to "cursor-xyz"))
            val sut = makeSut(transport)

            val page = sut.requestPaged(ApiEndpoint.get("/things"), ListSerializer(PagedRow.serializer()))

            assertEquals(listOf(PagedRow("a")), page.value)
            assertEquals("cursor-xyz", page.nextCursor)
        }

    @Test
    fun `requestPaged returns a null cursor when the header is absent (last page)`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, """[{"id":"a"}]""")
            val sut = makeSut(transport)

            val page = sut.requestPaged(ApiEndpoint.get("/things"), ListSerializer(PagedRow.serializer()))

            assertNull(page.nextCursor)
        }

    @Test
    fun `requestBytes returns the raw response body untouched`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, """{"opaque":"payload"}""")
            val sut = makeSut(transport)

            val bytes = sut.requestBytes(ApiEndpoint.get("/v1/me/data"))

            assertEquals("""{"opaque":"payload"}""", String(bytes))
        }
}
