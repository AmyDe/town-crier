package uk.towncrierapp.domain.profile

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/**
 * `DigestDay`'s wire strings are the server's English weekday names
 * (`weekdayName` in `api-go/internal/profiles/handler.go`); the enum itself
 * is declared Monday-first for direct use in a UK-facing picker (epic #770).
 */
class DigestDayTest {
    @Test
    fun `entries are declared Monday-first for a UK-facing picker`() {
        assertEquals(
            listOf(
                DigestDay.MONDAY,
                DigestDay.TUESDAY,
                DigestDay.WEDNESDAY,
                DigestDay.THURSDAY,
                DigestDay.FRIDAY,
                DigestDay.SATURDAY,
                DigestDay.SUNDAY,
            ),
            DigestDay.entries,
        )
    }

    @Test
    fun `wireValue matches the server's English weekday name`() {
        assertEquals("Monday", DigestDay.MONDAY.wireValue)
        assertEquals("Sunday", DigestDay.SUNDAY.wireValue)
    }

    @Test
    fun `fromWireValue round-trips the exact wire string`() {
        assertEquals(DigestDay.FRIDAY, DigestDay.fromWireValue("Friday"))
    }

    @Test
    fun `fromWireValue returns null for an unrecognised string`() {
        assertNull(DigestDay.fromWireValue("Fictionday"))
    }
}
