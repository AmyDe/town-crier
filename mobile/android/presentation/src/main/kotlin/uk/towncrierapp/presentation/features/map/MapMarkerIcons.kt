package uk.towncrierapp.presentation.features.map

import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.Paint
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.dp
import com.google.android.gms.maps.model.BitmapDescriptor
import com.google.android.gms.maps.model.BitmapDescriptorFactory
import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.presentation.designsystem.TownCrierColors
import uk.towncrierapp.presentation.designsystem.TownCrierTheme

private val PIN_DIAMETER_DP = 22.dp
private val BUBBLE_DIAMETER_DP = 26.dp
private const val PIN_BORDER_FRACTION = 0.08f
private const val BUBBLE_TEXT_SIZE_FRACTION = 0.4f
private const val BUBBLE_COUNT_CAP = 999

/** The count-bubble label for [count] — capped so a fully-zoomed-out dense cell stays legible (matches iOS `bubbleGlyph`). */
internal fun bubbleLabel(count: Int): String = if (count > BUBBLE_COUNT_CAP) "999+" else count.toString()

/**
 * Generated, per-status/per-label [BitmapDescriptor]s for map markers — the
 * Android counterpart to iOS's `MKMarkerAnnotationView` tinting (GH#776).
 * Icons are drawn once (a filled circle, white-bordered) and cached for the
 * lifetime of this instance, which callers `remember` for the composition's
 * lifetime (see [rememberMapMarkerIcons]) so re-tapping the map never
 * regenerates a bitmap it's already drawn.
 */
internal class MapMarkerIcons(
    private val density: Density,
    private val statusColors: TownCrierColors,
    private val bubbleColor: Color,
    private val bubbleTextColor: Color,
) {
    private val pinDiameterPx = with(density) { PIN_DIAMETER_DP.roundToPx() }
    private val bubbleDiameterPx = with(density) { BUBBLE_DIAMETER_DP.roundToPx() }

    private val statusCache = mutableMapOf<ApplicationStatus, BitmapDescriptor>()
    private val bubbleCache = mutableMapOf<String, BitmapDescriptor>()

    /** The status-tinted pin for a single-member cluster. */
    fun statusIcon(status: ApplicationStatus): BitmapDescriptor =
        statusCache.getOrPut(status) { solidCircleBitmap(colorFor(status), pinDiameterPx) }

    /** The amber count bubble for a splittable multi-member cluster. */
    fun bubbleIcon(count: Int): BitmapDescriptor {
        val label = bubbleLabel(count)
        return bubbleCache.getOrPut(label) { countBubbleBitmap(label) }
    }

    private fun colorFor(status: ApplicationStatus): Color =
        when (status) {
            ApplicationStatus.Permitted -> statusColors.statusPermitted

            ApplicationStatus.Conditions -> statusColors.statusConditions

            ApplicationStatus.Rejected -> statusColors.statusRejected

            ApplicationStatus.Undecided -> statusColors.statusPending

            ApplicationStatus.Withdrawn -> statusColors.statusWithdrawn

            ApplicationStatus.Appealed -> statusColors.statusAppealed

            ApplicationStatus.Unresolved,
            ApplicationStatus.Referred,
            ApplicationStatus.NotAvailable,
            is ApplicationStatus.Unknown,
            -> statusColors.textTertiary
        }

    private fun countBubbleBitmap(label: String): BitmapDescriptor {
        val bitmap = Bitmap.createBitmap(bubbleDiameterPx, bubbleDiameterPx, Bitmap.Config.ARGB_8888)
        val canvas = Canvas(bitmap)
        val radius = bubbleDiameterPx / 2f
        val fillPaint = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = bubbleColor.toArgb() }
        canvas.drawCircle(radius, radius, radius, fillPaint)
        val textPaint =
            Paint(Paint.ANTI_ALIAS_FLAG).apply {
                color = bubbleTextColor.toArgb()
                textAlign = Paint.Align.CENTER
                textSize = bubbleDiameterPx * BUBBLE_TEXT_SIZE_FRACTION
            }
        val textY = radius - (textPaint.descent() + textPaint.ascent()) / 2
        canvas.drawText(label, radius, textY, textPaint)
        return BitmapDescriptorFactory.fromBitmap(bitmap)
    }

    private fun solidCircleBitmap(
        color: Color,
        diameterPx: Int,
    ): BitmapDescriptor {
        val bitmap = Bitmap.createBitmap(diameterPx, diameterPx, Bitmap.Config.ARGB_8888)
        val canvas = Canvas(bitmap)
        val radius = diameterPx / 2f
        val fillPaint = Paint(Paint.ANTI_ALIAS_FLAG).apply { this.color = color.toArgb() }
        canvas.drawCircle(radius, radius, radius, fillPaint)
        val borderPaint =
            Paint(Paint.ANTI_ALIAS_FLAG).apply {
                this.color = Color.White.toArgb()
                style = Paint.Style.STROKE
                strokeWidth = diameterPx * PIN_BORDER_FRACTION
            }
        canvas.drawCircle(radius, radius, radius - borderPaint.strokeWidth / 2, borderPaint)
        return BitmapDescriptorFactory.fromBitmap(bitmap)
    }
}

/** Remembers one [MapMarkerIcons] instance per theme+density (colors change across light/dark/OLED, bitmap sizing changes across density buckets), so its bitmap caches survive recomposition but regenerate on a theme or density switch. */
@Composable
internal fun rememberMapMarkerIcons(): MapMarkerIcons {
    val density = LocalDensity.current
    val statusColors = TownCrierTheme.colors
    val bubbleColor = MaterialTheme.colorScheme.primary
    val bubbleTextColor = MaterialTheme.colorScheme.onPrimary
    return remember(density, statusColors, bubbleColor, bubbleTextColor) {
        MapMarkerIcons(density, statusColors, bubbleColor, bubbleTextColor)
    }
}
