package uk.towncrierapp.presentation.features.watchzones

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.google.android.gms.maps.model.CameraPosition
import com.google.maps.android.compose.GoogleMap
import com.google.maps.android.compose.Marker
import com.google.maps.android.compose.Polygon
import com.google.maps.android.compose.Polyline
import com.google.maps.android.compose.rememberCameraPositionState
import com.google.maps.android.compose.rememberUpdatedMarkerState
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.domain.watchzones.WatchZoneBoundary
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.features.map.toCoordinate
import uk.towncrierapp.presentation.features.map.toLatLng

/**
 * The custom-shape drawing surface (GH#1072 Phase 3): a `GoogleMap` built on
 * the same maps-compose patterns as `features/map/MapScreen.kt` — tap to add
 * a vertex, drag to move one, tap the first vertex to close, undo the last
 * action. Purely a rendering + gesture-routing layer; every rule (minimum 3
 * vertices, self-intersection, the 50-vertex cap) lives in
 * [WatchZoneEditorViewModel] and [WatchZoneBoundary], not here — this
 * composable only forwards taps/drags and renders the current state.
 */
@Composable
internal fun PolygonDrawingSection(
    state: WatchZoneEditorUiState,
    centre: Coordinate,
    onShapeEvent: (WatchZoneShapeEvent) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm)) {
        Text(
            text =
                stringResource(
                    if (state.isPolygonClosed) {
                        R.string.watch_zone_editor_polygon_closed_hint
                    } else {
                        R.string.watch_zone_editor_polygon_instructions
                    },
                ),
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        PolygonDrawingMap(
            vertices = state.polygonVertices,
            isClosed = state.isPolygonClosed,
            centre = centre,
            onMapTap = { coordinate -> onShapeEvent(WatchZoneShapeEvent.VertexAdded(coordinate)) },
            onVertexMoved = { index, coordinate -> onShapeEvent(WatchZoneShapeEvent.VertexMoved(index, coordinate)) },
            onFirstVertexTap = { onShapeEvent(WatchZoneShapeEvent.PolygonClosed) },
            modifier = Modifier.fillMaxWidth().height(DRAWING_MAP_HEIGHT),
        )
        Row(
            horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            TextButton(
                onClick = { onShapeEvent(WatchZoneShapeEvent.UndoRequested) },
                enabled = state.polygonVertices.isNotEmpty(),
            ) {
                Text(stringResource(R.string.watch_zone_editor_polygon_undo_button))
            }
            Text(
                text =
                    stringResource(
                        R.string.watch_zone_editor_polygon_vertex_count,
                        state.polygonVertices.size,
                        WatchZoneBoundary.MAX_VERTICES,
                    ),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        if (state.boundaryError) {
            Text(
                text = stringResource(R.string.watch_zone_editor_polygon_invalid_shape),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.error,
            )
        }
    }
}

/**
 * The `GoogleMap` itself. A background tap adds a vertex ([onMapTap]);
 * tapping the first vertex's [Marker] closes the polygon instead of adding
 * one on top of it ([onFirstVertexTap]) — maps-compose only invokes one of
 * `onMapClick`/a marker's `onClick` per tap, so no distance-based
 * hit-testing is needed here. Every vertex is a draggable [Marker]; a drag's
 * *final* position (not every intermediate frame) is reported via
 * [onVertexMoved] once `MarkerState.isDragging` flips back to `false`
 * (mirrors `MapScreen.kt`'s own `wasMoving` idle-detection pattern for the
 * camera).
 */
@Composable
private fun PolygonDrawingMap(
    vertices: List<Coordinate>,
    isClosed: Boolean,
    centre: Coordinate,
    onMapTap: (Coordinate) -> Unit,
    onVertexMoved: (Int, Coordinate) -> Unit,
    onFirstVertexTap: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val cameraPositionState =
        rememberCameraPositionState {
            position = CameraPosition.fromLatLngZoom(centre.toLatLng(), DRAWING_CAMERA_ZOOM)
        }

    GoogleMap(
        modifier = modifier,
        cameraPositionState = cameraPositionState,
        onMapClick = { latLng -> if (!isClosed) onMapTap(latLng.toCoordinate()) },
    ) {
        if (vertices.size >= MIN_POINTS_FOR_OUTLINE) {
            val outline = if (isClosed) vertices + vertices.first() else vertices
            Polyline(
                points = outline.map { it.toLatLng() },
                color = MaterialTheme.colorScheme.primary,
                width = POLYGON_STROKE_WIDTH_PX,
            )
        }
        if (isClosed && vertices.size >= WatchZoneBoundary.MIN_VERTICES) {
            Polygon(
                points = vertices.map { it.toLatLng() },
                fillColor = MaterialTheme.colorScheme.primary.copy(alpha = POLYGON_FILL_ALPHA),
                strokeColor = MaterialTheme.colorScheme.primary,
                strokeWidth = POLYGON_STROKE_WIDTH_PX,
            )
        }
        vertices.forEachIndexed { index, vertex ->
            val markerState = rememberUpdatedMarkerState(vertex.toLatLng())
            var wasDragging by remember { mutableStateOf(false) }
            LaunchedEffect(markerState) {
                snapshotFlow { markerState.isDragging }
                    .collect { isDragging ->
                        if (wasDragging && !isDragging) {
                            onVertexMoved(index, markerState.position.toCoordinate())
                        }
                        wasDragging = isDragging
                    }
            }
            Marker(
                state = markerState,
                draggable = true,
                onClick = {
                    if (index == 0 && !isClosed) onFirstVertexTap()
                    true
                },
            )
        }
    }
}

private val DRAWING_MAP_HEIGHT = 240.dp
private const val DRAWING_CAMERA_ZOOM = 15f
private const val MIN_POINTS_FOR_OUTLINE = 2
private const val POLYGON_FILL_ALPHA = 0.12f
private const val POLYGON_STROKE_WIDTH_PX = 3f
