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
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.Appearance
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import kotlin.math.cos
import kotlin.math.sin

private val TILE_SIZE = 96.dp

private val OUTLINE_STROKE_WIDTH = 2.dp

private val CIRCLE_DASH_ON = 18.dp
private val CIRCLE_DASH_OFF = 12.dp

/**
 * Decorative graphic contrasting the default circle watch zone with a custom
 * shape (polygon) watch zone, to explain the paid custom-shape entitlement.
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
                        pathEffect =
                            PathEffect.dashPathEffect(
                                floatArrayOf(CIRCLE_DASH_ON.toPx(), CIRCLE_DASH_OFF.toPx()),
                                phase = 0f,
                            ),
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

/**
 * Each multiplier is at most `1.0` so every vertex stays inside the given
 * [Size]; they are deliberately irregular so the outline reads as hand-drawn,
 * and match iOS verbatim for visual parity.
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
