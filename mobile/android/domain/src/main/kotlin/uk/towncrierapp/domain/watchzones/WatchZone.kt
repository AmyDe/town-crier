package uk.towncrierapp.domain.watchzones

/**
 * A geographic area a user monitors for planning applications. [authorityId]
 * of `0` means "not yet resolved" —
 * the server reverse-geocodes it from [centre] when a zone is created
 * without one.
 *
 * [boundary] is `null` for the original circle-only zones and non-null for a
 * custom-shape zone. A polygon zone still populates
 * [centre]/[radiusMetres] server-side, derived from the boundary's centroid
 * and enclosing radius, so no read path needs special-casing beyond checking
 * whether [boundary] is present.
 */
public data class WatchZone(
    public val id: WatchZoneId,
    public val name: String,
    public val centre: Coordinate,
    public val radiusMetres: Double,
    public val authorityId: Int = 0,
    public val pushEnabled: Boolean = true,
    public val emailInstantEnabled: Boolean = true,
    public val boundary: WatchZoneBoundary? = null,
) {
    init {
        require(name.isNotBlank()) { "watch zone name must not be blank" }
        require(radiusMetres > 0) { "radius must be > 0, was $radiusMetres" }
    }
}
