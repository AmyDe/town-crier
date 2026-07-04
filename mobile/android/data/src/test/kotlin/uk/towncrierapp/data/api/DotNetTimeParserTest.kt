package uk.towncrierapp.data.api

import org.junit.jupiter.api.Test
import java.time.OffsetDateTime
import java.time.ZoneOffset
import kotlin.test.assertEquals
import kotlin.test.assertNull

/**
 * Port of iOS `DotNetTimeParserTests` — one shared parser for the Go
 * backend's `platform.DotNetTime` wire format, applied per-field by DTO
 * mappers (epic #770 "API contract essentials"). Fractional seconds appear
 * on the wire only when the sub-second part is non-zero, and the offset is
 * spelled either `Z` or `±hh:mm` — the parser must accept every combination,
 * and never throw on garbage.
 */
class DotNetTimeParserTest {
    @Test
    fun `parses a fractional-seconds timestamp with a numeric offset`() {
        val parsed = DotNetTimeParser.parse("2026-07-20T14:23:45.6789123+00:00")

        assertEquals(
            OffsetDateTime.of(2026, 7, 20, 14, 23, 45, 678_912_300, ZoneOffset.UTC),
            parsed,
        )
    }

    @Test
    fun `parses a whole-second timestamp with an explicit numeric offset`() {
        val parsed = DotNetTimeParser.parse("2026-06-12T09:30:00+00:00")

        assertEquals(OffsetDateTime.of(2026, 6, 12, 9, 30, 0, 0, ZoneOffset.UTC), parsed)
    }

    @Test
    fun `parses a whole-second timestamp in Z form`() {
        val parsed = DotNetTimeParser.parse("2026-06-12T09:30:00Z")

        assertEquals(OffsetDateTime.of(2026, 6, 12, 9, 30, 0, 0, ZoneOffset.UTC), parsed)
    }

    @Test
    fun `parses a fractional-seconds timestamp in Z form`() {
        val parsed = DotNetTimeParser.parse("2026-07-20T14:23:45.5Z")

        assertEquals(OffsetDateTime.of(2026, 7, 20, 14, 23, 45, 500_000_000, ZoneOffset.UTC), parsed)
    }

    @Test
    fun `returns null on genuine garbage rather than throwing`() {
        assertNull(DotNetTimeParser.parse("not a date"))
        assertNull(DotNetTimeParser.parse(""))
        assertNull(DotNetTimeParser.parse("2026-13-45"))
    }

    @Test
    fun `date-only fields parse as a separate simple LocalDate, not conflated with the timestamp parser`() {
        val parsed = DotNetTimeParser.parseDate("2026-06-12")

        assertEquals(java.time.LocalDate.of(2026, 6, 12), parsed)
    }

    @Test
    fun `parseDate returns null on garbage`() {
        assertNull(DotNetTimeParser.parseDate("2026-06-12T09:30:00Z"))
        assertNull(DotNetTimeParser.parseDate("not a date"))
    }
}
