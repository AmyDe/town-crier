package uk.towncrierapp.data.watchzones

import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.double
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import okhttp3.Request
import okio.Buffer
import org.junit.jupiter.api.Test
import uk.towncrierapp.data.api.ApiClient
import uk.towncrierapp.data.api.FakeHttpTransport
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.auth.FakeAuthenticationService
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.domain.watchzones.WatchZoneBoundary
import uk.towncrierapp.domain.watchzones.WatchZoneBoundaryResult
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.domain.watchzones.aWatchZone
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertIs
import kotlin.test.assertTrue

/** Reads a request's JSON body back out for assertions — the request itself was already built and enqueued. */
private fun Request.bodyAsString(): String {
    val buffer = Buffer()
    body?.writeTo(buffer)
    return buffer.readUtf8()
}

/** A valid, simple triangular boundary near the fixture watch zone's centre. */
private fun aBoundary(): WatchZoneBoundary {
    val vertices =
        listOf(
            Coordinate(latitude = 52.20, longitude = 0.10),
            Coordinate(latitude = 52.21, longitude = 0.14),
            Coordinate(latitude = 52.19, longitude = 0.12),
        )
    return (WatchZoneBoundary.of(vertices) as WatchZoneBoundaryResult.Valid).boundary
}

/**
 * `GET/POST/PATCH/DELETE /v1/me/watch-zones` — port of the iOS
 * `APIWatchZoneRepositoryTests`/`APIWatchZoneRepositoryQuotaTests`/
 * `APIWatchZoneRepositoryNotificationFlagsTests` suites (tc-z95t).
 */
class ApiWatchZoneRepositoryTest {
    private fun makeSut(transport: FakeHttpTransport = FakeHttpTransport()): ApiWatchZoneRepository {
        val apiClient = ApiClient("https://api-dev.towncrierapp.uk", transport, FakeAuthenticationService())
        return ApiWatchZoneRepository(apiClient)
    }

    @Test
    fun `zones GETs and decodes the list`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(
                200,
                """{"zones":[{"id":"wz-1","name":"Home","latitude":51.5074,"longitude":-0.1278,""" +
                    """"radiusMetres":500.0,"authorityId":42,"pushEnabled":false,"emailInstantEnabled":true}]}""",
            )
            val sut = makeSut(transport)

            val zones = sut.zones()

