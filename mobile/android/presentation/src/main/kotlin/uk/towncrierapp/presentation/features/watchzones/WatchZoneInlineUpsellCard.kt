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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.em
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierShapes
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.designsystem.components.PrimaryButton

/** Border thickness of the upsell card's amber outline — see [WatchZoneInlineUpsellCard] doc. */
private val UPSELL_CARD_BORDER_THICKNESS = 1.5.dp

/** Letter-spacing of the upsell card's small-caps eyebrow — see [WatchZoneInlineUpsellCard] doc. */
private val UPSELL_EYEBROW_LETTER_SPACING = 0.12.em

/**
 * Richer inline upsell card shown beneath a free-tier user's single watch
 * zone once they hit their one-zone cap (tc-t8hc) — see
 * [WatchZoneListUiState.showsFreeTierUpsell]
 * for the single source of truth on visibility. Sells the whole plan, not
 * just one feature; copy is verbatim iOS wording (bead brief tc-z95t). Port
 * of iOS `WatchZoneInlineUpsellCard`.
 *
 * Styling follows the Public Notice upsell-surface language (epic #848 R5):
 * a 1.5dp amber border and a brass small-caps eyebrow, deliberately NOT the
 * shared [uk.towncrierapp.presentation.designsystem.noticeCard] treatment —
 * this card's border itself carries the brand accent. The "View Plans" CTA
 * ([PrimaryButton], amber-filled) is the only filled-amber container on the
 * card — the card itself stays bordered, never filled (amber-rationing
 * rule, same as web/iOS).
 */
@Composable
public fun WatchZoneInlineUpsellCard(
    onViewPlans: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Surface(
        modifier = modifier.fillMaxWidth(),
        shape = TownCrierShapes.medium,
        color = MaterialTheme.colorScheme.surfaceContainerHigh,
        border = BorderStroke(UPSELL_CARD_BORDER_THICKNESS, MaterialTheme.colorScheme.primary),
    ) {
        Column(
            modifier = Modifier.padding(TownCrierSpacing.md),
            verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.md),
        ) {
            Column(verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm)) {
                Text(
                    text = stringResource(R.string.watch_zone_upsell_eyebrow).uppercase(),
                    style =
                        MaterialTheme.typography.labelMedium.copy(
                            letterSpacing = UPSELL_EYEBROW_LETTER_SPACING,
                            fontWeight = FontWeight.SemiBold,
                        ),
                    color = MaterialTheme.colorScheme.primary,
                )
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
