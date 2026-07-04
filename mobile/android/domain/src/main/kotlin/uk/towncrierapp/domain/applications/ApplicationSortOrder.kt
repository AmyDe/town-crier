package uk.towncrierapp.domain.applications

/**
 * The five server-supported sort orders for `GET
 * /v1/me/watch-zones/{zoneId}/applications`. Android always sends one (never
 * exercises the legacy param-less path — epic #770 pre-resolved decision).
 * [DEFAULT] is the CLIENT default (`recent-activity`); the server's own
 * default (`distance`) is deliberately not the client default, and
 * `distance` itself is hidden from the sort picker whenever no zone is
 * active. Port of iOS `ApplicationSortOrder`.
 */
public enum class ApplicationSortOrder(
    public val wireValue: String,
) {
    DISTANCE("distance"),
    NEWEST("newest"),
    OLDEST("oldest"),
    STATUS("status"),
    RECENT_ACTIVITY("recent-activity"),
    ;

    public companion object {
        public val DEFAULT: ApplicationSortOrder = RECENT_ACTIVITY

        /** Decodes a persisted/wire sort token; anything unrecognised falls back to [DEFAULT]. */
        public fun fromWireValue(value: String): ApplicationSortOrder = entries.firstOrNull { it.wireValue == value } ?: DEFAULT
    }
}
