package uk.towncrierapp.domain.applications

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

/** Wire-value round trip and the client-side default (GH#775: server default `distance` is NOT the client default). */
class ApplicationSortOrderTest {
    @Test
    fun `wireValue matches the server's five sort tokens`() {
        assertEquals("distance", ApplicationSortOrder.DISTANCE.wireValue)
        assertEquals("newest", ApplicationSortOrder.NEWEST.wireValue)
        assertEquals("oldest", ApplicationSortOrder.OLDEST.wireValue)
        assertEquals("status", ApplicationSortOrder.STATUS.wireValue)
        assertEquals("recent-activity", ApplicationSortOrder.RECENT_ACTIVITY.wireValue)
    }

    @Test
    fun `fromWireValue is the inverse of wireValue for every case`() {
        ApplicationSortOrder.entries.forEach { sort ->
            assertEquals(sort, ApplicationSortOrder.fromWireValue(sort.wireValue))
        }
    }

    @Test
    fun `fromWireValue falls back to the client default for an unrecognised value`() {
        assertEquals(ApplicationSortOrder.DEFAULT, ApplicationSortOrder.fromWireValue("made-up"))
    }

    @Test
    fun `the client default is recent-activity, not the server default distance`() {
        assertEquals(ApplicationSortOrder.RECENT_ACTIVITY, ApplicationSortOrder.DEFAULT)
    }
}
