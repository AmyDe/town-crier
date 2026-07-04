package uk.towncrierapp.presentation.features.watchzones

import android.content.res.Configuration
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme

/**
 * Create or edit a watch zone: name → postcode lookup (create only) → radius
 * slider → static map placeholder (#776's job, see [ZoneMapPlaceholder]) →
 * gated instant-alert toggles → save. Section composables live in
 * `WatchZoneEditorSections.kt`. Port of iOS `WatchZoneEditorView`.
 * [onSaveSuccess]/[onDismiss]/[onUpgradeRequired] are one-shot navigation
 * reconciliations driven by
 * [WatchZoneEditorUiState.saveCompleted]/`navigateToPaywall` — ViewModels
 * never navigate (compose-ui.md). [onSaveSuccess] is distinct from
 * [onDismiss] (Cancel/back) so the nav layer can signal the zone list to
 * refetch only on an actual save (tc-yg0q) — see `WatchZoneNavGraph.kt`.
 */
@Composable
public fun WatchZoneEditorRoute(
    viewModel: WatchZoneEditorViewModel,
    onSaveSuccess: () -> Unit,
    onDismiss: () -> Unit,
    onUpgradeRequired: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val state by viewModel.uiState.collectAsStateWithLifecycle()

    LaunchedEffect(state.saveCompleted) {
        if (state.saveCompleted) {
            viewModel.consumeSaveCompleted()
            onSaveSuccess()
        }
    }
    LaunchedEffect(state.navigateToPaywall) {
        if (state.navigateToPaywall) {
            viewModel.consumeNavigateToPaywall()
            onUpgradeRequired()
        }
    }

    WatchZoneEditorScreen(
        state = state,
        onNameChange = viewModel::updateName,
        onPostcodeChange = viewModel::updatePostcode,
        onLookUpClick = viewModel::submitPostcode,
        onRadiusChange = viewModel::updateRadius,
        onPushEnabledChange = viewModel::updatePushEnabled,
        onEmailInstantEnabledChange = viewModel::updateEmailInstantEnabled,
        onUnlockLargerZonesClick = onUpgradeRequired,
        onUpgradeRequired = viewModel::requestInstantAlertUpgrade,
        onSaveClick = viewModel::save,
        onCancelClick = onDismiss,
        modifier = modifier,
    )
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun WatchZoneEditorScreen(
    state: WatchZoneEditorUiState,
    onNameChange: (String) -> Unit,
    onPostcodeChange: (String) -> Unit,
    onLookUpClick: () -> Unit,
    onRadiusChange: (Float) -> Unit,
    onPushEnabledChange: (Boolean) -> Unit,
    onEmailInstantEnabledChange: (Boolean) -> Unit,
    onUnlockLargerZonesClick: () -> Unit,
    onUpgradeRequired: () -> Unit,
    onSaveClick: () -> Unit,
    onCancelClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Scaffold(
        modifier = modifier,
        topBar = { EditorTopBar(state = state, onSaveClick = onSaveClick, onCancelClick = onCancelClick) },
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
            NameSection(name = state.name, onNameChange = onNameChange)

            if (!state.isEditing) {
                PostcodeSection(
                    postcode = state.postcode,
                    isLoading = state.isLoading,
                    onPostcodeChange = onPostcodeChange,
                    onLookUpClick = onLookUpClick,
                )
            }

            if (state.geocodedCoordinate != null) {
                RadiusSection(
                    state = state,
                    onRadiusChange = onRadiusChange,
                    onUnlockLargerZonesClick = onUnlockLargerZonesClick,
                )
                PreviewSection()
            }

            NotificationsSection(
                state = state,
                onPushEnabledChange = onPushEnabledChange,
                onEmailInstantEnabledChange = onEmailInstantEnabledChange,
                onUpgradeRequired = onUpgradeRequired,
            )

            state.error?.let { error -> ErrorSection(messageRes = watchZoneErrorMessageRes(error)) }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun EditorTopBar(
    state: WatchZoneEditorUiState,
    onSaveClick: () -> Unit,
    onCancelClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val titleRes = if (state.isEditing) R.string.watch_zone_editor_title_edit else R.string.watch_zone_editor_title_new
    TopAppBar(
        modifier = modifier,
        title = { Text(stringResource(titleRes)) },
        navigationIcon = {
            TextButton(onClick = onCancelClick) { Text(stringResource(R.string.watch_zone_editor_cancel_button)) }
        },
        actions = {
            TextButton(onClick = onSaveClick, enabled = state.isSaveEnabled && !state.isLoading) {
                Text(stringResource(R.string.watch_zone_editor_save_button))
            }
        },
        colors = TopAppBarDefaults.topAppBarColors(),
    )
}

@Preview(name = "new zone, light")
@Preview(name = "new zone, dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun WatchZoneEditorScreenNewPreview() {
    TownCrierTheme {
        WatchZoneEditorScreen(
            state = WatchZoneEditorUiState(name = "Home"),
            onNameChange = {},
            onPostcodeChange = {},
            onLookUpClick = {},
            onRadiusChange = {},
            onPushEnabledChange = {},
            onEmailInstantEnabledChange = {},
            onUnlockLargerZonesClick = {},
            onUpgradeRequired = {},
            onSaveClick = {},
            onCancelClick = {},
        )
    }
}

@Preview(name = "geocoded with warning + locked toggles")
@Composable
private fun WatchZoneEditorScreenGeocodedPreview() {
    TownCrierTheme {
        WatchZoneEditorScreen(
            state =
                WatchZoneEditorUiState(
                    name = "Home",
                    geocodedCoordinate = Coordinate(51.5074, -0.1278),
                    radiusMetres = 2_500f,
                    maxRadiusMetres = 5_000f,
                    canUnlockLargerRadius = true,
                    showsLargeRadiusWarning = true,
                    isSaveEnabled = true,
                ),
            onNameChange = {},
            onPostcodeChange = {},
            onLookUpClick = {},
            onRadiusChange = {},
            onPushEnabledChange = {},
            onEmailInstantEnabledChange = {},
            onUnlockLargerZonesClick = {},
            onUpgradeRequired = {},
            onSaveClick = {},
            onCancelClick = {},
        )
    }
}
