package uk.towncrierapp.domain.subscriptions

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/** Port of iOS `EntitlementMapTests` — full entitlement/quota matrix (tc-z95t, epic #770). */
class EntitlementMapTest {
    @Test
    fun `free tier has no entitlements`() {
        assertTrue(EntitlementMap.entitlements(SubscriptionTier.FREE).isEmpty())
    }

    @Test
    fun `personal tier has all three paid entitlements`() {
        val entitlements = EntitlementMap.entitlements(SubscriptionTier.PERSONAL)

        assertEquals(Entitlement.entries.toSet(), entitlements)
    }

    @Test
    fun `pro tier has all three paid entitlements`() {
        val entitlements = EntitlementMap.entitlements(SubscriptionTier.PRO)

        assertEquals(Entitlement.entries.toSet(), entitlements)
    }

    @Test
    fun `free tier gets 1 watch zone`() {
        assertEquals(1, EntitlementMap.limit(SubscriptionTier.FREE, Quota.WATCH_ZONES))
    }

    @Test
    fun `personal tier gets 3 watch zones`() {
        assertEquals(3, EntitlementMap.limit(SubscriptionTier.PERSONAL, Quota.WATCH_ZONES))
    }

    @Test
    fun `pro tier gets unlimited watch zones`() {
        assertEquals(Int.MAX_VALUE, EntitlementMap.limit(SubscriptionTier.PRO, Quota.WATCH_ZONES))
    }

    @Test
    fun `free tier does not have statusChangeAlerts`() {
        assertFalse(EntitlementMap.hasEntitlement(Entitlement.STATUS_CHANGE_ALERTS, SubscriptionTier.FREE))
    }

    @Test
    fun `personal tier has statusChangeAlerts`() {
        assertTrue(EntitlementMap.hasEntitlement(Entitlement.STATUS_CHANGE_ALERTS, SubscriptionTier.PERSONAL))
    }

    @Test
    fun `pro tier has hourlyDigestEmails`() {
        assertTrue(EntitlementMap.hasEntitlement(Entitlement.HOURLY_DIGEST_EMAILS, SubscriptionTier.PRO))
    }

    @Test
    fun `free tier can add its first watch zone`() {
        assertTrue(EntitlementMap.canAdd(SubscriptionTier.FREE, currentCount = 0, quota = Quota.WATCH_ZONES))
    }

    @Test
    fun `free tier cannot add a second watch zone`() {
        assertFalse(EntitlementMap.canAdd(SubscriptionTier.FREE, currentCount = 1, quota = Quota.WATCH_ZONES))
    }

    @Test
    fun `personal tier can add up to 3 watch zones`() {
        assertTrue(EntitlementMap.canAdd(SubscriptionTier.PERSONAL, currentCount = 2, quota = Quota.WATCH_ZONES))
        assertFalse(EntitlementMap.canAdd(SubscriptionTier.PERSONAL, currentCount = 3, quota = Quota.WATCH_ZONES))
    }

    @Test
    fun `pro tier can always add watch zones`() {
        assertTrue(EntitlementMap.canAdd(SubscriptionTier.PRO, currentCount = 1_000, quota = Quota.WATCH_ZONES))
    }
}
