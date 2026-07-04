package uk.towncrierapp.presentation.designsystem.components

import android.content.res.Configuration
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierTheme

/**
 * The small "Upgrade" pill shown wherever a free/lower-tier user has hit a
 * gate — the "+" button at watch-zone quota, [GatedToggle]'s locked row, and
 * (later) other entitlement-gated affordances. A thin, purpose-named wrapper
 * over [StatusBadge] rather than a second badge shape (design-language skill,
 * Status Badges).
 */
@Composable
public fun UpgradeBadge(modifier: Modifier = Modifier) {
    StatusBadge(
        label = stringResource(R.string.upgrade_badge_label),
        color = MaterialTheme.colorScheme.primary,
        icon = Icons.Filled.Lock,
        modifier = modifier,
    )
}

@Preview(name = "light")
@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun UpgradeBadgePreview() {
    TownCrierTheme {
        UpgradeBadge()
    }
}
