package uk.towncrierapp.presentation.features.onboarding

import android.content.res.Configuration
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Slider
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
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
 */
@Composable
internal fun RadiusPickerStep(
    state: OnboardingUiState,
    onRadiusChange: (Float) -> Unit,
    onConfirmClick: () -> Unit,
    onUnlockLargerZonesClick: () -> Unit,
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
        )
    }
}
