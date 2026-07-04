package uk.towncrierapp.presentation.features.watchzones

import android.content.res.Configuration
import androidx.annotation.StringRes
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import uk.towncrierapp.domain.subscriptions.Entitlement
import uk.towncrierapp.domain.subscriptions.FeatureGate
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.designsystem.components.GatedToggle

/**
 * Per-zone notification preferences: four toggles (push/email ×
 * new-application/decision) grouped under two sections. Locked for Free —
 * the server never delivers instant alerts to free accounts (tc-bd6i); the
 * same controls render for every tier, gated by [FeatureGate].
 * Port of iOS `ZonePreferencesView`.
 */
@Composable
public fun ZonePreferencesRoute(
    viewModel: ZonePreferencesViewModel,
    onDismiss: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val state by viewModel.uiState.collectAsStateWithLifecycle()

    LaunchedEffect(Unit) { viewModel.load() }
    LaunchedEffect(state.saveCompleted) {
        if (state.saveCompleted) {
            viewModel.consumeSaveCompleted()
            onDismiss()
        }
    }

    ZonePreferencesScreen(
        state = state,
        onNewApplicationPushChange = viewModel::updateNewApplicationPush,
        onNewApplicationEmailChange = viewModel::updateNewApplicationEmail,
        onDecisionPushChange = viewModel::updateDecisionPush,
        onDecisionEmailChange = viewModel::updateDecisionEmail,
        onSaveClick = viewModel::save,
        onCancelClick = onDismiss,
        modifier = modifier,
    )
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun ZonePreferencesScreen(
    state: ZonePreferencesUiState,
    onNewApplicationPushChange: (Boolean) -> Unit,
    onNewApplicationEmailChange: (Boolean) -> Unit,
    onDecisionPushChange: (Boolean) -> Unit,
    onDecisionEmailChange: (Boolean) -> Unit,
    onSaveClick: () -> Unit,
    onCancelClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    // These toggles are gated on the same paid-only entitlement as the editor's
    // instant-alert toggles (tc-bd6i) — any paid entitlement distinguishes
    // free (locked) from paid (open), since they all travel together.
    val isUnlocked = state.featureGate.hasEntitlement(Entitlement.STATUS_CHANGE_ALERTS)

    Scaffold(
        modifier = modifier,
        topBar = {
            TopAppBar(
                title = { Text(state.zoneName) },
                navigationIcon = {
                    TextButton(onClick = onCancelClick) {
                        Text(stringResource(R.string.zone_preferences_cancel_button))
                    }
                },
                actions = {
                    TextButton(onClick = onSaveClick, enabled = !state.isLoading) {
                        Text(stringResource(R.string.zone_preferences_save_button))
                    }
                },
            )
        },
    ) { contentPadding ->
        Column(
            modifier = Modifier.padding(contentPadding).fillMaxSize().padding(TownCrierSpacing.md),
            verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.lg),
        ) {
            PreferenceSection(
                headerRes = R.string.zone_preferences_new_applications_header,
                footerRes = R.string.zone_preferences_new_applications_footer,
                pushChecked = state.newApplicationPush,
                emailChecked = state.newApplicationEmail,
                isUnlocked = isUnlocked,
                onPushChange = onNewApplicationPushChange,
                onEmailChange = onNewApplicationEmailChange,
            )
            PreferenceSection(
                headerRes = R.string.zone_preferences_decision_updates_header,
                footerRes = R.string.zone_preferences_decision_updates_footer,
                pushChecked = state.decisionPush,
                emailChecked = state.decisionEmail,
                isUnlocked = isUnlocked,
                onPushChange = onDecisionPushChange,
                onEmailChange = onDecisionEmailChange,
            )
            state.error?.let { error ->
                Row(horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm)) {
                    Icon(
                        imageVector = Icons.Filled.Warning,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.error,
                    )
                    Text(
                        text = stringResource(watchZoneErrorMessageRes(error)),
                        style = MaterialTheme.typography.bodyLarge,
                        color = MaterialTheme.colorScheme.error,
                    )
                }
            }
        }
    }
}

@Composable
private fun PreferenceSection(
    @StringRes headerRes: Int,
    @StringRes footerRes: Int,
    pushChecked: Boolean,
    emailChecked: Boolean,
    isUnlocked: Boolean,
    onPushChange: (Boolean) -> Unit,
    onEmailChange: (Boolean) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
        Text(text = stringResource(headerRes), style = MaterialTheme.typography.labelMedium)
        GatedToggle(
            label = stringResource(R.string.zone_preferences_push_label),
            checked = pushChecked,
            onCheckedChange = onPushChange,
            isEnabled = isUnlocked,
        )
        GatedToggle(
            label = stringResource(R.string.zone_preferences_email_label),
            checked = emailChecked,
            onCheckedChange = onEmailChange,
            isEnabled = isUnlocked,
        )
        Text(
            text = stringResource(footerRes),
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Preview(name = "unlocked, light")
@Preview(name = "unlocked, dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun ZonePreferencesScreenUnlockedPreview() {
    TownCrierTheme {
        ZonePreferencesScreen(
            state =
                ZonePreferencesUiState(
                    zoneName = "Home",
                    featureGate = FeatureGate(SubscriptionTier.PERSONAL),
                ),
            onNewApplicationPushChange = {},
            onNewApplicationEmailChange = {},
            onDecisionPushChange = {},
            onDecisionEmailChange = {},
            onSaveClick = {},
            onCancelClick = {},
        )
    }
}

@Preview(name = "locked (Free)")
@Composable
private fun ZonePreferencesScreenLockedPreview() {
    TownCrierTheme {
        ZonePreferencesScreen(
            state = ZonePreferencesUiState(zoneName = "Home"),
            onNewApplicationPushChange = {},
            onNewApplicationEmailChange = {},
            onDecisionPushChange = {},
            onDecisionEmailChange = {},
            onSaveClick = {},
            onCancelClick = {},
        )
    }
}
