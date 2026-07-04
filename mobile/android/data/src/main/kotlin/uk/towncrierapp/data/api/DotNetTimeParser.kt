package uk.towncrierapp.data.api

import java.time.LocalDate
import java.time.OffsetDateTime
import java.time.format.DateTimeFormatter
import java.time.format.DateTimeParseException

/**
 * Parses timestamp strings emitted by the Go backend's `platform.DotNetTime`
 * format (`2006-01-02T15:04:05.9999999-07:00`) — one shared parser applied
 * per-field by DTO mappers, never a global kotlinx.serialization date
 * strategy (epic #770 "API contract essentials"; port of iOS
 * `DotNetTimeParser`).
 *
 * Fractional seconds appear on the wire **only** when the sub-second part is
 * non-zero, and the offset is spelled either `Z` or `±hh:mm`:
 *
 * - `2026-07-20T14:23:45.6789123+00:00` — fractional, numeric offset.
 * - `2026-06-12T09:30:00+00:00` — whole second, numeric offset.
 * - `2026-06-12T09:30:00Z` — whole second, `Z` offset.
 * - `2026-07-20T14:23:45.5Z` — fractional, `Z` offset.
 *
 * [DateTimeFormatter.ISO_OFFSET_DATE_TIME] already accepts every one of
 * these (0-9 optional fractional digits, `Z` or `±hh:mm`), so there is no
 * need to try multiple formatters like the iOS `ISO8601DateFormatter` port
 * does — `OffsetDateTime.parse` with the default formatter covers the whole
 * matrix in one call.
 */
public object DotNetTimeParser {
    /** Returns the parsed instant, or `null` on genuinely unparseable input — never throws. */
    public fun parse(value: String): OffsetDateTime? =
        try {
            OffsetDateTime.parse(value, DateTimeFormatter.ISO_OFFSET_DATE_TIME)
        } catch (e: DateTimeParseException) {
            null
        }

    /** Parses a bare `yyyy-MM-dd` date-only field (e.g. `startDate`, `decidedDate`) — a separate, simpler format. */
    public fun parseDate(value: String): LocalDate? =
        try {
            LocalDate.parse(value, DateTimeFormatter.ISO_LOCAL_DATE)
        } catch (e: DateTimeParseException) {
            null
        }
}
