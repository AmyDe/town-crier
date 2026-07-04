package uk.towncrierapp.domain.subscriptions

/**
 * Device-local latch for the last resolved [SubscriptionTier] (DataStore
 * Preferences key `cachedSubscriptionTier` in `:data`) — the fast-path read
 * on cold start, before the network round-trip in [resolveTier] completes,
 * and the fallback source when ensure-profile fails.
 */
public interface SubscriptionTierCache {
    /** The last persisted tier, or `null` if nothing has been cached yet. */
    public suspend fun read(): SubscriptionTier?

    public suspend fun write(tier: SubscriptionTier)
}
