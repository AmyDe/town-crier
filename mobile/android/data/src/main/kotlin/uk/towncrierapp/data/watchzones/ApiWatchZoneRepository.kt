package uk.towncrierapp.data.watchzones

import kotlinx.coroutines.CancellationException
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import uk.towncrierapp.data.api.ApiClient
import uk.towncrierapp.data.api.ApiEndpoint
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.domain.watchzones.WatchZone
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.domain.watchzones.WatchZoneRepository

/**
 * `WatchZoneRepository` over the Town Crier API. Port of iOS
 * `APIWatchZoneRepository`. `create`/`update`/`delete` discard their response
 * bodies via [ApiClient.requestBytes] — the domain port's return type is
 * `Unit` (parity with iOS `save`/`update`/`delete`), and the caller reloads
 * the list afterwards.
 */
public class ApiWatchZoneRepository(
    private val apiClient: ApiClient,
    private val json: Json = Json { ignoreUnknownKeys = true },
) : WatchZoneRepository {
    override suspend fun zones(): List<WatchZone> =
        apiClient
            .request(ApiEndpoint.get("/v1/me/watch-zones"), ListWatchZonesResponseDto.serializer())
            .zones
            // A malformed individual zone (e.g. a legacy record with an
            // out-of-range coordinate) is skipped rather than failing the
            // whole list — parity with iOS's `compactMap` behaviour.
            .mapNotNull { dto -> runCatchingDomainInvariant { dto.toDomain() } }

    override suspend fun create(zone: WatchZone) {
        val body = json.encodeToString(CreateWatchZoneRequestDto.serializer(), zone.toCreateRequestDto())
        try {
            apiClient.requestBytes(ApiEndpoint.post("/v1/me/watch-zones", body = body))
        } catch (e: CancellationException) {
            throw e
        } catch (e: DomainError) {
            throw e.normalisedForCreateQuota()
        }
    }

    override suspend fun update(zone: WatchZone) {
        val body = json.encodeToString(UpdateWatchZoneRequestDto.serializer(), zone.toUpdateRequestDto())
        apiClient.requestBytes(ApiEndpoint.patch("/v1/me/watch-zones/${zone.id.value}", body = body))
    }

    override suspend fun delete(id: WatchZoneId) {
        try {
            apiClient.requestBytes(ApiEndpoint.delete("/v1/me/watch-zones/${id.value}"))
        } catch (e: CancellationException) {
            throw e
        } catch (e: DomainError.NotFound) {
            // Idempotent delete — already gone counts as success.
        }
    }
}

/**
 * The create endpoint's only 403 is "quota exceeded" (tc-gpjk / tc-z95t bead
 * brief), so ANY 403 on create — whether the server already shaped it as an
 * entitlement body or a plain error string — normalises to
 * `insufficientEntitlement("personal")`. This is a deliberate Android
 * divergence from iOS, which preserves an already-entitlement-shaped body's
 * `required` value; here the bead brief calls for the simpler, always-
 * personal mapping since a plain create 403 is quota-only in practice.
 */
private fun DomainError.normalisedForCreateQuota(): DomainError =
    when (this) {
        is DomainError.InsufficientEntitlement -> DomainError.InsufficientEntitlement("personal")
        is DomainError.ServerError -> if (status == 403) DomainError.InsufficientEntitlement("personal") else this
        else -> this
    }

/** A domain-invariant violation (e.g. an out-of-range legacy coordinate) skips just that item rather than failing the whole call. */
private inline fun <T> runCatchingDomainInvariant(block: () -> T): T? =
    try {
        block()
    } catch (e: IllegalArgumentException) {
        null
    }

// MARK: - DTOs

@Serializable
internal data class ListWatchZonesResponseDto(
    val zones: List<WatchZoneDto>,
)

@Serializable
internal data class WatchZoneDto(
    val id: String,
    val name: String,
    val latitude: Double,
    val longitude: Double,
    val radiusMetres: Double,
    val authorityId: Int = 0,
    // Absent on legacy zones created before these flags existed — default to
    // true so those zones keep receiving every alert (epic #770 contract).
    val pushEnabled: Boolean = true,
    val emailInstantEnabled: Boolean = true,
)

@Serializable
internal data class CreateWatchZoneRequestDto(
    val name: String,
    val latitude: Double,
    val longitude: Double,
    val radiusMetres: Double,
    val authorityId: Int? = null,
    val pushEnabled: Boolean,
    val emailInstantEnabled: Boolean,
)

@Serializable
internal data class UpdateWatchZoneRequestDto(
    val name: String,
    val latitude: Double,
    val longitude: Double,
    val radiusMetres: Double,
    val authorityId: Int,
    val pushEnabled: Boolean,
    val emailInstantEnabled: Boolean,
)

internal fun WatchZoneDto.toDomain(): WatchZone =
    WatchZone(
        id = WatchZoneId(id),
        name = name,
        centre = Coordinate(latitude, longitude),
        radiusMetres = radiusMetres,
        authorityId = authorityId,
        pushEnabled = pushEnabled,
        emailInstantEnabled = emailInstantEnabled,
    )

internal fun WatchZone.toCreateRequestDto(): CreateWatchZoneRequestDto =
    CreateWatchZoneRequestDto(
        name = name,
        latitude = centre.latitude,
        longitude = centre.longitude,
        radiusMetres = radiusMetres,
        // 0 means "not yet resolved" — omit so the server reverse-geocodes it.
        authorityId = authorityId.takeIf { it > 0 },
        pushEnabled = pushEnabled,
        emailInstantEnabled = emailInstantEnabled,
    )

internal fun WatchZone.toUpdateRequestDto(): UpdateWatchZoneRequestDto =
    UpdateWatchZoneRequestDto(
        name = name,
        latitude = centre.latitude,
        longitude = centre.longitude,
        radiusMetres = radiusMetres,
        authorityId = authorityId,
        pushEnabled = pushEnabled,
        emailInstantEnabled = emailInstantEnabled,
    )
