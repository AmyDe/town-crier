package uk.towncrierapp.domain.applications

import uk.towncrierapp.domain.watchzones.WatchZoneId

/**
 * Browses a zone's planning applications and fetches a single application's
 * detail. Port of iOS `ApplicationRepository`.
 */
public interface PlanningApplicationRepository {
    /**
     * A page of [zoneId]'s applications under [sort]/[filter]. `cursor ==
     * null` requests the first page — the ONLY shape
     * [OfflineAwareRepository] caches; a non-null [cursor] (a continuation
     * fetch) always bypasses the cache entirely.
     */
    public suspend fun applications(
        zoneId: WatchZoneId,
        sort: ApplicationSortOrder,
        filter: ApplicationFilter = ApplicationFilter.All,
        cursor: String? = null,
    ): ApplicationPage

    /** By-id or by-slug detail fetch — [authority] may be either the decimal area id or the authority slug. */
    public suspend fun detail(
        authority: String,
        name: String,
    ): PlanningApplication
}
