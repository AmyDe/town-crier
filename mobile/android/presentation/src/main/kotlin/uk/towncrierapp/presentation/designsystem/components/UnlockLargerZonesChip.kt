package uk.towncrierapp.presentation.designsystem.components

import android.content.res.Configuration
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierRadius
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme

/**
 * In-context upsell affordance beneath the radius slider when the user's
 * tier caps their radius below Pro's 10 km ceiling — a tappable chip rather
 * than a drag-past-the-cap gesture. Sells the whole upgrade, not just a
 * bigger radius (copy shared verbatim with [uk.towncrierapp.presentation.features.watchzones.WatchZoneInlineUpsellCard]).
 * Port of iOS `UnlockLargerZonesChip`.
 */
@Composable
public fun UnlockLargerZonesChip(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier =
            modifier
                .fillMaxWidth()
                .heightIn(min = 44.dp)
                .clip(RoundedCornerShape(TownCrierRadius.sm))
                .background(TownCrierTheme.colors.amberMuted)
                .clickable(onClick = onClick)
                .padding(horizontal = TownCrierSpacing.md, vertical = TownCrierSpacing.sm),
        horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(imageVector = Icons.Filled.Lock, contentDescription = null, tint = MaterialTheme.colorScheme.primary)
        Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
            Text(
                text = stringResource(R.string.unlock_larger_zones_title),
                style = TownCrierTheme.bodyEmphasis,
                color = MaterialTheme.colorScheme.onSurface,
            )
            Text(
                text = stringResource(R.string.unlock_larger_zones_body),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        Icon(
            imageVector = Icons.Filled.KeyboardArrowRight,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Preview(name = "light")
@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun UnlockLargerZonesChipPreview() {
    TownCrierTheme {
        UnlockLargerZonesChip(onClick = {}, modifier = Modifier.padding(TownCrierSpacing.md))
    }
}
