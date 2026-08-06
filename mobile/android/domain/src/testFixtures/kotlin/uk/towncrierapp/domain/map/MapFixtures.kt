package uk.towncrierapp.domain.map

import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.PlanningApplicationId
import uk.towncrierapp.domain.applications.aPlanningApplicationId
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.domain.watchzones.aCoordinate

/** Fixture factory for [MapViewport] — a small rectangle around Cambridge city centre. */
public fun aMapViewport(
    west: Double = 0.10,
    south: Double = 52.19,
    east: Double = 0.14,
    north: Double = 52.22,
): MapViewport = MapViewport(west, south, east, north)

/** Fixture factory for a single-member [MapCluster] — a real pin, not a bubble. */
public fun aSingleMemberMapCluster(
    coordinate: Coordinate = aCoordinate(),
    status: ApplicationStatus = ApplicationStatus.Undecided,
    member: PlanningApplicationId = aPlanningApplicationId(),
): MapCluster =
    MapCluster(
        coordinate = coordinate,
        count = 1,
        statusCounts = mapOf(status to 1),
        member = member,
    )

/** Fixture factory for a splittable (bubble) [MapCluster] — no carried member identities. */
public fun aSplittableMapCluster(
    coordinate: Coordinate = aCoordinate(),
    count: Int = 5,
    statusCounts: Map<ApplicationStatus, Int> = mapOf(ApplicationStatus.Undecided to count),
): MapCluster =
    MapCluster(
        coordinate = coordinate,
        count = count,
        statusCounts = statusCounts,
        member = null,
    )

/** Fixture factory for a stacked (unsplittable) [MapCluster] — carries every member's identity. */
public fun aStackedMapCluster(
    coordinate: Coordinate = aCoordinate(),
    members: List<PlanningApplicationId> =
        listOf(aPlanningApplicationId(name = "24/0001"), aPlanningApplicationId(name = "24/0002")),
    statusCounts: Map<ApplicationStatus, Int> = mapOf(ApplicationStatus.Undecided to members.size),
): MapCluster =
    MapCluster(
        coordinate = coordinate,
        count = members.size,
        statusCounts = statusCounts,
        member = null,
        members = members,
    )
