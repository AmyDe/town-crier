package uk.towncrierapp.domain.watchzones

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith

/** Port of iOS `WatchZoneTests` — construction invariants and defaults (tc-z95t). */
class WatchZoneTest {
    @Test
    fun `a blank name is rejected`() {
        assertFailsWith<IllegalArgumentException> {
            WatchZone(
                id = WatchZoneId("wz-1"),
                name = "   ",
                centre = Coordinate(51.5074, -0.1278),
                radiusMetres = 500.0,
            )
        }
    }

    @Test
    fun `a non-positive radius is rejected`() {
        assertFailsWith<IllegalArgumentException> {
            WatchZone(
                id = WatchZoneId("wz-1"),
                name = "Home",
                centre = Coordinate(51.5074, -0.1278),
                radiusMetres = 0.0,
            )
        }
    }

    @Test
    fun `notification flags and authority id default per legacy-zone parity`() {
        val zone =
            WatchZone(
                id = WatchZoneId("wz-1"),
                name = "Home",
                centre = Coordinate(51.5074, -0.1278),
                radiusMetres = 500.0,
            )

        assertEquals(true, zone.pushEnabled)
        assertEquals(true, zone.emailInstantEnabled)
        assertEquals(0, zone.authorityId)
    }

    @Test
    fun `boundary defaults to null meaning a circle zone`() {
        val zone =
            WatchZone(
                id = WatchZoneId("wz-1"),
                name = "Home",
                centre = Coordinate(51.5074, -0.1278),
                radiusMetres = 500.0,
            )

        assertEquals(null, zone.boundary)
    }

    @Test
    fun `a custom-shape zone carries its boundary`() {
        val triangle =
            listOf(
                Coordinate(latitude = 51.50, longitude = -0.10),
                Coordinate(latitude = 51.51, longitude = -0.09),
                Coordinate(latitude = 51.50, longitude = -0.09),
            )
        val boundary = (WatchZoneBoundary.of(triangle) as WatchZoneBoundaryResult.Valid).boundary

        val zone =
            WatchZone(
                id = WatchZoneId("wz-1"),
                name = "Home",
                centre = Coordinate(51.5074, -0.1278),
                radiusMetres = 500.0,
                boundary = boundary,
            )

        assertEquals(boundary, zone.boundary)
    }
}
