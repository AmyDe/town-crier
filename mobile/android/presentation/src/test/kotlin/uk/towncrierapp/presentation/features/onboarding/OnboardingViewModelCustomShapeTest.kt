package uk.towncrierapp.presentation.features.onboarding

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import uk.towncrierapp.domain.onboarding.FakeOnboardingRepository
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.domain.watchzones.FakePostcodeGeocoder
import uk.towncrierapp.domain.watchzones.FakeWatchZoneRepository
import uk.towncrierapp.domain.watchzones.WatchZoneBoundary
import uk.towncrierapp.domain.watchzones.WatchZoneBoundaryResult
import uk.towncrierapp.domain.watchzones.aCoordinate
import uk.towncrierapp.presentation.MainDispatcherExtension
import uk.towncrierapp.presentation.features.watchzones.WatchZoneShapeEvent
import uk.towncrierapp.presentation.features.watchzones.WatchZoneShapeMode
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

// Thin wrappers over the single OnboardingViewModel.onShapeEvent entry point,
// same rationale as WatchZoneEditorViewModelShapeModeTest's equivalents.
private fun OnboardingViewModel.addPolygonVertex(coordinate: Coordinate) =
    onShapeEvent(WatchZoneShapeEvent.VertexAdded(coordinate))

private fun OnboardingViewModel.closePolygon() = onShapeEvent(WatchZoneShapeEvent.PolygonClosed)

/**
 * Onboarding's custom-shape (polygon) drawing step (GH#1072 Phase 5,
 * tc-v6fo0.5): the already-entitled re-entry point ([OnboardingViewModel.selectCustomShape]),
 * the mid-flow purchase swap ([OnboardingViewModel.reconcileTierAfterCustomShapeUpgrade]),
 * boundary confirmation ([OnboardingViewModel.confirmBoundary]), and
 * back-navigation routing to whichever of Radius/BoundaryDrawing the
 * in-progress zone came from. Port of the equivalent iOS
 * `OnboardingViewModelTests` cases (tc-6he3x.10).
 */
@ExtendWith(MainDispatcherExtension::class)
class OnboardingViewModelCustomShapeTest {
    private val triangle =
        listOf(
            Coordinate(latitude = 51.50, longitude = -0.10),
            Coordinate(latitude = 51.51, longitude = -0.09),
            Coordinate(latitude = 51.50, longitude = -0.09),
        )

    private fun aViewModelOnRadiusStep(tier: SubscriptionTier): OnboardingViewModel {
        val viewModel =
            OnboardingViewModel(
                FakePostcodeGeocoder(Result.success(aCoordinate())),
                FakeWatchZoneRepository(),
                FakeOnboardingRepository(),
                tier,
            )
        viewModel.advance() // Welcome -> Postcode
        viewModel.updatePostcode("SW1A 1AA")
        viewModel.lookUpPostcode()
        viewModel.advance() // Postcode -> Radius
        return viewModel
    }

    private fun aViewModelOnBoundaryDrawingStep(
        tier: SubscriptionTier = SubscriptionTier.PERSONAL,
    ): OnboardingViewModel {
        val viewModel = aViewModelOnRadiusStep(tier)
        viewModel.selectCustomShape()
        return viewModel
    }

    // MARK: - allowsCustomBoundary tracks tier

    @Test
    fun `allowsCustomBoundary is false for a fresh Free-tier wizard`() {
        assertFalse(aViewModelOnRadiusStep(SubscriptionTier.FREE).uiState.value.allowsCustomBoundary)
    }

    @Test
    fun `allowsCustomBoundary is true for a fresh Personal or Pro wizard`() {
        assertTrue(aViewModelOnRadiusStep(SubscriptionTier.PERSONAL).uiState.value.allowsCustomBoundary)
        assertTrue(aViewModelOnRadiusStep(SubscriptionTier.PRO).uiState.value.allowsCustomBoundary)
    }

    // MARK: - selectCustomShape (already-entitled re-entry)

    @Test
    fun `selectCustomShape switches to BoundaryDrawing in custom mode when entitled on the radius step`() {
        val viewModel = aViewModelOnRadiusStep(SubscriptionTier.PERSONAL)

        viewModel.selectCustomShape()

        val state = viewModel.uiState.value
        assertEquals(OnboardingStep.BoundaryDrawing, state.step)
        assertEquals(WatchZoneShapeMode.CUSTOM, state.shapeMode)
    }

