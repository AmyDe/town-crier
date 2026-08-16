package uk.towncrierapp.data.settings

import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import uk.towncrierapp.domain.settings.AppearancePreference
import uk.towncrierapp.domain.settings.AppearanceStore
import java.io.IOException

// Key name reused verbatim from iOS's local `appearanceMode` (epic #770
// pre-resolved decision) — cross-platform naming consistency.
private val APPEARANCE_MODE_KEY = stringPreferencesKey("appearanceMode")

/** DataStore Preferences-backed [AppearanceStore]. Port of iOS's `UserDefaults`-backed `appearanceMode` latch. */
public class DataStoreAppearanceStore(
    private val dataStore: DataStore<Preferences>,
) : AppearanceStore {
    override suspend fun read(): AppearancePreference? =
        dataStore.data
            .map { preferences -> preferences[APPEARANCE_MODE_KEY]?.let(AppearancePreference::fromWireValue) }
            .first()

    @Suppress("SwallowedException")
    // Best-effort fire-and-forget preference write (tc-l31ve, module-wide
    // convention): a disk-IO failure here is rare and recoverable per
    // DataStore's own edit() contract, and no caller handles a thrown
    // exception from this method — so it's swallowed rather than propagated.
    override suspend fun write(preference: AppearancePreference) {
        try {
            dataStore.edit { preferences -> preferences[APPEARANCE_MODE_KEY] = preference.wireValue }
        } catch (e: CancellationException) {
            throw e
        } catch (e: IOException) {
            // Swallowed: see class-level rationale above.
        }
    }
}
