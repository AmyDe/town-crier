package uk.towncrierapp.domain.subscriptions

/**
 * Maps subscription tiers to their granted entitlements and quota limits.
 * Must remain in sync with the API's `EntitlementMap.cs`. Port of iOS
 * `EntitlementMap` — a caseless object since it has no instances, only
 * static-style lookups.
 */
public object EntitlementMap {
    private val paidEntitlements: Set<Entitlement> =
        setOf(Entitlement.STATUS_CHANGE_ALERTS, Entitlement.DECISION_UPDATE_ALERTS, Entitlement.HOURLY_DIGEST_EMAILS)

    /** Returns the set of entitlements granted to the given subscription tier. */
    public fun entitlements(tier: SubscriptionTier): Set<Entitlement> =
        when (tier) {
            SubscriptionTier.FREE -> emptySet()
            SubscriptionTier.PERSONAL, SubscriptionTier.PRO -> paidEntitlements
        }

    /** Returns the numeric limit for the given quota at the given subscription tier. */
    public fun limit(
        tier: SubscriptionTier,
        quota: Quota,
    ): Int =
        when (quota) {
            Quota.WATCH_ZONES ->
                when (tier) {
                    SubscriptionTier.FREE -> 1
                    SubscriptionTier.PERSONAL -> 3
                    SubscriptionTier.PRO -> Int.MAX_VALUE
                }
        }

    /** Returns whether the given tier grants the specified entitlement. */
    public fun hasEntitlement(
        entitlement: Entitlement,
        tier: SubscriptionTier,
    ): Boolean = entitlements(tier).contains(entitlement)

    /** Returns whether the given tier can add another item given the current count for a quota. */
    public fun canAdd(
        tier: SubscriptionTier,
        currentCount: Int,
        quota: Quota,
    ): Boolean = currentCount < limit(tier, quota)
}
