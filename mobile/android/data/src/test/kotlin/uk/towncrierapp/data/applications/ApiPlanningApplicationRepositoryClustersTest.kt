package uk.towncrierapp.data.applications

import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import uk.towncrierapp.data.api.ApiClient
import uk.towncrierapp.data.api.FakeHttpTransport
import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.auth.FakeAuthenticationService
import uk.towncrierapp.domain.map.aMapViewport
import uk.towncrierapp.domain.watchzones.WatchZoneId
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * `GET /v1/me/watch-zones/{zoneId}/applications/clusters` (GH#698, GH#776):
 * bbox/zoom/status query building and DTO->domain mapping of the three
 * cluster shapes (single/splittable/stacked). Server contract per
 * `api-go/internal/applications/zoneclusters.go`'s `Cluster`.
 */
class ApiPlanningApplicationRepositoryClustersTest {
    private val zoneId = WatchZoneId("wz-1")
    private val viewport = aMapViewport()

    private fun makeSut(transport: FakeHttpTransport = FakeHttpTransport()): ApiPlanningApplicationRepository {
        val apiClient = ApiClient("https://api-dev.towncrierapp.uk", transport, FakeAuthenticationService())
        return ApiPlanningApplicationRepository(apiClient)
    }

    @Test
    fun `fetchClusters GETs the zone clusters endpoint sending bbox and zoom, no status by default`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, "[]")
            val sut = makeSut(transport)

            sut.fetchClusters(zoneId, viewport, zoom = 12)

            val request = transport.requests.single()
            assertEquals("GET", request.method)
            assertEquals("/v1/me/watch-zones/wz-1/applications/clusters", request.url.encodedPath)
            assertEquals(viewport.queryValue, request.url.queryParameter("bbox"))
            assertEquals("12", request.url.queryParameter("zoom"))
            assertNull(request.url.queryParameter("status"))
        }

    @Test
    fun `fetchClusters sends status as the raw app_state wire value when provided`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, "[]")
            val sut = makeSut(transport)

            sut.fetchClusters(zoneId, viewport, zoom = 12, status = ApplicationStatus.Permitted)

            assertEquals(
                "Permitted",
                transport.requests
                    .single()
                    .url
                    .queryParameter("status"),
            )
        }

    @Test
    fun `a count-1 cell maps to a single-member cluster carrying the member id`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(
                200,
                """[{"latitude":51.5,"longitude":-0.1,"count":1,"statusCounts":{"Undecided":1},""" +
                    """"applicationId":{"authority":"42","name":"24/0001"}}]""",
            )
            val sut = makeSut(transport)

            val cluster = sut.fetchClusters(zoneId, viewport, zoom = 12).single()

            assertTrue(cluster.isSingleMember)
            assertFalse(cluster.isStacked)
            assertEquals("42", cluster.member?.authority)
            assertEquals("24/0001", cluster.member?.name)
            assertEquals(51.5, cluster.coordinate.latitude)
            assertEquals(-0.1, cluster.coordinate.longitude)
            assertEquals<Map<ApplicationStatus, Int>>(mapOf(ApplicationStatus.Undecided to 1), cluster.statusCounts)
        }

    @Test
    fun `a splittable multi-member cell maps with no applicationId and no carried members`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(
                200,
                """[{"latitude":51.5,"longitude":-0.1,"count":5,""" +
                    """"statusCounts":{"Undecided":3,"Permitted":2},"applicationId":null}]""",
            )
            val sut = makeSut(transport)

            val cluster = sut.fetchClusters(zoneId, viewport, zoom = 3).single()

            assertFalse(cluster.isSingleMember)
            assertFalse(cluster.isStacked)
            assertNull(cluster.member)
            assertTrue(cluster.members.isEmpty())
            assertEquals(5, cluster.count)
            assertEquals<Map<ApplicationStatus, Int>>(
                mapOf(ApplicationStatus.Undecided to 3, ApplicationStatus.Permitted to 2),
                cluster.statusCounts,
            )
        }

    @Test
    fun `an unsplittable stacked cell maps with the full capped member list`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(
                200,
                """[{"latitude":51.5,"longitude":-0.1,"count":2,"statusCounts":{"Undecided":2},""" +
                    """"applicationId":null,"applicationIds":[{"authority":"42","name":"24/0001"},""" +
                    """{"authority":"42","name":"24/0002"}]}]""",
            )
            val sut = makeSut(transport)

            val cluster = sut.fetchClusters(zoneId, viewport, zoom = 20).single()

            assertTrue(cluster.isStacked)
            assertFalse(cluster.isSingleMember)
            assertNull(cluster.member)
            assertEquals(2, cluster.members.size)
            assertEquals("24/0001", cluster.members[0].name)
            assertEquals("24/0002", cluster.members[1].name)
        }

    @Test
    fun `an unrecognised status key decodes to Unknown, never crashes`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(
                200,
                """[{"latitude":51.5,"longitude":-0.1,"count":1,"statusCounts":{"BrandNewState":1},""" +
                    """"applicationId":{"authority":"42","name":"24/0001"}}]""",
            )
            val sut = makeSut(transport)

            val cluster = sut.fetchClusters(zoneId, viewport, zoom = 12).single()

            val status = cluster.statusCounts.keys.single()
            val unknown = assertIs<ApplicationStatus.Unknown>(status)
            assertEquals("BrandNewState", unknown.raw)
        }
}
