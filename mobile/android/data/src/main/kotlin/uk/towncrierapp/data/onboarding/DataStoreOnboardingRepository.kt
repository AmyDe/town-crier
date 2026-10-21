package uk.towncrierapp.data.onboarding

import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import uk.towncrierapp.domain.onboarding.OnboardingRepository
import java.io.IOException

private val IS_ONBOARDING_COMPLETE_KEY = booleanPreferencesKey("isOnboardingComplete")

/**
 * DataStore Preferences-backed [OnboardingRepository] - the
 * `isOnboardingComplete` device latch (epic #770, same key iOS uses in
 * `UserDefaults`). A one-shot suspend read/write, not a `Flow`: this is a
 * fast-path hint, not something the UI observes live.
 */
public class DataStoreOnboardingRepository(
    private val dataStore: DataStore<Preferences>,
) : OnboardingRepository {
    override suspend fun isOnboardingComplete(): Boolean =
        dataStore.data
            .map { preferences -> preferences[IS_ONBOARDING_COMPLETE_KEY] ?: false }
            .first()

    @Suppress("SwallowedException")
    // Best-effort write: no caller handles a thrown IOException, so it's swallowed.
    override suspend fun setOnboardingComplete(complete: Boolean) {
        try {
            dataStore.edit { preferences -> preferences[IS_ONBOARDING_COMPLETE_KEY] = complete }
        } catch (e: CancellationException) {
            throw e
        } catch (e: IOException) {
            // Swallowed: see the rationale above.
        }
    }
}
