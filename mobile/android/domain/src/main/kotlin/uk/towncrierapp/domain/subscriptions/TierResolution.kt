package uk.towncrierapp.domain.subscriptions

/**
 * Single-pass tier merge (port of iOS `SubscriptionTierResolver.resolveOnce`,
 * degraded to the 2-way form epic #772 assigns — no store tier until #783):
 *
 * `max(serverTier ?? max(cachedTier, jwtTier), jwtTier)`
 *
 * [serverTier] is `null` when ensure-profile failed (network/server error) —
 * the fallback to `max(cachedTier, jwtTier)` means a transient failure never
 * downgrades a paying user to Free (tc-exg6). The retry-once-when-Free
 * orchestration (refresh the session and call this again) lives in
 * `:presentation`'s auth coordinator, which owns the suspend/async glue this
 * pure function deliberately has none of.
 */
public fun resolveTier(
    serverTier: SubscriptionTier?,
    cachedTier: SubscriptionTier,
    jwtTier: SubscriptionTier,
): SubscriptionTier {
    val effectiveServerTier = serverTier ?: maxOf(cachedTier, jwtTier)
    return maxOf(effectiveServerTier, jwtTier)
}
