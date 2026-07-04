package uk.towncrierapp.domain.watchzones

import uk.towncrierapp.domain.subscriptions.EntitlementMap
import uk.towncrierapp.domain.subscriptions.Quota
import uk.towncrierapp.domain.subscriptions.SubscriptionTier

/**
 * Per-tier watch-zone limits. Zone count delegates to [EntitlementMap] so
 * there is a single source of truth kept in sync with the API's
 * `EntitlementMap.cs`; the radius cap is this type's own table and is
 * client-enforced only — the server ceiling is a flat 10 km for every tier
 * (bump the server first before ever raising these). Port of iOS
 * `WatchZoneLimits`.
 */
public data class WatchZoneLimits(
    public val tier: SubscriptionTier,
) {
    public val maxZones: Int = EntitlementMap.limit(tier, Quota.WATCH_ZONES)

    public val maxRadiusMetres: Double =
        when (tier) {
            SubscriptionTier.FREE -> 2_000.0
            SubscriptionTier.PERSONAL -> 5_000.0
            SubscriptionTier.PRO -> 10_000.0
        }

    /** Whether the user can add another zone given their current count. */
    public fun canAddZone(currentCount: Int): Boolean = currentCount < maxZones

    /** Clamps a radius to the tier's maximum. */
    public fun clampRadius(radiusMetres: Double): Double = minOf(radiusMetres, maxRadiusMetres)
}
