package uk.towncrierapp.presentation.features.watchzones

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
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
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Notifications
import androidx.compose.material.icons.filled.Place
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
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
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import uk.towncrierapp.domain.watchzones.WatchZone
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.designsystem.components.Masthead
import uk.towncrierapp.presentation.designsystem.components.PrimaryButton
import uk.towncrierapp.presentation.designsystem.components.UpgradeBadge
import uk.towncrierapp.presentation.designsystem.noticeCard

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
    onZonePreferencesSelected: (WatchZone) -> Unit,
    onAddZone: () -> Unit,
    onSettingsClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val state by viewModel.uiState.collectAsStateWithLifecycle()
    WatchZoneListScreen(
        state = state,
        onZoneSelected = onZoneSelected,
        onZonePreferencesSelected = onZonePreferencesSelected,
        onDeleteZone = viewModel::deleteZone,
        onAddZoneClick = { if (state.canAddZone) onAddZone() },
        onViewPlansClick = { /* no-op until #783 ships the paywall */ },
        onSettingsClick = onSettingsClick,
        modifier = modifier,
    )
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun WatchZoneListScreen(
    state: WatchZoneListUiState,
    onZoneSelected: (WatchZone) -> Unit,
    onZonePreferencesSelected: (WatchZone) -> Unit,
    onDeleteZone: (WatchZone) -> Unit,
    onAddZoneClick: () -> Unit,
    onViewPlansClick: () -> Unit,
    onSettingsClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Scaffold(
        modifier = modifier,
        topBar = {
            WatchZoneListTopBar(
                showUpgradeBadge = state.showUpgradeBadge,
                onAddZoneClick = onAddZoneClick,
                onSettingsClick = onSettingsClick,
            )
        },
    ) { contentPadding ->
        Column(modifier = Modifier.padding(contentPadding).fillMaxSize()) {
            Masthead(title = stringResource(R.string.watch_zones_title))
            Box(modifier = Modifier.weight(1f).fillMaxWidth()) {
                if (state.zones.isEmpty() && !state.isLoading) {
                    EmptyState(onAddZoneClick = onAddZoneClick, modifier = Modifier.align(Alignment.Center))
                } else {
                    LazyColumn(
                        modifier = Modifier.fillMaxSize(),
                        verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
                        contentPadding = PaddingValues(vertical = TownCrierSpacing.sm),
                    ) {
                        items(state.zones, key = { it.id.value }) { zone ->
                            SwipeToDeleteRow(
                                onDelete = { onDeleteZone(zone) },
                                modifier = Modifier.padding(horizontal = TownCrierSpacing.md),
                            ) {
                                WatchZoneRow(
                                    zone = zone,
                                    onClick = { onZoneSelected(zone) },
                                    onPreferencesClick = { onZonePreferencesSelected(zone) },
                                )
                            }
                        }
                        if (state.showsFreeTierUpsell) {
                            item {
                                WatchZoneInlineUpsellCard(
                                    onViewPlans = onViewPlansClick,
                                    modifier = Modifier.padding(horizontal = TownCrierSpacing.md),
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
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun WatchZoneListTopBar(
    showUpgradeBadge: Boolean,
    onAddZoneClick: () -> Unit,
    onSettingsClick: () -> Unit,
) {
    TopAppBar(
        title = { Text(stringResource(R.string.watch_zones_title)) },
        actions = {
            if (showUpgradeBadge) {
                // Not an IconButton: its fixed-size (40dp) content box clips a
                // wider pill-shaped badge with a text label, squeezing
                // "Upgrade" into a vertical wrap — verified live on-device
                // (tc-z95t). A clickable Box lets the badge size itself
                // naturally while staying tappable.
                Box(
                    modifier =
                        Modifier
                            .clickable(onClick = onAddZoneClick)
                            .padding(horizontal = TownCrierSpacing.md, vertical = TownCrierSpacing.sm),
                ) {
                    UpgradeBadge()
                }
            } else {
                IconButton(onClick = onAddZoneClick) {
                    Icon(
                        imageVector = Icons.Filled.Add,
                        contentDescription = stringResource(R.string.watch_zones_add_content_description),
                    )
                }
            }
            IconButton(onClick = onSettingsClick) {
                Icon(
                    imageVector = Icons.Filled.Settings,
                    contentDescription = stringResource(R.string.settings_content_description),
                )
            }
        },
    )
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

/**
 * A single watch-zone row, styled as a Public Notice "filed notice" card
 * ([noticeCard], epic #848 R5). The radius reads as the row's mono metadata
 * line ahead of the zone's name — mirrors the planning-reference strip on
 * [uk.towncrierapp.presentation.designsystem.components.ApplicationRow] and
 * iOS `WatchZoneRow`.
 */
@Composable
private fun WatchZoneRow(
    zone: WatchZone,
    onClick: () -> Unit,
    onPreferencesClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier =
            modifier
                .fillMaxWidth()
                .noticeCard()
                .clickable(onClick = onClick)
                .padding(horizontal = TownCrierSpacing.md, vertical = TownCrierSpacing.sm),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.md),
    ) {
        ZoneMapPlaceholder(modifier = Modifier.size(56.dp))
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = RadiusFormatter.format(zone.radiusMetres),
                style = TownCrierTheme.mono,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Text(text = zone.name, style = MaterialTheme.typography.titleMedium)
        }
        IconButton(onClick = onPreferencesClick) {
            Icon(
                imageVector = Icons.Filled.Notifications,
                contentDescription = stringResource(R.string.watch_zones_preferences_content_description),
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
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

// Previews live in WatchZoneListPreviews.kt (detekt per-file function-count budget).
