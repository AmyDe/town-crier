package uk.towncrierapp.presentation.designsystem.components

import android.content.res.Configuration
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import uk.towncrierapp.presentation.designsystem.TownCrierRadius
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme

/**
 * A compact capsule indicator for planning application status: the given
 * [color] at 15% fill for the background, full opacity for the icon and
 * label (design-language skill, Status Badges). This is the ONLY status
 * badge in this design system — do not add a second variant.
 *
 * Callers pass [color] from `TownCrierTheme.colors.status*` — this component
 * has no opinion on which domain status maps to which color; that mapping
 * lands with the domain status type in a later phase of the Android parity
 * epic (#770). The icon is always paired with a text [label] (never color
 * alone) for colour-blind accessibility; its content description is null
 * because [label] already conveys the same information.
 */
@Composable
public fun StatusBadge(
    label: String,
    color: Color,
    modifier: Modifier = Modifier,
    icon: ImageVector = Icons.Filled.CheckCircle,
) {
    Surface(
        modifier = modifier,
        shape = TownCrierRadius.full,
        color = color.copy(alpha = 0.15f),
    ) {
        Row(
            modifier = Modifier.padding(horizontal = TownCrierSpacing.sm, vertical = TownCrierSpacing.xs),
            horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(imageVector = icon, contentDescription = null, tint = color, modifier = Modifier.size(14.dp))
            Text(text = label, style = MaterialTheme.typography.labelMedium, color = color)
        }
    }
}

@Preview(name = "light")
@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun StatusBadgePreview() {
    TownCrierTheme {
        Row(horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm)) {
            StatusBadge(label = "Permitted", color = TownCrierTheme.colors.statusPermitted)
            StatusBadge(label = "Rejected", color = TownCrierTheme.colors.statusRejected)
            StatusBadge(label = "Pending", color = TownCrierTheme.colors.statusPending)
        }
    }
}
