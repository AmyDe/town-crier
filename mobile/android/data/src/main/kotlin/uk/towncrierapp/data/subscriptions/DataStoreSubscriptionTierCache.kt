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
 * DataStore Preferences-backed [SubscriptionTierCache] under the
 * `cachedSubscriptionTier` key (the same key iOS uses in `UserDefaults`).
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
    override suspend fun write(tier: SubscriptionTier) {
        try {
            dataStore.edit { preferences -> preferences[CACHED_SUBSCRIPTION_TIER_KEY] = tier.wireValue }
        } catch (e: CancellationException) {
            throw e
        } catch (e: IOException) {
            // Best-effort write: no caller handles a disk-IO failure.
        }
    }
}
