package uk.towncrierapp.domain.map

import uk.towncrierapp.domain.watchzones.Coordinate
import kotlin.math.cos
import kotlin.math.log2
import kotlin.math.max

/**
 * The visible map rectangle expressed as a WGS84 bounding box (GH#698,
 * GH#776). The map sends this with an integer slippy [zoom] so the server
 * returns only the cluster aggregates inside the current view. Edges are
 * decimal degrees: [west]/[east] are longitudes, [south]/[north] are
 * latitudes. Port of iOS `MapViewport`.
 */
public data class MapViewport(
    public val west: Double,
    public val south: Double,
    public val east: Double,
    public val north: Double,
) {
    /** The `bbox=west,south,east,north` query value the clusters endpoint expects. */
    public val queryValue: String get() = "$west,$south,$east,$north"

    public companion object {
        /**
         * The minimum lat/lon span (degrees) a zoom-in step will ever
         * produce — mirrors iOS `MKCoordinateSpan`'s 0.0005° floor so
         * repeated bubble taps stop shrinking the viewport once it's
         * already essentially street-level.
         */
        public const val MIN_SPAN_DEGREES: Double = 0.0005

        /**
         * The whole-zone viewport (centre ± 1.25x the radius, matching iOS's
         * 2.5x-radius camera framing) and its corresponding slippy zoom —
         * seeds the map before the first real camera-idle event supplies the
         * exact visible rect.
         */
        public fun initial(
            centre: Coordinate,
            radiusMetres: Double,
        ): Pair<MapViewport, Int> {
            val metresPerDegreeLat = 111_320.0
            val halfSpanMetres = radiusMetres * 2.5 / 2
            val halfLatDeg = halfSpanMetres / metresPerDegreeLat
            val cosLat = max(0.01, cos(centre.latitude * Math.PI / 180))
            val halfLonDeg = halfSpanMetres / (metresPerDegreeLat * cosLat)
            val viewport =
                MapViewport(
                    west = centre.longitude - halfLonDeg,
                    south = centre.latitude - halfLatDeg,
                    east = centre.longitude + halfLonDeg,
                    north = centre.latitude + halfLatDeg,
                )
            return viewport to slippyZoom(halfLonDeg * 2)
        }

        /** Standard slippy-map zoom for a longitude span: `log2(360 / span)`, clamped to the API's accepted 0..20 range. */
        public fun slippyZoom(longitudeSpanDegrees: Double): Int {
            if (longitudeSpanDegrees <= 0) return MAX_ZOOM
            val zoom = Math.round(log2(FULL_LONGITUDE_SPAN_DEGREES / longitudeSpanDegrees)).toInt()
            return zoom.coerceIn(MIN_ZOOM, MAX_ZOOM)
        }

        /** Clamps a raw (possibly fractional, possibly out-of-range) camera zoom into the API's accepted 0..20 slippy range. */
        public fun clampZoom(rawZoom: Double): Int = Math.round(rawZoom).toInt().coerceIn(MIN_ZOOM, MAX_ZOOM)

        private const val MIN_ZOOM = 0
        private const val MAX_ZOOM = 20
        private const val FULL_LONGITUDE_SPAN_DEGREES = 360.0
    }
}

/**
 * The viewport produced by zooming into a splittable bubble tapped at
 * [target]: both spans halve (floored at [MapViewport.MIN_SPAN_DEGREES]) and
 * the new rectangle is centred on [target] — no server round-trip, purely
 * client-side camera math (GH#776, matching iOS `ClusteredMapView`'s
 * `didSelect` handling). Port of iOS's halve-span-then-floor zoom-in.
 */
public fun MapViewport.zoomedInto(target: Coordinate): MapViewport {
    val newLatSpan = max((north - south) / 2, MapViewport.MIN_SPAN_DEGREES)
    val newLonSpan = max((east - west) / 2, MapViewport.MIN_SPAN_DEGREES)
    return MapViewport(
        west = target.longitude - newLonSpan / 2,
        south = target.latitude - newLatSpan / 2,
        east = target.longitude + newLonSpan / 2,
        north = target.latitude + newLatSpan / 2,
    )
}

/** This viewport's slippy zoom, derived from its longitude span. See [MapViewport.slippyZoom]. */
public val MapViewport.zoom: Int get() = MapViewport.slippyZoom(east - west)
