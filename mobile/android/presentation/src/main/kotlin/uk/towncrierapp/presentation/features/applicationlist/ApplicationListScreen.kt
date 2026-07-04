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
import androidx.compose.material.icons.automirrored.filled.Sort
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.DoneAll
import androidx.compose.material.icons.filled.Place
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import uk.towncrierapp.domain.applications.ApplicationFilter
import uk.towncrierapp.domain.applications.ApplicationSortOrder
import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.PlanningApplication
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.domain.watchzones.WatchZone
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.designsystem.components.ApplicationRow
import uk.towncrierapp.presentation.designsystem.components.CapsuleChip
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
    modifier: Modifier = Modifier,
) {
    Scaffold(
        modifier = modifier,
        topBar = {
            TopAppBar(
                title = { Text(stringResource(R.string.applications_title)) },
                actions = {
                    if (state.unreadCount > 0) {
                        IconButton(onClick = onMarkAllReadClick) {
                            Icon(
                                imageVector = Icons.Filled.DoneAll,
                                contentDescription = stringResource(R.string.applications_mark_all_read),
                            )
                        }
                    }
                    SortMenu(currentSort = state.sort, availableSorts = state.availableSorts, onSortSelected = onSortSelected)
                },
            )
        },
    ) { contentPadding ->
        Column(modifier = Modifier.padding(contentPadding).fillMaxSize()) {
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
                        state.isLoading && state.applications.isEmpty() ->
                            CircularProgressIndicator(modifier = Modifier.align(Alignment.Center))

                        state.applications.isEmpty() ->
                            EmptyApplicationsState(modifier = Modifier.align(Alignment.Center))

                        else ->
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

@Composable
private fun ApplicationsList(
    applications: List<PlanningApplication>,
    isLoadingMore: Boolean,
    onItemVisible: (Int) -> Unit,
    onApplicationClick: (PlanningApplication) -> Unit,
    modifier: Modifier = Modifier,
) {
    LazyColumn(modifier = modifier.fillMaxSize()) {
        itemsIndexed(applications, key = { _, application -> application.id.value }) { index, application ->
            // Hand-rolled prefetch trigger (no Paging 3): firing once per row
            // as it enters composition is enough for the ~40-line cursor
            // loop this app deliberately keeps simple.
            LaunchedEffect(index) { onItemVisible(index) }
            ApplicationRow(application = application, onClick = { onApplicationClick(application) })
            HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
        }
        if (isLoadingMore) {
            item {
                Box(modifier = Modifier.fillMaxWidth().padding(TownCrierSpacing.md), contentAlignment = Alignment.Center) {
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

private val FILTERABLE_STATUSES =
    listOf(
        ApplicationStatus.Undecided,
        ApplicationStatus.Permitted,
        ApplicationStatus.Conditions,
        ApplicationStatus.Rejected,
        ApplicationStatus.Withdrawn,
        ApplicationStatus.Appealed,
    )

@Composable
private fun FilterChipsRow(
    filter: ApplicationFilter,
    unreadCount: Int,
    onFilterSelected: (ApplicationFilter) -> Unit,
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
                selected = filter == ApplicationFilter.All,
                onClick = { onFilterSelected(ApplicationFilter.All) },
            )
        }
        items(FILTERABLE_STATUSES) { status ->
            CapsuleChip(
                label = statusFilterLabel(status),
                selected = filter == ApplicationFilter.Status(status),
                onClick = { onFilterSelected(ApplicationFilter.Status(status)) },
            )
        }
        item {
            CapsuleChip(
                label = "${stringResource(R.string.applications_filter_unread)} ($unreadCount)",
                selected = filter == ApplicationFilter.Unread,
                onClick = { onFilterSelected(ApplicationFilter.Unread) },
            )
        }
    }
}

@Composable
private fun statusFilterLabel(status: ApplicationStatus): String =
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
private fun SortMenu(
    currentSort: ApplicationSortOrder,
    availableSorts: List<ApplicationSortOrder>,
    onSortSelected: (ApplicationSortOrder) -> Unit,
) {
    var expanded by remember { mutableStateOf(false) }
    IconButton(onClick = { expanded = true }) {
        Icon(imageVector = Icons.AutoMirrored.Filled.Sort, contentDescription = stringResource(R.string.applications_sort_content_description))
    }
    DropdownMenu(expanded = expanded, onDismissRequest = { expanded = false }) {
        availableSorts.forEach { sort ->
            DropdownMenuItem(
                text = { Text(sortLabel(sort)) },
                onClick = {
                    onSortSelected(sort)
                    expanded = false
                },
                trailingIcon = {
                    if (sort == currentSort) Icon(imageVector = Icons.Filled.Check, contentDescription = null)
                },
            )
        }
    }
}

@Composable
private fun sortLabel(sort: ApplicationSortOrder): String =
    when (sort) {
        ApplicationSortOrder.DISTANCE -> stringResource(R.string.applications_sort_distance)
        ApplicationSortOrder.NEWEST -> stringResource(R.string.applications_sort_newest)
        ApplicationSortOrder.OLDEST -> stringResource(R.string.applications_sort_oldest)
        ApplicationSortOrder.STATUS -> stringResource(R.string.applications_sort_status)
        ApplicationSortOrder.RECENT_ACTIVITY -> stringResource(R.string.applications_sort_recent_activity)
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
            Text(text = stringResource(R.string.applications_no_zones_title), style = MaterialTheme.typography.titleMedium)
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
private val previewZone = WatchZone(id = WatchZoneId("wz-1"), name = "Home", centre = Coordinate(51.5074, -0.1278), radiusMetres = 500.0)

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
        )
    }
}
