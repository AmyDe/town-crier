package uk.towncrierapp.presentation.features.watchzones

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import org.junit.jupiter.api.Test
import uk.towncrierapp.domain.watchzones.Coordinate
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * Pure-JVM tests for [ZoneBoundaryGeometry] — the lon/lat-to-Canvas-pixel
 * mapping behind [ZoneBoundaryThumbnail]. Mirrors
 * `CustomShapePolygonGeometryTest`'s approach: exercise the geometry
 * directly, with no Compose rendering pipeline involved
 * (android-coding-standards skill, testing.md "What not to test").
 */
class ZoneBoundaryGeometryTest {
    @Test
    fun `canvasVertices maps a unit square to the full canvas without padding`() {
        val vertices =
            listOf(
                Coordinate(latitude = 0.0, longitude = 0.0),
                Coordinate(latitude = 0.0, longitude = 1.0),
                Coordinate(latitude = 1.0, longitude = 1.0),
                Coordinate(latitude = 1.0, longitude = 0.0),
            )

        val offsets = ZoneBoundaryGeometry.canvasVertices(vertices, Size(100f, 100f))

        assertEquals(
            listOf(
                Offset(0f, 100f),
                Offset(100f, 100f),
                Offset(100f, 0f),
                Offset(0f, 0f),
            ),
            offsets,
        )
    }

    @Test
    fun `canvasVertices flips latitude so the northernmost vertex has the smallest y`() {
        val southern = Coordinate(latitude = 10.0, longitude = 0.0)
        val northern = Coordinate(latitude = 20.0, longitude = 0.0)

        val offsets = ZoneBoundaryGeometry.canvasVertices(listOf(southern, northern), Size(100f, 100f))

        val (southernOffset, northernOffset) = offsets
        assertTrue(northernOffset.y < southernOffset.y)
    }

    @Test
    fun `canvasVertices centres a degenerate single-point boundary`() {
        val vertices = listOf(Coordinate(latitude = 51.5, longitude = -0.1))

        val offsets = ZoneBoundaryGeometry.canvasVertices(vertices, Size(100f, 100f))

        assertEquals(listOf(Offset(50f, 50f)), offsets)
    }

    @Test
    fun `canvasVertices letterboxes a wide boundary instead of stretching it to fill a square canvas`() {
        val vertices =
            listOf(
                Coordinate(latitude = 0.0, longitude = 0.0),
                Coordinate(latitude = 0.0, longitude = 2.0),
                Coordinate(latitude = 1.0, longitude = 2.0),
                Coordinate(latitude = 1.0, longitude = 0.0),
            )

        val offsets = ZoneBoundaryGeometry.canvasVertices(vertices, Size(100f, 100f))

        // 2:1 aspect ratio scaled into a 100x100 canvas is 100x50, centred
        // vertically -> y spans [25, 75], never the full [0, 100].
        assertEquals(25f, offsets.minOf { it.y })
        assertEquals(75f, offsets.maxOf { it.y })
        assertEquals(0f, offsets.minOf { it.x })
        assertEquals(100f, offsets.maxOf { it.x })
    }

    @Test
    fun `canvasVertices insets the shape by the given padding on every side`() {
        val vertices =
            listOf(
                Coordinate(latitude = 0.0, longitude = 0.0),
                Coordinate(latitude = 0.0, longitude = 1.0),
                Coordinate(latitude = 1.0, longitude = 1.0),
                Coordinate(latitude = 1.0, longitude = 0.0),
            )

        val offsets = ZoneBoundaryGeometry.canvasVertices(vertices, Size(100f, 100f), paddingPx = 10f)

        assertEquals(10f, offsets.minOf { it.x })
        assertEquals(90f, offsets.maxOf { it.x })
        assertEquals(10f, offsets.minOf { it.y })
        assertEquals(90f, offsets.maxOf { it.y })
    }

    @Test
    fun `canvasVertices returns one offset per input vertex`() {
        val vertices =
            listOf(
                Coordinate(latitude = 0.0, longitude = 0.0),
                Coordinate(latitude = 0.0, longitude = 1.0),
                Coordinate(latitude = 1.0, longitude = 1.0),
            )

        val offsets = ZoneBoundaryGeometry.canvasVertices(vertices, Size(100f, 100f))

        assertEquals(vertices.size, offsets.size)
    }

    @Test
    fun `canvasVertices returns an empty list for an empty boundary`() {
        val offsets = ZoneBoundaryGeometry.canvasVertices(emptyList(), Size(100f, 100f))

        assertEquals(emptyList(), offsets)
    }
}
