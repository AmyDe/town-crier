package uk.towncrierapp.presentation.features.watchzones

import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.domain.watchzones.WatchZoneBoundary
import uk.towncrierapp.domain.watchzones.WatchZoneBoundaryResult

/**
 * The shape-mode/polygon-drawing fields shared by the watch-zone editor and
 * onboarding's custom-shape drawing step (GH#1072 Phase 5, tc-v6fo0.5).
 * [applyShapeEvent] and its private `with*` helpers are the SINGLE
 * implementation of the vertex-add/move/close/undo rules (3-50 vertices,
 * [WatchZoneBoundary.of] validation) — both [WatchZoneEditorUiState] and
 * [uk.towncrierapp.presentation.features.onboarding.OnboardingUiState] carry
 * their own flat copies of these same five fields (so existing call sites,
 * previews, and tests keep reading e.g. `state.shapeMode` unchanged) and
 * convert to/from this holder only at the point they dispatch a
 * [WatchZoneShapeEvent] or resolve a save/confirm action.
 */
internal data class PolygonDrawingState(
    val shapeMode: WatchZoneShapeMode = WatchZoneShapeMode.CIRCLE,
    val polygonVertices: List<Coordinate> = emptyList(),
    val isPolygonClosed: Boolean = false,
    val canClosePolygon: Boolean = false,
    val boundaryError: Boolean = false,
)

/**
 * Routes a [WatchZoneShapeEvent] to the matching rule below — the single
 * dispatch point both [WatchZoneEditorViewModel.onShapeEvent] and
 * [uk.towncrierapp.presentation.features.onboarding.OnboardingViewModel.onShapeEvent]
 * delegate to. [allowsCustomBoundary] gates [WatchZoneShapeEvent.ModeSelected]
 * into [WatchZoneShapeMode.CUSTOM] — belt-and-braces alongside neither Screen
 * ever offering the toggle/affordance to an ineligible tier.
 */
internal fun PolygonDrawingState.applyShapeEvent(
    event: WatchZoneShapeEvent,
    allowsCustomBoundary: Boolean,
): PolygonDrawingState =
    when (event) {
        is WatchZoneShapeEvent.ModeSelected -> withShapeMode(event.mode, allowsCustomBoundary)
        is WatchZoneShapeEvent.VertexAdded -> withVertexAdded(event.coordinate)
        is WatchZoneShapeEvent.VertexMoved -> withVertexMoved(event.index, event.coordinate)
        WatchZoneShapeEvent.PolygonClosed -> withPolygonClosed()
        WatchZoneShapeEvent.UndoRequested -> withUndoApplied()
    }

private fun PolygonDrawingState.withShapeMode(
    mode: WatchZoneShapeMode,
    allowsCustomBoundary: Boolean,
): PolygonDrawingState {
    if (mode == WatchZoneShapeMode.CUSTOM && !allowsCustomBoundary) return this
    return copy(shapeMode = mode)
}

/** Ignored once closed (undo first) or at the 50-vertex cap ([WatchZoneBoundary.MAX_VERTICES]). */
private fun PolygonDrawingState.withVertexAdded(coordinate: Coordinate): PolygonDrawingState {
    if (isPolygonClosed || polygonVertices.size >= WatchZoneBoundary.MAX_VERTICES) return this
    val vertices = polygonVertices + coordinate
    return copy(
        polygonVertices = vertices,
        canClosePolygon = vertices.size >= WatchZoneBoundary.MIN_VERTICES,
        boundaryError = false,
    )
}

/**
 * Ignored for an out-of-range [index]. Dragging a vertex on an already-closed
 * polygon re-validates through [WatchZoneBoundary.of] the same way
 * [withPolygonClosed] does — a drag that makes the ring self-intersecting
 * reopens it and surfaces [PolygonDrawingState.boundaryError] rather than
 * leaving [PolygonDrawingState.isPolygonClosed] stale for a shape that would
 * silently fail on save/confirm.
 */
private fun PolygonDrawingState.withVertexMoved(
    index: Int,
    coordinate: Coordinate,
): PolygonDrawingState {
    if (index !in polygonVertices.indices) return this
    val vertices = polygonVertices.toMutableList().also { it[index] = coordinate }
    if (!isPolygonClosed) {
        return copy(polygonVertices = vertices, boundaryError = false)
    }
    return when (WatchZoneBoundary.of(vertices)) {
        is WatchZoneBoundaryResult.Valid -> copy(polygonVertices = vertices, boundaryError = false)
        else -> copy(polygonVertices = vertices, isPolygonClosed = false, boundaryError = true)
    }
}

/**
 * Requires at least [WatchZoneBoundary.MIN_VERTICES] and a simple,
 * non-self-intersecting ring — [WatchZoneBoundary.of] is the single source
 * of truth for both, so an invalid shape sets [PolygonDrawingState.boundaryError]
 * and stays open rather than closing anyway.
 */
private fun PolygonDrawingState.withPolygonClosed(): PolygonDrawingState {
    if (isPolygonClosed || polygonVertices.size < WatchZoneBoundary.MIN_VERTICES) return this
    return when (WatchZoneBoundary.of(polygonVertices)) {
        is WatchZoneBoundaryResult.Valid -> copy(isPolygonClosed = true, boundaryError = false)
        else -> copy(boundaryError = true)
    }
}

/** Reopens a just-closed polygon without dropping a vertex, otherwise drops the most recently added vertex; a no-op with nothing drawn. */
private fun PolygonDrawingState.withUndoApplied(): PolygonDrawingState =
    when {
        isPolygonClosed -> {
            copy(isPolygonClosed = false, boundaryError = false)
        }

        polygonVertices.isNotEmpty() -> {
            val vertices = polygonVertices.dropLast(1)
            copy(
                polygonVertices = vertices,
                canClosePolygon = vertices.size >= WatchZoneBoundary.MIN_VERTICES,
                boundaryError = false,
            )
        }

        else -> {
            this
        }
    }

/**
 * The shape a save/confirm action resolves from the current
 * [PolygonDrawingState] — shared by [WatchZoneEditorViewModel.save] and
 * [uk.towncrierapp.presentation.features.onboarding.OnboardingViewModel.confirmBoundary].
 */
internal sealed interface ShapeForSave {
    data object Circle : ShapeForSave

    data class Custom(
        val boundary: WatchZoneBoundary,
    ) : ShapeForSave

    /** Custom mode but the polygon isn't closed yet — the caller aborts silently, matching the disabled save/confirm button. */
    data object NotReady : ShapeForSave

    /** Custom mode, closed, but [WatchZoneBoundary.of] rejected the vertices (e.g. self-intersecting). */
    data object Invalid : ShapeForSave
}

internal fun PolygonDrawingState.resolveShapeForSave(): ShapeForSave =
    when (shapeMode) {
        WatchZoneShapeMode.CIRCLE -> {
            ShapeForSave.Circle
        }

        WatchZoneShapeMode.CUSTOM -> {
            if (!isPolygonClosed) {
                ShapeForSave.NotReady
            } else {
                when (val result = WatchZoneBoundary.of(polygonVertices)) {
                    is WatchZoneBoundaryResult.Valid -> ShapeForSave.Custom(result.boundary)
                    else -> ShapeForSave.Invalid
                }
            }
        }
    }
