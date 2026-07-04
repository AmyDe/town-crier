package uk.towncrierapp.presentation.features.watchzones

import androidx.annotation.StringRes
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Slider
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.unit.dp
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.designsystem.components.GatedToggle
import uk.towncrierapp.presentation.designsystem.components.LargeRadiusWarning
import uk.towncrierapp.presentation.designsystem.components.UnlockLargerZonesChip

/** The editor's per-section building blocks — split from `WatchZoneEditorScreen.kt` to keep both files under the file/class function-count budget. */
@Composable
internal fun NameSection(
    name: String,
    onNameChange: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
        Text(text = stringResource(R.string.watch_zone_editor_name_label), style = MaterialTheme.typography.labelMedium)
        OutlinedTextField(
            value = name,
            onValueChange = onNameChange,
            placeholder = { Text(stringResource(R.string.watch_zone_editor_name_placeholder)) },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
        )
    }
}

@Composable
internal fun PostcodeSection(
    postcode: String,
    isLoading: Boolean,
    onPostcodeChange: (String) -> Unit,
    onLookUpClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
        Text(
            text = stringResource(R.string.watch_zone_editor_postcode_label),
            style = MaterialTheme.typography.labelMedium,
        )
        Row(
            horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            OutlinedTextField(
                value = postcode,
                onValueChange = onPostcodeChange,
                placeholder = { Text(stringResource(R.string.watch_zone_editor_postcode_placeholder)) },
                singleLine = true,
                keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Characters),
                modifier = Modifier.weight(1f),
            )
            if (isLoading) {
                CircularProgressIndicator(modifier = Modifier.padding(TownCrierSpacing.sm))
            } else {
                TextButton(onClick = onLookUpClick, enabled = postcode.isNotBlank()) {
                    Text(stringResource(R.string.watch_zone_editor_lookup_button))
                }
            }
        }
        Text(
            text = stringResource(R.string.watch_zone_editor_postcode_footer),
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
internal fun RadiusSection(
    state: WatchZoneEditorUiState,
    onRadiusChange: (Float) -> Unit,
    onUnlockLargerZonesClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm)) {
        Text(
            text = stringResource(R.string.watch_zone_editor_radius_label),
            style = MaterialTheme.typography.labelMedium,
        )
        Text(text = RadiusFormatter.format(state.radiusMetres.toDouble()), style = TownCrierTheme.bodyEmphasis)
        Slider(
            value = state.radiusMetres,
            onValueChange = onRadiusChange,
            valueRange = state.minRadiusMetres..state.maxRadiusMetres,
            steps = radiusSliderSteps(state.minRadiusMetres, state.maxRadiusMetres),
        )
        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
            Text(
                text = RadiusFormatter.format(state.minRadiusMetres.toDouble()),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Text(
                text = RadiusFormatter.format(state.maxRadiusMetres.toDouble()),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        if (state.canUnlockLargerRadius) {
            UnlockLargerZonesChip(onClick = onUnlockLargerZonesClick)
        }
        if (state.showsLargeRadiusWarning) {
            LargeRadiusWarning()
        }
    }
}

/** 100 m steps between [min] and [max] (design-language radius slider spec). */
private fun radiusSliderSteps(
    min: Float,
    max: Float,
): Int = (((max - min) / RADIUS_STEP_METRES).toInt() - 1).coerceAtLeast(0)

private const val RADIUS_STEP_METRES = 100f

@Composable
internal fun PreviewSection(modifier: Modifier = Modifier) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm)) {
        Text(
            text = stringResource(R.string.watch_zone_editor_preview_label),
            style = MaterialTheme.typography.labelMedium,
        )
        ZoneMapPlaceholder(modifier = Modifier.fillMaxWidth().height(200.dp))
    }
}

@Composable
internal fun NotificationsSection(
    state: WatchZoneEditorUiState,
    onPushEnabledChange: (Boolean) -> Unit,
    onEmailInstantEnabledChange: (Boolean) -> Unit,
    onUpgradeRequired: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val isUnlocked = state.featureGate.hasEntitlement(state.instantAlertEntitlement)
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
        GatedToggle(
            label = stringResource(R.string.watch_zone_editor_push_label),
            checked = state.pushEnabled,
            onCheckedChange = onPushEnabledChange,
            isEnabled = isUnlocked,
            onUpgradeRequired = onUpgradeRequired,
        )
        GatedToggle(
            label = stringResource(R.string.watch_zone_editor_email_label),
            checked = state.emailInstantEnabled,
            onCheckedChange = onEmailInstantEnabledChange,
            isEnabled = isUnlocked,
            onUpgradeRequired = onUpgradeRequired,
        )
        Text(
            text =
                stringResource(
                    if (isUnlocked) {
                        R.string.watch_zone_editor_notifications_footer_unlocked
                    } else {
                        R.string.watch_zone_editor_notifications_footer_locked
                    },
                ),
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
internal fun ErrorSection(
    @StringRes messageRes: Int,
    modifier: Modifier = Modifier,
) {
    Row(modifier = modifier, horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm)) {
        Icon(imageVector = Icons.Filled.Warning, contentDescription = null, tint = MaterialTheme.colorScheme.error)
        Text(
            text = stringResource(messageRes),
            style = MaterialTheme.typography.bodyLarge,
            color = MaterialTheme.colorScheme.error,
        )
    }
}
