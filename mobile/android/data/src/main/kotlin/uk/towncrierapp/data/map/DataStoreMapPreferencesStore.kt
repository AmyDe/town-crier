package uk.towncrierapp.data.map

import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import uk.towncrierapp.domain.map.MapPreferencesStore
import uk.towncrierapp.domain.watchzones.WatchZoneId

// Key name reused verbatim from iOS UserDefaults (epic #770 pre-resolved
// decision) — distinct from the applications list's own
// "lastSelectedZone.applications" (DataStoreApplicationListPreferencesStore).
private val LAST_SELECTED_ZONE_KEY = stringPreferencesKey("lastSelectedZone.map")

/** DataStore Preferences-backed [MapPreferencesStore]. Port of iOS's `UserDefaults`-backed equivalent (GH#776). */
public class DataStoreMapPreferencesStore(
    private val dataStore: DataStore<Preferences>,
) : MapPreferencesStore {
    override suspend fun readLastSelectedZoneId(): WatchZoneId? =
        dataStore.data
            .map { preferences -> preferences[LAST_SELECTED_ZONE_KEY]?.let(::WatchZoneId) }
            .first()

    override suspend fun writeLastSelectedZoneId(zoneId: WatchZoneId) {
        dataStore.edit { preferences -> preferences[LAST_SELECTED_ZONE_KEY] = zoneId.value }
    }
}
