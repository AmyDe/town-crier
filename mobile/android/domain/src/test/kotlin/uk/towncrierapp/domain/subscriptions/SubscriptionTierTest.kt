package uk.towncrierapp.domain.subscriptions

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * `SubscriptionTier` is ordered Free < Personal < Pro (epic #770) so tier
 * merging (`maxOf`) picks the richer tier, and its wire strings are the
 * server's PascalCase vocabulary ("Free"/"Personal"/"Pro").
 */
class SubscriptionTierTest {
    @Test
    fun `tiers are ordered Free less than Personal less than Pro`() {
        assertTrue(SubscriptionTier.FREE < SubscriptionTier.PERSONAL)
        assertTrue(SubscriptionTier.PERSONAL < SubscriptionTier.PRO)
        assertTrue(SubscriptionTier.FREE < SubscriptionTier.PRO)
    }

    @Test
    fun `maxOf picks the richer tier`() {
        assertEquals(SubscriptionTier.PRO, maxOf(SubscriptionTier.PRO, SubscriptionTier.FREE))
        assertEquals(SubscriptionTier.PERSONAL, maxOf(SubscriptionTier.PERSONAL, SubscriptionTier.FREE))
    }

    @Test
    fun `wireValue matches the server's PascalCase vocabulary`() {
        assertEquals("Free", SubscriptionTier.FREE.wireValue)
        assertEquals("Personal", SubscriptionTier.PERSONAL.wireValue)
        assertEquals("Pro", SubscriptionTier.PRO.wireValue)
    }

    @Test
    fun `fromWireValue round-trips the exact wire string "Personal"`() {
        assertEquals(SubscriptionTier.PERSONAL, SubscriptionTier.fromWireValue("Personal"))
    }

    @Test
    fun `fromWireValue is case-insensitive to also accept the lowercase JWT claim spelling`() {
        assertEquals(SubscriptionTier.PRO, SubscriptionTier.fromWireValue("pro"))
    }

    @Test
    fun `fromWireValue returns null for an unrecognised string`() {
        assertNull(SubscriptionTier.fromWireValue("enterprise"))
    }
}
