package uk.towncrierapp.data.applications

import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import uk.towncrierapp.domain.applications.ApplicationListPreferencesStore
import uk.towncrierapp.domain.applications.ApplicationSortOrder
import uk.towncrierapp.domain.watchzones.WatchZoneId
import java.io.IOException

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

    @Suppress("SwallowedException")
    // Best-effort fire-and-forget preference write (tc-l31ve, module-wide
    // convention): a disk-IO failure here is rare and recoverable per
    // DataStore's own edit() contract, and no caller handles a thrown
    // exception from this method — so it's swallowed rather than propagated.
    override suspend fun writeSort(sort: ApplicationSortOrder) {
        try {
            dataStore.edit { preferences -> preferences[APPLICATIONS_LIST_SORT_KEY] = sort.wireValue }
        } catch (e: CancellationException) {
            throw e
        } catch (e: IOException) {
            // Swallowed: see writeSort's rationale comment above.
        }
    }

    override suspend fun readLastSelectedZoneId(): WatchZoneId? =
        dataStore.data
            .map { preferences -> preferences[LAST_SELECTED_ZONE_KEY]?.let(::WatchZoneId) }
            .first()

    @Suppress("SwallowedException")
    // Best-effort fire-and-forget preference write (tc-l31ve, module-wide
    // convention): see writeSort's rationale comment above.
    override suspend fun writeLastSelectedZoneId(zoneId: WatchZoneId) {
        try {
            dataStore.edit { preferences -> preferences[LAST_SELECTED_ZONE_KEY] = zoneId.value }
        } catch (e: CancellationException) {
            throw e
        } catch (e: IOException) {
            // Swallowed: see writeSort's rationale comment above.
        }
    }
}
