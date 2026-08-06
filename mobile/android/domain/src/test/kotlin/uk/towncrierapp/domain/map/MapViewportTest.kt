package uk.towncrierapp.domain.map

import org.junit.jupiter.api.Test
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.domain.watchzones.aCoordinate
import kotlin.test.assertEquals

/** [MapViewport]'s wire encoding and the pure camera-geometry helpers — port of iOS `MapViewModelTests`' viewport-geometry suite. */
class MapViewportTest {
    @Test
    fun `queryValue is a well-ordered west,south,east,north bbox string`() {
        val viewport = MapViewport(west = -0.14, south = 51.49, east = -0.10, north = 51.52)

        assertEquals("-0.14,51.49,-0.1,51.52", viewport.queryValue)
    }

    @Test
    fun `slippyZoom is 20 for a zero or negative span`() {
        assertEquals(20, MapViewport.slippyZoom(0.0))
        assertEquals(20, MapViewport.slippyZoom(-1.0))
    }

    @Test
    fun `slippyZoom is log2(360 divided by span), clamped to the 0 to 20 range`() {
        assertEquals(0, MapViewport.slippyZoom(360.0))
        assertEquals(10, MapViewport.slippyZoom(360.0 / 1024))
        assertEquals(20, MapViewport.slippyZoom(0.000001))
    }

    @Test
    fun `clampZoom rounds and clamps a raw camera zoom into the 0 to 20 range`() {
        assertEquals(0, MapViewport.clampZoom(-5.0))
        assertEquals(14, MapViewport.clampZoom(14.4))
        assertEquals(15, MapViewport.clampZoom(14.6))
        assertEquals(20, MapViewport.clampZoom(25.0))
    }

    @Test
    fun `initial derives a viewport spanning 2point5x the radius, centred on the zone`() {
        val centre = Coordinate(latitude = 0.0, longitude = 0.0)

        val (viewport, _) = MapViewport.initial(centre, radiusMetres = 1000.0)

        val expectedHalfSpanDeg = (1000.0 * 2.5 / 2) / 111_320.0
        assertApprox(-expectedHalfSpanDeg, viewport.west)
        assertApprox(expectedHalfSpanDeg, viewport.east)
        assertApprox(-expectedHalfSpanDeg, viewport.south)
        assertApprox(expectedHalfSpanDeg, viewport.north)
    }

    @Test
    fun `zoomedInto halves both spans and re-centres on the tapped coordinate`() {
        val current = MapViewport(west = -1.0, south = -1.0, east = 1.0, north = 1.0)
        val target = aCoordinate(latitude = 10.0, longitude = 20.0)

        val zoomed = current.zoomedInto(target)

        // The original 2°-wide span halves to 1° (full width), centred on the tap.
        assertApprox(19.5, zoomed.west)
        assertApprox(20.5, zoomed.east)
        assertApprox(9.5, zoomed.south)
        assertApprox(10.5, zoomed.north)
    }

    @Test
    fun `zoomedInto floors the span at MIN_SPAN_DEGREES rather than shrinking forever`() {
        val current = MapViewport(west = -0.0001, south = -0.0001, east = 0.0001, north = 0.0001)
        val target = aCoordinate(latitude = 0.0, longitude = 0.0)

        val zoomed = current.zoomedInto(target)

        val halfMinSpan = MapViewport.MIN_SPAN_DEGREES / 2
        assertApprox(-halfMinSpan, zoomed.west)
        assertApprox(halfMinSpan, zoomed.east)
    }
}

// kotlin.test has no built-in tolerance overload for Double equality; this
// local helper keeps the floating-point comparisons above readable.
private fun assertApprox(
    expected: Double,
    actual: Double,
    absoluteTolerance: Double = 1e-9,
) {
    kotlin.test.assertTrue(
        kotlin.math.abs(expected - actual) <= absoluteTolerance,
        "expected $expected to be within $absoluteTolerance of $actual",
    )
}
