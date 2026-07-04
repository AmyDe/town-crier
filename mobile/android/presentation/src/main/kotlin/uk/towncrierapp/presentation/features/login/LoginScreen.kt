package uk.towncrierapp.presentation.features.login

import android.content.res.Configuration
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.designsystem.components.PrimaryButton

@Composable
public fun LoginRoute(
    viewModel: LoginViewModel,
    modifier: Modifier = Modifier,
) {
    val state by viewModel.uiState.collectAsStateWithLifecycle()
    LaunchedEffect(Unit) { viewModel.checkExistingSession() }
    LoginScreen(state = state, onSignInClick = viewModel::login, modifier = modifier)
}

@Composable
internal fun LoginScreen(
    state: LoginUiState,
    onSignInClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Surface(modifier = modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(
            modifier =
                Modifier
                    .fillMaxSize()
                    .padding(PaddingValues(horizontal = TownCrierSpacing.lg, vertical = TownCrierSpacing.xxl)),
            verticalArrangement = Arrangement.SpaceBetween,
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            BrandingSection()
            SignInSection(state = state, onSignInClick = onSignInClick)
        }
    }
}

@Composable
private fun BrandingSection(modifier: Modifier = Modifier) {
    Column(
        modifier = modifier,
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        Text(
            text = stringResource(R.string.login_branding_title),
            style = MaterialTheme.typography.headlineLarge,
            color = MaterialTheme.colorScheme.onBackground,
        )
        Text(
            text = stringResource(R.string.login_branding_subtitle),
            style = MaterialTheme.typography.bodyLarge,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
private fun SignInSection(
    state: LoginUiState,
    onSignInClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier, horizontalAlignment = Alignment.CenterHorizontally) {
        if (state.isLoading) {
            CircularProgressIndicator(
                color = MaterialTheme.colorScheme.primary,
                modifier = Modifier.padding(TownCrierSpacing.md),
            )
        } else {
            PrimaryButton(text = stringResource(R.string.login_sign_in_button), onClick = onSignInClick)
        }
        state.errorMessageRes?.let { errorRes ->
            Text(
                text = stringResource(errorRes),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.error,
                modifier = Modifier.padding(top = TownCrierSpacing.sm),
            )
        }
    }
}

@Preview(name = "light")
@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun LoginScreenPreview() {
    TownCrierTheme {
        LoginScreen(state = LoginUiState(), onSignInClick = {})
    }
}

@Preview(name = "loading")
@Composable
private fun LoginScreenLoadingPreview() {
    TownCrierTheme {
        LoginScreen(state = LoginUiState(isLoading = true), onSignInClick = {})
    }
}

@Preview(name = "error")
@Composable
private fun LoginScreenErrorPreview() {
    TownCrierTheme {
        LoginScreen(
            state = LoginUiState(errorMessageRes = R.string.login_error_authentication_failed),
            onSignInClick = {},
        )
    }
}
