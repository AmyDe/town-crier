package uk.towncrierapp.data.onboarding

import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import uk.towncrierapp.domain.onboarding.OnboardingRepository

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

    override suspend fun setOnboardingComplete(complete: Boolean) {
        dataStore.edit { preferences -> preferences[IS_ONBOARDING_COMPLETE_KEY] = complete }
    }
}
