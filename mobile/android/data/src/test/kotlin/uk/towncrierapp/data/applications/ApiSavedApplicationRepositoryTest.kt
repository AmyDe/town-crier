package uk.towncrierapp.data.applications

import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import uk.towncrierapp.data.api.ApiClient
import uk.towncrierapp.data.api.FakeHttpTransport
import uk.towncrierapp.domain.applications.PlanningApplicationId
import uk.towncrierapp.domain.auth.FakeAuthenticationService
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * `GET/PUT/DELETE /v1/me/saved-applications` — the flat, cross-zone saved
 * list (client-sorted `savedAt` DESC by the ViewModel, not here), legacy
 * null-payload rows, and the save/unsave path segment built from
 * [PlanningApplicationId.value] with its slash(es) percent-encoded so the
 * whole id stays ONE path segment (GH#775). Port of the iOS
 * `APISavedApplicationRepositoryTests` suite.
 */
class ApiSavedApplicationRepositoryTest {
    private fun makeSut(transport: FakeHttpTransport = FakeHttpTransport()): ApiSavedApplicationRepository {
        val apiClient = ApiClient("https://api-dev.towncrierapp.uk", transport, FakeAuthenticationService())
        return ApiSavedApplicationRepository(apiClient)
    }

    @Test
    fun `savedApplications GETs the flat list and decodes each row`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(
                200,
                """[{"applicationUid":"42/24/0001","savedAt":"2026-01-16T09:00:00Z","application":{"name":"24/0001",""" +
                    """"uid":"uid-1","areaName":"Camden","areaId":42,"address":"1 Example Street",""" +
                    """"description":"Extension","appState":"Undecided","startDate":"2026-01-10",""" +
                    """"lastDifferent":"2026-01-11T09:00:00Z"}}]""",
            )
            val sut = makeSut(transport)

            val saved = sut.savedApplications()

            val request = transport.requests.single()
            assertEquals("GET", request.method)
            assertEquals("/v1/me/saved-applications", request.url.encodedPath)
            val row = saved.single()
            assertEquals(PlanningApplicationId("42", "24/0001"), row.applicationUid)
            assertEquals("24/0001", row.application?.reference)
        }

    @Test
    fun `a legacy row with a null application payload decodes with a null application`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(
                200,
                """[{"applicationUid":"42/24/9999","savedAt":"2026-01-16T09:00:00Z","application":null}]""",
            )
            val sut = makeSut(transport)

            val row = sut.savedApplications().single()

            assertNull(row.application)
        }

    @Test
    fun `save PUTs to a single, correctly-encoded path segment built from id-value`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200)
            val sut = makeSut(transport)

            sut.save(PlanningApplicationId(authority = "42", name = "24/0001"))

            val request = transport.requests.single()
            assertEquals("PUT", request.method)
            // Exactly 4 path segments: v1, me, saved-applications, <the whole id as one>.
            assertEquals(4, request.url.pathSegments.size)
            assertEquals("42/24/0001", request.url.pathSegments.last())
            assertTrue(request.url.encodedPath.endsWith("saved-applications/42%2F24%2F0001"))
        }

    @Test
    fun `unsave DELETEs the same correctly-encoded path segment`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(204)
            val sut = makeSut(transport)

            sut.unsave(PlanningApplicationId(authority = "42", name = "24/0001"))

            val request = transport.requests.single()
            assertEquals("DELETE", request.method)
            assertEquals(4, request.url.pathSegments.size)
            assertEquals("42/24/0001", request.url.pathSegments.last())
        }
}
