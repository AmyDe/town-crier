package uk.towncrierapp.data.applications

import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import uk.towncrierapp.domain.applications.ApplicationListPreferencesStore
import uk.towncrierapp.domain.applications.ApplicationSortOrder
import uk.towncrierapp.domain.watchzones.WatchZoneId

// Key names reused verbatim from iOS UserDefaults (epic #770 pre-resolved
// decision) — cross-platform naming consistency, even though nothing else
// reads these keys cross-platform.
private val APPLICATIONS_LIST_SORT_KEY = stringPreferencesKey("applicationsListSort")
private val LAST_SELECTED_ZONE_KEY = stringPreferencesKey("lastSelectedZone.applications")

/** DataStore Preferences-backed [ApplicationListPreferencesStore]. Port of iOS's `UserDefaults`-backed equivalent (GH#775). */
public class DataStoreApplicationListPreferencesStore(
    private val dataStore: DataStore<Preferences>,
) : ApplicationListPreferencesStore {
    override suspend fun readSort(): ApplicationSortOrder? =
        dataStore.data
            .map { preferences -> preferences[APPLICATIONS_LIST_SORT_KEY]?.let(ApplicationSortOrder::fromWireValue) }
            .first()

    override suspend fun writeSort(sort: ApplicationSortOrder) {
        dataStore.edit { preferences -> preferences[APPLICATIONS_LIST_SORT_KEY] = sort.wireValue }
    }

    override suspend fun readLastSelectedZoneId(): WatchZoneId? =
        dataStore.data
            .map { preferences -> preferences[LAST_SELECTED_ZONE_KEY]?.let(::WatchZoneId) }
            .first()

    override suspend fun writeLastSelectedZoneId(zoneId: WatchZoneId) {
        dataStore.edit { preferences -> preferences[LAST_SELECTED_ZONE_KEY] = zoneId.value }
    }
}
