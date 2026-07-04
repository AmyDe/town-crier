package uk.towncrierapp.domain.profile

/**
 * The day of the week the weekly email digest is sent on. Declared
 * Monday-first for direct use in a UK-facing picker (`entries` IS the
 * display order — no separate week-order list needed, unlike iOS's
 * Sunday-first `DayOfWeek` + `weekOrderUK`). [wireValue] is the server's
 * English weekday name (`weekdayName` in `api-go/internal/profiles/handler.go`).
 */
public enum class DigestDay(
    public val wireValue: String,
) {
    MONDAY("Monday"),
    TUESDAY("Tuesday"),
    WEDNESDAY("Wednesday"),
    THURSDAY("Thursday"),
    FRIDAY("Friday"),
    SATURDAY("Saturday"),
    SUNDAY("Sunday"),
    ;

    public companion object {
        /** Parses a digest day from its exact wire string. Returns `null` for anything unrecognised. */
        public fun fromWireValue(value: String): DigestDay? = entries.firstOrNull { it.wireValue == value }
    }
}
