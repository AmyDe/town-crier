package uk.towncrierapp.presentation.features.watchzones

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.domain.watchzones.FakePostcodeGeocoder
import uk.towncrierapp.domain.watchzones.FakeWatchZoneRepository
import uk.towncrierapp.domain.watchzones.WatchZoneBoundary
import uk.towncrierapp.domain.watchzones.WatchZoneBoundaryResult
import uk.towncrierapp.domain.watchzones.aCoordinate
import uk.towncrierapp.domain.watchzones.aWatchZone
import uk.towncrierapp.presentation.MainDispatcherExtension
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

// Thin wrappers over the single WatchZoneEditorViewModel.onShapeEvent entry
// point (see WatchZoneShapeEvent) so this test's bodies read as one action
// per line, same as production call sites did before the events consolidated
// the ViewModel's public surface for detekt's TooManyFunctions budget.
private fun WatchZoneEditorViewModel.selectShapeMode(mode: WatchZoneShapeMode) =
    onShapeEvent(WatchZoneShapeEvent.ModeSelected(mode))

private fun WatchZoneEditorViewModel.addPolygonVertex(coordinate: Coordinate) =
    onShapeEvent(WatchZoneShapeEvent.VertexAdded(coordinate))

private fun WatchZoneEditorViewModel.movePolygonVertex(
    index: Int,
    coordinate: Coordinate,
) = onShapeEvent(WatchZoneShapeEvent.VertexMoved(index, coordinate))

private fun WatchZoneEditorViewModel.closePolygon() = onShapeEvent(WatchZoneShapeEvent.PolygonClosed)

private fun WatchZoneEditorViewModel.undoLastPolygonAction() = onShapeEvent(WatchZoneShapeEvent.UndoRequested)

/**
 * Shape-mode toggle + polygon-drawing state machine (GH#1072 Phase 3,
 * tc-v6fo0.3): Circle/Custom shape selection gated on
 * [WatchZoneEditorUiState.allowsCustomBoundary], vertex add/drag/undo, the
 * minimum-3-vertex gate on closing, and the boundary that reaches
 * `save()`. Port of iOS `WatchZoneEditorViewModelShapeModeTests`.
 */
@ExtendWith(MainDispatcherExtension::class)
class WatchZoneEditorViewModelShapeModeTest {
    private fun makeViewModel(
        tier: SubscriptionTier,
        repository: FakeWatchZoneRepository = FakeWatchZoneRepository(),
        editingZone: uk.towncrierapp.domain.watchzones.WatchZone? = null,
    ) = WatchZoneEditorViewModel(FakePostcodeGeocoder(), repository, tier, editingZone = editingZone)

    private fun geocodedViewModel(
        tier: SubscriptionTier,
        repository: FakeWatchZoneRepository = FakeWatchZoneRepository(),
    ): WatchZoneEditorViewModel {
        val geocoder = FakePostcodeGeocoder(Result.success(aCoordinate()))
        val viewModel = WatchZoneEditorViewModel(geocoder, repository, tier)
        viewModel.updateName("Home")
        viewModel.updatePostcode("CB1 2AD")
        viewModel.submitPostcode()
        return viewModel
    }

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
    fun `a free tier editor exposes no path to custom shape mode`() {
        val viewModel = makeViewModel(SubscriptionTier.FREE)
        assertFalse(viewModel.uiState.value.allowsCustomBoundary)

        viewModel.selectShapeMode(WatchZoneShapeMode.CUSTOM)

        assertEquals(WatchZoneShapeMode.CIRCLE, viewModel.uiState.value.shapeMode)
    }

    @Test
    fun `a personal tier editor can select custom shape mode`() {
        val viewModel = makeViewModel(SubscriptionTier.PERSONAL)
        assertTrue(viewModel.uiState.value.allowsCustomBoundary)

        viewModel.selectShapeMode(WatchZoneShapeMode.CUSTOM)

        assertEquals(WatchZoneShapeMode.CUSTOM, viewModel.uiState.value.shapeMode)
    }

    @Test
    fun `a pro tier editor can select custom shape mode`() {
        val viewModel = makeViewModel(SubscriptionTier.PRO)
        assertTrue(viewModel.uiState.value.allowsCustomBoundary)
    }

