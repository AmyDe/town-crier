package uk.towncrierapp.domain.map

import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.PlanningApplicationId
import uk.towncrierapp.domain.watchzones.Coordinate

/**
 * A server-computed aggregate of planning applications for one grid cell of
 * the watch-zone map (GH#698, GH#776). The clusters endpoint grids the
 * in-viewport applications by zoom level in PostGIS and returns one of these
 * per non-empty cell, so the device renders a handful of aggregates instead
 * of holding the whole zone's applications.
 *
 * When [count] is 1 the cell holds a single application, so [member] carries
 * its identity and [statusCounts] its lone status — enough to draw a
 * status-coloured pin and, on tap, point-read the full record. When [count]
 * exceeds 1 the cell is an amber count bubble; a tap zooms in to spread its
 * members into finer cells — UNLESS the members are coincident (or closer
 * than the finest zoom-20 grid cell), in which case the server marks the
 * cell "unsplittable" and lists those members in [members] (GH#722): such a
 * stacked cell can never resolve into individual pins by zooming, so a tap
 * opens a disambiguation list instead. [members] is empty for every
 * splittable cell. Port of iOS `MapCluster`.
 */
public data class MapCluster(
    public val coordinate: Coordinate,
    public val count: Int,
    public val statusCounts: Map<ApplicationStatus, Int>,
    public val member: PlanningApplicationId?,
    public val members: List<PlanningApplicationId> = emptyList(),
) {
    init {
        require(count > 0) { "count must be > 0, was $count" }
    }

    /** Whether this cell holds exactly one application (a real pin, not a bubble). */
    public val isSingleMember: Boolean get() = count == 1

    /**
     * Whether this is an unsplittable multi-member cell: more than one
     * application stacked at (effectively) one location, with their
     * identities carried in [members]. A tap on such a cell opens the
     * disambiguation list rather than zooming in — zoom could never separate
     * coincident points (GH#722).
     */
    public val isStacked: Boolean get() = count > 1 && members.isNotEmpty()

    /** The lone application's status for a single-member cell — drives the status-coloured pin. `null` for multi-member cells. */
    public val memberStatus: ApplicationStatus? get() = if (isSingleMember) statusCounts.keys.firstOrNull() else null

    /** A stable identity for marker diffing: the member id for single cells, else the centroid plus count so distinct bubbles never collide. */
    public val id: String
        get() = member?.value ?: "cluster:${coordinate.latitude}:${coordinate.longitude}:$count"
}
