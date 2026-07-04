package uk.towncrierapp.data.applications

import kotlinx.serialization.Serializable
import kotlinx.serialization.builtins.ListSerializer
import uk.towncrierapp.data.api.ApiClient
import uk.towncrierapp.data.api.ApiEndpoint
import uk.towncrierapp.data.api.DotNetTimeParser
import uk.towncrierapp.domain.applications.ApplicationFilter
import uk.towncrierapp.domain.applications.ApplicationPage
import uk.towncrierapp.domain.applications.ApplicationSortOrder
import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.LatestUnreadEvent
import uk.towncrierapp.domain.applications.LocalAuthority
import uk.towncrierapp.domain.applications.PlanningApplication
import uk.towncrierapp.domain.applications.PlanningApplicationId
import uk.towncrierapp.domain.applications.PlanningApplicationRepository
import uk.towncrierapp.domain.applications.StatusEvent
import uk.towncrierapp.domain.applications.isDecided
import uk.towncrierapp.domain.applications.wireValue
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.domain.watchzones.WatchZoneId
import java.time.LocalDate

/**
 * `PlanningApplicationRepository` over the Town Crier API. Always sends
 * `sort` (epic #770: Android never exercises the legacy param-less path) and
 * `limit=`[PAGE_SIZE]; `status`/`unread` are mutually exclusive by
 * [ApplicationFilter]'s shape, not a runtime check. Continuation is the
 * opaque `X-Next-Cursor` response header via [ApiClient.requestPaged]. Port
 * of iOS `APIApplicationRepository` (GH#775).
 */
public class ApiPlanningApplicationRepository(
    private val apiClient: ApiClient,
) : PlanningApplicationRepository {
    override suspend fun applications(
        zoneId: WatchZoneId,
        sort: ApplicationSortOrder,
        filter: ApplicationFilter,
        cursor: String?,
    ): ApplicationPage {
        val query =
            buildList {
                add("sort" to sort.wireValue)
                add("limit" to PAGE_SIZE.toString())
                when (filter) {
                    ApplicationFilter.All -> Unit
                    is ApplicationFilter.Status -> add("status" to filter.status.wireValue)
                    ApplicationFilter.Unread -> add("unread" to "true")
                }
                cursor?.let { add("cursor" to it) }
            }
        val result =
            apiClient.requestPaged(
                ApiEndpoint.get("/v1/me/watch-zones/${zoneId.value}/applications", query = query),
                ListSerializer(PlanningApplicationRowDto.serializer()),
            )
        return ApplicationPage(result.value.map { it.toDomain() }, result.nextCursor)
    }

    override suspend fun detail(
        authority: String,
        name: String,
    ): PlanningApplication =
        apiClient
            .request(ApiEndpoint.get("/v1/applications/$authority/$name"), ApplicationDetailDto.serializer())
            .toDomain()

    public companion object {
        public const val PAGE_SIZE: Int = 150
    }
}

// MARK: - DTOs

@Serializable
internal data class LatestUnreadEventDto(
    val type: String,
    val decision: String? = null,
    val createdAt: String,
)

@Serializable
internal data class PlanningApplicationRowDto(
    val name: String,
    val uid: String,
    val areaName: String,
    val areaId: Int,
    val address: String,
    val postcode: String? = null,
    val description: String,
    val appType: String? = null,
    val appState: String? = null,
    val appSize: String? = null,
    val startDate: String? = null,
    val decidedDate: String? = null,
    val longitude: Double? = null,
    val latitude: Double? = null,
    val url: String? = null,
    val link: String? = null,
    val lastDifferent: String,
    val latestUnreadEvent: LatestUnreadEventDto? = null,
)

