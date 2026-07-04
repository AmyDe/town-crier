package uk.towncrierapp.presentation.features.onboarding

import android.content.res.Configuration
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.tooling.preview.Preview
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.designsystem.components.PrimaryButton

/**
 * Step 2 - free-text postcode entry. Shows a "Look up" button while there's
 * no resolved coordinate yet (disabled until [OnboardingUiState.postcodeInput]
 * is non-blank), then swaps to "Continue" once geocoding has succeeded.
 */
@Composable
internal fun PostcodeEntryStep(
    state: OnboardingUiState,
    onPostcodeChange: (String) -> Unit,
    onLookUpClick: () -> Unit,
    onContinueClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier.fillMaxSize().padding(TownCrierSpacing.lg),
        verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.md),
    ) {
        Text(
            text = stringResource(R.string.onboarding_postcode_title),
            style = MaterialTheme.typography.headlineMedium,
            color = MaterialTheme.colorScheme.onBackground,
        )
        Text(
            text = stringResource(R.string.onboarding_postcode_subtitle),
            style = MaterialTheme.typography.bodyLarge,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        OutlinedTextField(
            value = state.postcodeInput,
            onValueChange = onPostcodeChange,
            placeholder = { Text(stringResource(R.string.onboarding_postcode_placeholder)) },
            singleLine = true,
            keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Characters),
            modifier = Modifier.fillMaxWidth(),
        )
        state.postcodeError?.let { error ->
            Text(
                text = stringResource(onboardingPostcodeErrorMessageRes(error)),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.error,
            )
        }
        PostcodeStepAction(state = state, onLookUpClick = onLookUpClick, onContinueClick = onContinueClick)
    }
}

@Composable
private fun PostcodeStepAction(
    state: OnboardingUiState,
    onLookUpClick: () -> Unit,
    onContinueClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    when {
        state.isLookingUpPostcode ->
            CircularProgressIndicator(
                modifier = modifier.padding(TownCrierSpacing.sm),
                color = MaterialTheme.colorScheme.primary,
            )

        state.geocodedCoordinate != null ->
            PrimaryButton(
                text = stringResource(R.string.onboarding_postcode_continue_button),
                onClick = onContinueClick,
                modifier = modifier,
            )

        else ->
            PrimaryButton(
                text = stringResource(R.string.onboarding_postcode_lookup_button),
                onClick = onLookUpClick,
                enabled = state.postcodeInput.isNotBlank(),
                modifier = modifier,
            )
    }
}

@Preview(name = "empty")
@Composable
private fun PostcodeEntryStepEmptyPreview() {
    TownCrierTheme {
        PostcodeEntryStep(state = OnboardingUiState(), onPostcodeChange = {}, onLookUpClick = {}, onContinueClick = {})
    }
}

@Preview(name = "geocoded")
@Composable
private fun PostcodeEntryStepGeocodedPreview() {
    TownCrierTheme {
        PostcodeEntryStep(
            state =
                OnboardingUiState(
                    postcodeInput = "SW1A 1AA",
                    geocodedCoordinate = Coordinate(51.5074, -0.1278),
                ),
            onPostcodeChange = {},
            onLookUpClick = {},
            onContinueClick = {},
        )
    }
}

@Preview(name = "error", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun PostcodeEntryStepErrorPreview() {
    TownCrierTheme {
        PostcodeEntryStep(
            state = OnboardingUiState(postcodeInput = "NOTAPOSTCODE", postcodeError = DomainError.GeocodingFailed("NOTAPOSTCODE")),
            onPostcodeChange = {},
            onLookUpClick = {},
            onContinueClick = {},
        )
    }
}
