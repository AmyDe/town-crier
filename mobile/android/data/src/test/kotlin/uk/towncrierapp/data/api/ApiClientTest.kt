package uk.towncrierapp.data.api

import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.auth.FakeAuthenticationService
import uk.towncrierapp.domain.auth.anAuthSession
import java.io.IOException
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertIs
import kotlin.test.assertNull

/**
 * Core `ApiClient` behaviour: headers, JSON decoding, and HTTP status
 * mapping to `DomainError` — port of iOS `URLSessionAPIClientTests` (epic
 * #770 "API contract essentials"). 401-refresh-retry and paging have their
 * own files ([ApiClientTokenRefreshTest], [ApiClientPagedTest]).
 */
class ApiClientTest {
    private val baseUrl = "https://api-dev.towncrierapp.uk"

    private fun makeSut(
        transport: FakeHttpTransport = FakeHttpTransport(),
        authService: FakeAuthenticationService = FakeAuthenticationService(),
    ) = ApiClient(baseUrl = baseUrl, transport = transport, authService = authService)

    @Test
    fun `GET request attaches the Bearer token and Accept header, and no Content-Type without a body`() =
        runTest {
            val transport = FakeHttpTransport()
            val authService = FakeAuthenticationService(currentSessionResult = anAuthSession(accessToken = "test-token"))
            transport.enqueueResponse(200, """{"id":"1","name":"Test"}""")
            val sut = makeSut(transport, authService)

            sut.request(ApiEndpoint.get("/applications"), TestResponse.serializer())

            val request = transport.requests.single()
            assertEquals("Bearer test-token", request.header("Authorization"))
            assertEquals("application/json", request.header("Accept"))
            assertNull(request.header("Content-Type"))
            assertEquals("GET", request.method)
            assertEquals("https://api-dev.towncrierapp.uk/applications", request.url.toString())
        }

    @Test
    fun `a successful response is decoded to the requested type`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, """{"id":"42","name":"Extension"}""")
            val sut = makeSut(transport)

            val result = sut.request(ApiEndpoint.get("/applications"), TestResponse.serializer())

            assertEquals(TestResponse("42", "Extension"), result)
        }

    @Test
    fun `POST request sets Content-Type and encodes the body as JSON`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(201, """{"id":"new","name":"Created"}""")
            val sut = makeSut(transport)
            val body = Json.encodeToString(TestBody.serializer(), TestBody("New Application"))

            val result = sut.request(ApiEndpoint.post("/applications", body = body), TestResponse.serializer())

            assertEquals(TestResponse("new", "Created"), result)
            val request = transport.requests.single()
            assertEquals("POST", request.method)
            assertEquals("application/json", request.header("Content-Type"))
        }

    @Test
    fun `a bodyless POST sends no Content-Type header`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, """{"id":"1","name":"Test"}""")
            val sut = makeSut(transport)

            sut.request(ApiEndpoint.post("/v1/me"), TestResponse.serializer())

            val request = transport.requests.single()
            assertEquals("POST", request.method)
            assertNull(request.header("Content-Type"))
        }

    @Test
    fun `query parameters are included in the request URL`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, """{"id":"1","name":"Test"}""")
            val sut = makeSut(transport)

            sut.request(
                ApiEndpoint.get("/applications", query = listOf("authority" to "camden", "status" to "pending")),
                TestResponse.serializer(),
            )

            val url = transport.requests.single().url
            assertEquals("camden", url.queryParameter("authority"))
            assertEquals("pending", url.queryParameter("status"))
        }

    @Test
    fun `when no session exists, throws SessionExpired without calling the transport`() =
        runTest {
            val transport = FakeHttpTransport()
            val authService = FakeAuthenticationService(currentSessionResult = null)
            val sut = makeSut(transport, authService)

            assertIs<DomainError.SessionExpired>(
                assertFailsWith<DomainError> { sut.request(ApiEndpoint.get("/applications"), TestResponse.serializer()) },
            )
            assertEquals(0, transport.requests.size)
        }

    @Test
    fun `404 response throws DomainError NotFound`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(404)
            val sut = makeSut(transport)

            assertIs<DomainError.NotFound>(
                assertFailsWith<DomainError> { sut.request(ApiEndpoint.get("/applications/999"), TestResponse.serializer()) },
            )
        }

    @Test
    fun `500 response throws DomainError ServerError carrying the status and raw body, with no retry`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(500, "boom")
            val sut = makeSut(transport)

            val error =
                assertIs<DomainError.ServerError>(
                    assertFailsWith<DomainError> { sut.request(ApiEndpoint.get("/applications"), TestResponse.serializer()) },
                )

            assertEquals(500, error.status)
            assertEquals("boom", error.body)
            assertEquals(1, transport.requests.size)
        }

    @Test
    fun `403 with an insufficient_entitlement body throws the typed DomainError`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(403, """{"error":"insufficient_entitlement","required":"personal"}""")
            val sut = makeSut(transport)

            val error =
                assertIs<DomainError.InsufficientEntitlement>(
                    assertFailsWith<DomainError> { sut.request(ApiEndpoint.get("/notifications"), TestResponse.serializer()) },
                )

            assertEquals("personal", error.required)
        }

    @Test
    fun `403 without the insufficient_entitlement shape throws a generic ServerError`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(403, """{"error":"forbidden","message":"Access denied"}""")
            val sut = makeSut(transport)

            val error =
                assertIs<DomainError.ServerError>(
                    assertFailsWith<DomainError> { sut.request(ApiEndpoint.get("/admin"), TestResponse.serializer()) },
                )

            assertEquals(403, error.status)
        }

    @Test
    fun `a transport-level network failure maps to NetworkUnavailable`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueFailure(IOException("no connection"))
            val sut = makeSut(transport)

            assertIs<DomainError.NetworkUnavailable>(
                assertFailsWith<DomainError> { sut.request(ApiEndpoint.get("/applications"), TestResponse.serializer()) },
            )
        }
}
