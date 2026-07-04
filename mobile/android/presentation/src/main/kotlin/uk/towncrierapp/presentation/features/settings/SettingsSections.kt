package uk.towncrierapp.presentation.features.settings

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.selection.selectable
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import uk.towncrierapp.domain.auth.AuthMethod
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.Appearance
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing

// This file holds SettingsScreen's row/section composables, split out to
// keep SettingsScreen.kt under detekt's per-file function-count budget
// (same pattern as WatchZoneEditorScreen.kt / WatchZoneEditorSections.kt).

@Composable
internal fun SectionHeader(
    text: String,
    modifier: Modifier = Modifier,
) {
    Text(text = text, style = MaterialTheme.typography.labelMedium, modifier = modifier)
}

@Composable
internal fun SettingsRow(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    trailing: @Composable () -> Unit = {},
) {
    Row(
        modifier =
            modifier
                .fillMaxWidth()
                .heightIn(min = 44.dp)
                .clickable(onClick = onClick)
                .padding(vertical = TownCrierSpacing.xs),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.SpaceBetween,
    ) {
        Text(text = label, style = MaterialTheme.typography.bodyLarge)
        trailing()
    }
}

@Composable
internal fun AccountSection(
    email: String?,
    authMethod: AuthMethod?,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
        SectionHeader(stringResource(R.string.settings_account_header))
        // email may legitimately be blank (Sign in with Apple grants no email
        // scope) — render gracefully by simply omitting the row rather than
        // showing an empty line.
        if (!email.isNullOrBlank()) {
            Text(text = email, style = MaterialTheme.typography.bodyLarge)
        }
        Text(
            text = stringResource(authMethodLabelRes(authMethod)),
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

private fun authMethodLabelRes(authMethod: AuthMethod?): Int =
    when (authMethod) {
        AuthMethod.EMAIL_PASSWORD -> R.string.settings_signin_method_email_password
        AuthMethod.GOOGLE -> R.string.settings_signin_method_google
        AuthMethod.APPLE -> R.string.settings_signin_method_apple
        AuthMethod.UNKNOWN, null -> R.string.settings_signin_method_unknown
    }

/** All four appearance options are always visible (epic #770 pre-resolved decision) — never a collapsed picker. */
@Composable
internal fun AppearanceSection(
    selected: Appearance,
    onSelected: (Appearance) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
        SectionHeader(stringResource(R.string.settings_appearance_header))
        AppearanceOptionRow(Appearance.System, R.string.settings_appearance_system, selected, onSelected)
        AppearanceOptionRow(Appearance.Light, R.string.settings_appearance_light, selected, onSelected)
        AppearanceOptionRow(Appearance.Dark, R.string.settings_appearance_dark, selected, onSelected)
        AppearanceOptionRow(Appearance.OledDark, R.string.settings_appearance_oled_dark, selected, onSelected)
    }
}

@Composable
private fun AppearanceOptionRow(
    option: Appearance,
    labelRes: Int,
    selected: Appearance,
    onSelected: (Appearance) -> Unit,
) {
    val isSelected = option == selected
    Row(
        modifier =
            Modifier
                .fillMaxWidth()
                .heightIn(min = 44.dp)
                .selectable(selected = isSelected, onClick = { onSelected(option) })
                .padding(vertical = TownCrierSpacing.xs),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
    ) {
        RadioButton(selected = isSelected, onClick = { onSelected(option) })
        Text(text = stringResource(labelRes), style = MaterialTheme.typography.bodyLarge)
    }
}

@Composable
internal fun NotificationPreferencesRow(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    SettingsRow(
        label = stringResource(R.string.settings_notification_preferences_row),
        onClick = onClick,
        modifier = modifier,
    )
}

@Composable
internal fun SubscriptionSection(
    tier: SubscriptionTier,
    onManageSubscriptionClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
        SectionHeader(stringResource(R.string.settings_subscription_header))
        Text(text = stringResource(tierLabelRes(tier)), style = MaterialTheme.typography.bodyLarge)
        SettingsRow(
            label = stringResource(R.string.settings_subscription_manage_row),
            onClick = onManageSubscriptionClick,
        )
    }
}

private fun tierLabelRes(tier: SubscriptionTier): Int =
    when (tier) {
        SubscriptionTier.FREE -> R.string.settings_subscription_tier_free
        SubscriptionTier.PERSONAL -> R.string.settings_subscription_tier_personal
        SubscriptionTier.PRO -> R.string.settings_subscription_tier_pro
    }

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
