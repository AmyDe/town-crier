package uk.towncrierapp.data.versionconfig

import uk.towncrierapp.data.api.FakeHttpTransport
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.versionconfig.AppVersion
import java.io.IOException
import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertIs
import kotlin.test.assertNull

/**
 * `GET /v1/version-config` is anonymous — no session, no `Authorization`
 * header — so it goes over the raw [uk.towncrierapp.data.api.HttpTransport],
 * never through [uk.towncrierapp.data.api.ApiClient] (which always requires
 * a session first). Must work before login for the force-update gate.
 */
class ApiVersionConfigServiceTest {
    private val baseUrl = "https://api-dev.towncrierapp.uk"

    @Test
    fun `fetchMinimumVersion GETs v1 version-config with no Authorization header`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, """{"minimumVersion":"1.2.0"}""")
            val sut = ApiVersionConfigService(baseUrl, transport)

            val version = sut.fetchMinimumVersion()

            assertEquals(AppVersion(1, 2, 0), version)
            val request = transport.requests.single()
            assertEquals("GET", request.method)
            assertEquals("/v1/version-config", request.url.encodedPath)
            assertNull(request.header("Authorization"))
        }

    @Test
    fun `a transport failure maps to NetworkUnavailable`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueFailure(IOException("down"))
            val sut = ApiVersionConfigService(baseUrl, transport)

            assertIs<DomainError.NetworkUnavailable>(assertFailsWith<DomainError> { sut.fetchMinimumVersion() })
        }

    @Test
    fun `a non-2xx response maps to a DomainError rather than throwing raw`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(500, "boom")
            val sut = ApiVersionConfigService(baseUrl, transport)

            assertFailsWith<DomainError> { sut.fetchMinimumVersion() }
        }
}
