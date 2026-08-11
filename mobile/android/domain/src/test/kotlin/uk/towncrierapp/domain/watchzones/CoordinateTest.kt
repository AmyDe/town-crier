package uk.towncrierapp.domain.watchzones

import org.junit.jupiter.api.Test
import kotlin.math.PI
import kotlin.math.abs
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertTrue

/** Port of iOS `CoordinateTests` — latitude/longitude range validation (tc-z95t). */
class CoordinateTest {
    @Test
    fun `valid latitude and longitude construct a coordinate`() {
        val coordinate = Coordinate(latitude = 52.2053, longitude = 0.1218)

        assertEquals(52.2053, coordinate.latitude)
        assertEquals(0.1218, coordinate.longitude)
    }

    @Test
    fun `latitude above 90 is rejected`() {
        assertFailsWith<IllegalArgumentException> { Coordinate(latitude = 91.0, longitude = 0.0) }
    }

    @Test
    fun `latitude below -90 is rejected`() {
        assertFailsWith<IllegalArgumentException> { Coordinate(latitude = -91.0, longitude = 0.0) }
    }

    @Test
    fun `longitude above 180 is rejected`() {
        assertFailsWith<IllegalArgumentException> { Coordinate(latitude = 0.0, longitude = 181.0) }
    }

    @Test
    fun `longitude below -180 is rejected`() {
        assertFailsWith<IllegalArgumentException> { Coordinate(latitude = 0.0, longitude = -181.0) }
    }

    @Test
    fun `boundary values are accepted`() {
        Coordinate(latitude = 90.0, longitude = 180.0)
        Coordinate(latitude = -90.0, longitude = -180.0)
    }

    @Test
    fun `coordinates with the same values are equal`() {
        assertEquals(Coordinate(52.2053, 0.1218), Coordinate(52.2053, 0.1218))
    }

    // MARK: - haversineMetres

    @Test
    fun `haversine distance between the same point is zero`() {
        assertEquals(0.0, haversineMetres(52.2053, 0.1218, 52.2053, 0.1218))
    }

    // Antipodal points on the equator: haversine's `h` term computes to
    // exactly 1.0, the boundary this test exists to cover. Regression for a
    // CodeRabbit-flagged bug (ported from iOS) where `1 - h` could go
    // negative and `sqrt` return NaN.
    @Test
    fun `haversine distance between antipodal points on the equator is half the earth's circumference`() {
        val distance = haversineMetres(0.0, 0.0, 0.0, 180.0)

        assertTrue(distance.isFinite())
        assertTrue(abs(distance - PI * 6_371_000) < 1.0)
    }

    // A near-antipodal pair found (on iOS) by brute-force search over random
    // near-antipodal inputs to push the raw (pre-clamp) haversine `h` term
    // fractionally above 1.0 in double precision. Without the clamp, `1 - h`
    // goes negative and `sqrt` returns NaN, poisoning `atan2`.
    @Test
    fun `haversine distance between a near-antipodal pair is finite not NaN`() {
        val distance = haversineMetres(-58.97018354817475, -97.36776415155397, 58.970183085348935, 82.6322355333763)

        assertTrue(distance.isFinite())
        assertTrue(abs(distance - PI * 6_371_000) < 10.0)
    }
}