@Serializable
internal data class ApplicationDetailDto(
    val name: String,
    val uid: String,
    val areaName: String,
    val areaId: Int,
    val address: String,
    val postcode: String? = null,
    val description: String,
    val appType: String? = null,
    val appState: String? = null,
    val appSize: String? = null,
    val startDate: String? = null,
    val decidedDate: String? = null,
    val longitude: Double? = null,
    val latitude: Double? = null,
    val url: String? = null,
    val link: String? = null,
    val lastDifferent: String,
    val latestUnreadEvent: LatestUnreadEventDto? = null,
    val authoritySlug: String? = null,
    val areaType: String? = null,
)

internal fun PlanningApplicationRowDto.toDomain(): PlanningApplication {
    val status = resolveStatus(appState)
    return PlanningApplication(
        id = PlanningApplicationId(authority = areaId.toString(), name = name),
        reference = name,
        authority = LocalAuthority(code = areaId.toString(), name = areaName),
        status = status,
        receivedDate = resolveReceivedDate(startDate, lastDifferent),
        description = description,
        address = address,
        location = resolveLocation(latitude, longitude),
        // "link" (the external council-portal URL) takes priority over "url"
        // (PlanIt's own page) — the "View on Council Portal" action wants the
        // former (GH#775).
        portalUrl = link ?: url,
        statusHistory = synthesizeStatusHistory(status, startDate, decidedDate),
        latestUnreadEvent = latestUnreadEvent?.toDomain(),
    )
}

internal fun ApplicationDetailDto.toDomain(): PlanningApplication {
    val status = resolveStatus(appState)
    return PlanningApplication(
        id = PlanningApplicationId(authority = areaId.toString(), name = name),
        reference = name,
        authority = LocalAuthority(code = areaId.toString(), name = areaName, areaType = areaType, slug = authoritySlug),
        status = status,
        receivedDate = resolveReceivedDate(startDate, lastDifferent),
        description = description,
        address = address,
        location = resolveLocation(latitude, longitude),
        portalUrl = link ?: url,
        statusHistory = synthesizeStatusHistory(status, startDate, decidedDate),
        latestUnreadEvent = latestUnreadEvent?.toDomain(),
    )
}

// A missing/unparseable createdAt drops just the unread indicator, not the
// whole row — a stale or malformed event server-side shouldn't hide an
// otherwise-good application (GH#775).
internal fun LatestUnreadEventDto.toDomain(): LatestUnreadEvent? =
    DotNetTimeParser.parse(createdAt)?.let { LatestUnreadEvent(type = type, decision = decision, createdAt = it) }

// An absent appState is wire-legitimate (optional field) — treated the same
// as any other unrecognised raw value: Unknown, never a crash.
private fun resolveStatus(appState: String?): ApplicationStatus = appState?.let(ApplicationStatus::fromWireValue) ?: ApplicationStatus.Unknown("")

// startDate is the primary source; lastDifferent (always present) is the
// fallback for the rare row missing it — receivedDate is non-nullable
// domain-side and must resolve to SOMETHING (GH#775).
private fun resolveReceivedDate(
    startDate: String?,
    lastDifferent: String,
): LocalDate =
    startDate?.let(DotNetTimeParser::parseDate)
        ?: DotNetTimeParser.parse(lastDifferent)?.toLocalDate()
        ?: LocalDate.EPOCH

private fun resolveLocation(
    latitude: Double?,
    longitude: Double?,
): Coordinate? = if (latitude != null && longitude != null) Coordinate(latitude, longitude) else null

// Max two points: startDate -> Undecided, decidedDate -> the actual decided
// status, but ONLY when that status really is decided — a decidedDate on a
// still-Unresolved/Referred/etc. row is folded away (GH#775).
private fun synthesizeStatusHistory(
    status: ApplicationStatus,
    startDate: String?,
    decidedDate: String?,
): List<StatusEvent> =
    buildList {
        startDate?.let(DotNetTimeParser::parseDate)?.let { add(StatusEvent(ApplicationStatus.Undecided, it)) }
        if (status.isDecided) {
            decidedDate?.let(DotNetTimeParser::parseDate)?.let { add(StatusEvent(status, it)) }
        }
    }