    @Test
    fun `selectCustomShape is a no-op for a Free-tier user`() {
        val viewModel = aViewModelOnRadiusStep(SubscriptionTier.FREE)

        viewModel.selectCustomShape()

        assertEquals(OnboardingStep.Radius, viewModel.uiState.value.step)
    }

    @Test
    fun `selectCustomShape is a no-op when not on the radius step`() {
        val viewModel =
            OnboardingViewModel(
                FakePostcodeGeocoder(Result.success(aCoordinate())),
                FakeWatchZoneRepository(),
                FakeOnboardingRepository(),
                SubscriptionTier.PERSONAL,
            )
        viewModel.advance() // Welcome -> Postcode

        viewModel.selectCustomShape()

        assertEquals(OnboardingStep.Postcode, viewModel.uiState.value.step)
    }

    // MARK: - reconcileTierAfterCustomShapeUpgrade (mid-flow purchase swap)

    @Test
    fun `reconcileTierAfterCustomShapeUpgrade swaps Radius for BoundaryDrawing when landed on the radius step`() {
        val viewModel = aViewModelOnRadiusStep(SubscriptionTier.FREE)

        viewModel.reconcileTierAfterCustomShapeUpgrade(SubscriptionTier.PERSONAL)

        val state = viewModel.uiState.value
        assertEquals(OnboardingStep.BoundaryDrawing, state.step)
        assertEquals(WatchZoneShapeMode.CUSTOM, state.shapeMode)
        assertTrue(state.allowsCustomBoundary)
    }

    @Test
    fun `reconcileTierAfterCustomShapeUpgrade does not swap the step when not on the radius step`() {
        val viewModel = aViewModelOnRadiusStep(SubscriptionTier.FREE)
        viewModel.confirmRadius() // Radius -> NotificationPermission

        viewModel.reconcileTierAfterCustomShapeUpgrade(SubscriptionTier.PERSONAL)

        assertEquals(OnboardingStep.NotificationPermission, viewModel.uiState.value.step)
    }

    @Test
    fun `reconcileTierAfterCustomShapeUpgrade does not swap the step when the new tier still isn't entitled`() {
        val viewModel = aViewModelOnRadiusStep(SubscriptionTier.FREE)

        viewModel.reconcileTierAfterCustomShapeUpgrade(SubscriptionTier.FREE)

        assertEquals(OnboardingStep.Radius, viewModel.uiState.value.step)
        assertEquals(WatchZoneShapeMode.CIRCLE, viewModel.uiState.value.shapeMode)
    }

    @Test
    fun `reconcileTierAfterCustomShapeUpgrade preserves postcode, coordinate, and radius`() {
        val viewModel = aViewModelOnRadiusStep(SubscriptionTier.FREE)
        viewModel.updateRadius(1_800f)
        val beforeCoordinate = viewModel.uiState.value.geocodedCoordinate
        val beforePostcode = viewModel.uiState.value.postcodeInput

        viewModel.reconcileTierAfterCustomShapeUpgrade(SubscriptionTier.PERSONAL)

        val state = viewModel.uiState.value
        assertEquals(1_800f, state.radiusMetres)
        assertEquals(beforeCoordinate, state.geocodedCoordinate)
        assertEquals(beforePostcode, state.postcodeInput)
    }

    // MARK: - onShapeEvent wiring

    @Test
    fun `onShapeEvent accumulates vertices while drawing`() {
        val viewModel = aViewModelOnBoundaryDrawingStep()

        triangle.forEach(viewModel::addPolygonVertex)

        assertEquals(triangle, viewModel.uiState.value.polygonVertices)
    }

    @Test
    fun `onShapeEvent closes the polygon once 3 vertices are drawn`() {
        val viewModel = aViewModelOnBoundaryDrawingStep()
        triangle.forEach(viewModel::addPolygonVertex)

        viewModel.closePolygon()

        assertTrue(viewModel.uiState.value.isPolygonClosed)
    }

    // MARK: - confirmBoundary

