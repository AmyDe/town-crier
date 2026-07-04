package uk.towncrierapp.presentation.features.saved

import android.content.res.Configuration
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Bookmark
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.PlanningApplication
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.designsystem.components.ApplicationRow
import uk.towncrierapp.presentation.designsystem.components.CapsuleChip
import uk.towncrierapp.presentation.features.applicationlist.applicationErrorMessageRes

/**
 * The Saved tab: the flat, cross-zone saved list with a client-side status
 * filter. Tapping a row hands the already-cached [PlanningApplication]
 * straight to the caller so detail can render it instantly (stale-while-
 * revalidate happens there). Port of iOS `SavedApplicationListView`
 * (GH#775).
 */
@Composable
public fun SavedListRoute(
    viewModel: SavedListViewModel,
    onApplicationSelected: (PlanningApplication) -> Unit,
    modifier: Modifier = Modifier,
) {
    val state by viewModel.uiState.collectAsStateWithLifecycle()
    SavedListScreen(
        state = state,
        onFilterSelected = viewModel::selectFilter,
        onApplicationClick = onApplicationSelected,
        modifier = modifier,
    )
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun SavedListScreen(
    state: SavedListUiState,
    onFilterSelected: (ApplicationStatus?) -> Unit,
    onApplicationClick: (PlanningApplication) -> Unit,
    modifier: Modifier = Modifier,
) {
    Scaffold(
        modifier = modifier,
        topBar = { TopAppBar(title = { Text(stringResource(R.string.saved_title)) }) },
    ) { contentPadding ->
        Column(modifier = Modifier.padding(contentPadding).fillMaxSize()) {
            SavedFilterChipsRow(
                filter = state.filter,
                onFilterSelected = onFilterSelected,
                modifier = Modifier.padding(vertical = TownCrierSpacing.sm),
            )
            Box(modifier = Modifier.weight(1f).fillMaxWidth()) {
                val displayed = state.displayed
                when {
                    state.isLoading && displayed.isEmpty() -> {
                        CircularProgressIndicator(modifier = Modifier.align(Alignment.Center))
                    }

                    displayed.isEmpty() -> {
                        SavedEmptyState(modifier = Modifier.align(Alignment.Center))
                    }

                    else -> {
                        LazyColumn(modifier = Modifier.fillMaxSize()) {
                            items(displayed, key = { it.applicationUid.value }) { saved ->
                                val application = saved.application
                                if (application != null) {
                                    ApplicationRow(
                                        application = application,
                                        onClick = { onApplicationClick(application) },
                                    )
                                    HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                                }
                            }
                        }
                    }
                }
            }
            state.error?.let { error ->
                Text(
                    text = stringResource(applicationErrorMessageRes(error)),
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.error,
                    modifier = Modifier.padding(TownCrierSpacing.md),
                )
            }
        }
    }
}

private val SAVED_FILTERABLE_STATUSES =
    listOf(
        ApplicationStatus.Undecided,
        ApplicationStatus.Permitted,
        ApplicationStatus.Conditions,
        ApplicationStatus.Rejected,
        ApplicationStatus.Withdrawn,
        ApplicationStatus.Appealed,
    )

@Composable
private fun SavedFilterChipsRow(
    filter: ApplicationStatus?,
    onFilterSelected: (ApplicationStatus?) -> Unit,
    modifier: Modifier = Modifier,
) {
    LazyRow(
        modifier = modifier,
        horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
        contentPadding = PaddingValues(horizontal = TownCrierSpacing.md),
    ) {
        item {
            CapsuleChip(
                label = stringResource(R.string.applications_filter_all),
                selected = filter == null,
                onClick = { onFilterSelected(null) },
            )
        }
        items(SAVED_FILTERABLE_STATUSES) { status ->
            CapsuleChip(
                label = savedStatusLabel(status),
                selected = filter == status,
                onClick = { onFilterSelected(status) },
            )
        }
    }
}

@Composable
private fun savedStatusLabel(status: ApplicationStatus): String =
    when (status) {
        ApplicationStatus.Undecided -> stringResource(R.string.application_status_pending)
        ApplicationStatus.Permitted -> stringResource(R.string.application_status_permitted)
        ApplicationStatus.Conditions -> stringResource(R.string.application_status_conditions)
        ApplicationStatus.Rejected -> stringResource(R.string.application_status_rejected)
        ApplicationStatus.Withdrawn -> stringResource(R.string.application_status_withdrawn)
        ApplicationStatus.Appealed -> stringResource(R.string.application_status_appealed)
        else -> stringResource(R.string.application_status_unknown)
    }

@Composable
private fun SavedEmptyState(modifier: Modifier = Modifier) {
    Column(
        modifier = modifier.padding(TownCrierSpacing.xl),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.md),
    ) {
        Icon(
            imageVector = Icons.Filled.Bookmark,
            contentDescription = null,
            tint = TownCrierTheme.colors.textTertiary,
            modifier = Modifier.size(48.dp),
        )
        Text(text = stringResource(R.string.saved_empty_title), style = MaterialTheme.typography.titleMedium)
        Text(
            text = stringResource(R.string.saved_empty_message),
            style = MaterialTheme.typography.bodyLarge,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Preview(name = "empty, light")
@Preview(name = "empty, dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun SavedListScreenEmptyPreview() {
    TownCrierTheme {
        SavedListScreen(state = SavedListUiState(), onFilterSelected = {}, onApplicationClick = {})
    }
}
