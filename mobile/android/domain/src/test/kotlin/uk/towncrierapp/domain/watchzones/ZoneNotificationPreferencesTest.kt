package uk.towncrierapp.domain.watchzones

import org.junit.jupiter.api.Test
import kotlin.test.assertTrue

/**
 * Port of iOS `ZoneNotificationPreferencesTests` — all four per-channel
 * toggles default to `true` so newly-created zones opt in to every alert;
 * free-tier downgrades apply server-side at dispatch time (tc-z95t).
 */
class ZoneNotificationPreferencesTest {
    @Test
    fun `every channel defaults to enabled`() {
        val prefs = ZoneNotificationPreferences(zoneId = WatchZoneId("wz-1"))

        assertTrue(prefs.newApplicationPush)
        assertTrue(prefs.newApplicationEmail)
        assertTrue(prefs.decisionPush)
        assertTrue(prefs.decisionEmail)
    }
}
