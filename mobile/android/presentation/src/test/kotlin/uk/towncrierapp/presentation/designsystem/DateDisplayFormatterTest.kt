package uk.towncrierapp.presentation.designsystem

import org.junit.jupiter.api.Test
import java.time.LocalDate
import kotlin.test.assertEquals

/**
 * Absolute dates ONLY, never relative/"time ago" text, anywhere (GH#775 —
 * byte-exact port of iOS `Date+TownCrier.swift`'s shared formatter):
 * `d MMM yyyy`, en-GB.
 */
class DateDisplayFormatterTest {
    @Test
    fun `formats a date as day, short month, year`() {
        assertEquals("14 Nov 2023", DateDisplayFormatter.format(LocalDate.of(2023, 11, 14)))
    }

    @Test
    fun `does not zero-pad a single-digit day`() {
        assertEquals("1 Jan 2026", DateDisplayFormatter.format(LocalDate.of(2026, 1, 1)))
    }
}
