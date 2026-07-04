package uk.towncrierapp.presentation.designsystem.components

import android.content.res.Configuration
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme

/**
 * A toggle that is proactively disabled when the user's tier lacks the
 * required entitlement. Per the epic's Material-native-idiom chapter, the
 * locked state is NOT a greyed-out switch — it renders as a tappable row
 * (tertiary label + [UpgradeBadge]) that invokes [onUpgradeRequired] on tap.
 * Reused by the watch-zone editor's instant-alert toggles and, per tc-z95t's
 * bead brief, by #778's notification-preferences screen. Port of iOS
 * `GatedToggle`.
 */
@Composable
public fun GatedToggle(
    label: String,
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit,
    isEnabled: Boolean,
    modifier: Modifier = Modifier,
    onUpgradeRequired: () -> Unit = {},
) {
    if (isEnabled) {
        Row(
            modifier =
                modifier
                    .fillMaxWidth()
                    .heightIn(min = 44.dp)
                    .padding(vertical = TownCrierSpacing.xs),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            Text(text = label, style = MaterialTheme.typography.bodyLarge, color = MaterialTheme.colorScheme.onSurface)
            Switch(
                checked = checked,
                onCheckedChange = onCheckedChange,
                colors = SwitchDefaults.colors(checkedTrackColor = MaterialTheme.colorScheme.primary),
            )
        }
    } else {
        Row(
            modifier =
                modifier
                    .fillMaxWidth()
                    .heightIn(min = 44.dp)
                    .clickable(onClick = onUpgradeRequired)
                    .padding(vertical = TownCrierSpacing.xs),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            Text(
                text = label,
                style = MaterialTheme.typography.bodyLarge,
                color = TownCrierTheme.colors.textTertiary,
            )
            UpgradeBadge()
        }
    }
}

@Preview(name = "unlocked, light")
@Preview(name = "unlocked, dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun GatedToggleUnlockedPreview() {
    TownCrierTheme {
        GatedToggle(label = "Send push notifications", checked = true, onCheckedChange = {}, isEnabled = true)
    }
}

@Preview(name = "locked, light")
@Preview(name = "locked, dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun GatedToggleLockedPreview() {
    TownCrierTheme {
        GatedToggle(label = "Send push notifications", checked = false, onCheckedChange = {}, isEnabled = false)
    }
}
