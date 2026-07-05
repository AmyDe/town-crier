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

    /** By-id detail fetch (authed) — [authority] is the decimal area id. */
    public suspend fun detail(
        authority: String,
        name: String,
    ): PlanningApplication

    /**
     * Anonymous by-slug detail fetch, used to resolve an inbound public share
     * link (`/a/{authoritySlug}/{ref...}`, GH#782). The server route itself
     * requires no auth, but Android only ever calls it once the user is
     * signed in (no signed-out detail view day-1 — see `PendingLinkHolder`).
     * [authoritySlug] is the API-emitted slug and [ref] is the application's
     * full area-prefixed PlanIt name, verbatim (slashes preserved).
     */
    public suspend fun detailBySlug(
        authoritySlug: String,
        ref: String,
    ): PlanningApplication
}
