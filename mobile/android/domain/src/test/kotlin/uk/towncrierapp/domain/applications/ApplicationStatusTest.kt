package uk.towncrierapp.domain.applications

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.test.assertTrue

/**
 * PlanIt `app_state` raw-value decode, incl. the `unknown` fallback for any
 * genuinely unrecognised value, and the [isDecided] classification that
 * drives `statusHistory` synthesis (GH#775).
 */
class ApplicationStatusTest {
    @Test
    fun `fromWireValue decodes every known raw value`() {
        assertEquals(ApplicationStatus.Undecided, ApplicationStatus.fromWireValue("Undecided"))
        assertEquals(ApplicationStatus.Permitted, ApplicationStatus.fromWireValue("Permitted"))
        assertEquals(ApplicationStatus.Conditions, ApplicationStatus.fromWireValue("Conditions"))
        assertEquals(ApplicationStatus.Rejected, ApplicationStatus.fromWireValue("Rejected"))
        assertEquals(ApplicationStatus.Withdrawn, ApplicationStatus.fromWireValue("Withdrawn"))
        assertEquals(ApplicationStatus.Appealed, ApplicationStatus.fromWireValue("Appealed"))
        assertEquals(ApplicationStatus.Unresolved, ApplicationStatus.fromWireValue("Unresolved"))
        assertEquals(ApplicationStatus.Referred, ApplicationStatus.fromWireValue("Referred"))
        assertEquals(ApplicationStatus.NotAvailable, ApplicationStatus.fromWireValue("Not Available"))
    }

    @Test
    fun `fromWireValue falls back to Unknown for a genuinely unrecognised value`() {
        val status = ApplicationStatus.fromWireValue("SomeFutureState")

        val unknown = assertIs<ApplicationStatus.Unknown>(status)
        assertEquals("SomeFutureState", unknown.raw)
    }

    @Test
    fun `wireValue round-trips every known status`() {
        val known =
            listOf(
                ApplicationStatus.Undecided,
                ApplicationStatus.Permitted,
                ApplicationStatus.Conditions,
                ApplicationStatus.Rejected,
                ApplicationStatus.Withdrawn,
                ApplicationStatus.Appealed,
                ApplicationStatus.Unresolved,
                ApplicationStatus.Referred,
                ApplicationStatus.NotAvailable,
            )
        known.forEach { status ->
            assertEquals(status, ApplicationStatus.fromWireValue(status.wireValue))
        }
    }

    @Test
    fun `wireValue for Unknown is the original raw string`() {
        assertEquals("SomeFutureState", ApplicationStatus.Unknown("SomeFutureState").wireValue)
    }

    @Test
    fun `isDecided is true only for the five decided statuses`() {
        assertTrue(ApplicationStatus.Permitted.isDecided)
        assertTrue(ApplicationStatus.Conditions.isDecided)
        assertTrue(ApplicationStatus.Rejected.isDecided)
        assertTrue(ApplicationStatus.Withdrawn.isDecided)
        assertTrue(ApplicationStatus.Appealed.isDecided)

        assertFalse(ApplicationStatus.Undecided.isDecided)
        assertFalse(ApplicationStatus.Unresolved.isDecided)
        assertFalse(ApplicationStatus.Referred.isDecided)
        assertFalse(ApplicationStatus.NotAvailable.isDecided)
        assertFalse(ApplicationStatus.Unknown("SomeFutureState").isDecided)
    }
}
