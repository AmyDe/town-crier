package uk.towncrierapp.domain.subscriptions

/**
 * Single-pass tier merge: `max(serverTier ?? max(cachedTier, jwtTier), jwtTier)`.
 *
 * [serverTier] is `null` when ensure-profile failed; the fallback means a
 * transient failure never downgrades a paying user to Free.
 */
public fun resolveTier(
    serverTier: SubscriptionTier?,
    cachedTier: SubscriptionTier,
    jwtTier: SubscriptionTier,
): SubscriptionTier {
    val effectiveServerTier = serverTier ?: maxOf(cachedTier, jwtTier)
    return maxOf(effectiveServerTier, jwtTier)
}
