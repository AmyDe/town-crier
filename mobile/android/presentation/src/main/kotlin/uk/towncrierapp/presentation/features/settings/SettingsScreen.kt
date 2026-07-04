package uk.towncrierapp.presentation.features.settings

import android.content.Context
import android.content.Intent
import android.content.res.Configuration
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import androidx.core.content.FileProvider
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import uk.towncrierapp.domain.auth.AuthMethod
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.Appearance
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import java.io.File

/**
 * The Settings screen: account display, appearance picker, notification
 * preferences entry, subscription, data attribution, legal documents,
 * GDPR export, sign-out, account deletion, and about. Port of iOS
 * `SettingsView`. Row order is the epic's fixed inventory — do not reorder.
 */
@Composable
public fun SettingsRoute(
    viewModel: SettingsViewModel,
    onBack: () -> Unit,
    onNotificationPreferencesClick: () -> Unit,
    onPrivacyPolicyClick: () -> Unit,
    onTermsOfServiceClick: () -> Unit,
    onRateAppClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val state by viewModel.uiState.collectAsStateWithLifecycle()
    val context = LocalContext.current

    LaunchedEffect(Unit) { viewModel.load() }

    LaunchedEffect(state.exportedData) {
        state.exportedData?.let { exported ->
            shareExportedData(context, exported.bytes)
            viewModel.dismissExportShare()
        }
    }

    SettingsScreen(
        state = state,
        onBack = onBack,
        onAppearanceSelected = viewModel::setAppearance,
        onNotificationPreferencesClick = onNotificationPreferencesClick,
        onPrivacyPolicyClick = onPrivacyPolicyClick,
        onTermsOfServiceClick = onTermsOfServiceClick,
        onExportDataClick = viewModel::exportData,
        onSignOutClick = viewModel::signOut,
        onDeleteAccountClick = viewModel::requestAccountDeletion,
        onCancelDeleteAccount = viewModel::cancelAccountDeletion,
        onConfirmDeleteAccount = viewModel::confirmDeleteAccount,
        onRateAppClick = onRateAppClick,
        modifier = modifier,
    )
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun SettingsScreen(
    state: SettingsUiState,
    onBack: () -> Unit,
    onAppearanceSelected: (Appearance) -> Unit,
    onNotificationPreferencesClick: () -> Unit,
    onPrivacyPolicyClick: () -> Unit,
    onTermsOfServiceClick: () -> Unit,
    onExportDataClick: () -> Unit,
    onSignOutClick: () -> Unit,
    onDeleteAccountClick: () -> Unit,
    onCancelDeleteAccount: () -> Unit,
    onConfirmDeleteAccount: () -> Unit,
    onRateAppClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    var isShowingManageSubscriptionInfo by remember { mutableStateOf(false) }

    Scaffold(
        modifier = modifier,
        topBar = {
            TopAppBar(
                title = { Text(stringResource(R.string.settings_title)) },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(imageVector = Icons.AutoMirrored.Filled.ArrowBack, contentDescription = null)
                    }
                },
            )
        },
    ) { contentPadding ->
        Column(
            modifier =
                Modifier
                    .padding(contentPadding)
                    .fillMaxSize()
                    .verticalScroll(rememberScrollState())
                    .padding(TownCrierSpacing.md),
            verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.lg),
        ) {
            AccountSection(email = state.email, authMethod = state.authMethod)
            AppearanceSection(selected = state.appearance, onSelected = onAppearanceSelected)
            NotificationPreferencesRow(onClick = onNotificationPreferencesClick)
            SubscriptionSection(
                tier = state.subscriptionTier,
                onManageSubscriptionClick = { isShowingManageSubscriptionInfo = true },
            )
            AttributionSection()
            LegalSection(onPrivacyPolicyClick = onPrivacyPolicyClick, onTermsOfServiceClick = onTermsOfServiceClick)
            ExportDataRow(isExporting = state.isExporting, onClick = onExportDataClick)
            DangerZoneSection(
                isDeletingAccount = state.isDeletingAccount,
                deletionError = state.deletionError,
                onSignOutClick = onSignOutClick,
                onDeleteAccountClick = onDeleteAccountClick,
                onRetryDeleteAccount = onConfirmDeleteAccount,
            )
            AboutSection(appVersion = state.appVersion, onRateAppClick = onRateAppClick)
        }
    }

    if (isShowingManageSubscriptionInfo) {
        ManageSubscriptionComingSoonDialog(onDismiss = { isShowingManageSubscriptionInfo = false })
    }

    if (state.isShowingDeleteConfirmation) {
        DeleteAccountConfirmationDialog(onConfirm = onConfirmDeleteAccount, onDismiss = onCancelDeleteAccount)
    }
}

