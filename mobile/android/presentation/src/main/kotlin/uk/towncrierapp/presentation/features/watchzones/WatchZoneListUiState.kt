package uk.towncrierapp.presentation.features.watchzones

import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.watchzones.WatchZone

/**
 * `WatchZoneListScreen` state. [canAddZone]/[showUpgradeBadge]/
 * [showsFreeTierUpsell] are derived from the [uk.towncrierapp.domain.subscriptions.FeatureGate]
 * every time [zones] changes, so the Screen renders as a pure function of
 * this one snapshot (compose-ui.md UDF) — port of iOS
 * `WatchZoneListViewModel`'s computed properties.
 */
public data class WatchZoneListUiState(
    val zones: List<WatchZone> = emptyList(),
    val isLoading: Boolean = false,
    val error: DomainError? = null,
    val canAddZone: Boolean = true,
    val showUpgradeBadge: Boolean = false,
    val showsFreeTierUpsell: Boolean = false,
)
