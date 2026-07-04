package uk.towncrierapp.data.subscriptions

import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.subscriptions.SubscriptionTierCache
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map

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
            .map { preferences -> preferences[CACHED_SUBSCRIPTION_TIER_KEY]?.let { SubscriptionTier.fromWireValue(it) } }
            .first()

    override suspend fun write(tier: SubscriptionTier) {
        dataStore.edit { preferences -> preferences[CACHED_SUBSCRIPTION_TIER_KEY] = tier.wireValue }
    }
}
