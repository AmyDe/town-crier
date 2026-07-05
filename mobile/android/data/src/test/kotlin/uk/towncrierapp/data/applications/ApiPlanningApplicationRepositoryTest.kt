package uk.towncrierapp.data.applications

import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import uk.towncrierapp.data.api.ApiClient
import uk.towncrierapp.data.api.FakeHttpTransport
import uk.towncrierapp.domain.applications.ApplicationFilter
import uk.towncrierapp.domain.applications.ApplicationSortOrder
import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.auth.FakeAuthenticationService
import uk.towncrierapp.domain.watchzones.WatchZoneId
import java.time.LocalDate
import kotlin.test.assertEquals
import kotlin.test.assertIs
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * `GET /v1/me/watch-zones/{zoneId}/applications` — cursor paging via
 * `X-Next-Cursor`, the always-sent `sort` (never the legacy param-less path),
 * `status`/`unread` mutual exclusivity (guaranteed by [ApplicationFilter]'s
 * shape, exercised here at the query-building boundary), and DTO->domain
 * mapping including `statusHistory` synthesis. Port of the iOS
 * `APIApplicationRepositoryTests` suites (GH#775).
 */
class ApiPlanningApplicationRepositoryTest {
    private val zoneId = WatchZoneId("wz-1")

    private fun makeSut(transport: FakeHttpTransport = FakeHttpTransport()): ApiPlanningApplicationRepository {
        val apiClient = ApiClient("https://api-dev.towncrierapp.uk", transport, FakeAuthenticationService())
        return ApiPlanningApplicationRepository(apiClient)
    }

    // name/areaId never vary across this file's tests — dropped as parameters
    // (rather than left as unused-in-practice knobs) to keep this fixture
    // under detekt's LongParameterList budget.
    private fun row(
        appState: String? = "Undecided",
        startDate: String? = "2026-01-10",
        decidedDate: String? = null,
        latestUnreadEvent: String? = null,
    ): String =
        """{"name":"24/0001","uid":"uid-1","areaName":"Camden","areaId":42,"address":"1 Example Street",""" +
            """"description":"Extension","appState":${appState?.let { "\"$it\"" }},""" +
            """"startDate":${startDate?.let { "\"$it\"" }},"decidedDate":${decidedDate?.let { "\"$it\"" }},""" +
            """"lastDifferent":"2026-01-11T09:00:00Z","latestUnreadEvent":$latestUnreadEvent}"""

    @Test
    fun `applications GETs the zone endpoint always sending sort and limit`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, "[]")
            val sut = makeSut(transport)

            sut.applications(zoneId, ApplicationSortOrder.NEWEST)

            val request = transport.requests.single()
            assertEquals("GET", request.method)
            assertEquals("/v1/me/watch-zones/wz-1/applications", request.url.encodedPath)
            assertEquals("newest", request.url.queryParameter("sort"))
            assertEquals("150", request.url.queryParameter("limit"))
            assertNull(request.url.queryParameter("status"))
            assertNull(request.url.queryParameter("unread"))
            assertNull(request.url.queryParameter("cursor"))
        }

    @Test
    fun `applications sends status for a Status filter, never unread`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, "[]")
            val sut = makeSut(transport)

            sut.applications(
                zoneId,
                ApplicationSortOrder.NEWEST,
                filter = ApplicationFilter.Status(ApplicationStatus.Permitted),
            )

            val url = transport.requests.single().url
            assertEquals("Permitted", url.queryParameter("status"))
            assertNull(url.queryParameter("unread"))
        }

    @Test
    fun `applications sends unread=true for the Unread filter, never status`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, "[]")
            val sut = makeSut(transport)

            sut.applications(zoneId, ApplicationSortOrder.NEWEST, filter = ApplicationFilter.Unread)

            val url = transport.requests.single().url
            assertEquals("true", url.queryParameter("unread"))
            assertNull(url.queryParameter("status"))
        }

    @Test
    fun `applications forwards a continuation cursor as a query parameter`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, "[]")
            val sut = makeSut(transport)

            sut.applications(zoneId, ApplicationSortOrder.NEWEST, cursor = "cursor-abc")

            assertEquals(
                "cursor-abc",
                transport.requests
                    .single()
                    .url
                    .queryParameter("cursor"),
            )
        }

    @Test
    fun `applications surfaces the X-Next-Cursor header as nextCursor`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, "[${row()}]", headers = mapOf("X-Next-Cursor" to "cursor-xyz"))
            val sut = makeSut(transport)

            val page = sut.applications(zoneId, ApplicationSortOrder.NEWEST)

            assertEquals("cursor-xyz", page.nextCursor)
            assertEquals(1, page.applications.size)
        }

    @Test
    fun `applications returns a null nextCursor when the header is absent (last page)`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, "[${row()}]")
            val sut = makeSut(transport)

            val page = sut.applications(zoneId, ApplicationSortOrder.NEWEST)

            assertNull(page.nextCursor)
        }

    @Test
    fun `a row decodes its identity, authority and address fields`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, "[${row()}]")
            val sut = makeSut(transport)

            val application = sut.applications(zoneId, ApplicationSortOrder.NEWEST).applications.single()

            assertEquals("42", application.id.authority)
            assertEquals("24/0001", application.id.name)
            assertEquals("24/0001", application.reference)
            assertEquals("42", application.authority.code)
            assertEquals("Camden", application.authority.name)
            assertNull(application.authority.slug)
            assertEquals("1 Example Street", application.address)
            assertEquals("Extension", application.description)
        }

    @Test
    fun `appState 'Not Available' decodes to the NotAvailable status`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, "[${row(appState = "Not Available")}]")
            val sut = makeSut(transport)

            val application = sut.applications(zoneId, ApplicationSortOrder.NEWEST).applications.single()

            assertEquals(ApplicationStatus.NotAvailable, application.status)
        }

    @Test
    fun `an unrecognised appState decodes to Unknown`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, "[${row(appState = "BrandNewState")}]")
            val sut = makeSut(transport)

            val application = sut.applications(zoneId, ApplicationSortOrder.NEWEST).applications.single()

            val unknown = assertIs<ApplicationStatus.Unknown>(application.status)
            assertEquals("BrandNewState", unknown.raw)
        }

    @Test
    fun `statusHistory has only an Undecided event when there is no decidedDate`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(
                200,
                "[${row(appState = "Undecided", startDate = "2026-01-10", decidedDate = null)}]",
            )
            val sut = makeSut(transport)

            val application = sut.applications(zoneId, ApplicationSortOrder.NEWEST).applications.single()

            assertEquals(
                listOf(
                    uk.towncrierapp.domain.applications.StatusEvent(
                        ApplicationStatus.Undecided,
                        LocalDate.of(2026, 1, 10),
                    ),
                ),
                application.statusHistory,
            )
        }

    @Test
    fun `statusHistory adds a second decided event when the status is actually decided`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(
                200,
                "[${row(appState = "Permitted", startDate = "2026-01-10", decidedDate = "2026-02-01")}]",
            )
            val sut = makeSut(transport)

            val application = sut.applications(zoneId, ApplicationSortOrder.NEWEST).applications.single()

            assertEquals(2, application.statusHistory.size)
            assertEquals(ApplicationStatus.Undecided, application.statusHistory[0].status)
            assertEquals(LocalDate.of(2026, 1, 10), application.statusHistory[0].date)
            assertEquals(ApplicationStatus.Permitted, application.statusHistory[1].status)
            assertEquals(LocalDate.of(2026, 2, 1), application.statusHistory[1].date)
        }

    @Test
    fun `a decidedDate is folded away (dropped) when the status is not actually decided`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(
                200,
                "[${row(appState = "Unresolved", startDate = "2026-01-10", decidedDate = "2026-02-01")}]",
            )
            val sut = makeSut(transport)

            val application = sut.applications(zoneId, ApplicationSortOrder.NEWEST).applications.single()

            assertEquals(1, application.statusHistory.size)
            assertEquals(ApplicationStatus.Undecided, application.statusHistory.single().status)
        }

    @Test
    fun `latestUnreadEvent is null when absent from the row`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, "[${row(latestUnreadEvent = null)}]")
            val sut = makeSut(transport)

            val application = sut.applications(zoneId, ApplicationSortOrder.NEWEST).applications.single()

            assertNull(application.latestUnreadEvent)
        }

    @Test
    fun `latestUnreadEvent decodes type, decision and createdAt when present`() =
        runTest {
            val transport = FakeHttpTransport()
            val unreadEventJson =
                """{"type":"DecisionUpdate","decision":"Permitted","createdAt":"2026-02-02T10:00:00Z"}"""
            transport.enqueueResponse(200, "[${row(latestUnreadEvent = unreadEventJson)}]")
            val sut = makeSut(transport)

            val event =
                sut
                    .applications(zoneId, ApplicationSortOrder.NEWEST)
                    .applications
                    .single()
                    .latestUnreadEvent
            assertEquals("DecisionUpdate", event?.type)
            assertEquals("Permitted", event?.decision)
        }

    @Test
    fun `detail GETs applications by authority and name, decoding the authoritySlug`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(
                200,
                """{"name":"24/0001","uid":"uid-1","areaName":"Camden","areaId":42,"address":"1 Example Street",""" +
                    """"description":"Extension","appState":"Undecided","startDate":"2026-01-10",""" +
                    """"lastDifferent":"2026-01-11T09:00:00Z","authoritySlug":"camden"}""",
            )
            val sut = makeSut(transport)

            val application = sut.detail("42", "24/0001")

            val request = transport.requests.single()
            assertEquals("/v1/applications/42/24/0001", request.url.encodedPath)
            assertEquals("camden", application.authority.slug)
        }

    @Test
    fun `portalUrl prefers link over url when both are present`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(
                200,
                """[{"name":"24/0001","uid":"uid-1","areaName":"Camden","areaId":42,"address":"1 Example Street",""" +
                    """"description":"Extension","appState":"Undecided","startDate":"2026-01-10",""" +
                    """"url":"https://planit.example/24-0001","link":"https://council.example/24-0001",""" +
                    """"lastDifferent":"2026-01-11T09:00:00Z"}]""",
            )
            val sut = makeSut(transport)

            val application = sut.applications(zoneId, ApplicationSortOrder.NEWEST).applications.single()

            assertEquals("https://council.example/24-0001", application.portalUrl)
        }

    @Test
    fun `location is null when latitude or longitude is absent`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, "[${row()}]")
            val sut = makeSut(transport)

            val application = sut.applications(zoneId, ApplicationSortOrder.NEWEST).applications.single()

            assertNull(application.location)
        }

    @Test
    fun `a legacy default sort call is never made — sort is always present`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, "[]")
            val sut = makeSut(transport)

            sut.applications(zoneId, ApplicationSortOrder.DEFAULT)

            assertTrue(
                transport.requests
                    .single()
                    .url.queryParameterNames
                    .contains("sort"),
            )
        }

    // ── detailBySlug (GH#782: resolves an inbound public share link) ──

    @Test
    fun `detailBySlug GETs the anonymous by-slug endpoint, decoding the authoritySlug`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(
                200,
                """{"name":"24/0001","uid":"uid-1","areaName":"Camden","areaId":42,"address":"1 Example Street",""" +
                    """"description":"Extension","appState":"Undecided","startDate":"2026-01-10",""" +
                    """"lastDifferent":"2026-01-11T09:00:00Z","authoritySlug":"camden"}""",
            )
            val sut = makeSut(transport)

            val application = sut.detailBySlug("camden", "24/0001")

            val request = transport.requests.single()
            assertEquals("/v1/applications/by-slug/camden/24/0001", request.url.encodedPath)
            assertEquals("camden", application.authority.slug)
        }

    @Test
    fun `detailBySlug preserves a slashed ref verbatim in the request path`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(
                200,
                """{"name":"Kingston/25/02755/CLC","uid":"uid-2","areaName":"Kingston","areaId":19,""" +
                    """"address":"1 Example Street","description":"Extension","appState":"Undecided",""" +
                    """"startDate":"2026-01-10","lastDifferent":"2026-01-11T09:00:00Z","authoritySlug":"kingston"}""",
            )
            val sut = makeSut(transport)

            sut.detailBySlug("kingston", "Kingston/25/02755/CLC")

            assertEquals(
                "/v1/applications/by-slug/kingston/Kingston/25/02755/CLC",
                transport.requests.single().url.encodedPath,
            )
        }
}
