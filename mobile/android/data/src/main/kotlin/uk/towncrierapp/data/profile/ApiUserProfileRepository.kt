package uk.towncrierapp.data.profile

import kotlinx.serialization.Serializable
import uk.towncrierapp.data.api.ApiClient
import uk.towncrierapp.data.api.ApiEndpoint
import uk.towncrierapp.domain.profile.ServerProfile
import uk.towncrierapp.domain.profile.UserProfileRepository
import uk.towncrierapp.domain.subscriptions.SubscriptionTier

/** `UserProfileRepository` over the Town Crier API — `POST /v1/me` (ensure-profile) only; fetch/update/delete/export land with #778. */
public class ApiUserProfileRepository(
    private val apiClient: ApiClient,
) : UserProfileRepository {
    override suspend fun ensureProfile(): ServerProfile =
        apiClient.request(ApiEndpoint.post("/v1/me"), ServerProfileDto.serializer()).toDomain()
}

@Serializable
internal data class ServerProfileDto(
    val userId: String,
    val pushEnabled: Boolean,
    val tier: String,
)

internal fun ServerProfileDto.toDomain(): ServerProfile =
    ServerProfile(
        userId = userId,
        pushEnabled = pushEnabled,
        // Falls back to Free on an unrecognised wire value rather than crashing —
        // the server is authoritative and may add tiers this build doesn't know yet.
        tier = SubscriptionTier.fromWireValue(tier) ?: SubscriptionTier.FREE,
    )
