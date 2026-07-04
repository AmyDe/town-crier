package uk.towncrierapp.domain.watchzones

/**
 * A circular geographic area a user monitors for planning applications. Port
 * of iOS `WatchZone` (epic #770). [authorityId] of `0` means "not yet
 * resolved" — the server reverse-geocodes it from [centre] when a zone is
 * created without one.
 */
public data class WatchZone(
    public val id: WatchZoneId,
    public val name: String,
    public val centre: Coordinate,
    public val radiusMetres: Double,
    public val authorityId: Int = 0,
    public val pushEnabled: Boolean = true,
    public val emailInstantEnabled: Boolean = true,
) {
    init {
        require(name.isNotBlank()) { "watch zone name must not be blank" }
        require(radiusMetres > 0) { "radius must be > 0, was $radiusMetres" }
    }
}
