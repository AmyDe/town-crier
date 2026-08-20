package uk.towncrierapp.presentation.features.watchzones

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.size
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.drawscope.Fill
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.domain.watchzones.WatchZoneBoundary
import uk.towncrierapp.domain.watchzones.WatchZoneBoundaryResult
import uk.towncrierapp.presentation.designsystem.TownCrierTheme

/**
 * Fill/stroke alpha for the drawn polygon — the same values `MapScreen`
 * uses for a circle zone's overlay (`ZONE_CIRCLE_FILL_ALPHA`/
 * `ZONE_CIRCLE_STROKE_ALPHA`), so a custom-shape zone's list thumbnail reads
 * consistently with how its boundary would look on the map tab.
 */
private const val BOUNDARY_FILL_ALPHA = 0.08f
private const val BOUNDARY_STROKE_ALPHA = 0.3f

private val BOUNDARY_STROKE_WIDTH = 1.5.dp
private val THUMBNAIL_PADDING = 6.dp

/**
 * Static Canvas-drawn thumbnail of a custom-shape watch zone's
 * [WatchZoneBoundary] polygon — the list-row equivalent of [ZoneMapPlaceholder]
 * for zones with a drawn shape rather than a circle (GH#1072 Scope: "list-row
 * rendering of a drawn shape"). A real GoogleMap-based thumbnail is out of
 * scope until #776 (see [ZoneMapPlaceholder]'s doc comment), so this draws the
 * polygon geometry itself — scaled and letterboxed to fit — rather than a live
 * map tile.
 *
 * Matches [ZoneMapPlaceholder]'s size/shape/background contract exactly so
 * [WatchZoneRow]'s layout doesn't shift between circle and custom-shape zones;
 * callers apply the same `Modifier.size(56.dp)`.
 */
@Composable
internal fun ZoneBoundaryThumbnail(
    boundary: WatchZoneBoundary,
    modifier: Modifier = Modifier,
) {
    val fillColor = MaterialTheme.colorScheme.primary.copy(alpha = BOUNDARY_FILL_ALPHA)
    val strokeColor = MaterialTheme.colorScheme.primary.copy(alpha = BOUNDARY_STROKE_ALPHA)

    Canvas(
        modifier =
            modifier
                .clip(MaterialTheme.shapes.small)
                .background(MaterialTheme.colorScheme.surfaceContainerHigh, shape = MaterialTheme.shapes.small),
    ) {
        val vertices = ZoneBoundaryGeometry.canvasVertices(boundary.vertices, size, THUMBNAIL_PADDING.toPx())
        val path = polygonPath(vertices)
        drawPath(path = path, color = fillColor, style = Fill)
        drawPath(path = path, color = strokeColor, style = Stroke(width = BOUNDARY_STROKE_WIDTH.toPx()))
    }
}

/** Builds a closed [Path] through [vertices]. */
private fun polygonPath(vertices: List<Offset>): Path =
    Path().apply {
        val first = vertices.firstOrNull() ?: return@apply
        moveTo(first.x, first.y)
        vertices.drop(1).forEach { vertex -> lineTo(vertex.x, vertex.y) }
        close()
    }

/**
 * Pure geometry mapping a [WatchZoneBoundary]'s lon/lat [Coordinate]
 * vertices into Canvas pixel space for [ZoneBoundaryThumbnail]:
 * bounding-box normalised, scaled to fit within a given canvas size (minus
 * padding on every side) while preserving aspect ratio — letterboxed, never
 * stretched — with latitude's north-is-up sense flipped to Canvas's
 * y-grows-downward sense. Factored out so the mapping is unit-testable
 * without a Compose rendering pipeline, the same split `CustomShapePolygonGeometry`
 * uses for the onboarding graphic (android-coding-standards skill, testing.md).
 */
internal object ZoneBoundaryGeometry {
    internal fun canvasVertices(
        vertices: List<Coordinate>,
        canvasSize: Size,
        paddingPx: Float = 0f,
    ): List<Offset> {
        if (vertices.isEmpty()) return emptyList()

        val minLongitude = vertices.minOf { it.longitude }
        val maxLongitude = vertices.maxOf { it.longitude }
        val minLatitude = vertices.minOf { it.latitude }
        val maxLatitude = vertices.maxOf { it.latitude }

        val longitudeSpan = (maxLongitude - minLongitude).toFloat()
        val latitudeSpan = (maxLatitude - minLatitude).toFloat()

        val availableWidth = (canvasSize.width - 2f * paddingPx).coerceAtLeast(0f)
        val availableHeight = (canvasSize.height - 2f * paddingPx).coerceAtLeast(0f)

        val scale = fitScale(longitudeSpan, latitudeSpan, availableWidth, availableHeight)
        val scaledWidth = longitudeSpan * scale
        val scaledHeight = latitudeSpan * scale

        // Letterbox: centre the scaled shape within the full canvas rather
        // than stretching it to fill a non-matching aspect ratio.
        val originX = (canvasSize.width - scaledWidth) / 2f
        val originY = (canvasSize.height - scaledHeight) / 2f

        return vertices.map { vertex ->
            Offset(
                x = originX + (vertex.longitude - minLongitude).toFloat() * scale,
                // Latitude increases northward (up); Canvas y increases
                // downward, so the highest latitude maps to the smallest y.
                y = originY + (maxLatitude - vertex.latitude).toFloat() * scale,
            )
        }
    }

    /**
     * The uniform scale fitting a [longitudeSpan] x [latitudeSpan] box into
     * [availableWidth] x [availableHeight] without distortion. Falls back to
     * the single non-degenerate axis for a boundary that collapses to a
     * line, or to `0f` (every vertex centred as one point) for a boundary
     * that collapses to a single point.
     */
    private fun fitScale(
        longitudeSpan: Float,
        latitudeSpan: Float,
        availableWidth: Float,
        availableHeight: Float,
    ): Float =
        when {
            longitudeSpan <= 0f && latitudeSpan <= 0f -> 0f
            longitudeSpan <= 0f -> availableHeight / latitudeSpan
            latitudeSpan <= 0f -> availableWidth / longitudeSpan
            else -> minOf(availableWidth / longitudeSpan, availableHeight / latitudeSpan)
        }
}

@Preview(name = "light")
@Composable
private fun ZoneBoundaryThumbnailPreview() {
    TownCrierTheme {
        ZoneBoundaryThumbnail(boundary = previewBoundary, modifier = Modifier.size(56.dp))
    }
}

private val previewBoundary: WatchZoneBoundary =
    when (
        val result =
            WatchZoneBoundary.of(
                listOf(
                    Coordinate(latitude = 52.2053, longitude = 0.1218),
                    Coordinate(latitude = 52.2063, longitude = 0.1258),
                    Coordinate(latitude = 52.2043, longitude = 0.1278),
                    Coordinate(latitude = 52.2023, longitude = 0.1238),
                ),
            )
    ) {
        is WatchZoneBoundaryResult.Valid -> result.boundary
        else -> error("preview boundary fixture is invalid: $result")
    }
