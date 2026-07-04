package uk.towncrierapp.presentation.features.notificationprefs

import android.Manifest
import android.content.pm.PackageManager
import android.content.res.Configuration
import android.os.Build
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Check
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import uk.towncrierapp.domain.profile.DigestDay
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme

/**
 * Global notification preferences: saved-application push/email, the weekly
 * email digest, and its day (Monday-Sunday). A "permission not granted"
 * banner appears when the OS `POST_NOTIFICATIONS` permission is
 * not-determined or denied. Port of iOS `NotificationPreferencesView`.
 */
@Composable
public fun NotificationPreferencesRoute(
    viewModel: NotificationPreferencesViewModel,
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val state by viewModel.uiState.collectAsStateWithLifecycle()
    val context = LocalContext.current
    val isPushPermissionGranted =
        Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU ||
            ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) ==
            PackageManager.PERMISSION_GRANTED

    NotificationPreferencesScreen(
        state = state,
        isPushPermissionGranted = isPushPermissionGranted,
        onBack = onBack,
        onSavedDecisionPushChange = viewModel::setSavedDecisionPush,
        onSavedDecisionEmailChange = viewModel::setSavedDecisionEmail,
        onEmailDigestEnabledChange = viewModel::setEmailDigestEnabled,
        onDigestDayChange = viewModel::setDigestDay,
        modifier = modifier,
    )
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun NotificationPreferencesScreen(
    state: NotificationPreferencesUiState,
    isPushPermissionGranted: Boolean,
    onBack: () -> Unit,
    onSavedDecisionPushChange: (Boolean) -> Unit,
    onSavedDecisionEmailChange: (Boolean) -> Unit,
    onEmailDigestEnabledChange: (Boolean) -> Unit,
    onDigestDayChange: (DigestDay) -> Unit,
    modifier: Modifier = Modifier,
) {
    Scaffold(
        modifier = modifier,
        topBar = {
            TopAppBar(
                title = { Text(stringResource(R.string.notification_prefs_title)) },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(imageVector = Icons.AutoMirrored.Filled.ArrowBack, contentDescription = null)
                    }
                },
            )
        },
    ) { contentPadding ->
        Column(
            modifier = Modifier.padding(contentPadding).fillMaxSize().padding(TownCrierSpacing.md),
            verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.lg),
        ) {
            if (!isPushPermissionGranted) {
                PermissionBanner()
            }
            SavedApplicationsSection(
                savedDecisionPush = state.savedDecisionPush,
                savedDecisionEmail = state.savedDecisionEmail,
                onPushChange = onSavedDecisionPushChange,
                onEmailChange = onSavedDecisionEmailChange,
            )
            DigestSection(
                emailDigestEnabled = state.emailDigestEnabled,
                digestDay = state.digestDay,
                onEmailDigestEnabledChange = onEmailDigestEnabledChange,
                onDigestDayChange = onDigestDayChange,
            )
        }
    }
}

@Composable
private fun PermissionBanner(modifier: Modifier = Modifier) {
    Card(
        modifier = modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.errorContainer),
    ) {
        Column(
            modifier = Modifier.padding(TownCrierSpacing.md),
            verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs),
        ) {
            Text(
                text = stringResource(R.string.notification_prefs_permission_banner_title),
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onErrorContainer,
            )
            Text(
                text = stringResource(R.string.notification_prefs_permission_banner_body),
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onErrorContainer,
            )
        }
    }
}

@Composable
private fun SavedApplicationsSection(
    savedDecisionPush: Boolean,
    savedDecisionEmail: Boolean,
    onPushChange: (Boolean) -> Unit,
    onEmailChange: (Boolean) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
        Text(
            text = stringResource(R.string.notification_prefs_saved_applications_header),
            style = MaterialTheme.typography.labelMedium,
        )
        ToggleRow(
            label = stringResource(R.string.notification_prefs_saved_decision_push_label),
            checked = savedDecisionPush,
            onCheckedChange = onPushChange,
        )
        ToggleRow(
            label = stringResource(R.string.notification_prefs_saved_decision_email_label),
            checked = savedDecisionEmail,
            onCheckedChange = onEmailChange,
        )
    }
}

@Composable
private fun DigestSection(
    emailDigestEnabled: Boolean,
    digestDay: DigestDay,
    onEmailDigestEnabledChange: (Boolean) -> Unit,
    onDigestDayChange: (DigestDay) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
        Text(text = stringResource(R.string.notification_prefs_digest_header), style = MaterialTheme.typography.labelMedium)
        ToggleRow(
            label = stringResource(R.string.notification_prefs_email_digest_label),
            checked = emailDigestEnabled,
            onCheckedChange = onEmailDigestEnabledChange,
        )
        DigestDayPicker(
            label = stringResource(R.string.notification_prefs_digest_day_label),
            selected = digestDay,
            onSelected = onDigestDayChange,
        )
    }
}

@Composable
private fun ToggleRow(
    label: String,
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier.fillMaxWidth().padding(vertical = TownCrierSpacing.xs),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.SpaceBetween,
    ) {
        Text(text = label, style = MaterialTheme.typography.bodyLarge)
        Switch(checked = checked, onCheckedChange = onCheckedChange)
    }
}

@Composable
private fun DigestDayPicker(
    label: String,
    selected: DigestDay,
    onSelected: (DigestDay) -> Unit,
    modifier: Modifier = Modifier,
) {
    var expanded by remember { mutableStateOf(false) }
    Row(
        modifier = modifier.fillMaxWidth().padding(vertical = TownCrierSpacing.xs),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.SpaceBetween,
    ) {
        Text(text = label, style = MaterialTheme.typography.bodyLarge)
        TextButton(onClick = { expanded = true }) {
            Text(selected.wireValue)
        }
        DropdownMenu(expanded = expanded, onDismissRequest = { expanded = false }) {
            DigestDay.entries.forEach { day ->
                DropdownMenuItem(
                    text = { Text(day.wireValue) },
                    onClick = {
                        onSelected(day)
                        expanded = false
                    },
                    trailingIcon = {
                        if (day == selected) Icon(imageVector = Icons.Filled.Check, contentDescription = null)
                    },
                )
            }
        }
    }
}

@Preview(name = "light")
@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun NotificationPreferencesScreenPreview() {
    TownCrierTheme {
        NotificationPreferencesScreen(
            state = NotificationPreferencesUiState(isLoading = false),
            isPushPermissionGranted = true,
            onBack = {},
            onSavedDecisionPushChange = {},
            onSavedDecisionEmailChange = {},
            onEmailDigestEnabledChange = {},
            onDigestDayChange = {},
        )
    }
}

@Preview(name = "permission denied")
@Composable
private fun NotificationPreferencesScreenPermissionDeniedPreview() {
    TownCrierTheme {
        NotificationPreferencesScreen(
            state = NotificationPreferencesUiState(isLoading = false),
            isPushPermissionGranted = false,
            onBack = {},
            onSavedDecisionPushChange = {},
            onSavedDecisionEmailChange = {},
            onEmailDigestEnabledChange = {},
            onDigestDayChange = {},
        )
    }
}
