package uk.towncrierapp.domain.versionconfig

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class AppVersionTest {
    @Test
    fun `parses a well-formed semver string`() {
        assertEquals(AppVersion(1, 2, 3), AppVersion.parse("1.2.3"))
    }

    @Test
    fun `parse returns null for a malformed string`() {
        assertNull(AppVersion.parse("1.2"))
        assertNull(AppVersion.parse("not-a-version"))
        assertNull(AppVersion.parse(""))
    }

    @Test
    fun `compares major, then minor, then patch`() {
        assertTrue(AppVersion(1, 0, 0) < AppVersion(2, 0, 0))
        assertTrue(AppVersion(1, 1, 0) < AppVersion(1, 2, 0))
        assertTrue(AppVersion(1, 0, 1) < AppVersion(1, 0, 2))
        assertTrue(AppVersion(1, 2, 3) == AppVersion(1, 2, 3))
    }

    @Test
    fun `a version below the minimum compares less than it`() {
        val current = AppVersion.parse("1.0.0")
        val minimum = AppVersion.parse("1.1.0")

        assertTrue(current != null && minimum != null && current < minimum)
    }

    @Test
    fun `a version equal to or above the minimum does not compare less than it`() {
        val current = AppVersion.parse("1.1.0")
        val minimum = AppVersion.parse("1.1.0")

        assertTrue(current != null && minimum != null && !(current < minimum))
    }
}