    @Test
    fun `switching back to circle mode after drawing keeps the drawn vertices`() {
        val viewModel = makeViewModel(SubscriptionTier.PERSONAL)
        viewModel.selectShapeMode(WatchZoneShapeMode.CUSTOM)
        triangle.forEach(viewModel::addPolygonVertex)

        viewModel.selectShapeMode(WatchZoneShapeMode.CIRCLE)

        assertEquals(triangle, viewModel.uiState.value.polygonVertices)
    }

    // MARK: - Adding vertices

    @Test
    fun `adding vertices accumulates them in tap order`() {
        val viewModel = makeViewModel(SubscriptionTier.PERSONAL)
        viewModel.selectShapeMode(WatchZoneShapeMode.CUSTOM)

        triangle.forEach(viewModel::addPolygonVertex)

        assertEquals(triangle, viewModel.uiState.value.polygonVertices)
    }

    @Test
    fun `a vertex tapped after the polygon is closed is ignored`() {
        val viewModel = makeViewModel(SubscriptionTier.PERSONAL)
        viewModel.selectShapeMode(WatchZoneShapeMode.CUSTOM)
        triangle.forEach(viewModel::addPolygonVertex)
        viewModel.closePolygon()

        viewModel.addPolygonVertex(aCoordinate(latitude = 51.52))

        assertEquals(triangle, viewModel.uiState.value.polygonVertices)
    }

    // MARK: - Closing (minimum 3 vertices)

    @Test
    fun `closing is not allowed below 3 vertices`() {
        val viewModel = makeViewModel(SubscriptionTier.PERSONAL)
        viewModel.selectShapeMode(WatchZoneShapeMode.CUSTOM)
        viewModel.addPolygonVertex(triangle[0])
        viewModel.addPolygonVertex(triangle[1])
        assertFalse(viewModel.uiState.value.canClosePolygon)

        viewModel.closePolygon()

        assertFalse(viewModel.uiState.value.isPolygonClosed)
    }

    @Test
    fun `closing is allowed once 3 vertices are drawn`() {
        val viewModel = makeViewModel(SubscriptionTier.PERSONAL)
        viewModel.selectShapeMode(WatchZoneShapeMode.CUSTOM)
        triangle.forEach(viewModel::addPolygonVertex)
        assertTrue(viewModel.uiState.value.canClosePolygon)

        viewModel.closePolygon()

        assertTrue(viewModel.uiState.value.isPolygonClosed)
        assertFalse(viewModel.uiState.value.boundaryError)
    }

    @Test
    fun `closing a self-intersecting shape surfaces a boundary error and stays open`() {
        val viewModel = makeViewModel(SubscriptionTier.PERSONAL)
        viewModel.selectShapeMode(WatchZoneShapeMode.CUSTOM)
        bowtie.forEach(viewModel::addPolygonVertex)

        viewModel.closePolygon()

        assertFalse(viewModel.uiState.value.isPolygonClosed)
        assertTrue(viewModel.uiState.value.boundaryError)
    }

    // MARK: - Dragging (moving) a vertex

    @Test
    fun `moving a vertex replaces its position without changing the vertex count`() {
        val viewModel = makeViewModel(SubscriptionTier.PERSONAL)
        viewModel.selectShapeMode(WatchZoneShapeMode.CUSTOM)
        triangle.forEach(viewModel::addPolygonVertex)
        val moved = Coordinate(latitude = 51.505, longitude = -0.095)

        viewModel.movePolygonVertex(1, moved)

        val vertices = viewModel.uiState.value.polygonVertices
        assertEquals(3, vertices.size)
        assertEquals(moved, vertices[1])
    }

    @Test
    fun `moving an out-of-range vertex index is ignored`() {
        val viewModel = makeViewModel(SubscriptionTier.PERSONAL)
        viewModel.selectShapeMode(WatchZoneShapeMode.CUSTOM)
        triangle.forEach(viewModel::addPolygonVertex)

        viewModel.movePolygonVertex(5, aCoordinate())

        assertEquals(triangle, viewModel.uiState.value.polygonVertices)
    }

    // MARK: - Undo

    @Test
    fun `undo removes the last added vertex`() {
        val viewModel = makeViewModel(SubscriptionTier.PERSONAL)
        viewModel.selectShapeMode(WatchZoneShapeMode.CUSTOM)
        triangle.forEach(viewModel::addPolygonVertex)

        viewModel.undoLastPolygonAction()

        assertEquals(triangle.dropLast(1), viewModel.uiState.value.polygonVertices)
    }

