package uk.towncrierapp.presentation.features.applicationlist

import uk.towncrierapp.domain.applications.ApplicationFilter
import uk.towncrierapp.domain.applications.ApplicationSortOrder
import uk.towncrierapp.domain.applications.PlanningApplication
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.watchzones.WatchZone
import uk.towncrierapp.domain.watchzones.WatchZoneId

/**
 * [unreadCount] is derived from the loaded [applications], never the server `totalUnreadCount` (which drives
 * the OS badge only). [availableSorts] hides [ApplicationSortOrder.DISTANCE] whenever no zone is active.
 */
public data class ApplicationListUiState(
    val zones: List<WatchZone> = emptyList(),
    val selectedZoneId: WatchZoneId? = null,
    val applications: List<PlanningApplication> = emptyList(),
    val filter: ApplicationFilter = ApplicationFilter.All,
    val sort: ApplicationSortOrder = ApplicationSortOrder.DEFAULT,
    val nextCursor: String? = null,
    val isLoading: Boolean = false,
    val isLoadingMore: Boolean = false,
    val error: DomainError? = null,
) {
    public val hasMore: Boolean get() = nextCursor != null

    public val unreadCount: Int get() = applications.count { it.latestUnreadEvent != null }

    public val availableSorts: List<ApplicationSortOrder>
        get() = ApplicationSortOrder.entries.filter { it != ApplicationSortOrder.DISTANCE || selectedZoneId != null }
}
