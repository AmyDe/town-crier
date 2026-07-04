package uk.towncrierapp.domain.subscriptions

/**
 * Proactive UI-gating helper: wraps a [SubscriptionTier] and delegates to
 * [EntitlementMap] for all business logic, letting ViewModels check
 * entitlements and quotas — and badge/disable controls before the user taps —
 * without a network round-trip. Port of iOS `FeatureGate`.
 */
public data class FeatureGate(
    public val tier: SubscriptionTier,
) {
    /** Returns whether the current tier grants the specified entitlement. */
    public fun hasEntitlement(entitlement: Entitlement): Boolean = EntitlementMap.hasEntitlement(entitlement, tier)

    /** Returns whether the current tier allows adding another item for the given quota. */
    public fun canAdd(
        quota: Quota,
        currentCount: Int,
    ): Boolean = EntitlementMap.canAdd(tier, currentCount, quota)

    /** True when the current tier does not grant [entitlement] — drives an "Upgrade" badge. */
    public fun shouldShowUpgradeBadge(entitlement: Entitlement): Boolean = !hasEntitlement(entitlement)

    /** True when [currentCount] has reached the tier's limit for [quota] — drives an "Upgrade" badge. */
    public fun shouldShowUpgradeBadge(
        quota: Quota,
        currentCount: Int,
    ): Boolean = !canAdd(quota, currentCount)
}
