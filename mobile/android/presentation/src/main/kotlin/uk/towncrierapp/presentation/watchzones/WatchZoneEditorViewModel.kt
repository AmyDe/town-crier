package uk.towncrierapp.presentation.watchzones

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.subscriptions.Entitlement
import uk.towncrierapp.domain.subscriptions.FeatureGate
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.watchzones.PostcodeGeocoder
import uk.towncrierapp.domain.watchzones.WatchZone
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.domain.watchzones.WatchZoneLimits
import uk.towncrierapp.domain.watchzones.WatchZoneRepository
import java.util.UUID

/**
 * Drives create/edit of a single watch zone: postcode geocoding, tier-based
 * radius limits, and the two instant-alert toggles. Port of iOS
 * `WatchZoneEditorViewModel`. Navigation (dismiss-on-save, route-to-paywall)
 * is modelled as state the Route reconciles — see [WatchZoneEditorUiState].
 */
public class WatchZoneEditorViewModel(
    private val geocoder: PostcodeGeocoder,
    private val repository: WatchZoneRepository,
    tier: SubscriptionTier,
    private val editingZone: WatchZone? = null,
) : ViewModel() {
    private val limits = WatchZoneLimits(tier)

    private val _uiState = MutableStateFlow(initialState(tier, limits, editingZone))
    public val uiState: StateFlow<WatchZoneEditorUiState> = _uiState.asStateFlow()

    public fun updateName(value: String) {
        _uiState.update { it.copy(name = value).withSaveEnabled() }
    }

    public fun updatePostcode(value: String) {
        _uiState.update { it.copy(postcode = value) }
    }

    public fun updateRadius(value: Float) {
        _uiState.update {
            it.copy(
                radiusMetres = value,
                showsLargeRadiusWarning = value >= WatchZoneEditorUiState.LARGE_RADIUS_WARNING_THRESHOLD_METRES,
            )
        }
    }

    public fun updatePushEnabled(value: Boolean) {
        _uiState.update { it.copy(pushEnabled = value) }
    }

    public fun updateEmailInstantEnabled(value: Boolean) {
        _uiState.update { it.copy(emailInstantEnabled = value) }
    }

    /** Surfaces the in-editor subscription upsell when a free-tier user taps a locked instant-alert toggle. */
    public fun requestInstantAlertUpgrade() {
        _uiState.update { it.copy(navigateToPaywall = true) }
    }

    public fun submitPostcode() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                val coordinate = geocoder.geocode(_uiState.value.postcode)
                _uiState.update {
                    it.copy(
                        geocodedCoordinate = coordinate,
                        isLoading = false,
                        name = it.name.ifBlank { it.postcode.trim() },
                    ).withSaveEnabled()
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                _uiState.update { it.copy(isLoading = false, error = e) }
            }
        }
    }

    /**
     * Persists the zone. On a quota breach
     * ([DomainError.InsufficientEntitlement]) routes to the paywall
     * placeholder and leaves `error` unset — the Route dismisses the editor.
     * Any other failure sets the inline error and leaves the editor open.
     */
    public fun save() {
        val state = _uiState.value
        val coordinate = state.geocodedCoordinate ?: return
        viewModelScope.launch {
            _uiState.update { it.copy(error = null) }
            val zone =
                WatchZone(
                    id = editingZone?.id ?: WatchZoneId(UUID.randomUUID().toString()),
                    name = state.name.trim(),
                    centre = coordinate,
                    radiusMetres = limits.clampRadius(state.radiusMetres.toDouble()),
                    authorityId = editingZone?.authorityId ?: 0,
                    pushEnabled = state.pushEnabled,
                    emailInstantEnabled = state.emailInstantEnabled,
                )
            try {
                if (editingZone != null) repository.update(zone) else repository.create(zone)
                _uiState.update { it.copy(saveCompleted = true) }
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError.InsufficientEntitlement) {
                _uiState.update { it.copy(navigateToPaywall = true) }
            } catch (e: DomainError) {
                _uiState.update { it.copy(error = e) }
            }
        }
    }

    /** Resets the one-shot dismiss-editor signal once the Route has acted on it. */
    public fun consumeSaveCompleted() {
        _uiState.update { it.copy(saveCompleted = false) }
    }

    /** Resets the one-shot route-to-paywall signal once the Route has acted on it. */
    public fun consumeNavigateToPaywall() {
        _uiState.update { it.copy(navigateToPaywall = false) }
    }

    private fun WatchZoneEditorUiState.withSaveEnabled(): WatchZoneEditorUiState =
        copy(isSaveEnabled = geocodedCoordinate != null && name.isNotBlank())
}

private fun initialState(
    tier: SubscriptionTier,
    limits: WatchZoneLimits,
    editingZone: WatchZone?,
): WatchZoneEditorUiState {
    val radius = (editingZone?.radiusMetres?.toFloat() ?: WatchZoneEditorUiState.DEFAULT_RADIUS_METRES)
    return WatchZoneEditorUiState(
        isEditing = editingZone != null,
        name = editingZone?.name.orEmpty(),
        radiusMetres = radius,
        maxRadiusMetres = limits.maxRadiusMetres.toFloat(),
        geocodedCoordinate = editingZone?.centre,
        pushEnabled = editingZone?.pushEnabled ?: true,
        emailInstantEnabled = editingZone?.emailInstantEnabled ?: true,
        featureGate = FeatureGate(tier),
        instantAlertEntitlement = Entitlement.STATUS_CHANGE_ALERTS,
        canUnlockLargerRadius = tier < SubscriptionTier.PRO,
        showsLargeRadiusWarning = radius >= WatchZoneEditorUiState.LARGE_RADIUS_WARNING_THRESHOLD_METRES,
        isSaveEnabled = editingZone != null,
    )
}
