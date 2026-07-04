package uk.towncrierapp.data.watchzones

import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import uk.towncrierapp.data.api.ApiClient
import uk.towncrierapp.data.api.FakeHttpTransport
import uk.towncrierapp.domain.auth.FakeAuthenticationService
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.domain.watchzones.aZoneNotificationPreferences
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/** `GET/PUT /v1/me/watch-zones/{zoneId}/preferences` — port of iOS `APIZonePreferencesRepositoryTests` (tc-z95t). */
class ApiZonePreferencesRepositoryTest {
    private fun makeSut(transport: FakeHttpTransport = FakeHttpTransport()): ApiZonePreferencesRepository {
        val apiClient = ApiClient("https://api-dev.towncrierapp.uk", transport, FakeAuthenticationService())
        return ApiZonePreferencesRepository(apiClient)
    }

    @Test
    fun `fetchPreferences GETs and decodes the preferences`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(
                200,
                """{"zoneId":"wz-1","newApplicationPush":false,"newApplicationEmail":true,""" +
                    """"decisionPush":true,"decisionEmail":false}""",
            )
            val sut = makeSut(transport)

            val prefs = sut.fetchPreferences(WatchZoneId("wz-1"))

            assertEquals("wz-1", prefs.zoneId.value)
            assertEquals(false, prefs.newApplicationPush)
            assertEquals(true, prefs.newApplicationEmail)
            assertEquals(true, prefs.decisionPush)
            assertEquals(false, prefs.decisionEmail)
            val request = transport.requests.single()
            assertEquals("GET", request.method)
            assertEquals("/v1/me/watch-zones/wz-1/preferences", request.url.encodedPath)
        }

    @Test
    fun `updatePreferences PUTs the full preferences body`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, "{}")
            val sut = makeSut(transport)

            sut.updatePreferences(
                aZoneNotificationPreferences(zoneId = WatchZoneId("wz-1"), newApplicationPush = false),
            )

            val request = transport.requests.single()
            assertEquals("PUT", request.method)
            assertEquals("/v1/me/watch-zones/wz-1/preferences", request.url.encodedPath)
            assertTrue(request.header("Content-Type") == "application/json")
        }
}
