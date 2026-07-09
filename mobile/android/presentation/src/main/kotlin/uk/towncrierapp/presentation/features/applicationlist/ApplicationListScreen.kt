package uk.towncrierapp.presentation.features.applicationlist

import android.content.res.Configuration
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.List
import androidx.compose.material.icons.filled.DoneAll
import androidx.compose.material.icons.filled.Place
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import uk.towncrierapp.domain.applications.ApplicationFilter
import uk.towncrierapp.domain.applications.ApplicationSortOrder
import uk.towncrierapp.domain.applications.PlanningApplication
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.domain.watchzones.WatchZone
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.designsystem.components.ApplicationRow
import uk.towncrierapp.presentation.designsystem.components.CapsuleChip
import uk.towncrierapp.presentation.designsystem.components.Masthead
import uk.towncrierapp.presentation.designsystem.components.PrimaryButton

/**
 * The Applications tab: zone chips, status/unread filter chips, a sort menu
 * in the top app bar, and cursor-based infinite scroll. Port of iOS
 * `ApplicationListView` (GH#775).
 */
@Composable
public fun ApplicationListRoute(
    viewModel: ApplicationListViewModel,
    onApplicationSelected: (PlanningApplication) -> Unit,
    onAddZoneClick: () -> Unit,
    onSettingsClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val state by viewModel.uiState.collectAsStateWithLifecycle()
    ApplicationListScreen(
        state = state,
        onZoneSelected = viewModel::selectZone,
        onSortSelected = viewModel::selectSort,
        onFilterSelected = viewModel::selectFilter,
        onItemVisible = viewModel::onItemVisible,
        onApplicationClick = { application ->
            viewModel.markAsRead(application)
            onApplicationSelected(application)
        },
        onMarkAllReadClick = viewModel::markAllRead,
        onAddZoneClick = onAddZoneClick,
        onSettingsClick = onSettingsClick,
        modifier = modifier,
    )
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun ApplicationListScreen(
    state: ApplicationListUiState,
    onZoneSelected: (WatchZoneId) -> Unit,
    onSortSelected: (ApplicationSortOrder) -> Unit,
    onFilterSelected: (ApplicationFilter) -> Unit,
    onItemVisible: (Int) -> Unit,
    onApplicationClick: (PlanningApplication) -> Unit,
    onMarkAllReadClick: () -> Unit,
    onAddZoneClick: () -> Unit,
    onSettingsClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Scaffold(
        modifier = modifier,
        topBar = {
            ApplicationListTopBar(
                unreadCount = state.unreadCount,
                sort = state.sort,
                availableSorts = state.availableSorts,
                onMarkAllReadClick = onMarkAllReadClick,
                onSortSelected = onSortSelected,
                onSettingsClick = onSettingsClick,
            )
        },
    ) { contentPadding ->
        Column(modifier = Modifier.padding(contentPadding).fillMaxSize()) {
            Masthead(title = stringResource(R.string.applications_title))
            if (state.zones.isEmpty() && !state.isLoading) {
                NoZonesEmptyState(onAddZoneClick = onAddZoneClick, modifier = Modifier.fillMaxSize())
            } else {
                if (state.zones.size > 1) {
                    ZoneChipsRow(
                        zones = state.zones,
                        selectedZoneId = state.selectedZoneId,
                        onZoneSelected = onZoneSelected,
                        modifier = Modifier.padding(top = TownCrierSpacing.sm),
                    )
                }
                FilterChipsRow(
                    filter = state.filter,
                    unreadCount = state.unreadCount,
                    onFilterSelected = onFilterSelected,
                    modifier = Modifier.padding(vertical = TownCrierSpacing.sm),
                )
                Box(modifier = Modifier.weight(1f).fillMaxWidth()) {
                    when {
                        state.isLoading && state.applications.isEmpty() -> {
                            CircularProgressIndicator(modifier = Modifier.align(Alignment.Center))
                        }

                        state.applications.isEmpty() -> {
                            EmptyApplicationsState(modifier = Modifier.align(Alignment.Center))
                        }

                        else -> {
                            ApplicationsList(
                                applications = state.applications,
                                isLoadingMore = state.isLoadingMore,
                                onItemVisible = onItemVisible,
                                onApplicationClick = onApplicationClick,
                            )
                        }
                    }
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ApplicationListTopBar(
    unreadCount: Int,
    sort: ApplicationSortOrder,
    availableSorts: List<ApplicationSortOrder>,
    onMarkAllReadClick: () -> Unit,
    onSortSelected: (ApplicationSortOrder) -> Unit,
    onSettingsClick: () -> Unit,
) {
    TopAppBar(
        title = { Text(stringResource(R.string.applications_title)) },
        actions = {
            if (unreadCount > 0) {
                IconButton(onClick = onMarkAllReadClick) {
                    Icon(
                        imageVector = Icons.Filled.DoneAll,
                        contentDescription = stringResource(R.string.applications_mark_all_read),
                    )
                }
            }
            SortMenu(currentSort = sort, availableSorts = availableSorts, onSortSelected = onSortSelected)
            IconButton(onClick = onSettingsClick) {
                Icon(
                    imageVector = Icons.Filled.Settings,
                    contentDescription = stringResource(R.string.settings_content_description),
                )
            }
        },
    )
}

@Composable
private fun ApplicationsList(
    applications: List<PlanningApplication>,
    isLoadingMore: Boolean,
    onItemVisible: (Int) -> Unit,
    onApplicationClick: (PlanningApplication) -> Unit,
    modifier: Modifier = Modifier,
) {
    LazyColumn(
        modifier = modifier.fillMaxSize(),
        verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
        contentPadding = PaddingValues(horizontal = TownCrierSpacing.md, vertical = TownCrierSpacing.sm),
    ) {
        itemsIndexed(applications, key = { _, application -> application.id.value }) { index, application ->
            // Hand-rolled prefetch trigger (no Paging 3): firing once per row
            // as it enters composition is enough for the ~40-line cursor
            // loop this app deliberately keeps simple.
            LaunchedEffect(index) { onItemVisible(index) }
            ApplicationRow(application = application, onClick = { onApplicationClick(application) })
        }
        if (isLoadingMore) {
            item {
                Box(
                    modifier = Modifier.fillMaxWidth().padding(TownCrierSpacing.md),
                    contentAlignment = Alignment.Center,
                ) {
                    CircularProgressIndicator()
                }
            }
        }
    }
}

@Composable
private fun ZoneChipsRow(
    zones: List<WatchZone>,
    selectedZoneId: WatchZoneId?,
    onZoneSelected: (WatchZoneId) -> Unit,
    modifier: Modifier = Modifier,
) {
    LazyRow(
        modifier = modifier,
        horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
        contentPadding = PaddingValues(horizontal = TownCrierSpacing.md),
    ) {
        items(zones, key = { it.id.value }) { zone ->
            CapsuleChip(label = zone.name, selected = zone.id == selectedZoneId, onClick = { onZoneSelected(zone.id) })
        }
    }
}

@Composable
private fun NoZonesEmptyState(
    onAddZoneClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Box(modifier = modifier, contentAlignment = Alignment.Center) {
        Column(
            modifier = Modifier.padding(TownCrierSpacing.xl),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.md),
        ) {
            Icon(
                imageVector = Icons.Filled.Place,
                contentDescription = null,
                tint = TownCrierTheme.colors.textTertiary,
                modifier = Modifier.size(48.dp),
            )
            Text(
                text = stringResource(R.string.applications_no_zones_title),
                style = MaterialTheme.typography.titleMedium,
            )
            Text(
                text = stringResource(R.string.applications_no_zones_message),
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            PrimaryButton(text = stringResource(R.string.applications_no_zones_cta), onClick = onAddZoneClick)
        }
    }
}

@Composable
private fun EmptyApplicationsState(modifier: Modifier = Modifier) {
    Column(
        modifier = modifier.padding(TownCrierSpacing.xl),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.md),
    ) {
        Icon(
            imageVector = Icons.AutoMirrored.Filled.List,
            contentDescription = null,
            tint = TownCrierTheme.colors.textTertiary,
            modifier = Modifier.size(48.dp),
        )
        Text(text = stringResource(R.string.applications_empty_title), style = MaterialTheme.typography.titleMedium)
        Text(
            text = stringResource(R.string.applications_empty_message),
            style = MaterialTheme.typography.bodyLarge,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

// Preview-only sample data — cannot reuse :domain's testFixtures from the
// main source set (compose-ui.md).
private val previewZone =
    WatchZone(id = WatchZoneId("wz-1"), name = "Home", centre = Coordinate(51.5074, -0.1278), radiusMetres = 500.0)

@Preview(name = "light")
@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun ApplicationListScreenEmptyPreview() {
    TownCrierTheme {
        ApplicationListScreen(
            state = ApplicationListUiState(zones = listOf(previewZone), selectedZoneId = previewZone.id),
            onZoneSelected = {},
            onSortSelected = {},
            onFilterSelected = {},
            onItemVisible = {},
            onApplicationClick = {},
            onMarkAllReadClick = {},
            onAddZoneClick = {},
            onSettingsClick = {},
        )
    }
}

@Preview(name = "no zones")
@Composable
private fun ApplicationListScreenNoZonesPreview() {
    TownCrierTheme {
        ApplicationListScreen(
            state = ApplicationListUiState(),
            onZoneSelected = {},
            onSortSelected = {},
            onFilterSelected = {},
            onItemVisible = {},
            onApplicationClick = {},
            onMarkAllReadClick = {},
            onAddZoneClick = {},
            onSettingsClick = {},
        )
    }
}
