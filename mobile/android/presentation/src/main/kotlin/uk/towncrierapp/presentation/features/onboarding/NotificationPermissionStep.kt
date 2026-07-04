package uk.towncrierapp.presentation.features.onboarding

import android.content.res.Configuration
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.designsystem.components.PrimaryButton

/**
 * Step 4 - tier-aware. A paid tier (has the instant-alert entitlement) sees
 * "Enable notifications" + "Skip for now"; a free-tier user sees honest
 * digest-only copy and a single "Finish" - no OS permission is ever
 * requested for them, since free accounts get no pushes.
 */
@Composable
internal fun NotificationPermissionStep(
    state: OnboardingUiState,
    onEnableClick: () -> Unit,
    onSkipClick: () -> Unit,
    onFinishClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier.fillMaxSize().padding(TownCrierSpacing.lg),
        verticalArrangement = Arrangement.SpaceBetween,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Column(
            modifier = Modifier.weight(1f),
            verticalArrangement = Arrangement.Center,
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text(
                text = stringResource(R.string.onboarding_notifications_title),
                style = MaterialTheme.typography.headlineMedium,
                color = MaterialTheme.colorScheme.onBackground,
            )
            Text(
                text =
                    stringResource(
                        if (state.hasInstantAlertEntitlement) {
                            R.string.onboarding_notifications_paid_body
                        } else {
                            R.string.onboarding_notifications_free_body
                        },
                    ),
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        if (state.hasInstantAlertEntitlement) {
            PrimaryButton(text = stringResource(R.string.onboarding_notifications_enable_button), onClick = onEnableClick)
            TextButton(onClick = onSkipClick) {
                Text(stringResource(R.string.onboarding_notifications_skip_button))
            }
        } else {
            PrimaryButton(text = stringResource(R.string.onboarding_notifications_finish_button), onClick = onFinishClick)
        }
    }
}

@Preview(name = "paid tier")
@Composable
private fun NotificationPermissionStepPaidPreview() {
    TownCrierTheme {
        NotificationPermissionStep(
            state = OnboardingUiState(hasInstantAlertEntitlement = true),
            onEnableClick = {},
            onSkipClick = {},
            onFinishClick = {},
        )
    }
}

@Preview(name = "free tier", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun NotificationPermissionStepFreePreview() {
    TownCrierTheme {
        NotificationPermissionStep(
            state = OnboardingUiState(hasInstantAlertEntitlement = false),
            onEnableClick = {},
            onSkipClick = {},
            onFinishClick = {},
        )
    }
}
