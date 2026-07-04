package uk.towncrierapp.presentation.watchzones

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
) {
    public companion object {
        public const val MIN_RADIUS_METRES: Float = 100f
        public const val DEFAULT_RADIUS_METRES: Float = 1_000f

        /**
         * Threshold (metres) at or above which the large-radius warning shows.
         * Set just above the free tier's 2 km cap so a free user pinned at
         * their maximum never trips it — only paid tiers, which can exceed
         * 2 km, see it. Port of iOS `LargeRadiusWarning.thresholdMetres`.
         */
        public const val LARGE_RADIUS_WARNING_THRESHOLD_METRES: Float = 2_100f
    }
}
