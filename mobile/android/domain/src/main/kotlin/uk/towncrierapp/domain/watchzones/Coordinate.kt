package uk.towncrierapp.domain.watchzones

/**
 * A geographic coordinate (latitude/longitude pair). Out-of-range values are
 * a programmer-error invariant here — coordinates only ever arrive already
 * validated, from a geocoding response or a server-decoded watch zone — so a
 * violation fails fast via [require] rather than a sealed outcome the caller
 * must handle (android-coding-standards skill, kotlin-idiom.md). Port of iOS
 * `Coordinate`.
 */
public data class Coordinate(
    public val latitude: Double,
    public val longitude: Double,
) {
    init {
        require(latitude in -90.0..90.0) { "latitude must be in -90..90, was $latitude" }
        require(longitude in -180.0..180.0) { "longitude must be in -180..180, was $longitude" }
    }
}
