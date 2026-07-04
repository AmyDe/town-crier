package uk.towncrierapp.domain.subscriptions

/** Hand-written fake for [SubscriptionTierCache] — a plain in-memory box standing in for DataStore. */
public class FakeSubscriptionTierCache(
    public var stored: SubscriptionTier? = null,
) : SubscriptionTierCache {
    public val writeCalls: MutableList<SubscriptionTier> = mutableListOf()

    override suspend fun read(): SubscriptionTier? = stored

    override suspend fun write(tier: SubscriptionTier) {
        writeCalls += tier
        stored = tier
    }
}
