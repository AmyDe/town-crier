package uk.towncrierapp.domain.watchzones

import org.junit.jupiter.api.Test
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/** Port of iOS `WatchZoneLimitsTests` — full tier matrix (tc-z95t, epic #770). */
class WatchZoneLimitsTest {
    @Test
    fun `free tier max zones is 1`() {
        assertEquals(1, WatchZoneLimits(SubscriptionTier.FREE).maxZones)
    }

    @Test
    fun `personal tier max zones is 3`() {
        assertEquals(3, WatchZoneLimits(SubscriptionTier.PERSONAL).maxZones)
    }

    @Test
    fun `pro tier max zones is unlimited`() {
        assertEquals(Int.MAX_VALUE, WatchZoneLimits(SubscriptionTier.PRO).maxZones)
    }

    @Test
    fun `free tier max radius is 2km`() {
        assertEquals(2_000.0, WatchZoneLimits(SubscriptionTier.FREE).maxRadiusMetres)
    }

    @Test
    fun `personal tier max radius is 5km`() {
        assertEquals(5_000.0, WatchZoneLimits(SubscriptionTier.PERSONAL).maxRadiusMetres)
    }

    @Test
    fun `pro tier max radius is 10km`() {
        assertEquals(10_000.0, WatchZoneLimits(SubscriptionTier.PRO).maxRadiusMetres)
    }

    @Test
    fun `canAddZone under the limit returns true`() {
        assertTrue(WatchZoneLimits(SubscriptionTier.PERSONAL).canAddZone(currentCount = 0))
    }

    @Test
    fun `canAddZone at the limit returns false`() {
        assertFalse(WatchZoneLimits(SubscriptionTier.PERSONAL).canAddZone(currentCount = 3))
    }

    @Test
    fun `canAddZone pro with many zones returns true`() {
        assertTrue(WatchZoneLimits(SubscriptionTier.PRO).canAddZone(currentCount = 50))
    }

    @Test
    fun `clampRadius within the limit returns the original value`() {
        assertEquals(3_000.0, WatchZoneLimits(SubscriptionTier.PERSONAL).clampRadius(3_000.0))
    }

    @Test
    fun `clampRadius exceeding the limit returns the max`() {
        assertEquals(5_000.0, WatchZoneLimits(SubscriptionTier.PERSONAL).clampRadius(8_000.0))
    }
}
