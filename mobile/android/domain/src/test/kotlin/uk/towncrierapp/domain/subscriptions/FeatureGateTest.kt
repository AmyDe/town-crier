package uk.towncrierapp.domain.subscriptions

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/** Port of iOS `FeatureGateTests` (tc-z95t). */
class FeatureGateTest {
    @Test
    fun `carries the tier it was constructed with`() {
        assertEquals(SubscriptionTier.PERSONAL, FeatureGate(SubscriptionTier.PERSONAL).tier)
    }

    @Test
    fun `hasEntitlement delegates to EntitlementMap`() {
        assertFalse(FeatureGate(SubscriptionTier.FREE).hasEntitlement(Entitlement.STATUS_CHANGE_ALERTS))
        assertTrue(FeatureGate(SubscriptionTier.PERSONAL).hasEntitlement(Entitlement.STATUS_CHANGE_ALERTS))
    }

    @Test
    fun `canAdd delegates to EntitlementMap`() {
        val gate = FeatureGate(SubscriptionTier.FREE)

        assertTrue(gate.canAdd(Quota.WATCH_ZONES, currentCount = 0))
        assertFalse(gate.canAdd(Quota.WATCH_ZONES, currentCount = 1))
    }

    @Test
    fun `shouldShowUpgradeBadge for an entitlement is true only when the tier lacks it`() {
        assertTrue(FeatureGate(SubscriptionTier.FREE).shouldShowUpgradeBadge(Entitlement.STATUS_CHANGE_ALERTS))
        assertFalse(FeatureGate(SubscriptionTier.PERSONAL).shouldShowUpgradeBadge(Entitlement.STATUS_CHANGE_ALERTS))
    }

    @Test
    fun `shouldShowUpgradeBadge for a quota is true only at the cap`() {
        val gate = FeatureGate(SubscriptionTier.FREE)

        assertFalse(gate.shouldShowUpgradeBadge(Quota.WATCH_ZONES, currentCount = 0))
        assertTrue(gate.shouldShowUpgradeBadge(Quota.WATCH_ZONES, currentCount = 1))
    }
}