@Composable
private fun ManageSubscriptionComingSoonDialog(onDismiss: () -> Unit) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(R.string.settings_subscription_manage_coming_soon_title)) },
        text = { Text(stringResource(R.string.settings_subscription_manage_coming_soon_message)) },
        confirmButton = {
            TextButton(onClick = onDismiss) {
                Text(stringResource(R.string.settings_subscription_manage_coming_soon_dismiss))
            }
        },
    )
}

@Composable
private fun DeleteAccountConfirmationDialog(
    onConfirm: () -> Unit,
    onDismiss: () -> Unit,
) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(R.string.settings_delete_confirm_title)) },
        text = { Text(stringResource(R.string.settings_delete_confirm_message)) },
        confirmButton = {
            TextButton(onClick = onConfirm) {
                Text(stringResource(R.string.settings_delete_confirm_button), color = MaterialTheme.colorScheme.error)
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) { Text(stringResource(R.string.settings_delete_cancel_button)) }
        },
    )
}

/** Writes [bytes] verbatim to a cache-dir export file and launches the share sheet via [FileProvider]. */
private fun shareExportedData(
    context: Context,
    bytes: ByteArray,
) {
    val exportsDir = File(context.cacheDir, "exports").apply { mkdirs() }
    val file = File(exportsDir, "towncrier-data-export.json")
    file.writeBytes(bytes)
    val uri = FileProvider.getUriForFile(context, "${context.packageName}.fileprovider", file)
    val sendIntent =
        Intent(Intent.ACTION_SEND).apply {
            type = "application/json"
            putExtra(Intent.EXTRA_STREAM, uri)
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        }
    context.startActivity(Intent.createChooser(sendIntent, null))
}

@Preview(name = "light")
@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun SettingsScreenPreview() {
    TownCrierTheme {
        SettingsScreen(
            state =
                SettingsUiState(
                    isLoading = false,
                    email = "resident@example.test",
                    name = "Resident",
                    authMethod = AuthMethod.EMAIL_PASSWORD,
                    subscriptionTier = SubscriptionTier.PERSONAL,
                    appVersion = "1.0.0",
                ),
            onBack = {},
            onAppearanceSelected = {},
            onNotificationPreferencesClick = {},
            onPrivacyPolicyClick = {},
            onTermsOfServiceClick = {},
            onExportDataClick = {},
            onSignOutClick = {},
            onDeleteAccountClick = {},
            onCancelDeleteAccount = {},
            onConfirmDeleteAccount = {},
            onRateAppClick = {},
        )
    }
}

@Preview(name = "SIWA — blank email")
@Composable
private fun SettingsScreenSiwaPreview() {
    TownCrierTheme {
        SettingsScreen(
            state =
                SettingsUiState(
                    isLoading = false,
                    email = "",
                    authMethod = AuthMethod.APPLE,
                    appVersion = "1.0.0",
                ),
            onBack = {},
            onAppearanceSelected = {},
            onNotificationPreferencesClick = {},
            onPrivacyPolicyClick = {},
            onTermsOfServiceClick = {},
            onExportDataClick = {},
            onSignOutClick = {},
            onDeleteAccountClick = {},
            onCancelDeleteAccount = {},
            onConfirmDeleteAccount = {},
            onRateAppClick = {},
        )
    }
}
