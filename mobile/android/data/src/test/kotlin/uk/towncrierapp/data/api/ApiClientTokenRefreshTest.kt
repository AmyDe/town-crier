package uk.towncrierapp.data.api

import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.auth.FakeAuthenticationService
import uk.towncrierapp.domain.auth.anAuthSession
import java.io.IOException
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertIs

/**
 * The single 401-refresh-retry policy (epic #770 "API contract essentials",
 * acceptance criteria) — port of iOS `URLSessionAPIClientTokenRefreshTests`.
 * Refresh throwing [IOException] maps to [DomainError.NetworkUnavailable];
 * any other refresh failure maps to [DomainError.SessionExpired]. No status
 * code other than 401 ever triggers a retry.
 */
class ApiClientTokenRefreshTest {
    private val baseUrl = "https://api-dev.towncrierapp.uk"

    @Test
    fun `on 401, refreshes the session exactly once and retries the request exactly once`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(401)
            transport.enqueueResponse(200, """{"id":"1","name":"Test"}""")
            val refreshed = anAuthSession(accessToken = "refreshed-token")
            val authService =
                FakeAuthenticationService(currentSessionResult = anAuthSession(accessToken = "stale-token")).apply {
                    refreshSessionResult = Result.success(refreshed)
                }
            val sut = ApiClient(baseUrl, transport, authService)

            val result = sut.request(ApiEndpoint.get("/applications"), TestResponse.serializer())

            assertEquals(TestResponse("1", "Test"), result)
            assertEquals(1, authService.refreshSessionCalls.size)
            assertEquals(2, transport.requests.size)
            assertEquals("Bearer stale-token", transport.requests[0].header("Authorization"))
            assertEquals("Bearer refreshed-token", transport.requests[1].header("Authorization"))
        }

    @Test
    fun `on 401, when refresh throws IOException, throws NetworkUnavailable`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(401)
            val authService =
                FakeAuthenticationService().apply {
                    refreshSessionResult = Result.failure(IOException("no connection"))
                }
            val sut = ApiClient(baseUrl, transport, authService)

            assertIs<DomainError.NetworkUnavailable>(
                assertFailsWith<DomainError> {
                    sut.request(
                        ApiEndpoint.get("/applications"),
                        TestResponse.serializer(),
                    )
                },
            )
        }

    @Test
    fun `on 401, when refresh fails for any other reason, throws SessionExpired`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(401)
            val authService =
                FakeAuthenticationService().apply {
                    refreshSessionResult = Result.failure(DomainError.SessionExpired)
                }
            val sut = ApiClient(baseUrl, transport, authService)

            assertIs<DomainError.SessionExpired>(
                assertFailsWith<DomainError> {
                    sut.request(
                        ApiEndpoint.get("/applications"),
                        TestResponse.serializer(),
                    )
                },
            )
        }

    @Test
    fun `a 5xx response never triggers a refresh — only 401 does`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(500)
            val authService = FakeAuthenticationService()
            val sut = ApiClient(baseUrl, transport, authService)

            assertFailsWith<DomainError.ServerError> {
                sut.request(ApiEndpoint.get("/applications"), TestResponse.serializer())
            }

            assertEquals(0, authService.refreshSessionCalls.size)
            assertEquals(1, transport.requests.size)
        }

    @Test
    fun `on 401, when the refreshed retry also fails over the network, throws NetworkUnavailable`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(401)
            transport.enqueueFailure(IOException("dropped"))
            val authService =
                FakeAuthenticationService().apply {
                    refreshSessionResult = Result.success(anAuthSession())
                }
            val sut = ApiClient(baseUrl, transport, authService)

            assertIs<DomainError.NetworkUnavailable>(
                assertFailsWith<DomainError> {
                    sut.request(
                        ApiEndpoint.get("/applications"),
                        TestResponse.serializer(),
                    )
                },
            )
        }

    @Test
    fun `on 401, when the refreshed retry also gets a server error, propagates it rather than looping`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(401)
            transport.enqueueResponse(500)
            val authService =
                FakeAuthenticationService().apply {
                    refreshSessionResult = Result.success(anAuthSession())
                }
            val sut = ApiClient(baseUrl, transport, authService)

            assertFailsWith<DomainError.ServerError> {
                sut.request(ApiEndpoint.get("/applications"), TestResponse.serializer())
            }
            assertEquals(1, authService.refreshSessionCalls.size)
        }
}
