package uk.towncrierapp.presentation.features.settings

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing

// The second half of SettingsScreen's row/section composables (data
// attribution through about), split from SettingsSections.kt to keep both
// files under detekt's per-file function-count budget (same pattern as
// WatchZoneEditorScreen.kt / WatchZoneEditorSections.kt).

private data class AttributionItem(
    val nameRes: Int,
    val detailRes: Int,
)

private val ATTRIBUTION_ITEMS =
    listOf(
        AttributionItem(R.string.settings_attribution_planit_name, R.string.settings_attribution_planit_detail),
        AttributionItem(
            R.string.settings_attribution_crown_copyright_name,
            R.string.settings_attribution_crown_copyright_detail,
        ),
        AttributionItem(
            R.string.settings_attribution_ordnance_survey_name,
            R.string.settings_attribution_ordnance_survey_detail,
        ),
        // Google Maps, not Apple Maps — Android attribution set (Maps SDK legal requirement).
        AttributionItem(
            R.string.settings_attribution_google_maps_name,
            R.string.settings_attribution_google_maps_detail,
        ),
    )

@Composable
internal fun AttributionSection(modifier: Modifier = Modifier) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
        SectionHeader(stringResource(R.string.settings_attribution_header))
        ATTRIBUTION_ITEMS.forEach { item ->
            Column(modifier = Modifier.padding(vertical = TownCrierSpacing.xs)) {
                Text(text = stringResource(item.nameRes), style = MaterialTheme.typography.bodyLarge)
                Text(
                    text = stringResource(item.detailRes),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
    }
}

@Composable
internal fun LegalSection(
    onPrivacyPolicyClick: () -> Unit,
    onTermsOfServiceClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
        SectionHeader(stringResource(R.string.settings_legal_header))
        SettingsRow(label = stringResource(R.string.settings_legal_privacy_row), onClick = onPrivacyPolicyClick)
        SettingsRow(label = stringResource(R.string.settings_legal_terms_row), onClick = onTermsOfServiceClick)
    }
}

@Composable
internal fun ExportDataRow(
    isExporting: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    SettingsRow(
        label = stringResource(R.string.settings_export_row),
        onClick = onClick,
        modifier = modifier,
        trailing = { if (isExporting) CircularProgressIndicator(modifier = Modifier.heightIn(max = 20.dp)) },
    )
}

@Composable
internal fun DangerZoneSection(
    isDeletingAccount: Boolean,
    deletionError: DomainError?,
    onSignOutClick: () -> Unit,
    onDeleteAccountClick: () -> Unit,
    onRetryDeleteAccount: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
        SectionHeader(stringResource(R.string.settings_danger_header))
        SettingsRow(label = stringResource(R.string.settings_sign_out_row), onClick = onSignOutClick)
        SettingsRow(
            label = stringResource(R.string.settings_delete_account_row),
            onClick = onDeleteAccountClick,
            trailing = { if (isDeletingAccount) CircularProgressIndicator(modifier = Modifier.heightIn(max = 20.dp)) },
        )
        if (deletionError != null) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    text = stringResource(R.string.settings_delete_error_generic),
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.error,
                )
                TextButton(onClick = onRetryDeleteAccount) {
                    Text(stringResource(R.string.settings_delete_retry_button))
                }
            }
        }
    }
}

@Composable
internal fun AboutSection(
    appVersion: String,
    onRateAppClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
        SectionHeader(stringResource(R.string.settings_about_header))
        SettingsRow(label = stringResource(R.string.settings_rate_app_row), onClick = onRateAppClick)
        Text(
            text = stringResource(R.string.settings_version_row, appVersion),
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}
