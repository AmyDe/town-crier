package uk.towncrierapp.domain.applications

import uk.towncrierapp.domain.watchzones.WatchZoneId

/** Hand-written fake for [ApplicationListPreferencesStore] — state-based, per testing.md conventions. */
public class FakeApplicationListPreferencesStore(
    public var storedSort: ApplicationSortOrder? = null,
    public var storedZoneId: WatchZoneId? = null,
) : ApplicationListPreferencesStore {
    override suspend fun readSort(): ApplicationSortOrder? = storedSort

    override suspend fun writeSort(sort: ApplicationSortOrder) {
        storedSort = sort
    }

    override suspend fun readLastSelectedZoneId(): WatchZoneId? = storedZoneId

    override suspend fun writeLastSelectedZoneId(zoneId: WatchZoneId) {
        storedZoneId = zoneId
    }
}
