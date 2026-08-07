package uk.towncrierapp.presentation.features.map

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.google.android.gms.maps.model.CameraPosition
import com.google.maps.android.compose.Circle
import com.google.maps.android.compose.GoogleMap
import com.google.maps.android.compose.Marker
import com.google.maps.android.compose.MarkerState
import com.google.maps.android.compose.rememberCameraPositionState
import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.PlanningApplication
import uk.towncrierapp.domain.applications.wireValue
import uk.towncrierapp.domain.map.MapCluster
import uk.towncrierapp.domain.map.MapViewport
import uk.towncrierapp.domain.map.zoom
import uk.towncrierapp.domain.watchzones.WatchZone
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierRadius
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.components.CapsuleChip
import uk.towncrierapp.presentation.features.applicationlist.applicationErrorMessageRes

/**
 * The Map tab: server-side cluster rendering via `GoogleMap`/maps-compose
 * (GH#776) — zone/status chips, status-tinted single pins, amber count
 * bubbles, the zone radius overlay, and the summary/disambiguation bottom
 * sheets. Route + Screen split per compose-ui.md; camera-idle events (a
 * pan/zoom settling) drive [MapViewModel.onCameraIdle]'s debounced refetch.
 */
@Composable
public fun MapRoute(
    viewModel: MapViewModel,
    onSettingsClick: () -> Unit,
    onViewFullDetails: (PlanningApplication) -> Unit,
    modifier: Modifier = Modifier,
) {
    val state by viewModel.uiState.collectAsStateWithLifecycle()
    MapScreen(
        state = state,
        onZoneSelected = viewModel::selectZone,
        onStatusFilterSelected = viewModel::applyStatusFilter,
        onClusterTapped = viewModel::onClusterTapped,
        onCameraIdle = viewModel::onCameraIdle,
        onCameraTargetConsumed = viewModel::consumeCameraTarget,
        onSummaryDismiss = viewModel::clearSelection,
        onSaveToggleClick = viewModel::toggleSaveSelectedApplication,
        onViewFullDetailsClick = { state.selectedApplication?.let(onViewFullDetails) },
        onStackedRowSelected = viewModel::selectFromStack,
        onStackedDismiss = viewModel::clearStack,
        onSettingsClick = onSettingsClick,
        modifier = modifier,
    )
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun MapScreen(
    state: MapUiState,
    onZoneSelected: (WatchZone) -> Unit,
    onStatusFilterSelected: (ApplicationStatus?) -> Unit,
    onClusterTapped: (MapCluster) -> Unit,
    onCameraIdle: (MapViewport, Double) -> Unit,
    onCameraTargetConsumed: () -> Unit,
    onSummaryDismiss: () -> Unit,
    onSaveToggleClick: () -> Unit,
    onViewFullDetailsClick: () -> Unit,
    onStackedRowSelected: (PlanningApplication) -> Unit,
    onStackedDismiss: () -> Unit,
    onSettingsClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Scaffold(
        modifier = modifier,
        topBar = {
            TopAppBar(
                title = { Text(text = stringResource(R.string.bottom_nav_map)) },
                actions = {
                    IconButton(onClick = onSettingsClick) {
                        Icon(
                            imageVector = Icons.Filled.Settings,
                            contentDescription = stringResource(R.string.settings_content_description),
                        )
                    }
                },
            )
        },
    ) { contentPadding ->
        Column(modifier = Modifier.padding(contentPadding).fillMaxSize()) {
            if (state.showZonePicker) {
                ZonePickerRow(zones = state.zones, selectedZone = state.selectedZone, onZoneSelected = onZoneSelected)
            }
            if (state.showStatusFilters) {
                StatusFilterRow(selected = state.selectedStatusFilter, onSelected = onStatusFilterSelected)
            }
            Box(modifier = Modifier.weight(1f).fillMaxSize()) {
                MapBody(
                    state = state,
                    onClusterTapped = onClusterTapped,
                    onCameraIdle = onCameraIdle,
                    onCameraTargetConsumed = onCameraTargetConsumed,
                )
            }
        }
    }

    val selectedApplication = state.selectedApplication
    if (selectedApplication != null) {
        ModalBottomSheet(onDismissRequest = onSummaryDismiss) {
            ApplicationSummarySheetContent(
                application = selectedApplication,
                isSaved = state.isSelectedApplicationSaved,
                onSaveToggleClick = onSaveToggleClick,
                onViewFullDetailsClick = onViewFullDetailsClick,
            )
        }
    }

    val stackedApplications = state.stackedApplications
    if (stackedApplications != null) {
        ModalBottomSheet(onDismissRequest = onStackedDismiss) {
            StackedApplicationsSheetContent(applications = stackedApplications, onSelect = onStackedRowSelected)
        }
    }
}

@Composable
private fun ZonePickerRow(
    zones: List<WatchZone>,
    selectedZone: WatchZone?,
    onZoneSelected: (WatchZone) -> Unit,
) {
    LazyRow(
        horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
        contentPadding = PaddingValues(horizontal = TownCrierSpacing.md, vertical = TownCrierSpacing.sm),
    ) {
        items(zones, key = { it.id.value }) { zone ->
            CapsuleChip(label = zone.name, selected = zone.id == selectedZone?.id, onClick = { onZoneSelected(zone) })
        }
    }
}

/** `null` represents "All" — the map has no unread filter, only All|Status (GH#776). */
private val STATUS_FILTER_OPTIONS: List<ApplicationStatus?> =
    listOf(
        null,
        ApplicationStatus.Undecided,
        ApplicationStatus.Permitted,
        ApplicationStatus.Conditions,
        ApplicationStatus.Rejected,
        ApplicationStatus.Withdrawn,
        ApplicationStatus.Appealed,
    )

@Composable
private fun StatusFilterRow(
    selected: ApplicationStatus?,
    onSelected: (ApplicationStatus?) -> Unit,
) {
    LazyRow(
        horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
        contentPadding = PaddingValues(horizontal = TownCrierSpacing.md, vertical = TownCrierSpacing.sm),
    ) {
        items(STATUS_FILTER_OPTIONS, key = { it?.wireValue ?: "All" }) { status ->
            CapsuleChip(
                label = statusFilterLabel(status),
                selected = status == selected,
                onClick = { onSelected(status) },
            )
        }
    }
}

@Composable
private fun statusFilterLabel(status: ApplicationStatus?): String =
    when (status) {
        null -> stringResource(R.string.map_status_filter_all)
        ApplicationStatus.Undecided -> stringResource(R.string.application_status_pending)
        ApplicationStatus.Permitted -> stringResource(R.string.application_status_permitted)
        ApplicationStatus.Conditions -> stringResource(R.string.application_status_conditions)
        ApplicationStatus.Rejected -> stringResource(R.string.application_status_rejected)
        ApplicationStatus.Withdrawn -> stringResource(R.string.application_status_withdrawn)
        ApplicationStatus.Appealed -> stringResource(R.string.application_status_appealed)
        else -> status.wireValue
    }

@Composable
private fun BoxScope.MapBody(
    state: MapUiState,
    onClusterTapped: (MapCluster) -> Unit,
    onCameraIdle: (MapViewport, Double) -> Unit,
    onCameraTargetConsumed: () -> Unit,
) {
    when {
        state.isLoading && !state.hasLoaded -> {
            CircularProgressIndicator(modifier = Modifier.align(Alignment.Center))
        }

        // The map itself always renders once something has loaded — an error or an
        // empty viewport is an overlay ON TOP of it, never a replacement for it.
        // Swapping the map out here would strand the user: with no map, there is no
        // way left to pan or zoom back to an area that has applications.
        else -> {
            GoogleMapContent(
                state = state,
                onClusterTapped = onClusterTapped,
                onCameraIdle = onCameraIdle,
                onCameraTargetConsumed = onCameraTargetConsumed,
            )
            when {
                state.error != null -> {
                    MapOverlayCard(modifier = Modifier.align(Alignment.Center).padding(TownCrierSpacing.md)) {
                        Text(
                            text = stringResource(applicationErrorMessageRes(state.error)),
                            style = MaterialTheme.typography.bodyMedium,
                        )
                    }
                }

                state.isEmpty -> {
                    MapOverlayCard(modifier = Modifier.align(Alignment.Center).padding(TownCrierSpacing.md)) {
                        Text(
                            text = stringResource(R.string.map_empty_title),
                            style = MaterialTheme.typography.titleLarge,
                        )
                        Text(
                            text = stringResource(R.string.map_empty_description),
                            style = MaterialTheme.typography.bodyMedium,
                        )
                    }
                }
            }
            if (state.isLoading) {
                CircularProgressIndicator(modifier = Modifier.align(Alignment.TopCenter).padding(TownCrierSpacing.md))
            }
        }
    }
}

/** A legible surface behind text overlaid on the map — otherwise error/empty copy can land directly on map tiles of any colour. */
@Composable
private fun MapOverlayCard(
    modifier: Modifier = Modifier,
    content: @Composable ColumnScope.() -> Unit,
) {
    Column(
        modifier =
            modifier
                .clip(RoundedCornerShape(TownCrierRadius.md))
                .background(MaterialTheme.colorScheme.surface)
                .padding(TownCrierSpacing.md),
        content = content,
    )
}

/**
 * The `GoogleMap` itself: zone radius [Circle], per-cluster [Marker]s
 * (status-tinted pin or amber count bubble, see [rememberMapMarkerIcons]),
 * and the two camera effects — [MapUiState.pendingCameraTarget] (a zone
 * switch or a splittable-bubble zoom-in, moved with no server round-trip)
 * and the debounced camera-idle refetch (a pan/zoom settling, GH#776).
 */
@Composable
private fun GoogleMapContent(
    state: MapUiState,
    onClusterTapped: (MapCluster) -> Unit,
    onCameraIdle: (MapViewport, Double) -> Unit,
    onCameraTargetConsumed: () -> Unit,
) {
    val zone = state.selectedZone
    val cameraPositionState =
        rememberCameraPositionState {
            if (zone != null) {
                position = CameraPosition.fromLatLngZoom(zone.centre.toLatLng(), DEFAULT_CAMERA_ZOOM)
            }
        }
    var wasMoving by remember { mutableStateOf(false) }
    val markerIcons = rememberMapMarkerIcons()

    LaunchedEffect(state.pendingCameraTarget) {
        val target = state.pendingCameraTarget ?: return@LaunchedEffect
        cameraPositionState.position =
            CameraPosition.fromLatLngZoom(target.toLatLngBounds().center, target.zoom.toFloat())
        onCameraTargetConsumed()
    }

    LaunchedEffect(cameraPositionState.isMoving) {
        if (wasMoving && !cameraPositionState.isMoving) {
            val bounds = cameraPositionState.projection?.visibleRegion?.latLngBounds
            if (bounds != null) {
                onCameraIdle(bounds.toMapViewport(), cameraPositionState.position.zoom.toDouble())
            }
        }
        wasMoving = cameraPositionState.isMoving
    }

    GoogleMap(modifier = Modifier.fillMaxSize(), cameraPositionState = cameraPositionState) {
        if (zone != null) {
            Circle(
                center = zone.centre.toLatLng(),
                radius = zone.radiusMetres,
                fillColor = MaterialTheme.colorScheme.primary.copy(alpha = ZONE_CIRCLE_FILL_ALPHA),
                strokeColor = MaterialTheme.colorScheme.primary.copy(alpha = ZONE_CIRCLE_STROKE_ALPHA),
                strokeWidth = ZONE_CIRCLE_STROKE_WIDTH_PX,
            )
        }
        state.clusters.forEach { cluster ->
            val icon =
                if (cluster.isSingleMember) {
                    markerIcons.statusIcon(cluster.memberStatus ?: ApplicationStatus.Unknown(""))
                } else {
                    markerIcons.bubbleIcon(cluster.count)
                }
            Marker(
                state = MarkerState(position = cluster.coordinate.toLatLng()),
                icon = icon,
                onClick = {
                    onClusterTapped(cluster)
                    true
                },
            )
        }
    }
}

private const val DEFAULT_CAMERA_ZOOM = 14f
private const val ZONE_CIRCLE_FILL_ALPHA = 0.08f
private const val ZONE_CIRCLE_STROKE_ALPHA = 0.3f
private const val ZONE_CIRCLE_STROKE_WIDTH_PX = 1.5f
