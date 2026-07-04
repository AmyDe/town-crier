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
import uk.towncrierapp.domain.subscriptions.Quota
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.watchzones.WatchZone
import uk.towncrierapp.domain.watchzones.WatchZoneRepository

/**
 * Lists the user's watch zones with proactive tier-based gating. [featureGate]
 * is injected at construction time from the session's subscription tier
 * (the NavHost recreates this ViewModel — keyed on tier — whenever it
 * changes, so the badge/upsell state never goes stale; see
 * `WatchZonesRoute`). Port of iOS `WatchZoneListViewModel`.
 */
public class WatchZoneListViewModel(
    private val repository: WatchZoneRepository,
    public val featureGate: FeatureGate,
) : ViewModel() {
    private val _uiState = MutableStateFlow(WatchZoneListUiState().deriveFromGate())
    public val uiState: StateFlow<WatchZoneListUiState> = _uiState.asStateFlow()

    public fun load() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                val zones = repository.zones()
                _uiState.update { it.copy(zones = zones, isLoading = false, error = null).deriveFromGate() }
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                _uiState.update { it.copy(isLoading = false, error = e) }
            }
        }
    }

    public fun deleteZone(zone: WatchZone) {
        viewModelScope.launch {
            _uiState.update { it.copy(error = null) }
            try {
                repository.delete(zone.id)
                _uiState.update { state ->
                    state.copy(zones = state.zones.filterNot { it.id == zone.id }).deriveFromGate()
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                _uiState.update { it.copy(error = e) }
            }
        }
    }

    private fun WatchZoneListUiState.deriveFromGate(): WatchZoneListUiState =
        copy(
            canAddZone = featureGate.canAdd(Quota.WATCH_ZONES, zones.size),
            showUpgradeBadge = featureGate.shouldShowUpgradeBadge(Quota.WATCH_ZONES, zones.size),
            // Single source of truth for the richer free-tier inline upsell card
            // (tc-t8hc): true only for a free-tier user pinned at their one-zone
            // cap. A Personal user at their finite 3-zone cap also trips
            // showUpgradeBadge, which is why this must not simply piggyback on
            // it — see WatchZoneListViewModelTest.
            showsFreeTierUpsell =
                featureGate.tier == SubscriptionTier.FREE &&
                    !featureGate.canAdd(
                        Quota.WATCH_ZONES,
                        zones.size,
                    ),
        )
}
