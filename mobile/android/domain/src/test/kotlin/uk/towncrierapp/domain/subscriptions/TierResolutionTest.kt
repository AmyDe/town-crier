package uk.towncrierapp.domain.subscriptions

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

/**
 * Pure port of iOS `SubscriptionTierResolver`'s per-pass formula, degraded to
 * the 2-way form epic #770 assigns this issue (no store tier until #783):
 * `max(serverTier ?? max(cachedTier, jwtTier), jwtTier)`. The retry-once-on-
 * free orchestration (calling this twice around a session refresh) lives in
 * `:presentation`'s auth coordinator — this function is the single-pass
 * arithmetic it calls.
 */
class TierResolutionTest {
    @Test
    fun `a present server tier wins outright when it is the richest source`() {
        val resolved =
            resolveTier(
                serverTier = SubscriptionTier.PRO,
                cachedTier = SubscriptionTier.FREE,
                jwtTier = SubscriptionTier.FREE,
            )

        assertEquals(SubscriptionTier.PRO, resolved)
    }

    @Test
    fun `a richer jwt tier still wins over a lower server tier`() {
        val resolved =
            resolveTier(
                serverTier = SubscriptionTier.FREE,
                cachedTier = SubscriptionTier.FREE,
                jwtTier = SubscriptionTier.PERSONAL,
            )

        assertEquals(SubscriptionTier.PERSONAL, resolved)
    }

    @Test
    fun `a null server tier falls back to max of cached and jwt, never Free outright`() {
        val resolved =
            resolveTier(
                serverTier = null,
                cachedTier = SubscriptionTier.PRO,
                jwtTier = SubscriptionTier.FREE,
            )

        assertEquals(SubscriptionTier.PRO, resolved)
    }

    @Test
    fun `a null server tier and both fallbacks free resolves free`() {
        val resolved =
            resolveTier(
                serverTier = null,
                cachedTier = SubscriptionTier.FREE,
                jwtTier = SubscriptionTier.FREE,
            )

        assertEquals(SubscriptionTier.FREE, resolved)
    }
}
