package uk.towncrierapp.presentation.features.onboarding

import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.domain.watchzones.Postcode
import uk.towncrierapp.domain.watchzones.WatchZone
import uk.towncrierapp.presentation.features.watchzones.WatchZoneShapeMode

/**
 * `OnboardingScreen` state. [isComplete] is a one-shot signal the Route
 * reconciles (coroutines-and-flow.md "One-shot effects") by leaving the
 * wizard for the main app - the ViewModel itself never navigates. Port of
 * iOS `OnboardingViewModel`'s published state.
 */
public data class OnboardingUiState(
    val step: OnboardingStep = OnboardingStep.Welcome,
    val tier: SubscriptionTier = SubscriptionTier.FREE,
    val postcodeInput: String = "",
    val isLookingUpPostcode: Boolean = false,
    val resolvedPostcode: Postcode? = null,
    val geocodedCoordinate: Coordinate? = null,
    val postcodeError: DomainError? = null,
    val radiusMetres: Float = DEFAULT_RADIUS_METRES,
    val minRadiusMetres: Float = MIN_RADIUS_METRES,
    val maxRadiusMetres: Float = DEFAULT_RADIUS_METRES,
    val showsLargeRadiusWarning: Boolean = false,
    val canUnlockLargerRadius: Boolean = false,
    val pendingZone: WatchZone? = null,
    val hasInstantAlertEntitlement: Boolean = false,
    val isCompleting: Boolean = false,
    val isComplete: Boolean = false,
    // GH#1072 Phase 5 (tc-v6fo0.5): custom-shape (polygon) drawing, mirroring
    // WatchZoneEditorUiState's equivalent fields (see PolygonDrawingState,
    // shared between the two so the vertex-add/move/close/undo rules aren't
    // duplicated). [allowsCustomBoundary] mirrors WatchZoneLimits.allowsCustomBoundary
    // for the CURRENT [tier] - kept in step with [tier] wherever that field
    // changes (initialState, reconcileTierAfterUpgrade). [navigateToPaywall]
    // is a one-shot signal, same shape as
    // WatchZoneEditorUiState.navigateToPaywall, for the free-tier upsell CTA
    // on the radius step - there is no paywall to route to yet (#783), so it
    // is consumed as a no-op until that ships.
    val allowsCustomBoundary: Boolean = false,
    val shapeMode: WatchZoneShapeMode = WatchZoneShapeMode.CIRCLE,
    val polygonVertices: List<Coordinate> = emptyList(),
    val isPolygonClosed: Boolean = false,
    val canClosePolygon: Boolean = false,
    val boundaryError: Boolean = false,
    val navigateToPaywall: Boolean = false,
) {
    public companion object {
        public const val MIN_RADIUS_METRES: Float = 100f
        public const val DEFAULT_RADIUS_METRES: Float = 1_000f
    }
}
