package uk.towncrierapp.presentation.features.watchzones

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.subscriptions.FeatureGate
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.domain.watchzones.ZoneNotificationPreferences
import uk.towncrierapp.domain.watchzones.ZonePreferencesRepository

/**
 * Drives the per-zone notification preferences screen: `GET`/`PUT
 * /v1/me/watch-zones/{zoneId}/preferences`. All four toggles default to
 * `true`; the free-tier downgrade is applied server-side at dispatch time,
 * so the same controls render for every tier — [featureGate] lets the Screen
 * decide whether to lock them. Port of iOS `ZonePreferencesViewModel`.
 */
public class ZonePreferencesViewModel(
    private val repository: ZonePreferencesRepository,
    private val zoneId: WatchZoneId,
    zoneName: String,
    tier: SubscriptionTier,
) : ViewModel() {
    private val _uiState = MutableStateFlow(ZonePreferencesUiState(zoneName = zoneName, featureGate = FeatureGate(tier)))
    public val uiState: StateFlow<ZonePreferencesUiState> = _uiState.asStateFlow()

    public fun load() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                val prefs = repository.fetchPreferences(zoneId)
                _uiState.update {
                    it.copy(
                        isLoading = false,
                        newApplicationPush = prefs.newApplicationPush,
                        newApplicationEmail = prefs.newApplicationEmail,
                        decisionPush = prefs.decisionPush,
                        decisionEmail = prefs.decisionEmail,
                    )
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                _uiState.update { it.copy(isLoading = false, error = e) }
            }
        }
    }

    public fun updateNewApplicationPush(value: Boolean) {
        _uiState.update { it.copy(newApplicationPush = value) }
    }

    public fun updateNewApplicationEmail(value: Boolean) {
        _uiState.update { it.copy(newApplicationEmail = value) }
    }

    public fun updateDecisionPush(value: Boolean) {
        _uiState.update { it.copy(decisionPush = value) }
    }

    public fun updateDecisionEmail(value: Boolean) {
        _uiState.update { it.copy(decisionEmail = value) }
    }

    public fun save() {
        viewModelScope.launch {
            _uiState.update { it.copy(error = null) }
            val state = _uiState.value
            val prefs =
                ZoneNotificationPreferences(
                    zoneId = zoneId,
                    newApplicationPush = state.newApplicationPush,
                    newApplicationEmail = state.newApplicationEmail,
                    decisionPush = state.decisionPush,
                    decisionEmail = state.decisionEmail,
                )
            try {
                repository.updatePreferences(prefs)
                _uiState.update { it.copy(saveCompleted = true) }
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                _uiState.update { it.copy(error = e) }
            }
        }
    }

    /** Resets the one-shot dismiss-screen signal once the Route has acted on it. */
    public fun consumeSaveCompleted() {
        _uiState.update { it.copy(saveCompleted = false) }
    }
}
