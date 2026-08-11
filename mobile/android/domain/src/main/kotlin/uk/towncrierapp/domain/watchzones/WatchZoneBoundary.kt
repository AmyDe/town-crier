package uk.towncrierapp.domain.watchzones

/**
 * A closed polygon ring for a custom-shape watch zone, mirroring the
 * server's `watchzones.Boundary` (`api-go/internal/watchzones/zone.go`).
 *
 * `WatchZone.boundary == null` means the zone is a circle; a non-null,
 * validated [WatchZoneBoundary] means the zone is a custom shape. [of] is
 * the only way to obtain one — the private constructor guarantees every
 * instance in hand is already a valid, closed ring, so nothing downstream
 * re-validates it (the smart-constructor idiom, android-coding-standards
 * skill: architecture-and-modules.md). A hand-drawn shape failing validation
 * is expected user input, not a programmer error — unlike [Coordinate]'s
 * `require`-based invariants — so [of] returns a [WatchZoneBoundaryResult]
 * the caller must handle exhaustively rather than throwing (kotlin-idiom.md).
 * Port of iOS `WatchZoneBoundary`.
 */
public class WatchZoneBoundary private constructor(
    public val vertices: List<Coordinate>,
) {
    /**
     * The polygon centroid (area-weighted, shoelace formula) — not the
     * arithmetic mean of the vertices, which differs from the true centroid
     * for any non-uniform shape (e.g. an L-shape). Longitude and latitude
     * are treated as planar x/y, an acceptable approximation at the
     * sub-tens-of-km scale a hand-drawn watch zone can reach. Mirrors the
     * server's `Boundary.Centroid()`.
     */
    public val centroid: Coordinate by lazy { computeCentroid() }

    /**
     * The great-circle distance in metres from [centroid] to the furthest
     * vertex — the radius of the smallest centroid-centred circle that
     * fully encloses the shape. Mirrors the server's
     * `Boundary.EnclosingRadiusMetres()`, and what a custom-shape
     * `WatchZone` derives into its `radiusMetres` field so every read path
     * built for circles keeps working unchanged.
     */
    public val enclosingRadiusMetres: Double by lazy { computeEnclosingRadiusMetres() }

    private fun computeCentroid(): Coordinate {
        val edgeCount = vertices.size - 1
        var area = 0.0
        var centreX = 0.0
        var centreY = 0.0
        for (index in 0 until edgeCount) {
            val vertex = vertices[index]
            val next = vertices[index + 1]
            val cross = vertex.longitude * next.latitude - next.longitude * vertex.latitude
            area += cross
            centreX += (vertex.longitude + next.longitude) * cross
            centreY += (vertex.latitude + next.latitude) * cross
        }
        area /= 2
        centreX /= 6 * area
        centreY /= 6 * area
        return Coordinate(latitude = centreY, longitude = centreX)
    }

    private fun computeEnclosingRadiusMetres(): Double {
        val centre = centroid
        var maxDistance = 0.0
        for (vertex in vertices.dropLast(1)) {
            val distance = haversineMetres(centre.latitude, centre.longitude, vertex.latitude, vertex.longitude)
            maxDistance = maxOf(maxDistance, distance)
        }
        return maxDistance
    }

    override fun equals(other: Any?): Boolean = other is WatchZoneBoundary && vertices == other.vertices

    override fun hashCode(): Int = vertices.hashCode()

    public companion object {
        // 3 is the geometric minimum for a polygon, 50 is generous for a
        // hand-drawn shape while bounding payload size and matching-query
        // cost. Both exclude the closing repeat.
        private const val MIN_VERTICES = 3
        private const val MAX_VERTICES = 50

        // A generous bounding box around Great Britain and Northern Ireland,
        // used only to reject vertices that are obviously not the UK — not a
        // precise coastline check. Mirrors the server's uk* bounds constants.
        private const val UK_MIN_LATITUDE = 49.5
        private const val UK_MAX_LATITUDE = 61.0
        private const val UK_MIN_LONGITUDE = -8.5
        private const val UK_MAX_LONGITUDE = 2.0

        /**
         * Validates and normalises raw vertices into a closed polygon ring:
         * 3-50 distinct vertices (after auto-closing, not counting the
         * closing repeat), an open ring is auto-closed by appending a copy
         * of the first vertex (a ring that already closes is left as-is),
         * all vertices within UK bounds, and the ring must be simple (no two
         * non-adjacent edges cross). Mirrors the server's `NewBoundary`.
         */
        public fun of(vertices: List<Coordinate>): WatchZoneBoundaryResult {
            if (vertices.isEmpty()) return WatchZoneBoundaryResult.InvalidVertexCount

            val ring = if (vertices.first() != vertices.last()) vertices + vertices.first() else vertices
            val distinct = ring.dropLast(1)
            if (distinct.size < MIN_VERTICES || distinct.size > MAX_VERTICES) {
                return WatchZoneBoundaryResult.InvalidVertexCount
            }
            if (distinct.toSet().size != distinct.size) {
                return WatchZoneBoundaryResult.DuplicateVertex
            }
            val allWithinUkBounds =
                distinct.all { vertex ->
                    vertex.latitude in UK_MIN_LATITUDE..UK_MAX_LATITUDE &&
                        vertex.longitude in UK_MIN_LONGITUDE..UK_MAX_LONGITUDE
                }
            if (!allWithinUkBounds) return WatchZoneBoundaryResult.OutOfBounds

            // A collinear ring has no interior, so no point can ever fall
            // inside it; treat it as a degenerate (self-touching) shape.
            if (signedArea(ring) == 0.0) return WatchZoneBoundaryResult.SelfIntersecting
            if (selfIntersects(ring)) return WatchZoneBoundaryResult.SelfIntersecting

            return WatchZoneBoundaryResult.Valid(WatchZoneBoundary(ring))
        }

        /**
         * Twice the signed shoelace area of the closed ring. Zero only for a
         * degenerate (zero-area, e.g. collinear) ring.
         */
        private fun signedArea(ring: List<Coordinate>): Double {
            val edgeCount = ring.size - 1
            if (edgeCount < 1) return 0.0
            var sum = 0.0
            for (index in 0 until edgeCount) {
                sum +=
                    ring[index].longitude * ring[index + 1].latitude -
                    ring[index + 1].longitude * ring[index].latitude
            }
            return sum
        }

        /**
         * Whether any two non-adjacent edges of the closed ring cross. O(n²)
         * over the edges; n is capped at 50 vertices by [of], so this is
         * cheap and a sweep-line algorithm is not warranted.
         */
        private fun selfIntersects(ring: List<Coordinate>): Boolean {
            val edgeCount = ring.size - 1
            if (edgeCount < 3) return false
            for (first in 0 until edgeCount) {
                val firstStart = ring[first]
                val firstEnd = ring[first + 1]
                if (first + 2 > edgeCount) continue
                for (second in (first + 2) until edgeCount) {
                    // The last edge and the first edge share vertex
                    // ring[0]==ring[edgeCount]: adjacent via the ring's
                    // wraparound, not a crossing.
                    if (first == 0 && second == edgeCount - 1) continue
                    val secondStart = ring[second]
                    val secondEnd = ring[second + 1]
                    if (segmentsIntersect(firstStart, firstEnd, secondStart, secondEnd)) return true
                }
            }
            return false
        }

        /**
         * Whether segments pt1-pt2 and pt3-pt4 intersect, including one
         * touching the other at an endpoint or lying collinearly on it.
         * Standard orientation-based test (see e.g. CLRS §33.1), split
         * across [straddles] and [touchesCollinearly] to keep this
         * function's own branching (and detekt's cyclomatic-complexity
         * count) low.
         */
        private fun segmentsIntersect(
            pt1: Coordinate,
            pt2: Coordinate,
            pt3: Coordinate,
            pt4: Coordinate,
        ): Boolean {
            val ori1 = orientation(pt3, pt4, pt1)
            val ori2 = orientation(pt3, pt4, pt2)
            val ori3 = orientation(pt1, pt2, pt3)
            val ori4 = orientation(pt1, pt2, pt4)

            if (straddles(ori1, ori2) && straddles(ori3, ori4)) return true
            return touchesCollinearly(pt1, pt2, pt3, pt4)
        }

        /** Whether points with orientations [oriA] and [oriB] (relative to the same edge) fall on opposite sides of it. */
        private fun straddles(
            oriA: Double,
            oriB: Double,
        ): Boolean = (oriA > 0 && oriB < 0) || (oriA < 0 && oriB > 0)

        /**
         * Whether either endpoint of one segment is collinear with, and
         * lies on, the other segment — the "touching" case [straddles]
         * alone doesn't catch. Recomputes each orientation locally (cheap;
         * the ring is capped at 50 vertices) to keep this function's
         * parameter list small rather than threading the four orientations
         * through from [segmentsIntersect].
         */
        private fun touchesCollinearly(
            pt1: Coordinate,
            pt2: Coordinate,
            pt3: Coordinate,
            pt4: Coordinate,
        ): Boolean {
            if (orientation(pt3, pt4, pt1) == 0.0 && onSegment(pt3, pt4, pt1)) return true
            if (orientation(pt3, pt4, pt2) == 0.0 && onSegment(pt3, pt4, pt2)) return true
            if (orientation(pt1, pt2, pt3) == 0.0 && onSegment(pt1, pt2, pt3)) return true
            if (orientation(pt1, pt2, pt4) == 0.0 && onSegment(pt1, pt2, pt4)) return true
            return false
        }

        /**
         * Twice the signed area of triangle origin,next,point: positive for
         * counter-clockwise, negative for clockwise, zero when the three
         * points are collinear.
         */
        private fun orientation(
            origin: Coordinate,
            next: Coordinate,
            point: Coordinate,
        ): Double =
            (next.longitude - origin.longitude) * (point.latitude - origin.latitude) -
                (next.latitude - origin.latitude) * (point.longitude - origin.longitude)

        /**
         * Whether [point], already known to be collinear with segment
         * [segmentStart]-[segmentEnd], lies within its bounding box — i.e.
         * actually on the segment rather than on the line through it
         * extended beyond either endpoint.
         */
        private fun onSegment(
            segmentStart: Coordinate,
            segmentEnd: Coordinate,
            point: Coordinate,
        ): Boolean =
            point.longitude >= minOf(segmentStart.longitude, segmentEnd.longitude) &&
                point.longitude <= maxOf(segmentStart.longitude, segmentEnd.longitude) &&
                point.latitude >= minOf(segmentStart.latitude, segmentEnd.latitude) &&
                point.latitude <= maxOf(segmentStart.latitude, segmentEnd.latitude)
    }
}

/** The outcome of [WatchZoneBoundary.of] — an expected, exhaustively-handled user-input validation result. */
public sealed interface WatchZoneBoundaryResult {
    /** [vertices] formed a valid, simple polygon ring within UK bounds. */
    public data class Valid(
        public val boundary: WatchZoneBoundary,
    ) : WatchZoneBoundaryResult

    /** Fewer than 3 or more than 50 distinct vertices (excluding the closing repeat). */
    public data object InvalidVertexCount : WatchZoneBoundaryResult

    /** Two non-adjacent (non-closing) vertices were identical. */
    public data object DuplicateVertex : WatchZoneBoundaryResult

    /** At least one vertex fell outside the generous UK bounding box. */
    public data object OutOfBounds : WatchZoneBoundaryResult

    /** The ring is degenerate (zero-area/collinear) or two non-adjacent edges cross. */
    public data object SelfIntersecting : WatchZoneBoundaryResult
}
