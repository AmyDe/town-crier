package uk.towncrierapp.data.reviewprompt

import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.MutablePreferences
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.core.longPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import uk.towncrierapp.domain.reviewprompt.ReviewPromptState
import uk.towncrierapp.domain.reviewprompt.ReviewPromptStore
import java.time.Instant

// Key names are this feature's own device-local namespace (GH #628) — no
// cross-platform naming constraint applies since nothing else reads them.
private val FIRST_LAUNCH_DATE_KEY = longPreferencesKey("reviewPrompt.firstLaunchDate")
private val ENGAGEMENT_SCORE_KEY = intPreferencesKey("reviewPrompt.engagementScore")
private val SAVE_COUNT_KEY = intPreferencesKey("reviewPrompt.saveCount")
private val LAST_ACTIVE_DAY_KEY_KEY = stringPreferencesKey("reviewPrompt.lastActiveDayKey")
private val DISTINCT_ACTIVE_DAYS_KEY = intPreferencesKey("reviewPrompt.distinctActiveDays")
private val LAST_PROMPT_DATE_KEY = longPreferencesKey("reviewPrompt.lastPromptDate")
private val PROMPT_TIMESTAMPS_KEY = stringPreferencesKey("reviewPrompt.promptTimestamps")
private val HAS_RECORDED_UPGRADE_KEY = booleanPreferencesKey("reviewPrompt.hasRecordedUpgrade")

private const val TIMESTAMP_SEPARATOR = ","

/**
 * DataStore Preferences-backed [ReviewPromptStore] (GH #628). Every field is
 * device-local; nothing here is sent to a server or used for analytics.
 * Dates are stored as epoch-second `Long`s and the rolling prompt-timestamp
 * list as a delimited string. Port of iOS `UserDefaultsReviewPromptStore`.
 */
public class DataStoreReviewPromptStore(
    private val dataStore: DataStore<Preferences>,
) : ReviewPromptStore {
    override suspend fun load(): ReviewPromptState =
        dataStore.data
            .map { preferences ->
                ReviewPromptState(
                    firstLaunchDate = preferences[FIRST_LAUNCH_DATE_KEY]?.let(Instant::ofEpochSecond),
                    engagementScore = preferences[ENGAGEMENT_SCORE_KEY] ?: 0,
                    saveCount = preferences[SAVE_COUNT_KEY] ?: 0,
                    lastActiveDayKey = preferences[LAST_ACTIVE_DAY_KEY_KEY],
                    distinctActiveDays = preferences[DISTINCT_ACTIVE_DAYS_KEY] ?: 0,
                    lastPromptDate = preferences[LAST_PROMPT_DATE_KEY]?.let(Instant::ofEpochSecond),
                    promptTimestamps = preferences[PROMPT_TIMESTAMPS_KEY].toTimestamps(),
                    hasRecordedUpgrade = preferences[HAS_RECORDED_UPGRADE_KEY] ?: false,
                )
            }.first()

    override suspend fun save(state: ReviewPromptState) {
        dataStore.edit { preferences ->
            setOrRemove(preferences, FIRST_LAUNCH_DATE_KEY, state.firstLaunchDate?.epochSecond)
            preferences[ENGAGEMENT_SCORE_KEY] = state.engagementScore
            preferences[SAVE_COUNT_KEY] = state.saveCount
            setOrRemove(preferences, LAST_ACTIVE_DAY_KEY_KEY, state.lastActiveDayKey)
            preferences[DISTINCT_ACTIVE_DAYS_KEY] = state.distinctActiveDays
            setOrRemove(preferences, LAST_PROMPT_DATE_KEY, state.lastPromptDate?.epochSecond)
            preferences[PROMPT_TIMESTAMPS_KEY] =
                state.promptTimestamps.joinToString(TIMESTAMP_SEPARATOR) { it.epochSecond.toString() }
            preferences[HAS_RECORDED_UPGRADE_KEY] = state.hasRecordedUpgrade
        }
    }
}

private fun <T : Any> setOrRemove(
    preferences: MutablePreferences,
    key: Preferences.Key<T>,
    value: T?,
) {
    if (value != null) {
        preferences[key] = value
    } else {
        preferences.remove(key)
    }
}

private fun String?.toTimestamps(): List<Instant> =
    if (this.isNullOrEmpty()) {
        emptyList()
    } else {
        split(TIMESTAMP_SEPARATOR).map { Instant.ofEpochSecond(it.toLong()) }
    }
