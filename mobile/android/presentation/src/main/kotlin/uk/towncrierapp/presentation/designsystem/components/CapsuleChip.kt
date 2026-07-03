package uk.towncrierapp.presentation.designsystem.components

import android.content.res.Configuration
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import uk.towncrierapp.presentation.designsystem.TownCrierRadius
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme

/**
 * A capsule filter/selection chip. Selected: amber fill, `onAccent` text, no
 * border. Unselected: surface fill, bordered (design-language skill). Purely
 * stateless — [selected] and [onClick] are hoisted; this component owns no
 * internal state.
 */
@Composable
public fun CapsuleChip(
    label: String,
    selected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val containerColor = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.surface
    val contentColor = if (selected) MaterialTheme.colorScheme.onPrimary else MaterialTheme.colorScheme.onSurface
    val border = if (selected) null else BorderStroke(1.dp, MaterialTheme.colorScheme.outline)

    Surface(
        modifier = modifier,
        onClick = onClick,
        shape = TownCrierRadius.full,
        color = containerColor,
        contentColor = contentColor,
        border = border,
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.labelMedium,
            modifier = Modifier.padding(horizontal = TownCrierSpacing.md, vertical = TownCrierSpacing.sm),
        )
    }
}

@Preview(name = "light")
@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun CapsuleChipPreview() {
    TownCrierTheme {
        Row(horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm)) {
            CapsuleChip(label = "Nearby", selected = true, onClick = {})
            CapsuleChip(label = "This week", selected = false, onClick = {})
        }
    }
}