    @Test
    fun `confirmBoundary does nothing while the polygon isn't closed`() {
        val repository = FakeWatchZoneRepository()
        val viewModel =
            OnboardingViewModel(
                FakePostcodeGeocoder(Result.success(aCoordinate())),
                repository,
                FakeOnboardingRepository(),
                SubscriptionTier.PERSONAL,
            )
        viewModel.advance()
        viewModel.updatePostcode("SW1A 1AA")
        viewModel.lookUpPostcode()
        viewModel.advance()
        viewModel.selectCustomShape()
        triangle.forEach(viewModel::addPolygonVertex)

        viewModel.confirmBoundary()

        assertEquals(OnboardingStep.BoundaryDrawing, viewModel.uiState.value.step)
        assertNull(viewModel.uiState.value.pendingZone)
        assertTrue(repository.createCalls.isEmpty())
    }

    @Test
    fun `confirmBoundary builds the zone from the boundary's centroid and enclosing radius`() {
        val viewModel = aViewModelOnBoundaryDrawingStep()
        triangle.forEach(viewModel::addPolygonVertex)
        viewModel.closePolygon()

        viewModel.confirmBoundary()

        val expectedBoundary = (WatchZoneBoundary.of(triangle) as WatchZoneBoundaryResult.Valid).boundary
        val zone = viewModel.uiState.value.pendingZone
        assertEquals(expectedBoundary, zone?.boundary)
        assertEquals(expectedBoundary.centroid, zone?.centre)
        assertEquals(expectedBoundary.enclosingRadiusMetres, zone?.radiusMetres)
    }

    @Test
    fun `confirmBoundary advances to the notification-permission step without saving`() {
        val repository = FakeWatchZoneRepository()
        val viewModel =
            OnboardingViewModel(
                FakePostcodeGeocoder(Result.success(aCoordinate())),
                repository,
                FakeOnboardingRepository(),
                SubscriptionTier.PERSONAL,
            )
        viewModel.advance()
        viewModel.updatePostcode("SW1A 1AA")
        viewModel.lookUpPostcode()
        viewModel.advance()
        viewModel.selectCustomShape()
        triangle.forEach(viewModel::addPolygonVertex)
        viewModel.closePolygon()

        viewModel.confirmBoundary()

        assertEquals(OnboardingStep.NotificationPermission, viewModel.uiState.value.step)
        assertTrue(repository.createCalls.isEmpty())
    }

    // MARK: - Back navigation

    @Test
    fun `back from BoundaryDrawing returns to Radius and discards the drawn shape`() {
        val viewModel = aViewModelOnBoundaryDrawingStep()
        triangle.forEach(viewModel::addPolygonVertex)

        viewModel.back()

        val state = viewModel.uiState.value
        assertEquals(OnboardingStep.Radius, state.step)
        assertEquals(WatchZoneShapeMode.CIRCLE, state.shapeMode)
        assertTrue(state.polygonVertices.isEmpty())
    }

    @Test
    fun `back from NotificationPermission returns to BoundaryDrawing when the zone was built from a custom shape`() {
        val viewModel = aViewModelOnBoundaryDrawingStep()
        triangle.forEach(viewModel::addPolygonVertex)
        viewModel.closePolygon()
        viewModel.confirmBoundary()

        viewModel.back()

        val state = viewModel.uiState.value
        assertEquals(OnboardingStep.BoundaryDrawing, state.step)
        assertEquals(WatchZoneShapeMode.CUSTOM, state.shapeMode)
        assertEquals(triangle, state.polygonVertices)
    }

    @Test
    fun `back from NotificationPermission still returns to Radius when the zone was built from a circle`() {
        val viewModel = aViewModelOnRadiusStep(SubscriptionTier.FREE)
        viewModel.confirmRadius()

        viewModel.back()

        assertEquals(OnboardingStep.Radius, viewModel.uiState.value.step)
    }

    // MARK: - requestCustomShapeUpgrade / consumeNavigateToPaywall

    @Test
    fun `requestCustomShapeUpgrade sets the one-shot paywall signal`() {
        val viewModel = aViewModelOnRadiusStep(SubscriptionTier.FREE)

        viewModel.requestCustomShapeUpgrade()

        assertTrue(viewModel.uiState.value.navigateToPaywall)
    }

    @Test
    fun `consumeNavigateToPaywall resets the one-shot paywall signal`() {
        val viewModel = aViewModelOnRadiusStep(SubscriptionTier.FREE)
        viewModel.requestCustomShapeUpgrade()

        viewModel.consumeNavigateToPaywall()

        assertFalse(viewModel.uiState.value.navigateToPaywall)
    }
}