            assertEquals(1, zones.size)
            val zone = zones.single()
            assertEquals("wz-1", zone.id.value)
            assertEquals("Home", zone.name)
            assertEquals(51.5074, zone.centre.latitude)
            assertEquals(-0.1278, zone.centre.longitude)
            assertEquals(500.0, zone.radiusMetres)
            assertEquals(42, zone.authorityId)
            assertEquals(false, zone.pushEnabled)
            assertEquals(true, zone.emailInstantEnabled)
            val request = transport.requests.single()
            assertEquals("GET", request.method)
            assertEquals("/v1/me/watch-zones", request.url.encodedPath)
        }

    @Test
    fun `zones defaults absent pushEnabled and emailInstantEnabled to true for legacy zones`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(
                200,
                """{"zones":[{"id":"wz-1","name":"Home","latitude":51.5074,"longitude":-0.1278,""" +
                    """"radiusMetres":500.0,"authorityId":42}]}""",
            )
            val sut = makeSut(transport)

            val zone = sut.zones().single()

            assertTrue(zone.pushEnabled)
            assertTrue(zone.emailInstantEnabled)
        }

    @Test
    fun `create POSTs the full field set`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(201, """{"nearbyApplications":[]}""")
            val sut = makeSut(transport)

            sut.create(aWatchZone(name = "Office", radiusMetres = 1_000.0))

            val request = transport.requests.single()
            assertEquals("POST", request.method)
            assertEquals("/v1/me/watch-zones", request.url.encodedPath)
            val body = request.bodyAsString()
            assertTrue(body.contains(""""name":"Office""""))
            assertTrue(body.contains(""""radiusMetres":1000.0"""))
            assertTrue(body.contains(""""pushEnabled":true"""))
            assertTrue(body.contains(""""emailInstantEnabled":true"""))
        }

    @Test
    fun `create with a plain quota 403 maps to InsufficientEntitlement personal`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(403, "Watch zone quota exceeded.")
            val sut = makeSut(transport)

            val error =
                assertIs<DomainError.InsufficientEntitlement>(
                    assertFailsWith<DomainError> { sut.create(aWatchZone()) },
                )
            assertEquals("personal", error.required)
        }

    @Test
    fun `create with an entitlement-shaped 403 still maps to InsufficientEntitlement personal`() =
        runTest {
            // Deliberate Android divergence from iOS (tc-z95t bead brief): create-403
            // ALWAYS normalises to "personal" regardless of body shape, since the
            // create endpoint's only 403 is quota-exceeded in practice.
            val transport = FakeHttpTransport()
            transport.enqueueResponse(403, """{"error":"insufficient_entitlement","required":"pro"}""")
            val sut = makeSut(transport)

            val error =
                assertIs<DomainError.InsufficientEntitlement>(
                    assertFailsWith<DomainError> { sut.create(aWatchZone()) },
                )
            assertEquals("personal", error.required)
        }

    @Test
    fun `update PATCHes the full field set`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, """{"zone":{}}""")
            val sut = makeSut(transport)

            sut.update(aWatchZone(id = WatchZoneId("wz-9"), name = "Updated", radiusMetres = 2_500.0))

            val request = transport.requests.single()
            assertEquals("PATCH", request.method)
            assertEquals("/v1/me/watch-zones/wz-9", request.url.encodedPath)
            val body = request.bodyAsString()
            assertTrue(body.contains(""""name":"Updated""""))
            assertTrue(body.contains(""""radiusMetres":2500.0"""))
        }

    @Test
    fun `update with a 403 is left as a generic ServerError — quota only applies to create`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(403, "Forbidden")
            val sut = makeSut(transport)

            val error =
                assertIs<DomainError.ServerError>(
                    assertFailsWith<DomainError> { sut.update(aWatchZone()) },
                )
            assertEquals(403, error.status)
        }

    @Test
    fun `delete DELETEs the zone`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(204)
            val sut = makeSut(transport)

            sut.delete(WatchZoneId("wz-1"))

            val request = transport.requests.single()
            assertEquals("DELETE", request.method)
            assertEquals("/v1/me/watch-zones/wz-1", request.url.encodedPath)
        }

    @Test
    fun `delete treats a 404 as success — idempotent`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(404)
            val sut = makeSut(transport)

            sut.delete(WatchZoneId("wz-missing"))
        }

    @Test
    fun `delete propagates other failures`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(500, "boom")
            val sut = makeSut(transport)

            assertFailsWith<DomainError.ServerError> { sut.delete(WatchZoneId("wz-1")) }
        }

    @Test
    fun `zones GETs and decodes a boundary`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(
                200,
                """{"zones":[{"id":"wz-1","name":"Home","latitude":51.5074,"longitude":-0.1278,""" +
                    """"radiusMetres":500.0,"authorityId":42,"pushEnabled":true,"emailInstantEnabled":true,""" +
                    """"boundary":{"type":"Polygon","coordinates":""" +
                    """[[[0.1,52.2],[0.14,52.21],[0.12,52.19],[0.1,52.2]]]}}]}""",
            )
            val sut = makeSut(transport)

            val zone = sut.zones().single()

            assertEquals(aBoundary(), zone.boundary)
        }

    @Test
    fun `zones skips only the zone with a malformed boundary, keeping the rest of the list`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(
                200,
                """{"zones":[""" +
                    """{"id":"wz-bad","name":"Malformed","latitude":51.5,"longitude":-0.1,""" +
                    """"radiusMetres":500.0,"boundary":{"type":"Polygon","coordinates":[[[0.1,52.2]]]}},""" +
                    """{"id":"wz-good","name":"Good","latitude":51.6,"longitude":-0.2,"radiusMetres":300.0}""" +
                    """]}""",
            )
            val sut = makeSut(transport)

            val zones = sut.zones()

            assertEquals(listOf("wz-good"), zones.map { it.id.value })
        }

    @Test
    fun `create POSTs the encoded boundary when the zone has one`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(201, """{"nearbyApplications":[]}""")
            val sut = makeSut(transport)

            sut.create(aWatchZone().copy(boundary = aBoundary()))

            val request = transport.requests.single()
            val boundaryJson =
                Json
                    .parseToJsonElement(
                        request.bodyAsString(),
                    ).jsonObject
                    .getValue("boundary")
                    .jsonObject
            assertEquals("Polygon", boundaryJson.getValue("type").jsonPrimitive.content)
            val ring =
                boundaryJson
                    .getValue("coordinates")
                    .jsonArray
                    .single()
                    .jsonArray
            assertEquals(4, ring.size)
            val firstVertex = ring.first().jsonArray
            assertEquals(0.1, firstVertex[0].jsonPrimitive.double)
            assertEquals(52.2, firstVertex[1].jsonPrimitive.double)
        }

    @Test
    fun `create omits the boundary key entirely when the zone has none`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(201, """{"nearbyApplications":[]}""")
            val sut = makeSut(transport)

            sut.create(aWatchZone())

            val request = transport.requests.single()
            val body = Json.parseToJsonElement(request.bodyAsString()).jsonObject
            assertTrue("boundary" !in body)
        }

    @Test
    fun `update PATCHes the encoded boundary when the zone has one`() =
        runTest {
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, """{"zone":{}}""")
            val sut = makeSut(transport)

            sut.update(aWatchZone(id = WatchZoneId("wz-9")).copy(boundary = aBoundary()))

            val request = transport.requests.single()
            val boundaryJson =
                Json
                    .parseToJsonElement(
                        request.bodyAsString(),
                    ).jsonObject
                    .getValue("boundary")
                    .jsonObject
            assertEquals("Polygon", boundaryJson.getValue("type").jsonPrimitive.content)
            val ring =
                boundaryJson
                    .getValue("coordinates")
                    .jsonArray
                    .single()
                    .jsonArray
            assertEquals(4, ring.size)
        }

    @Test
    fun `update PATCHes an explicit null boundary for a circle zone — revert, not omitted`() =
        runTest {
            // Load-bearing (tc-v6fo0.2 brief): the update DTO must always emit the
            // "boundary" key, including an explicit JSON null, because the server's
            // PATCH handler treats an absent key as "leave untouched" and an
            // explicit null as "revert to circle" — different instructions. Asserts
            // on the raw JSON, not DTO equality, so an accidental `= null` default
            // added to the field later (which would make kotlinx.serialization omit
            // it) fails this test.
            val transport = FakeHttpTransport()
            transport.enqueueResponse(200, """{"zone":{}}""")
            val sut = makeSut(transport)

            sut.update(aWatchZone(id = WatchZoneId("wz-9")))

            val request = transport.requests.single()
            val body = Json.parseToJsonElement(request.bodyAsString()).jsonObject
            assertTrue("boundary" in body)
            assertEquals(JsonNull, body.getValue("boundary"))
        }
}
