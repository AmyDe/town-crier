package uk.towncrierapp.domain.map

import uk.towncrierapp.domain.watchzones.WatchZoneId

/** Hand-written fake for [MapPreferencesStore] — state-based, per testing.md conventions. */
public class FakeMapPreferencesStore(
    public var storedZoneId: WatchZoneId? = null,
) : MapPreferencesStore {
    override suspend fun readLastSelectedZoneId(): WatchZoneId? = storedZoneId

    override suspend fun writeLastSelectedZoneId(zoneId: WatchZoneId) {
        storedZoneId = zoneId
    }
}
