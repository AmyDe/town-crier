package uk.towncrierapp.presentation.features.watchzones

import org.junit.jupiter.api.Test
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.domain.watchzones.WatchZoneBoundary
import uk.towncrierapp.domain.watchzones.WatchZoneBoundaryResult
import uk.towncrierapp.domain.watchzones.aCoordinate
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.test.assertTrue

/**
 * The shared vertex-add/move/close/undo rules (GH#1072 Phase 5, tc-v6fo0.5)
 * both [WatchZoneEditorViewModel.onShapeEvent] and
 * [uk.towncrierapp.presentation.features.onboarding.OnboardingViewModel.onShapeEvent]
 * delegate to via [applyShapeEvent]/[resolveShapeForSave] - pure functions,
 * so these tests exercise them directly rather than through either
 * ViewModel. Covers the same behavioural ground as
 * `WatchZoneEditorViewModelShapeModeTest`, one level lower.
 */
class PolygonDrawingStateTest {
    private val triangle =
        listOf(
            Coordinate(latitude = 51.50, longitude = -0.10),
            Coordinate(latitude = 51.51, longitude = -0.09),
            Coordinate(latitude = 51.50, longitude = -0.09),
        )

    private val bowtie =
        listOf(
            Coordinate(latitude = 51.50, longitude = -0.10),
            Coordinate(latitude = 51.51, longitude = -0.09),
            Coordinate(latitude = 51.50, longitude = -0.09),
            Coordinate(latitude = 51.51, longitude = -0.10),
        )

    // MARK: - Mode gating

    @Test
    fun `mode select to CUSTOM is ignored when custom boundaries aren't allowed`() {
        val state = PolygonDrawingState()

        val result =
            state.applyShapeEvent(
                WatchZoneShapeEvent.ModeSelected(WatchZoneShapeMode.CUSTOM),
                allowsCustomBoundary = false,
            )

        assertEquals(WatchZoneShapeMode.CIRCLE, result.shapeMode)
    }

    @Test
    fun `mode select to CUSTOM is applied when custom boundaries are allowed`() {
        val state = PolygonDrawingState()

        val result =
            state.applyShapeEvent(
                WatchZoneShapeEvent.ModeSelected(WatchZoneShapeMode.CUSTOM),
                allowsCustomBoundary = true,
            )

        assertEquals(WatchZoneShapeMode.CUSTOM, result.shapeMode)
    }

    // MARK: - Adding vertices

    @Test
    fun `adding vertices accumulates them in tap order`() {
        var state = PolygonDrawingState()

        triangle.forEach {
            state =
                state.applyShapeEvent(WatchZoneShapeEvent.VertexAdded(it), allowsCustomBoundary = true)
        }

        assertEquals(triangle, state.polygonVertices)
    }

    @Test
    fun `a vertex added after the polygon is closed is ignored`() {
        var state = closedTriangle()

        state =
            state.applyShapeEvent(
                WatchZoneShapeEvent.VertexAdded(aCoordinate(latitude = 51.52)),
                allowsCustomBoundary = true,
            )

        assertEquals(triangle, state.polygonVertices)
    }

    @Test
    fun `a vertex added at the max vertex cap is ignored`() {
        val fullRing = List(WatchZoneBoundary.MAX_VERTICES) { index -> aCoordinate(latitude = 51.0 + index * 0.001) }
        var state = PolygonDrawingState(polygonVertices = fullRing)

        state =
            state.applyShapeEvent(
                WatchZoneShapeEvent.VertexAdded(aCoordinate(latitude = 60.0)),
                allowsCustomBoundary = true,
            )

        assertEquals(fullRing, state.polygonVertices)
    }

    // MARK: - Closing (minimum 3 vertices)

    @Test
    fun `closing is not allowed below 3 vertices`() {
        var state = PolygonDrawingState()
        state = state.applyShapeEvent(WatchZoneShapeEvent.VertexAdded(triangle[0]), allowsCustomBoundary = true)
        state = state.applyShapeEvent(WatchZoneShapeEvent.VertexAdded(triangle[1]), allowsCustomBoundary = true)
        assertFalse(state.canClosePolygon)

        state = state.applyShapeEvent(WatchZoneShapeEvent.PolygonClosed, allowsCustomBoundary = true)

        assertFalse(state.isPolygonClosed)
    }

    @Test
    fun `closing is allowed once 3 vertices are drawn`() {
        var state = PolygonDrawingState()
        triangle.forEach {
            state =
                state.applyShapeEvent(WatchZoneShapeEvent.VertexAdded(it), allowsCustomBoundary = true)
        }
        assertTrue(state.canClosePolygon)

        state = state.applyShapeEvent(WatchZoneShapeEvent.PolygonClosed, allowsCustomBoundary = true)

        assertTrue(state.isPolygonClosed)
        assertFalse(state.boundaryError)
    }

    @Test
    fun `closing a self-intersecting shape surfaces a boundary error and stays open`() {
        var state = PolygonDrawingState()
        bowtie.forEach {
            state =
                state.applyShapeEvent(
                    WatchZoneShapeEvent.VertexAdded(it),
                    allowsCustomBoundary = true,
                )
        }

        state = state.applyShapeEvent(WatchZoneShapeEvent.PolygonClosed, allowsCustomBoundary = true)

        assertFalse(state.isPolygonClosed)
        assertTrue(state.boundaryError)
    }

    // MARK: - Dragging (moving) a vertex

    @Test
    fun `moving a vertex replaces its position without changing the vertex count`() {
        var state = PolygonDrawingState()
        triangle.forEach {
            state =
                state.applyShapeEvent(WatchZoneShapeEvent.VertexAdded(it), allowsCustomBoundary = true)
        }
        val moved = Coordinate(latitude = 51.505, longitude = -0.095)

        state = state.applyShapeEvent(WatchZoneShapeEvent.VertexMoved(1, moved), allowsCustomBoundary = true)

        assertEquals(3, state.polygonVertices.size)
        assertEquals(moved, state.polygonVertices[1])
    }

