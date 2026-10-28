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
 * `NotificationStateRepository` over the Town Crier API (ADR 0035). [markRead]
 * batches at [MARK_READ_BATCH_SIZE] ids per request.
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
                    MarkReadRequestDto(
                        batch.map { MarkReadItemDto(applicationUid = it.name, authorityId = it.authority.toInt()) },
                    ),
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

// The `applicationUid` key holds the case name, not a uid: a server misnomer. Do not rename it.
@Serializable
internal data class MarkReadItemDto(
    val applicationUid: String,
    val authorityId: Int,
)

@Serializable
internal data class MarkReadRequestDto(
    val applications: List<MarkReadItemDto>,
)
