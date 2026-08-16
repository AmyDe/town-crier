package uk.towncrierapp.data.subscriptions

import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.subscriptions.SubscriptionTierCache
import java.io.IOException

private val CACHED_SUBSCRIPTION_TIER_KEY = stringPreferencesKey("cachedSubscriptionTier")

/**
 * DataStore Preferences-backed [SubscriptionTierCache] — the
 * `cachedSubscriptionTier` device latch (epic #770, same key iOS uses in
 * `UserDefaults`). A one-shot suspend read/write, not a `Flow`: this is the
 * cold-start fast-path value, not something the UI observes live.
 */
public class DataStoreSubscriptionTierCache(
    private val dataStore: DataStore<Preferences>,
) : SubscriptionTierCache {
    override suspend fun read(): SubscriptionTier? =
        dataStore.data
            .map { preferences ->
                preferences[CACHED_SUBSCRIPTION_TIER_KEY]?.let { SubscriptionTier.fromWireValue(it) }
            }.first()

    @Suppress("SwallowedException")
    // Best-effort fire-and-forget preference write (tc-l31ve, module-wide
    // convention): a disk-IO failure here is rare and recoverable per
    // DataStore's own edit() contract, and no caller handles a thrown
    // exception from this method — so it's swallowed rather than propagated.
    override suspend fun write(tier: SubscriptionTier) {
        try {
            dataStore.edit { preferences -> preferences[CACHED_SUBSCRIPTION_TIER_KEY] = tier.wireValue }
        } catch (e: CancellationException) {
            throw e
        } catch (e: IOException) {
            // Swallowed: see class-level rationale above.
        }
    }
}