    @Test
    fun `undo on a closed polygon reopens it without removing a vertex`() {
        val viewModel = makeViewModel(SubscriptionTier.PERSONAL)
        viewModel.selectShapeMode(WatchZoneShapeMode.CUSTOM)
        triangle.forEach(viewModel::addPolygonVertex)
        viewModel.closePolygon()

        viewModel.undoLastPolygonAction()

        assertFalse(viewModel.uiState.value.isPolygonClosed)
        assertEquals(triangle, viewModel.uiState.value.polygonVertices)
    }

    @Test
    fun `undo on an empty polygon does nothing`() {
        val viewModel = makeViewModel(SubscriptionTier.PERSONAL)
        viewModel.selectShapeMode(WatchZoneShapeMode.CUSTOM)

        viewModel.undoLastPolygonAction()

        assertTrue(
            viewModel.uiState.value.polygonVertices
                .isEmpty(),
        )
    }

    // MARK: - Save enablement

    @Test
    fun `save is disabled in custom mode until the polygon is closed`() {
        val viewModel = geocodedViewModel(SubscriptionTier.PERSONAL)
        viewModel.selectShapeMode(WatchZoneShapeMode.CUSTOM)
        assertFalse(viewModel.uiState.value.isSaveEnabled)

        triangle.forEach(viewModel::addPolygonVertex)
        assertFalse(viewModel.uiState.value.isSaveEnabled)

        viewModel.closePolygon()
        assertTrue(viewModel.uiState.value.isSaveEnabled)
    }

    @Test
    fun `saving with an unclosed custom shape does nothing`() {
        val repository = FakeWatchZoneRepository()
        val viewModel = geocodedViewModel(SubscriptionTier.PERSONAL, repository)
        viewModel.selectShapeMode(WatchZoneShapeMode.CUSTOM)
        triangle.forEach(viewModel::addPolygonVertex)

        viewModel.save()

        assertTrue(repository.createCalls.isEmpty())
    }

    @Test
    fun `saving a closed custom shape sends the drawn boundary`() {
        val repository = FakeWatchZoneRepository()
        val viewModel = geocodedViewModel(SubscriptionTier.PERSONAL, repository)
        viewModel.selectShapeMode(WatchZoneShapeMode.CUSTOM)
        triangle.forEach(viewModel::addPolygonVertex)
        viewModel.closePolygon()

        viewModel.save()

        val expectedBoundary = (WatchZoneBoundary.of(triangle) as WatchZoneBoundaryResult.Valid).boundary
        val saved = repository.createCalls.single()
        assertEquals(expectedBoundary, saved.boundary)
        assertEquals(expectedBoundary.centroid, saved.centre)
        assertEquals(expectedBoundary.enclosingRadiusMetres, saved.radiusMetres)
        assertTrue(viewModel.uiState.value.saveCompleted)
    }

    @Test
    fun `saving a circle mode zone sends no boundary`() {
        val repository = FakeWatchZoneRepository()
        val viewModel = geocodedViewModel(SubscriptionTier.PERSONAL, repository)

        viewModel.save()

        assertNull(repository.createCalls.single().boundary)
    }

    // MARK: - Editing an existing custom-shape zone

    @Test
    fun `editing an existing custom-shape zone pre-fills the drawn vertices and shape mode`() {
        val boundary = (WatchZoneBoundary.of(triangle) as WatchZoneBoundaryResult.Valid).boundary
        val zone = aWatchZone(boundary = boundary)
        val viewModel = makeViewModel(SubscriptionTier.PERSONAL, editingZone = zone)

        val state = viewModel.uiState.value
        assertEquals(WatchZoneShapeMode.CUSTOM, state.shapeMode)
        assertTrue(state.isPolygonClosed)
        assertEquals(triangle, state.polygonVertices)
    }

    @Test
    fun `editing an existing circle zone starts in circle mode with no vertices`() {
        val zone = aWatchZone()
        val viewModel = makeViewModel(SubscriptionTier.PERSONAL, editingZone = zone)

        val state = viewModel.uiState.value
        assertEquals(WatchZoneShapeMode.CIRCLE, state.shapeMode)
        assertFalse(state.isPolygonClosed)
        assertTrue(state.polygonVertices.isEmpty())
    }
}
