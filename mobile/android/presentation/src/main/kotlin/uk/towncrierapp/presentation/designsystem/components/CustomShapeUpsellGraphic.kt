package uk.towncrierapp.presentation.designsystem.components

import android.content.res.Configuration
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowForward
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import kotlin.math.cos
import kotlin.math.sin
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.Appearance
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme

/** Side length of each of the graphic's two square tiles. */
private val TILE_SIZE = 96.dp

/** Stroke width for both the dashed circle and the solid polygon outline. */
private val OUTLINE_STROKE_WIDTH = 2.dp

/** On/off dash lengths for the default-shape circle's stroke. */
private val CIRCLE_DASH_PATTERN = floatArrayOf(18f, 12f)

/**
 * Decorative, Canvas-drawn onboarding graphic contrasting the default
 * **circle** watch zone with a **custom shape** (polygon) watch zone, to
 * visually explain the paid "draw a custom shape instead of a circle"
 * entitlement (GH#1072 Phase 4). Port of iOS `CustomShapeUpsellGraphic` /
 * web `CustomShapeUpsell.tsx`.
 *
 * Drawn entirely with Compose `Canvas`/`Path`, never a raster asset, so it
 * follows the light/dark/OLED theme like every other design-system token.
 * The muted, dashed circle ([TownCrierTheme.colors.textTertiary]) reads as
 * the existing, unremarkable default; the solid amber
 * ([MaterialTheme.colorScheme.primary]) polygon reads as the premium
 * capability, matching the app's convention that amber marks brand and
 * interactive emphasis. Purely presentational: no ViewModel, no external
 * state.
 *
 * Not yet wired into any screen — that's a separate bead (tc-v6fo0.5,
 * blocked on the Phase 3 editor shape-toggle UI landing first).
 */
@Composable
public fun CustomShapeUpsellGraphic(modifier: Modifier = Modifier) {
    val description = stringResource(R.string.custom_shape_upsell_graphic_content_description)
    val defaultShapeColor = TownCrierTheme.colors.textTertiary
    val customShapeColor = MaterialTheme.colorScheme.primary

    Row(
        modifier =
            modifier
                .padding(TownCrierSpacing.lg)
                .clearAndSetSemantics { contentDescription = description },
        horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.lg),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Canvas(modifier = Modifier.size(TILE_SIZE)) {
            drawCircle(
                color = defaultShapeColor,
                radius = size.minDimension / 2f,
                style =
                    Stroke(
                        width = OUTLINE_STROKE_WIDTH.toPx(),
                        cap = StrokeCap.Round,
                        pathEffect = PathEffect.dashPathEffect(CIRCLE_DASH_PATTERN, phase = 0f),
                    ),
            )
        }

        Icon(
            imageVector = Icons.AutoMirrored.Filled.ArrowForward,
            contentDescription = null,
            tint = TownCrierTheme.colors.textTertiary,
        )

        Canvas(modifier = Modifier.size(TILE_SIZE)) {
            val vertices = CustomShapePolygonGeometry.vertices(size)
            drawPath(
                path = polygonPath(vertices),
                color = customShapeColor,
                style =
                    Stroke(
                        width = OUTLINE_STROKE_WIDTH.toPx(),
                        join = StrokeJoin.Round,
                    ),
            )
        }
    }
}

/** Builds a closed [Path] through [vertices], mirroring iOS `CustomShapePolygon.path(in:)`. */
private fun polygonPath(vertices: List<Offset>): Path =
    Path().apply {
        val first = vertices.firstOrNull() ?: return@apply
        moveTo(first.x, first.y)
        vertices.drop(1).forEach { vertex -> lineTo(vertex.x, vertex.y) }
        close()
    }

/**
 * Pure geometry for [CustomShapeUpsellGraphic]'s irregular polygon ("blob")
 * tile, factored out of the drawing code so vertex generation is
 * unit-testable without a Compose rendering pipeline. Mirrors iOS
 * `CustomShapePolygonGeometry`.
 *
 * Vertices are placed at evenly-spaced angles around the centre of the given
 * [Size], each pulled in from a shared base radius by a fixed, hand-picked
 * multiplier no greater than `1.0` (so every vertex stays inscribed within
 * the size passed to it — no overflow into a neighbouring element). The
 * multipliers are deliberately irregular (not a smooth curve or true
 * randomness) so the outline reads as a hand-drawn boundary rather than a
 * regular polygon — deterministic and simple, since this is a decorative
 * graphic rather than data-driven geometry. Values reused verbatim from iOS
 * for visual parity across platforms.
 */
internal object CustomShapePolygonGeometry {
    internal val radiusMultipliers: List<Float> = listOf(0.9f, 0.6f, 1.0f, 0.55f, 0.85f, 0.5f, 0.95f, 0.65f)

    internal fun vertices(size: Size): List<Offset> {
        val center = Offset(size.width / 2f, size.height / 2f)
        val baseRadius = minOf(size.width, size.height) / 2f
        val count = radiusMultipliers.size

        return radiusMultipliers.mapIndexed { index, multiplier ->
            val angle = (index.toFloat() / count) * 2f * Math.PI.toFloat()
            val radius = baseRadius * multiplier
            Offset(
                x = center.x + radius * cos(angle),
                y = center.y + radius * sin(angle),
            )
        }
    }
}

@Preview(name = "light")
@Composable
private fun CustomShapeUpsellGraphicLightPreview() {
    TownCrierTheme {
        CustomShapeUpsellGraphic()
    }
}

@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun CustomShapeUpsellGraphicDarkPreview() {
    TownCrierTheme(appearance = Appearance.Dark) {
        CustomShapeUpsellGraphic()
    }
}

@Preview(name = "dark, OLED", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun CustomShapeUpsellGraphicOledDarkPreview() {
    TownCrierTheme(appearance = Appearance.OledDark) {
        CustomShapeUpsellGraphic()
    }
}
