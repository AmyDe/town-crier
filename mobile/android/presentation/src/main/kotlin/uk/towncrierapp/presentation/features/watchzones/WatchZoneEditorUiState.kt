package uk.towncrierapp.presentation.features.watchzones

import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.subscriptions.Entitlement
import uk.towncrierapp.domain.subscriptions.FeatureGate
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.watchzones.Coordinate

/**
 * `WatchZoneEditorScreen` state. [saveCompleted]/[navigateToPaywall] are
 * one-shot signals the Route reconciles (coroutines-and-flow.md "One-shot
 * effects") rather than navigating from the ViewModel: the Route observes
 * them, performs the dismiss/paywall-route side effect, then calls
 * [WatchZoneEditorViewModel.consumeSaveCompleted]/[WatchZoneEditorViewModel.consumeNavigateToPaywall]
 * to reset the flag. Port of iOS `WatchZoneEditorViewModel`'s published state.
 */
public data class WatchZoneEditorUiState(
    val isEditing: Boolean = false,
    val name: String = "",
    val postcode: String = "",
    val radiusMetres: Float = DEFAULT_RADIUS_METRES,
    val minRadiusMetres: Float = MIN_RADIUS_METRES,
    val maxRadiusMetres: Float = DEFAULT_RADIUS_METRES,
    val geocodedCoordinate: Coordinate? = null,
    val pushEnabled: Boolean = true,
    val emailInstantEnabled: Boolean = true,
    val isLoading: Boolean = false,
    val error: DomainError? = null,
    val featureGate: FeatureGate = FeatureGate(SubscriptionTier.FREE),
    val instantAlertEntitlement: Entitlement = Entitlement.STATUS_CHANGE_ALERTS,
    val canUnlockLargerRadius: Boolean = false,
    val showsLargeRadiusWarning: Boolean = false,
    val isSaveEnabled: Boolean = false,
    val saveCompleted: Boolean = false,
    val navigateToPaywall: Boolean = false,
    // GH#1072 Phase 3 (tc-v6fo0.3): custom-shape (polygon) drawing.
    // [allowsCustomBoundary] mirrors WatchZoneLimits.allowsCustomBoundary for
    // this editor's tier — the Screen never offers [WatchZoneShapeMode.CUSTOM]
    // when it's false (Free tier sees the upsell instead, never a hidden
    // toggle). [polygonVertices] holds the open ring as drawn so far (no
    // closing repeat); [isPolygonClosed] flips once the user taps the first
    // vertex with >= 3 vertices down. [canClosePolygon] is the same
    // 3-vertex gate surfaced for the UI to disable the close affordance
    // early. [boundaryError] is set when a close/save attempt fails
    // [uk.towncrierapp.domain.watchzones.WatchZoneBoundary.of]'s validation
    // (e.g. a self-intersecting shape).
    val allowsCustomBoundary: Boolean = false,
    val shapeMode: WatchZoneShapeMode = WatchZoneShapeMode.CIRCLE,
    val polygonVertices: List<Coordinate> = emptyList(),
    val isPolygonClosed: Boolean = false,
    val canClosePolygon: Boolean = false,
    val boundaryError: Boolean = false,
) {
    public companion object {
        public const val MIN_RADIUS_METRES: Float = 100f
        public const val DEFAULT_RADIUS_METRES: Float = 1_000f
    }
}
