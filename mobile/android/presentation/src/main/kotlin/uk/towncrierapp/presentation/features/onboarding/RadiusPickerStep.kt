package uk.towncrierapp.presentation.features.onboarding

import android.content.res.Configuration
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Edit
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Slider
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.designsystem.components.CustomShapeUpsellGraphic
import uk.towncrierapp.presentation.designsystem.components.LargeRadiusWarning
import uk.towncrierapp.presentation.designsystem.components.PrimaryButton
import uk.towncrierapp.presentation.designsystem.components.UnlockLargerZonesChip
import uk.towncrierapp.presentation.features.watchzones.RadiusFormatter

private const val RADIUS_STEP_METRES = 100f

/**
 * Step 3 - a slider from [OnboardingUiState.minRadiusMetres] to the tier's
 * max, defaulting to 1 km. [OnboardingUiState.canUnlockLargerRadius] is
 * false whenever the paywall isn't available yet (#783), which hides the
 * chip entirely rather than routing to a dead tap target.
 *
 * Custom-shape placement (GH#1072 Phase 5, tc-v6fo0.5, mirrors iOS
 * `RadiusPickerStepView`): directly below the radius control, ahead of the
 * larger-radius chip, so the polygon capability isn't the last thing a user
 * scrolls past. A tier without [OnboardingUiState.allowsCustomBoundary] sees
 * [CustomShapeUpsellCard]; an already-entitled tier sees [CustomShapeAvailableCard]
 * instead - the re-entry point after backing out of drawing once.
 */
@Composable
internal fun RadiusPickerStep(
    state: OnboardingUiState,
    onRadiusChange: (Float) -> Unit,
    onConfirmClick: () -> Unit,
    onUnlockLargerZonesClick: () -> Unit,
    onCustomShapeUpsellClick: () -> Unit,
    onDrawCustomShapeClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier.fillMaxSize().padding(TownCrierSpacing.lg),
        verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.md),
    ) {
        Text(
            text = stringResource(R.string.onboarding_radius_title),
            style = MaterialTheme.typography.headlineMedium,
            color = MaterialTheme.colorScheme.onBackground,
        )
        Text(
            text = stringResource(R.string.onboarding_radius_subtitle),
            style = MaterialTheme.typography.bodyLarge,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Text(text = RadiusFormatter.format(state.radiusMetres.toDouble()), style = TownCrierTheme.bodyEmphasis)
        Slider(
            value = state.radiusMetres,
            onValueChange = onRadiusChange,
            valueRange = state.minRadiusMetres..state.maxRadiusMetres,
            steps = radiusSliderSteps(state.minRadiusMetres, state.maxRadiusMetres),
        )
        Row(modifier = Modifier.fillMaxWidth()) {
            Text(
                text = RadiusFormatter.format(state.minRadiusMetres.toDouble()),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Spacer(modifier = Modifier.weight(1f))
            Text(
                text = RadiusFormatter.format(state.maxRadiusMetres.toDouble()),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        if (state.allowsCustomBoundary) {
            CustomShapeAvailableCard(onClick = onDrawCustomShapeClick)
        } else {
            CustomShapeUpsellCard(onUpgradeClick = onCustomShapeUpsellClick)
        }
        if (state.canUnlockLargerRadius) {
            UnlockLargerZonesChip(onClick = onUnlockLargerZonesClick)
        }
        if (state.showsLargeRadiusWarning) {
            LargeRadiusWarning()
        }
        Spacer(modifier = Modifier.weight(1f))
        PrimaryButton(text = stringResource(R.string.onboarding_radius_confirm_button), onClick = onConfirmClick)
    }
}

/**
 * What a Free-tier user sees on the radius step: the Phase 4 upsell graphic
 * plus a CTA to the future (#783) in-wizard paywall - the onboarding
 * placement of the same upsell `WatchZoneEditorSections.CustomShapeUpsellSection`
 * shows in the editor. Mirrors iOS `CustomShapeUpsellCard`.
 */
@Composable
private fun CustomShapeUpsellCard(
    onUpgradeClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm)) {
        CustomShapeUpsellGraphic()
        Text(
            text = stringResource(R.string.onboarding_custom_shape_upsell_body),
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        TextButton(onClick = onUpgradeClick) {
            Text(stringResource(R.string.watch_zone_upsell_view_plans))
        }
    }
}

/**
 * Re-entry affordance for a user whose tier already allows a custom shape
 * (an already-Personal/Pro user, or one who just upgraded and backed out of
 * drawing once) - mirrors iOS `CustomShapeAvailableCard` /
 * [OnboardingViewModel.selectCustomShape]. A direct tap target, never a
 * hidden/disabled toggle.
 */
@Composable
private fun CustomShapeAvailableCard(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier =
            modifier
                .fillMaxWidth()
                .heightIn(min = 44.dp)
                .clickable(onClick = onClick),
        horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(imageVector = Icons.Filled.Edit, contentDescription = null, tint = MaterialTheme.colorScheme.primary)
        Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
            Text(
                text = stringResource(R.string.onboarding_custom_shape_available_title),
                style = TownCrierTheme.bodyEmphasis,
                color = MaterialTheme.colorScheme.onSurface,
            )
            Text(
                text = stringResource(R.string.onboarding_custom_shape_available_body),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

/** 100 m steps between [min] and [max] (design-language radius slider spec) - same arithmetic as `WatchZoneEditorSections.radiusSliderSteps`. */
private fun radiusSliderSteps(
    min: Float,
    max: Float,
): Int = (((max - min) / RADIUS_STEP_METRES).toInt() - 1).coerceAtLeast(0)

@Preview(name = "light")
@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun RadiusPickerStepPreview() {
    TownCrierTheme {
        RadiusPickerStep(
            state = OnboardingUiState(radiusMetres = 1_500f, maxRadiusMetres = 2_000f),
            onRadiusChange = {},
            onConfirmClick = {},
            onUnlockLargerZonesClick = {},
            onCustomShapeUpsellClick = {},
            onDrawCustomShapeClick = {},
        )
    }
}

@Preview(name = "warning + unlock chip")
@Composable
private fun RadiusPickerStepWarningPreview() {
    TownCrierTheme {
        RadiusPickerStep(
            state =
                OnboardingUiState(
                    radiusMetres = 2_500f,
                    maxRadiusMetres = 5_000f,
                    canUnlockLargerRadius = true,
                    showsLargeRadiusWarning = true,
                ),
            onRadiusChange = {},
            onConfirmClick = {},
            onUnlockLargerZonesClick = {},
            onCustomShapeUpsellClick = {},
            onDrawCustomShapeClick = {},
        )
    }
}

@Preview(name = "custom-shape entitled")
@Composable
private fun RadiusPickerStepEntitledPreview() {
    TownCrierTheme {
        RadiusPickerStep(
            state =
                OnboardingUiState(
                    radiusMetres = 2_500f,
                    maxRadiusMetres = 10_000f,
                    allowsCustomBoundary = true,
                ),
            onRadiusChange = {},
            onConfirmClick = {},
            onUnlockLargerZonesClick = {},
            onCustomShapeUpsellClick = {},
            onDrawCustomShapeClick = {},
        )
    }
}
