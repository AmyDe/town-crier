package uk.towncrierapp.presentation.features.watchzones

import android.content.res.Configuration
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Place
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SwipeToDismissBox
import androidx.compose.material3.SwipeToDismissBoxValue
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.rememberSwipeToDismissBoxState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.domain.watchzones.WatchZone
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.designsystem.components.PrimaryButton
import uk.towncrierapp.presentation.designsystem.components.UpgradeBadge

/**
 * The watch-zones tab: list, swipe-to-delete, tap-to-edit, "+" to add (badged
 * when at quota), and the free-tier inline upsell card at cap. Port of iOS
 * `WatchZoneListView`. "View Plans"/the upgrade badge are no-ops until #783
 * ships the paywall (bead brief tc-z95t) — the locked/upsell UI state is
 * still correct, just not yet wired to a destination.
 */
@Composable
public fun WatchZonesRoute(
    viewModel: WatchZoneListViewModel,
    onZoneSelected: (WatchZone) -> Unit,
    onAddZone: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val state by viewModel.uiState.collectAsStateWithLifecycle()
    WatchZoneListScreen(
        state = state,
        onZoneSelected = onZoneSelected,
        onDeleteZone = viewModel::deleteZone,
        onAddZoneClick = { if (state.canAddZone) onAddZone() },
        onViewPlansClick = { /* no-op until #783 ships the paywall */ },
        modifier = modifier,
    )
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun WatchZoneListScreen(
    state: WatchZoneListUiState,
    onZoneSelected: (WatchZone) -> Unit,
    onDeleteZone: (WatchZone) -> Unit,
    onAddZoneClick: () -> Unit,
    onViewPlansClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Scaffold(
        modifier = modifier,
        topBar = {
            TopAppBar(
                title = { Text(stringResource(R.string.watch_zones_title)) },
                actions = {
                    IconButton(onClick = onAddZoneClick) {
                        if (state.showUpgradeBadge) {
                            UpgradeBadge()
                        } else {
                            Icon(
                                imageVector = Icons.Filled.Add,
                                contentDescription = stringResource(R.string.watch_zones_add_content_description),
                            )
                        }
                    }
                },
            )
        },
    ) { contentPadding ->
        Box(modifier = Modifier.padding(contentPadding).fillMaxSize()) {
            if (state.zones.isEmpty() && !state.isLoading) {
                EmptyState(onAddZoneClick = onAddZoneClick, modifier = Modifier.align(Alignment.Center))
            } else {
                LazyColumn(modifier = Modifier.fillMaxSize()) {
                    items(state.zones, key = { it.id.value }) { zone ->
                        SwipeToDeleteRow(onDelete = { onDeleteZone(zone) }) {
                            WatchZoneRow(zone = zone, onClick = { onZoneSelected(zone) })
                        }
                        HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                    }
                    if (state.showsFreeTierUpsell) {
                        item {
                            WatchZoneInlineUpsellCard(
                                onViewPlans = onViewPlansClick,
                                modifier = Modifier.padding(TownCrierSpacing.md),
                            )
                        }
                    }
                }
            }
            if (state.isLoading) {
                CircularProgressIndicator(modifier = Modifier.align(Alignment.Center))
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun SwipeToDeleteRow(
    onDelete: () -> Unit,
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit,
) {
    val dismissState =
        rememberSwipeToDismissBoxState(
            confirmValueChange = { value ->
                val dismissed = value == SwipeToDismissBoxValue.EndToStart || value == SwipeToDismissBoxValue.StartToEnd
                if (dismissed) onDelete()
                dismissed
            },
        )
    SwipeToDismissBox(
        state = dismissState,
        modifier = modifier,
        backgroundContent = {
            Box(
                modifier =
                    Modifier
                        .fillMaxSize()
                        .background(MaterialTheme.colorScheme.errorContainer)
                        .padding(horizontal = TownCrierSpacing.md),
                contentAlignment = Alignment.CenterEnd,
            ) {
                Icon(
                    imageVector = Icons.Filled.Delete,
                    contentDescription = stringResource(R.string.watch_zones_delete_content_description),
                    tint = MaterialTheme.colorScheme.onErrorContainer,
                )
            }
        },
        content = { content() },
    )
}

@Composable
private fun WatchZoneRow(
    zone: WatchZone,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier =
            modifier
                .fillMaxWidth()
                .clickable(onClick = onClick)
                .padding(horizontal = TownCrierSpacing.md, vertical = TownCrierSpacing.sm),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.md),
    ) {
        ZoneMapPlaceholder(modifier = Modifier.size(56.dp))
        Column(modifier = Modifier.weight(1f)) {
            Text(text = zone.name, style = MaterialTheme.typography.titleMedium)
            Text(
                text = RadiusFormatter.format(zone.radiusMetres),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        Icon(
            imageVector = Icons.AutoMirrored.Filled.KeyboardArrowRight,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

/**
 * Static placeholder for the zone's map preview — Google Maps lands in #776;
 * the editor and list both use this box until then (issue's Out of Scope).
 */
@Composable
internal fun ZoneMapPlaceholder(modifier: Modifier = Modifier) {
    Box(
        modifier =
            modifier
                .background(MaterialTheme.colorScheme.surfaceContainerHigh, shape = MaterialTheme.shapes.small),
        contentAlignment = Alignment.Center,
    ) {
        Icon(
            imageVector = Icons.Filled.Place,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
private fun EmptyState(
    onAddZoneClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier.padding(TownCrierSpacing.xl),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.md),
    ) {
        Icon(
            imageVector = Icons.Filled.Place,
            contentDescription = null,
            tint = TownCrierTheme.colors.textTertiary,
            modifier = Modifier.size(48.dp),
        )
        Text(text = stringResource(R.string.watch_zones_empty_title), style = MaterialTheme.typography.titleMedium)
        Text(
            text = stringResource(R.string.watch_zones_empty_message),
            style = MaterialTheme.typography.bodyLarge,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        PrimaryButton(text = stringResource(R.string.watch_zones_empty_cta), onClick = onAddZoneClick)
    }
}

// Preview-only sample data — cannot reuse :domain's testFixtures from the
// main source set (compose-ui.md: previews can't see test/testFixtures
// source sets), so a small duplicate lives here instead.
private val previewHomeZone =
    WatchZone(
        id = WatchZoneId("preview-home"),
        name = "Home",
        centre = Coordinate(51.5074, -0.1278),
        radiusMetres = 500.0,
    )
private val previewOfficeZone =
    WatchZone(
        id = WatchZoneId("preview-office"),
        name = "Office",
        centre = Coordinate(51.5155, -0.0922),
        radiusMetres = 1_500.0,
    )
private val previewParentsZone =
    WatchZone(
        id = WatchZoneId("preview-parents"),
        name = "Parents",
        centre = Coordinate(52.2053, 0.1218),
        radiusMetres = 2_000.0,
    )

@Preview(name = "light")
@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun WatchZoneListScreenPreview() {
    TownCrierTheme {
        WatchZoneListScreen(
            state = WatchZoneListUiState(zones = listOf(previewHomeZone, previewOfficeZone)),
            onZoneSelected = {},
            onDeleteZone = {},
            onAddZoneClick = {},
            onViewPlansClick = {},
        )
    }
}

@Preview(name = "empty")
@Composable
private fun WatchZoneListScreenEmptyPreview() {
    TownCrierTheme {
        WatchZoneListScreen(
            state = WatchZoneListUiState(),
            onZoneSelected = {},
            onDeleteZone = {},
            onAddZoneClick = {},
            onViewPlansClick = {},
        )
    }
}

@Preview(name = "free tier at cap — badge + inline upsell")
@Composable
private fun WatchZoneListScreenFreeAtCapPreview() {
    TownCrierTheme {
        WatchZoneListScreen(
            state =
                WatchZoneListUiState(
                    zones = listOf(previewHomeZone),
                    canAddZone = false,
                    showUpgradeBadge = true,
                    showsFreeTierUpsell = true,
                ),
            onZoneSelected = {},
            onDeleteZone = {},
            onAddZoneClick = {},
            onViewPlansClick = {},
        )
    }
}

@Preview(name = "personal tier at cap — badge only")
@Composable
private fun WatchZoneListScreenPersonalAtCapPreview() {
    TownCrierTheme {
        WatchZoneListScreen(
            state =
                WatchZoneListUiState(
                    zones = listOf(previewHomeZone, previewOfficeZone, previewParentsZone),
                    canAddZone = false,
                    showUpgradeBadge = true,
                    showsFreeTierUpsell = false,
                ),
            onZoneSelected = {},
            onDeleteZone = {},
            onAddZoneClick = {},
            onViewPlansClick = {},
        )
    }
}