    @Test
    fun `moving an out-of-range vertex index is ignored`() {
        var state = PolygonDrawingState()
        triangle.forEach {
            state =
                state.applyShapeEvent(WatchZoneShapeEvent.VertexAdded(it), allowsCustomBoundary = true)
        }

        state = state.applyShapeEvent(WatchZoneShapeEvent.VertexMoved(5, aCoordinate()), allowsCustomBoundary = true)

        assertEquals(triangle, state.polygonVertices)
    }

    @Test
    fun `moving a vertex on a closed polygon that stays valid keeps it closed`() {
        var state = closedTriangle()
        val moved = Coordinate(latitude = 51.505, longitude = -0.095)

        state = state.applyShapeEvent(WatchZoneShapeEvent.VertexMoved(1, moved), allowsCustomBoundary = true)

        assertTrue(state.isPolygonClosed)
        assertFalse(state.boundaryError)
        assertEquals(moved, state.polygonVertices[1])
    }

    @Test
    fun `dragging a closed polygon's vertex into a self-intersecting shape reopens it with a boundary error`() {
        val rectangle =
            listOf(
                Coordinate(latitude = 51.50, longitude = -0.10),
                Coordinate(latitude = 51.50, longitude = -0.09),
                Coordinate(latitude = 51.51, longitude = -0.09),
                Coordinate(latitude = 51.51, longitude = -0.10),
            )
        var state = PolygonDrawingState()
        rectangle.forEach {
            state =
                state.applyShapeEvent(WatchZoneShapeEvent.VertexAdded(it), allowsCustomBoundary = true)
        }
        state = state.applyShapeEvent(WatchZoneShapeEvent.PolygonClosed, allowsCustomBoundary = true)
        assertTrue(state.isPolygonClosed)

        state =
            state.applyShapeEvent(
                WatchZoneShapeEvent.VertexMoved(1, Coordinate(latitude = 51.52, longitude = -0.095)),
                allowsCustomBoundary = true,
            )

        assertFalse(state.isPolygonClosed)
        assertTrue(state.boundaryError)
    }

    // MARK: - Undo

    @Test
    fun `undo removes the last added vertex`() {
        var state = PolygonDrawingState()
        triangle.forEach {
            state =
                state.applyShapeEvent(WatchZoneShapeEvent.VertexAdded(it), allowsCustomBoundary = true)
        }

        state = state.applyShapeEvent(WatchZoneShapeEvent.UndoRequested, allowsCustomBoundary = true)

        assertEquals(triangle.dropLast(1), state.polygonVertices)
    }

    @Test
    fun `undo on a closed polygon reopens it without removing a vertex`() {
        var state = closedTriangle()

        state = state.applyShapeEvent(WatchZoneShapeEvent.UndoRequested, allowsCustomBoundary = true)

        assertFalse(state.isPolygonClosed)
        assertEquals(triangle, state.polygonVertices)
    }

    @Test
    fun `undo on an empty polygon does nothing`() {
        val state =
            PolygonDrawingState().applyShapeEvent(
                WatchZoneShapeEvent.UndoRequested,
                allowsCustomBoundary = true,
            )

        assertTrue(state.polygonVertices.isEmpty())
    }

    // MARK: - resolveShapeForSave

    @Test
    fun `resolveShapeForSave returns Circle for CIRCLE mode regardless of vertices`() {
        val state = PolygonDrawingState(shapeMode = WatchZoneShapeMode.CIRCLE, polygonVertices = triangle)

        assertEquals(ShapeForSave.Circle, state.resolveShapeForSave())
    }

    @Test
    fun `resolveShapeForSave returns NotReady for an open CUSTOM polygon`() {
        val state =
            PolygonDrawingState(
                shapeMode = WatchZoneShapeMode.CUSTOM,
                polygonVertices = triangle,
                isPolygonClosed = false,
            )

        assertEquals(ShapeForSave.NotReady, state.resolveShapeForSave())
    }

    @Test
    fun `resolveShapeForSave returns the drawn boundary for a closed valid CUSTOM polygon`() {
        val state = closedTriangle()

        val shape = assertIs<ShapeForSave.Custom>(state.resolveShapeForSave())

        val expected = (WatchZoneBoundary.of(triangle) as WatchZoneBoundaryResult.Valid).boundary
        assertEquals(expected, shape.boundary)
    }

    @Test
    fun `resolveShapeForSave returns Invalid for a closed but self-intersecting CUSTOM state`() {
        // Not reachable via applyShapeEvent (withPolygonClosed only sets
        // isPolygonClosed=true once WatchZoneBoundary.of validates) - this
        // constructs the state directly to prove resolveShapeForSave is
        // still defensive about it.
        val state =
            PolygonDrawingState(shapeMode = WatchZoneShapeMode.CUSTOM, polygonVertices = bowtie, isPolygonClosed = true)

        assertEquals(ShapeForSave.Invalid, state.resolveShapeForSave())
    }

    private fun closedTriangle(): PolygonDrawingState {
        var state =
            PolygonDrawingState().applyShapeEvent(
                WatchZoneShapeEvent.ModeSelected(WatchZoneShapeMode.CUSTOM),
                allowsCustomBoundary = true,
            )
        triangle.forEach {
            state =
                state.applyShapeEvent(WatchZoneShapeEvent.VertexAdded(it), allowsCustomBoundary = true)
        }
        return state.applyShapeEvent(WatchZoneShapeEvent.PolygonClosed, allowsCustomBoundary = true)
    }
}
