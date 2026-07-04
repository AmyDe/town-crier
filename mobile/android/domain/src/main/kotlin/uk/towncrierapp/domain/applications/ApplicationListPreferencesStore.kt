package uk.towncrierapp.domain.applications

import uk.towncrierapp.domain.watchzones.WatchZoneId

/**
 * The two applications-list device latches (epic #770 iOS-key parity):
 * `applicationsListSort` and `lastSelectedZone.applications`. A one-shot
 * suspend read/write, not a `Flow` — this is the cold-start restore value,
 * not something the UI observes live (mirrors
 * [uk.towncrierapp.domain.subscriptions.SubscriptionTierCache]'s shape).
 */
public interface ApplicationListPreferencesStore {
    public suspend fun readSort(): ApplicationSortOrder?

    public suspend fun writeSort(sort: ApplicationSortOrder)

    public suspend fun readLastSelectedZoneId(): WatchZoneId?

    public suspend fun writeLastSelectedZoneId(zoneId: WatchZoneId)
}
