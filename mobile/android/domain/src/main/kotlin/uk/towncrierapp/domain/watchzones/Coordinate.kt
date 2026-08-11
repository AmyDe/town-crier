package uk.towncrierapp.domain.watchzones

import kotlin.math.atan2
import kotlin.math.cos
import kotlin.math.sin
import kotlin.math.sqrt

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

private const val EARTH_RADIUS_METRES = 6_371_000.0

/**
 * Great-circle distance between two raw lat/lon pairs, in metres. Takes raw
 * values rather than two [Coordinate]s so a derived, not-yet-validated point
 * — e.g. [WatchZoneBoundary.enclosingRadiusMetres]'s centroid — doesn't need
 * to round-trip it through [Coordinate]'s `init` just to measure a distance.
 * Port of iOS `Coordinate.haversineMetres`.
 */
internal fun haversineMetres(
    latitude1: Double,
    longitude1: Double,
    latitude2: Double,
    longitude2: Double,
): Double {
    val degToRad = Math.PI / 180
    val phi1 = latitude1 * degToRad
    val phi2 = latitude2 * degToRad
    val deltaPhi = (latitude2 - latitude1) * degToRad
    val deltaLambda = (longitude2 - longitude1) * degToRad
    val haversine =
        sin(deltaPhi / 2) * sin(deltaPhi / 2) +
            cos(phi1) * cos(phi2) * sin(deltaLambda / 2) * sin(deltaLambda / 2)
    // Clamp before the sqrts: floating-point round-off can push `haversine`
    // fractionally above 1 for near-antipodal coordinates, which would make
    // `1 - haversine` negative and `sqrt` return NaN.
    val clampedHaversine = minOf(1.0, haversine)
    val angularDistance = 2 * atan2(sqrt(clampedHaversine), sqrt(1 - clampedHaversine))
    return EARTH_RADIUS_METRES * angularDistance
}
