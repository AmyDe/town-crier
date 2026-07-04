package uk.towncrierapp.presentation.watchzones

import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.subscriptions.FeatureGate
import uk.towncrierapp.domain.subscriptions.SubscriptionTier

/**
 * `ZonePreferencesScreen` state: four per-channel toggles (push/email ×
 * new-application/decision), all defaulting to `true` so a not-yet-loaded
 * screen never flashes an "everything off" state. Port of iOS
 * `ZonePreferencesViewModel`'s published state.
 */
public data class ZonePreferencesUiState(
    val zoneName: String,
    val newApplicationPush: Boolean = true,
    val newApplicationEmail: Boolean = true,
    val decisionPush: Boolean = true,
    val decisionEmail: Boolean = true,
    val isLoading: Boolean = false,
    val error: DomainError? = null,
    val featureGate: FeatureGate = FeatureGate(SubscriptionTier.FREE),
    val saveCompleted: Boolean = false,
)
