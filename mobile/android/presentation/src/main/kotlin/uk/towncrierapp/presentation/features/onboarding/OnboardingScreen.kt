package uk.towncrierapp.presentation.features.onboarding

import android.Manifest
import android.content.res.Configuration
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.Scaffold
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierTheme

/**
 * The onboarding wizard: one route hosting all four steps behind a single
 * [OnboardingViewModel] instance (never re-created per step), with a linear
 * progress affordance across the top. Port of iOS `OnboardingView`. The
 * device-permission request (API 33+) is wired here rather than in the
 * ViewModel, since `rememberLauncherForActivityResult` needs a composable
 * scope - its result is deliberately never observed (epic #770 parity:
 * completion proceeds regardless of grant/deny).
 */
@Composable
public fun OnboardingRoute(
    viewModel: OnboardingViewModel,
    onOnboardingComplete: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val state by viewModel.uiState.collectAsStateWithLifecycle()

    val notificationPermissionLauncher =
        rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) {
            // Result intentionally ignored - see the class doc above.
        }

    LaunchedEffect(state.isComplete) {
        if (state.isComplete) onOnboardingComplete()
    }

    OnboardingScreen(
        state = state,
        onGetStartedClick = viewModel::advance,
        onBackClick = viewModel::back,
        onPostcodeChange = viewModel::updatePostcode,
        onLookUpClick = viewModel::lookUpPostcode,
        onContinueFromPostcodeClick = viewModel::advance,
        onRadiusChange = viewModel::updateRadius,
        onConfirmRadiusClick = viewModel::confirmRadius,
        // #783 hasn't shipped - the chip is hidden (OnboardingUiState.canUnlockLargerRadius), so this is never reached.
        onUnlockLargerZonesClick = {},
        onEnableNotificationsClick = {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
            }
            viewModel.requestNotificationPermission()
        },
        onSkipNotificationsClick = viewModel::skipNotifications,
        onFinishClick = viewModel::completeOnboarding,
        modifier = modifier,
    )
}

@Composable
internal fun OnboardingScreen(
    state: OnboardingUiState,
    onGetStartedClick: () -> Unit,
    onBackClick: () -> Unit,
    onPostcodeChange: (String) -> Unit,
    onLookUpClick: () -> Unit,
    onContinueFromPostcodeClick: () -> Unit,
    onRadiusChange: (Float) -> Unit,
    onConfirmRadiusClick: () -> Unit,
    onUnlockLargerZonesClick: () -> Unit,
    onEnableNotificationsClick: () -> Unit,
    onSkipNotificationsClick: () -> Unit,
    onFinishClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Scaffold(
        modifier = modifier,
        topBar = {
            if (state.step != OnboardingStep.Welcome) {
                OnboardingTopBar(step = state.step, onBackClick = onBackClick)
            }
        },
    ) { contentPadding ->
        Column(modifier = Modifier.padding(contentPadding).fillMaxSize()) {
            when (state.step) {
                OnboardingStep.Welcome -> {
                    WelcomeStep(onGetStartedClick = onGetStartedClick, modifier = Modifier.weight(1f))
                }

                OnboardingStep.Postcode -> {
                    PostcodeEntryStep(
                        state = state,
                        onPostcodeChange = onPostcodeChange,
                        onLookUpClick = onLookUpClick,
                        onContinueClick = onContinueFromPostcodeClick,
                        modifier = Modifier.weight(1f),
                    )
                }

                OnboardingStep.Radius -> {
                    RadiusPickerStep(
                        state = state,
                        onRadiusChange = onRadiusChange,
                        onConfirmClick = onConfirmRadiusClick,
                        onUnlockLargerZonesClick = onUnlockLargerZonesClick,
                        modifier = Modifier.weight(1f),
                    )
                }

                OnboardingStep.NotificationPermission -> {
                    NotificationPermissionStep(
                        state = state,
                        onEnableClick = onEnableNotificationsClick,
                        onSkipClick = onSkipNotificationsClick,
                        onFinishClick = onFinishClick,
                        modifier = Modifier.weight(1f),
                    )
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun OnboardingTopBar(
    step: OnboardingStep,
    onBackClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier.fillMaxWidth()) {
        TopAppBar(
            title = {},
            navigationIcon = {
                IconButton(onClick = onBackClick) {
                    Icon(
                        imageVector = Icons.AutoMirrored.Filled.ArrowBack,
                        contentDescription = stringResource(R.string.onboarding_back_content_description),
                    )
                }
            },
        )
        LinearProgressIndicator(progress = { onboardingStepProgress(step) }, modifier = Modifier.fillMaxWidth())
    }
}

/** 1 of 3 once past Welcome (which shows no bar at all) through to the last step. */
private fun onboardingStepProgress(step: OnboardingStep): Float =
    when (step) {
        OnboardingStep.Welcome -> 0f
        OnboardingStep.Postcode -> 1f / 3f
        OnboardingStep.Radius -> 2f / 3f
        OnboardingStep.NotificationPermission -> 1f
    }

@Preview(name = "postcode step")
@Composable
private fun OnboardingScreenPostcodePreview() {
    TownCrierTheme {
        OnboardingScreen(
            state = OnboardingUiState(step = OnboardingStep.Postcode),
            onGetStartedClick = {},
            onBackClick = {},
            onPostcodeChange = {},
            onLookUpClick = {},
            onContinueFromPostcodeClick = {},
            onRadiusChange = {},
            onConfirmRadiusClick = {},
            onUnlockLargerZonesClick = {},
            onEnableNotificationsClick = {},
            onSkipNotificationsClick = {},
            onFinishClick = {},
        )
    }
}

@Preview(name = "radius step", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun OnboardingScreenRadiusPreview() {
    TownCrierTheme {
        OnboardingScreen(
            state = OnboardingUiState(step = OnboardingStep.Radius, radiusMetres = 1_500f, maxRadiusMetres = 2_000f),
            onGetStartedClick = {},
            onBackClick = {},
            onPostcodeChange = {},
            onLookUpClick = {},
            onContinueFromPostcodeClick = {},
            onRadiusChange = {},
            onConfirmRadiusClick = {},
            onUnlockLargerZonesClick = {},
            onEnableNotificationsClick = {},
            onSkipNotificationsClick = {},
            onFinishClick = {},
        )
    }
}
