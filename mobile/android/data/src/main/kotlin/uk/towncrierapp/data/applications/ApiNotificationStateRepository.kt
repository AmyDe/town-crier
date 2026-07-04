package uk.towncrierapp.data.applications

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import uk.towncrierapp.data.api.ApiClient
import uk.towncrierapp.data.api.ApiEndpoint
import uk.towncrierapp.data.api.DotNetTimeParser
import uk.towncrierapp.domain.applications.NotificationState
import uk.towncrierapp.domain.applications.NotificationStateRepository
import uk.towncrierapp.domain.applications.PlanningApplicationId

/**
 * `NotificationStateRepository` over the Town Crier API (ADR 0035). ⚠️
 * [MarkReadItemDto.applicationUid] carries [PlanningApplicationId.name] (the
 * case reference), NOT the uid — a wire misnomer ported as-is, never "fixed"
 * (GH#775). [markRead] batches at [MARK_READ_BATCH_SIZE] ids per request. The
 * legacy `/advance` endpoint is deliberately not implemented.
 */
public class ApiNotificationStateRepository(
    private val apiClient: ApiClient,
    private val json: Json = Json { ignoreUnknownKeys = true },
) : NotificationStateRepository {
    override suspend fun state(): NotificationState =
        apiClient.request(ApiEndpoint.get("/v1/me/notification-state"), NotificationStateDto.serializer()).toDomain()

    override suspend fun markRead(ids: List<PlanningApplicationId>) {
        ids.chunked(MARK_READ_BATCH_SIZE).forEach { batch ->
            val body =
                json.encodeToString(
                    MarkReadRequestDto.serializer(),
                    MarkReadRequestDto(batch.map { MarkReadItemDto(applicationUid = it.name, authorityId = it.authority.toInt()) }),
                )
            apiClient.requestBytes(ApiEndpoint.post("/v1/me/applications/mark-read", body = body))
        }
    }

    override suspend fun markAllRead() {
        apiClient.requestBytes(ApiEndpoint.post("/v1/me/notification-state/mark-all-read"))
    }

    public companion object {
        public const val MARK_READ_BATCH_SIZE: Int = 500
    }
}

// MARK: - DTOs

@Serializable
internal data class NotificationStateDto(
    val lastReadAt: String? = null,
    val version: Int,
    val totalUnreadCount: Int,
)

internal fun NotificationStateDto.toDomain(): NotificationState =
    NotificationState(
        lastReadAt = lastReadAt?.let(DotNetTimeParser::parse),
        version = version,
        totalUnreadCount = totalUnreadCount,
    )

// The `applicationUid` key literally holds the case NAME, not a uid — an
// intentional, ported-as-is server misnomer (GH#775). Do not rename this to
// match its actual content; the wire contract is the wire contract.
@Serializable
internal data class MarkReadItemDto(
    val applicationUid: String,
    val authorityId: Int,
)

@Serializable
internal data class MarkReadRequestDto(
    val applications: List<MarkReadItemDto>,
)
