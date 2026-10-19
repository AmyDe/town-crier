package uk.towncrierapp.domain.watchzones

import org.junit.jupiter.api.Test
import kotlin.test.assertTrue

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
