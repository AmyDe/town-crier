package uk.towncrierapp.presentation.features.applicationlist

import uk.towncrierapp.domain.applications.ApplicationFilter
import uk.towncrierapp.domain.applications.ApplicationSortOrder
import uk.towncrierapp.domain.applications.PlanningApplication
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.domain.watchzones.WatchZone

/**
 * `ApplicationListScreen` state. [unreadCount] is CLIENT-derived from
 * currently-loaded [applications] (never a server `totalUnreadCount` — that
 * field drives the OS badge only, #777 D7). [hasMore] is a page's
 * `nextCursor != null`. [availableSorts] hides
 * [ApplicationSortOrder.DISTANCE] whenever no zone is active (nothing to be
 * distant from). Port of iOS `ApplicationListViewModel`'s published state
 * (GH#775).
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
