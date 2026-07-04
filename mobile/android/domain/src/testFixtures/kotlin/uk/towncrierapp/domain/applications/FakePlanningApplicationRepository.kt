package uk.towncrierapp.domain.applications

import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.watchzones.WatchZoneId

/** One recorded call to [FakePlanningApplicationRepository.applications]. */
public data class ApplicationsCall(
    public val zoneId: WatchZoneId,
    public val sort: ApplicationSortOrder,
    public val filter: ApplicationFilter,
    public val cursor: String?,
)

/** Hand-written fake for [PlanningApplicationRepository] — state-based, per testing.md conventions. */
public class FakePlanningApplicationRepository : PlanningApplicationRepository {
    public var applicationsResult: ApplicationPage = ApplicationPage(emptyList())
    public var applicationsFailWith: DomainError? = null
    public val applicationsCalls: MutableList<ApplicationsCall> = mutableListOf()

    public var detailResult: PlanningApplication = aPlanningApplication()
    public var detailFailWith: DomainError? = null
    public val detailCalls: MutableList<Pair<String, String>> = mutableListOf()

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
        detailFailWith?.let { throw it }
        return detailResult
    }
}
