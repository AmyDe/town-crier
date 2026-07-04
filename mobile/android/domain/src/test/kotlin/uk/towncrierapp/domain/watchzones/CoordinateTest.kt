package uk.towncrierapp.domain.watchzones

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith

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
}
