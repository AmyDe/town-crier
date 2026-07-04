package uk.towncrierapp.data.watchzones

import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import uk.towncrierapp.data.api.ApiClient
import uk.towncrierapp.data.api.FakeHttpTransport
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.auth.FakeAuthenticationService
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith

/** `GET /v1/geocode/{postcode}` — port of iOS `APIPostcodeGeocoder` behaviour (tc-z95t). */
class ApiPostcodeGeocoderTest {
    private fun makeSut(transport: FakeHttpTransport = FakeHttpTransport()): ApiPostcodeGeocoder {
        val apiClient = ApiClient("https://api-dev.towncrierapp.uk", transport, FakeAuthenticationService())
        return ApiPostcodeGeocoder(apiClient)
    }

    @Test
    fun `geocode GETs the postcode path and decodes the coordinate`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, """{"coordinates":{"latitude":52.2053,"longitude":0.1218}}""")
            val sut = makeSut(transport)

            val coordinate = sut.geocode("CB1 2AD")

            assertEquals(52.2053, coordinate.latitude)
            assertEquals(0.1218, coordinate.longitude)
            val request = transport.requests.single()
            assertEquals("GET", request.method)
            assertEquals("/v1/geocode/CB1 2AD", request.url.let { java.net.URLDecoder.decode(it.encodedPath, "UTF-8") })
        }

    @Test
    fun `an unresolvable postcode surfaces NotFound`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(404, """{"error":"Postcode 'ZZ99 9ZZ' could not be geocoded."}""")
            val sut = makeSut(transport)

            assertFailsWith<DomainError.NotFound> { sut.geocode("ZZ99 9ZZ") }
        }
}
