package uk.towncrierapp.presentation.designsystem.components

import android.content.res.Configuration
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierRadius
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme

/**
 * Threshold (metres) at or above which [LargeRadiusWarning] should be shown.
 * Set just above the free tier's 2 km cap so a free user pinned at their
 * maximum never trips it — only paid tiers, which can exceed 2 km, see it.
 * Port of iOS `LargeRadiusWarning.thresholdMetres`.
 */
public const val LARGE_RADIUS_WARNING_THRESHOLD_METRES: Float = 2_100f

/**
 * Callout that a watch zone with a large radius may generate a high volume
 * of notifications — amber-at-15%-opacity so it reads as a heads-up, not an
 * error (the radius itself is still allowed). Port of iOS
 * `LargeRadiusWarningView`.
 */
@Composable
public fun LargeRadiusWarning(modifier: Modifier = Modifier) {
    Row(
        modifier =
            modifier
                .fillMaxWidth()
                .clip(RoundedCornerShape(TownCrierRadius.md))
                .background(TownCrierTheme.colors.amberMuted)
                .padding(TownCrierSpacing.md),
        horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
    ) {
        Icon(
            imageVector = Icons.Filled.Warning,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.primary,
        )
        Column(verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
            Text(
                text = stringResource(R.string.large_radius_warning_title),
                style = TownCrierTheme.bodyEmphasis,
                color = MaterialTheme.colorScheme.onSurface,
            )
            Text(
                text = stringResource(R.string.large_radius_warning_body),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

@Preview(name = "light")
@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun LargeRadiusWarningPreview() {
    TownCrierTheme {
        LargeRadiusWarning(modifier = Modifier.padding(TownCrierSpacing.md))
    }
}
