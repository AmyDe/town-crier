package uk.towncrierapp.data.watchzones

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import uk.towncrierapp.data.api.ApiClient
import uk.towncrierapp.data.api.ApiEndpoint
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.domain.watchzones.ZoneNotificationPreferences
import uk.towncrierapp.domain.watchzones.ZonePreferencesRepository

/** `ZonePreferencesRepository` over the Town Crier API. Port of iOS `APIZonePreferencesRepository`. */
public class ApiZonePreferencesRepository(
    private val apiClient: ApiClient,
    private val json: Json = Json { ignoreUnknownKeys = true },
) : ZonePreferencesRepository {
    override suspend fun fetchPreferences(zoneId: WatchZoneId): ZoneNotificationPreferences =
        apiClient
            .request(ApiEndpoint.get("/v1/me/watch-zones/${zoneId.value}/preferences"), ZonePreferencesDto.serializer())
            .toDomain()

    override suspend fun updatePreferences(preferences: ZoneNotificationPreferences) {
        val body = json.encodeToString(ZonePreferencesDto.serializer(), preferences.toDto())
        apiClient.requestBytes(
            ApiEndpoint.put("/v1/me/watch-zones/${preferences.zoneId.value}/preferences", body = body),
        )
    }
}

@Serializable
internal data class ZonePreferencesDto(
    val zoneId: String,
    val newApplicationPush: Boolean = true,
    val newApplicationEmail: Boolean = true,
    val decisionPush: Boolean = true,
    val decisionEmail: Boolean = true,
)

internal fun ZonePreferencesDto.toDomain(): ZoneNotificationPreferences =
    ZoneNotificationPreferences(
        zoneId = WatchZoneId(zoneId),
        newApplicationPush = newApplicationPush,
        newApplicationEmail = newApplicationEmail,
        decisionPush = decisionPush,
        decisionEmail = decisionEmail,
    )

internal fun ZoneNotificationPreferences.toDto(): ZonePreferencesDto =
    ZonePreferencesDto(
        zoneId = zoneId.value,
        newApplicationPush = newApplicationPush,
        newApplicationEmail = newApplicationEmail,
        decisionPush = decisionPush,
        decisionEmail = decisionEmail,
    )
