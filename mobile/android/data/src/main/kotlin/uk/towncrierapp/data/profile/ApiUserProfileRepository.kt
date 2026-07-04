package uk.towncrierapp.data.profile

import kotlinx.coroutines.CancellationException
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import uk.towncrierapp.data.api.ApiClient
import uk.towncrierapp.data.api.ApiEndpoint
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.profile.DigestDay
import uk.towncrierapp.domain.profile.ServerProfile
import uk.towncrierapp.domain.profile.UserPreferences
import uk.towncrierapp.domain.profile.UserProfileRepository
import uk.towncrierapp.domain.subscriptions.SubscriptionTier

/**
 * `UserProfileRepository` over the Town Crier API (epic #770 / #778):
 * `POST`/`GET`/`PATCH`/`DELETE /v1/me` + `GET /v1/me/data`. Port of iOS
 * `APIUserProfileRepository`.
 */
public class ApiUserProfileRepository(
    private val apiClient: ApiClient,
    private val json: Json = Json { ignoreUnknownKeys = true },
) : UserProfileRepository {
    override suspend fun ensureProfile(): ServerProfile =
        apiClient.request(ApiEndpoint.post("/v1/me"), ServerProfileDto.serializer()).toDomain()

    @Suppress("SwallowedException")
    // A 404 means "no profile yet" — a routine, expected outcome for this
    // port (mirrors iOS `APIUserProfileRepository.fetch()`), not a failure
    // the caller needs the original exception for.
    override suspend fun fetchProfile(): ServerProfile? =
        try {
            apiClient.request(ApiEndpoint.get("/v1/me"), ServerProfileDto.serializer()).toDomain()
        } catch (e: CancellationException) {
            throw e
        } catch (e: DomainError.NotFound) {
            null
        }

    override suspend fun updatePreferences(preferences: UserPreferences): ServerProfile {
        val body = json.encodeToString(UpdateProfileRequestDto.serializer(), preferences.toRequestDto())
        return apiClient.request(ApiEndpoint.patch("/v1/me", body = body), ServerProfileDto.serializer()).toDomain()
    }

    override suspend fun deleteAccount() {
        apiClient.requestBytes(ApiEndpoint.delete("/v1/me"))
    }

    override suspend fun exportData(): ByteArray = apiClient.requestBytes(ApiEndpoint.get("/v1/me/data"))
}

// MARK: - DTOs

/**
 * Shared by `POST`/`GET`/`PATCH /v1/me`: `POST`'s response carries only
 * `userId`/`pushEnabled`/`tier` — the remaining four fields fall back to
 * their declared defaults (the server's own `DefaultPreferences`), matching
 * iOS's `decodeIfPresent`-with-fallback DTO.
 */
@Serializable
internal data class ServerProfileDto(
    val userId: String,
    val pushEnabled: Boolean,
    val tier: String,
    val digestDay: String = "Monday",
    val emailDigestEnabled: Boolean = true,
    val savedDecisionPush: Boolean = true,
    val savedDecisionEmail: Boolean = true,
)

internal fun ServerProfileDto.toDomain(): ServerProfile =
    ServerProfile(
        userId = userId,
        pushEnabled = pushEnabled,
        // Falls back to Free on an unrecognised wire value rather than crashing —
        // the server is authoritative and may add tiers this build doesn't know yet.
        tier = SubscriptionTier.fromWireValue(tier) ?: SubscriptionTier.FREE,
        digestDay = DigestDay.fromWireValue(digestDay) ?: DigestDay.MONDAY,
        emailDigestEnabled = emailDigestEnabled,
        savedDecisionPush = savedDecisionPush,
        savedDecisionEmail = savedDecisionEmail,
    )

/** The `PATCH /v1/me` body — always all five fields (epic #770 pre-resolved decision: never a partial update). */
@Serializable
internal data class UpdateProfileRequestDto(
    val pushEnabled: Boolean,
    val digestDay: String,
    val emailDigestEnabled: Boolean,
    val savedDecisionPush: Boolean,
    val savedDecisionEmail: Boolean,
)

internal fun UserPreferences.toRequestDto(): UpdateProfileRequestDto =
    UpdateProfileRequestDto(
        pushEnabled = pushEnabled,
        digestDay = digestDay.wireValue,
        emailDigestEnabled = emailDigestEnabled,
        savedDecisionPush = savedDecisionPush,
        savedDecisionEmail = savedDecisionEmail,
    )
