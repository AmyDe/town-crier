package uk.towncrierapp.presentation.features.watchzones

import android.content.res.Configuration
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Notifications
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.designsystem.components.PrimaryButton

/**
 * Richer inline upsell card shown beneath a free-tier user's single watch
 * zone once they hit their one-zone cap (tc-t8hc) — see
 * [WatchZoneListUiState.showsFreeTierUpsell]
 * for the single source of truth on visibility. Sells the whole plan, not
 * just one feature; copy is verbatim iOS wording (bead brief tc-z95t). Port
 * of iOS `WatchZoneInlineUpsellCard`.
 */
@Composable
public fun WatchZoneInlineUpsellCard(
    onViewPlans: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Surface(
        modifier = modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surfaceContainerHigh,
        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
    ) {
        Column(
            modifier = Modifier.padding(TownCrierSpacing.md),
            verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.md),
        ) {
            Column(verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm)) {
                Text(
                    text = stringResource(R.string.watch_zone_upsell_title),
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                )
                BenefitRow(icon = Icons.Filled.Add, text = stringResource(R.string.watch_zone_upsell_bigger_zones))
                BenefitRow(
                    icon = Icons.Filled.Add,
                    text = stringResource(R.string.watch_zone_upsell_more_than_one_zone),
                )
                BenefitRow(
                    icon = Icons.Filled.Notifications,
                    text = stringResource(R.string.watch_zone_upsell_instant_alerts),
                )
                Text(
                    text = stringResource(R.string.watch_zone_upsell_free_clarifier),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            PrimaryButton(text = stringResource(R.string.watch_zone_upsell_view_plans), onClick = onViewPlans)
        }
    }
}

@Composable
private fun BenefitRow(
    icon: ImageVector,
    text: String,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier,
        horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
        verticalAlignment = Alignment.Top,
    ) {
        Icon(imageVector = icon, contentDescription = null, tint = MaterialTheme.colorScheme.primary)
        Text(text = text, style = MaterialTheme.typography.bodyLarge, color = MaterialTheme.colorScheme.onSurface)
    }
}

@Preview(name = "light")
@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun WatchZoneInlineUpsellCardPreview() {
    TownCrierTheme {
        WatchZoneInlineUpsellCard(onViewPlans = {}, modifier = Modifier.padding(TownCrierSpacing.md))
    }
}
