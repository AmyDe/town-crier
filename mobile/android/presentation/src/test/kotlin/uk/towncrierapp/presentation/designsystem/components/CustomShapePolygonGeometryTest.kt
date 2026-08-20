package uk.towncrierapp.presentation.designsystem.components

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import org.junit.jupiter.api.Test
import kotlin.math.sqrt
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * Pure-JVM tests for [CustomShapePolygonGeometry] — the vertex generator
 * behind the irregular "blob" half of [CustomShapeUpsellGraphic]. Mirrors
 * iOS `CustomShapePolygonGeometryTests`: vertex count, determinism, and
 * containment within the given size's inscribed radius, with no Compose
 * rendering involved (android-coding-standards skill, testing.md).
 */
class CustomShapePolygonGeometryTest {
    private val size = Size(width = 100f, height = 100f)

    @Test
    fun `vertices returns one point per radius multiplier`() {
        val vertices = CustomShapePolygonGeometry.vertices(size)

        assertEquals(CustomShapePolygonGeometry.radiusMultipliers.size, vertices.size)
    }

    @Test
    fun `vertices is deterministic for the same size`() {
        val first = CustomShapePolygonGeometry.vertices(size)
        val second = CustomShapePolygonGeometry.vertices(size)

        assertEquals(first, second)
    }

    @Test
    fun `vertices stay within the sizes bounding radius`() {
        val center = Offset(size.width / 2f, size.height / 2f)
        val maxRadius = minOf(size.width, size.height) / 2f

        val vertices = CustomShapePolygonGeometry.vertices(size)

        vertices.forEach { vertex ->
            assertTrue(distanceBetween(vertex, center) <= maxRadius + 0.001f)
        }
    }

    @Test
    fun `vertices are not all identical`() {
        val vertices = CustomShapePolygonGeometry.vertices(size)

        val distinctVertices = vertices.toSet()

        assertEquals(vertices.size, distinctVertices.size)
    }

    @Test
    fun `vertices scale with a smaller size`() {
        val smallSize = Size(width = 10f, height = 10f)
        val center = Offset(smallSize.width / 2f, smallSize.height / 2f)
        val maxRadius = minOf(smallSize.width, smallSize.height) / 2f

        val vertices = CustomShapePolygonGeometry.vertices(smallSize)

        vertices.forEach { vertex ->
            assertTrue(distanceBetween(vertex, center) <= maxRadius + 0.001f)
        }
    }

    private fun distanceBetween(
        a: Offset,
        b: Offset,
    ): Float {
        val deltaX = a.x - b.x
        val deltaY = a.y - b.y
        return sqrt(deltaX * deltaX + deltaY * deltaY)
    }
}
