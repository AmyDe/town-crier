package uk.towncrierapp.domain.applications

import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.map.MapCluster
import uk.towncrierapp.domain.map.MapViewport
import uk.towncrierapp.domain.watchzones.WatchZoneId

/** One recorded call to [FakePlanningApplicationRepository.applications]. */
public data class ApplicationsCall(
    public val zoneId: WatchZoneId,
    public val sort: ApplicationSortOrder,
    public val filter: ApplicationFilter,
    public val cursor: String?,
)

/** One recorded call to [FakePlanningApplicationRepository.fetchClusters]. */
public data class FetchClustersCall(
    public val zoneId: WatchZoneId,
    public val viewport: MapViewport,
    public val zoom: Int,
    public val status: ApplicationStatus?,
)

/** Hand-written fake for [PlanningApplicationRepository] — state-based, per testing.md conventions. */
public class FakePlanningApplicationRepository : PlanningApplicationRepository {
    public var applicationsResult: ApplicationPage = ApplicationPage(emptyList())
    public var applicationsFailWith: DomainError? = null
    public val applicationsCalls: MutableList<ApplicationsCall> = mutableListOf()

    public var detailResult: PlanningApplication = aPlanningApplication()
    public var detailFailWith: DomainError? = null
    public val detailCalls: MutableList<Pair<String, String>> = mutableListOf()

    /**
     * Per-[PlanningApplicationId.value] overrides for [detail] — used by
     * concurrent-fan-out tests (e.g. a stacked map cluster's point-reads,
     * GH#776) that need distinct results/failures per id rather than one
     * global [detailResult]/[detailFailWith]. A key present in
     * [detailFailFor] takes priority over one present in [detailResults];
     * either falls back to the global fields when the id isn't listed.
     */
    public val detailResults: MutableMap<String, PlanningApplication> = mutableMapOf()
    public val detailFailFor: MutableMap<String, DomainError> = mutableMapOf()

    public var detailBySlugResult: PlanningApplication = aPlanningApplication()
    public var detailBySlugFailWith: DomainError? = null
    public val detailBySlugCalls: MutableList<Pair<String, String>> = mutableListOf()

    /**
     * A cooperative gate hook run before [detail] returns — the "iOS
     * spy-gate idiom" (testing.md) for re-entrancy tests. A test sets this to
     * `{ someDeferred.await() }` to hold a call in flight until it explicitly
     * releases it, so it can assert a second concurrent call was suppressed.
     * A no-op by default.
     */
    public var beforeDetail: suspend () -> Unit = {}

    override suspend fun applications(
        zoneId: WatchZoneId,
        sort: ApplicationSortOrder,
        filter: ApplicationFilter,
        cursor: String?,
    ): ApplicationPage {
        applicationsCalls += ApplicationsCall(zoneId, sort, filter, cursor)
        applicationsFailWith?.let { throw it }
        return applicationsResult
    }

    override suspend fun detail(
        authority: String,
        name: String,
    ): PlanningApplication {
        detailCalls += authority to name
        beforeDetail()
        val key = PlanningApplicationId(authority, name).value
        (detailFailFor[key] ?: detailFailWith)?.let { throw it }
        return detailResults[key] ?: detailResult
    }

    override suspend fun detailBySlug(
        authoritySlug: String,
        ref: String,
    ): PlanningApplication {
        detailBySlugCalls += authoritySlug to ref
        detailBySlugFailWith?.let { throw it }
        return detailBySlugResult
    }

    public var fetchClustersResult: List<MapCluster> = emptyList()
    public var fetchClustersFailWith: DomainError? = null
    public val fetchClustersCalls: MutableList<FetchClustersCall> = mutableListOf()

    override suspend fun fetchClusters(
        zoneId: WatchZoneId,
        viewport: MapViewport,
        zoom: Int,
        status: ApplicationStatus?,
    ): List<MapCluster> {
        fetchClustersCalls += FetchClustersCall(zoneId, viewport, zoom, status)
        fetchClustersFailWith?.let { throw it }
        return fetchClustersResult
    }
}
